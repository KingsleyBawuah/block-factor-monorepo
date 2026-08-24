package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/cache"
)

const (
	// ctaGTFSCacheKey/TTL: the raw CTA GTFS schedule zip is cached in
	// cache_entries (PostgreSQL) so repeated local development runs reuse the
	// downloaded archive instead of hitting the CTA server repeatedly.
	// In production, CTA updates static schedules quarterly (90-day TTL).
	ctaGTFSCacheKey = "cta:gtfs:zip"
	ctaGTFSCacheTTL = 90 * 24 * time.Hour

	// defaultCTAGTFSURL is the official public CTA static GTFS feed download URL.
	defaultCTAGTFSURL = "https://www.transitchicago.com/downloads/sch_data/google_transit.zip"

	// amPeakStartSecs and amPeakEndSecs define the weekday AM rush hour window
	// (07:00:00 to 09:00:00, 120 minutes) per PRD §5.2.
	amPeakStartSecs     = 7 * 3600
	amPeakEndSecs       = 9 * 3600
	amPeakWindowMinutes = 120.0

	// noAMPeakHeadwaySentinel is assigned to routes that have zero scheduled
	// weekday AM rush hour trips (e.g. Night Owl overnight routes or seasonal
	// midday tourist shuttles).
	noAMPeakHeadwaySentinel = 999.00
)

type gtfsRoute struct {
	id        string
	shortName string
	longName  string
	routeType string
}

type gtfsTrip struct {
	routeID     string
	serviceID   string
	directionID string
	shapeID     string
}

type shapePoint struct {
	lat float64
	lon float64
	seq int
}

type derivedCorridor struct {
	routeID     string
	routeName   string
	meanHeadway float64
	wkt         string
}

// parseTimeSeconds parses a GTFS time string (HH:MM:SS) into seconds from midnight.
// Note: GTFS allows hours >= 24 for late-night service past midnight.
func parseTimeSeconds(t string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(t), ":")
	if len(parts) != 3 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	s, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return h*3600 + m*60 + s, true
}

// fetchOrLoadGTFS retrieves the CTA GTFS zip archive, checking local overrides,
// the Postgres cache_entries table, and downloading from CTA as a fallback.
func fetchOrLoadGTFS(ctx context.Context, c *cache.Cache) ([]byte, error) {
	// 1. Check local file override (CTA_GTFS_PATH)
	if localPath := os.Getenv("CTA_GTFS_PATH"); localPath != "" {
		slog.Info("bus_corridors: loading GTFS from local file override", slog.String("path", localPath))
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("read local GTFS file %q: %w", localPath, err)
		}
		return data, nil
	}

	// 2. Check Postgres cache_entries table
	if c != nil {
		var cachedBytes []byte
		cached, err := c.Get(ctx, ctaGTFSCacheKey, &cachedBytes)
		if err != nil {
			slog.Warn("bus_corridors: cache check failed; will fetch live", slog.String("error", err.Error()))
		} else if cached && len(cachedBytes) > 0 {
			slog.Info("bus_corridors: using cached GTFS archive", slog.Int("bytes", len(cachedBytes)))
			return cachedBytes, nil
		}
	}

	// 3. Download from CTA live URL
	feedURL := os.Getenv("CTA_GTFS_URL")
	if feedURL == "" {
		feedURL = defaultCTAGTFSURL
	}

	slog.Info("bus_corridors: downloading CTA GTFS archive", slog.String("url", feedURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create GTFS download request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download GTFS feed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download GTFS feed: unexpected HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read GTFS response body: %w", err)
	}

	slog.Info("bus_corridors: downloaded GTFS archive", slog.Int("bytes", len(data)))

	// Save to Postgres cache for future runs
	if c != nil {
		if err := c.Set(ctx, ctaGTFSCacheKey, data, ctaGTFSCacheTTL); err != nil {
			slog.Warn("bus_corridors: failed to cache GTFS archive", slog.String("error", err.Error()))
		}
	}

	return data, nil
}

// deriveBusCorridorsFromZip parses a GTFS zip archive, computes weekday AM-peak
// headways for all bus routes, and constructs MULTILINESTRING WKT geometries from shapes.txt.
func deriveBusCorridorsFromZip(zipBytes []byte) ([]derivedCorridor, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("open zip reader: %w", err)
	}

	files := make(map[string]*zip.File)
	for _, f := range zr.File {
		files[f.Name] = f
	}

	// 1. Parse calendar.txt to identify active weekday service IDs
	weekdayServices, err := parseWeekdayServices(files)
	if err != nil {
		return nil, err
	}

	// 2. Parse routes.txt for bus routes (route_type = 3)
	busRoutes, err := parseBusRoutes(files)
	if err != nil {
		return nil, err
	}

	// 3. Parse trips.txt to map trip_id -> gtfsTrip and collect all route shapes
	trips, routeShapes, err := parseTrips(files, weekdayServices, busRoutes)
	if err != nil {
		return nil, err
	}

	// 4. Parse stop_times.txt to find origin departure times for trips
	tripFirstDeparture, err := parseFirstDepartures(files, trips)
	if err != nil {
		return nil, err
	}

	// 5. Aggregate AM-peak trips per route per direction
	routeDirTrips := make(map[string]map[string]int)

	for tripID, depTime := range tripFirstDeparture {
		secs, ok := parseTimeSeconds(depTime)
		if !ok {
			continue
		}
		if secs >= amPeakStartSecs && secs < amPeakEndSecs {
			meta := trips[tripID]
			if routeDirTrips[meta.routeID] == nil {
				routeDirTrips[meta.routeID] = make(map[string]int)
			}
			routeDirTrips[meta.routeID][meta.directionID]++
		}
	}

	// 6. Calculate mean scheduled headways for all bus routes
	type routeCalculation struct {
		routeID     string
		routeName   string
		meanHeadway float64
		shapeIDs    []string
	}
	var routeCalcs []routeCalculation
	neededShapeIDs := make(map[string]bool)

	for routeID, info := range busRoutes {
		name := info.shortName
		if info.longName != "" {
			if name != "" {
				name = fmt.Sprintf("%s - %s", info.shortName, info.longName)
			} else {
				name = info.longName
			}
		}

		dirCounts := routeDirTrips[routeID]
		meanHeadway := noAMPeakHeadwaySentinel
		if dirCounts != nil {
			dir0 := dirCounts["0"]
			dir1 := dirCounts["1"]
			total := dir0 + dir1
			if total > 0 {
				numDirs := 0
				if dir0 > 0 {
					numDirs++
				}
				if dir1 > 0 {
					numDirs++
				}
				avgTripsPerDir := float64(total) / float64(numDirs)
				meanHeadway = amPeakWindowMinutes / avgTripsPerDir
			}
		}

		var shapes []string
		for sID := range routeShapes[routeID] {
			shapes = append(shapes, sID)
			neededShapeIDs[sID] = true
		}

		routeCalcs = append(routeCalcs, routeCalculation{
			routeID:     routeID,
			routeName:   name,
			meanHeadway: meanHeadway,
			shapeIDs:    shapes,
		})
	}

	// 7. Parse shapes.txt for all required shapes
	shapePoints, err := parseShapePoints(files, neededShapeIDs)
	if err != nil {
		return nil, err
	}

	// 8. Assemble MULTILINESTRING WKT for each route
	var corridors []derivedCorridor
	for _, rc := range routeCalcs {
		var lineParts []string
		for _, sID := range rc.shapeIDs {
			pts := shapePoints[sID]
			if len(pts) < 2 {
				continue
			}
			var coordStrs []string
			for _, pt := range pts {
				coordStrs = append(coordStrs, fmt.Sprintf("%f %f", pt.lon, pt.lat))
			}
			lineParts = append(lineParts, fmt.Sprintf("(%s)", strings.Join(coordStrs, ", ")))
		}
		if len(lineParts) == 0 {
			continue
		}
		wkt := fmt.Sprintf("MULTILINESTRING(%s)", strings.Join(lineParts, ", "))
		corridors = append(corridors, derivedCorridor{
			routeID:     rc.routeID,
			routeName:   rc.routeName,
			meanHeadway: rc.meanHeadway,
			wkt:         wkt,
		})
	}

	sort.Slice(corridors, func(i, j int) bool {
		if corridors[i].meanHeadway == corridors[j].meanHeadway {
			return corridors[i].routeID < corridors[j].routeID
		}
		return corridors[i].meanHeadway < corridors[j].meanHeadway
	})

	return corridors, nil
}

func parseWeekdayServices(files map[string]*zip.File) (map[string]bool, error) {
	calFile, ok := files["calendar.txt"]
	if !ok {
		return nil, fmt.Errorf("calendar.txt not found in GTFS archive")
	}
	rc, err := calFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open calendar.txt: %w", err)
	}
	defer rc.Close() //nolint:errcheck

	calReader := csv.NewReader(rc)
	calHeader, err := calReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read calendar.txt header: %w", err)
	}

	serviceIdx, monIdx, tueIdx, wedIdx, thuIdx, friIdx := -1, -1, -1, -1, -1, -1
	for i, h := range calHeader {
		switch strings.TrimSpace(h) {
		case "service_id":
			serviceIdx = i
		case "monday":
			monIdx = i
		case "tuesday":
			tueIdx = i
		case "wednesday":
			wedIdx = i
		case "thursday":
			thuIdx = i
		case "friday":
			friIdx = i
		}
	}
	if serviceIdx < 0 || monIdx < 0 || tueIdx < 0 || wedIdx < 0 || thuIdx < 0 || friIdx < 0 {
		return nil, fmt.Errorf("calendar.txt missing required columns")
	}

	weekdayServices := make(map[string]bool)
	for {
		row, err := calReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if row[monIdx] == "1" && row[tueIdx] == "1" && row[wedIdx] == "1" && row[thuIdx] == "1" && row[friIdx] == "1" {
			weekdayServices[row[serviceIdx]] = true
		}
	}
	return weekdayServices, nil
}

func parseBusRoutes(files map[string]*zip.File) (map[string]gtfsRoute, error) {
	routesFile, ok := files["routes.txt"]
	if !ok {
		return nil, fmt.Errorf("routes.txt not found in GTFS archive")
	}
	rc, err := routesFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open routes.txt: %w", err)
	}
	defer rc.Close() //nolint:errcheck

	routesReader := csv.NewReader(rc)
	routesHeader, err := routesReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read routes.txt header: %w", err)
	}
	rIDIdx, shortIdx, longIdx, typeIdx := -1, -1, -1, -1
	for i, h := range routesHeader {
		switch strings.TrimSpace(h) {
		case "route_id":
			rIDIdx = i
		case "route_short_name":
			shortIdx = i
		case "route_long_name":
			longIdx = i
		case "route_type":
			typeIdx = i
		}
	}
	if rIDIdx < 0 || typeIdx < 0 {
		return nil, fmt.Errorf("routes.txt missing required columns")
	}

	busRoutes := make(map[string]gtfsRoute)
	for {
		row, err := routesReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if strings.TrimSpace(row[typeIdx]) == "3" {
			short := ""
			if shortIdx >= 0 && shortIdx < len(row) {
				short = strings.TrimSpace(row[shortIdx])
			}
			long := ""
			if longIdx >= 0 && longIdx < len(row) {
				long = strings.TrimSpace(row[longIdx])
			}
			rID := strings.TrimSpace(row[rIDIdx])
			busRoutes[rID] = gtfsRoute{
				id:        rID,
				shortName: short,
				longName:  long,
				routeType: "3",
			}
		}
	}
	return busRoutes, nil
}

func parseTrips(files map[string]*zip.File, weekdayServices map[string]bool, busRoutes map[string]gtfsRoute) (map[string]gtfsTrip, map[string]map[string]bool, error) {
	tripsFile, ok := files["trips.txt"]
	if !ok {
		return nil, nil, fmt.Errorf("trips.txt not found in GTFS archive")
	}
	rc, err := tripsFile.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("open trips.txt: %w", err)
	}
	defer rc.Close() //nolint:errcheck

	tripsReader := csv.NewReader(rc)
	tripsHeader, err := tripsReader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read trips.txt header: %w", err)
	}
	tripIDIdx, tripRouteIdx, tripServiceIdx, tripDirIdx, tripShapeIdx := -1, -1, -1, -1, -1
	for i, h := range tripsHeader {
		switch strings.TrimSpace(h) {
		case "trip_id":
			tripIDIdx = i
		case "route_id":
			tripRouteIdx = i
		case "service_id":
			tripServiceIdx = i
		case "direction_id":
			tripDirIdx = i
		case "shape_id":
			tripShapeIdx = i
		}
	}
	if tripIDIdx < 0 || tripRouteIdx < 0 || tripServiceIdx < 0 {
		return nil, nil, fmt.Errorf("trips.txt missing required columns")
	}

	trips := make(map[string]gtfsTrip)
	routeShapes := make(map[string]map[string]bool)

	for {
		row, err := tripsReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		routeID := strings.TrimSpace(row[tripRouteIdx])
		if _, isBus := busRoutes[routeID]; !isBus {
			continue
		}
		dirID := "0"
		if tripDirIdx >= 0 && tripDirIdx < len(row) && strings.TrimSpace(row[tripDirIdx]) != "" {
			dirID = strings.TrimSpace(row[tripDirIdx])
		}
		shapeID := ""
		if tripShapeIdx >= 0 && tripShapeIdx < len(row) {
			shapeID = strings.TrimSpace(row[tripShapeIdx])
		}
		if shapeID != "" {
			if routeShapes[routeID] == nil {
				routeShapes[routeID] = make(map[string]bool)
			}
			routeShapes[routeID][shapeID] = true
		}

		serviceID := strings.TrimSpace(row[tripServiceIdx])
		if !weekdayServices[serviceID] {
			continue
		}

		tripID := strings.TrimSpace(row[tripIDIdx])
		trips[tripID] = gtfsTrip{
			routeID:     routeID,
			serviceID:   serviceID,
			directionID: dirID,
			shapeID:     shapeID,
		}
	}
	return trips, routeShapes, nil
}

func parseFirstDepartures(files map[string]*zip.File, trips map[string]gtfsTrip) (map[string]string, error) {
	stopTimesFile, ok := files["stop_times.txt"]
	if !ok {
		return nil, fmt.Errorf("stop_times.txt not found in GTFS archive")
	}
	rc, err := stopTimesFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open stop_times.txt: %w", err)
	}
	defer rc.Close() //nolint:errcheck

	stReader := csv.NewReader(rc)
	stHeader, err := stReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read stop_times.txt header: %w", err)
	}
	stTripIdx, stDepIdx, stSeqIdx := -1, -1, -1
	for i, h := range stHeader {
		switch strings.TrimSpace(h) {
		case "trip_id":
			stTripIdx = i
		case "departure_time":
			stDepIdx = i
		case "stop_sequence":
			stSeqIdx = i
		}
	}
	if stTripIdx < 0 || stDepIdx < 0 || stSeqIdx < 0 {
		return nil, fmt.Errorf("stop_times.txt missing required columns")
	}

	tripFirstDeparture := make(map[string]string)
	for {
		row, err := stReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		tripID := strings.TrimSpace(row[stTripIdx])
		if _, isRelevant := trips[tripID]; !isRelevant {
			continue
		}
		if strings.TrimSpace(row[stSeqIdx]) == "1" {
			tripFirstDeparture[tripID] = strings.TrimSpace(row[stDepIdx])
		}
	}
	return tripFirstDeparture, nil
}

func parseShapePoints(files map[string]*zip.File, neededShapeIDs map[string]bool) (map[string][]shapePoint, error) {
	shapesFile, ok := files["shapes.txt"]
	if !ok {
		return nil, nil // shapes are optional
	}
	rc, err := shapesFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open shapes.txt: %w", err)
	}
	defer rc.Close() //nolint:errcheck

	sReader := csv.NewReader(rc)
	sHeader, err := sReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read shapes.txt header: %w", err)
	}
	sIDIdx, sLatIdx, sLonIdx, sSeqIdx := -1, -1, -1, -1
	for i, h := range sHeader {
		switch strings.TrimSpace(h) {
		case "shape_id":
			sIDIdx = i
		case "shape_pt_lat":
			sLatIdx = i
		case "shape_pt_lon":
			sLonIdx = i
		case "shape_pt_sequence":
			sSeqIdx = i
		}
	}
	if sIDIdx < 0 || sLatIdx < 0 || sLonIdx < 0 || sSeqIdx < 0 {
		return nil, fmt.Errorf("shapes.txt missing required columns")
	}

	shapePoints := make(map[string][]shapePoint)
	for {
		row, err := sReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		sID := strings.TrimSpace(row[sIDIdx])
		if !neededShapeIDs[sID] {
			continue
		}
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(row[sLatIdx]), 64)
		lon, err2 := strconv.ParseFloat(strings.TrimSpace(row[sLonIdx]), 64)
		seq, err3 := strconv.Atoi(strings.TrimSpace(row[sSeqIdx]))
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		shapePoints[sID] = append(shapePoints[sID], shapePoint{
			lat: lat,
			lon: lon,
			seq: seq,
		})
	}

	for sID := range shapePoints {
		pts := shapePoints[sID]
		sort.Slice(pts, func(i, j int) bool {
			return pts[i].seq < pts[j].seq
		})
		shapePoints[sID] = pts
	}
	return shapePoints, nil
}

// ingestBusCorridors derives bus corridors with their scheduled headways from the CTA GTFS feed
// and loads them into bus_corridors using a zero-downtime staging swap.
func ingestBusCorridors(ctx context.Context, c *cache.Cache, pool *pgxpool.Pool) error {
	var boundaryCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM city_boundary").Scan(&boundaryCount); err != nil {
		return fmt.Errorf("check city_boundary is populated: %w", err)
	}
	if boundaryCount == 0 {
		return fmt.Errorf("city_boundary is empty; run --dataset=boundary first")
	}

	startTime := time.Now()

	// 1. Fetch GTFS zip archive (cached or live)
	zipBytes, err := fetchOrLoadGTFS(ctx, c)
	if err != nil {
		return fmt.Errorf("fetch GTFS archive: %w", err)
	}

	// 2. Parse and derive all bus corridors
	corridors, err := deriveBusCorridorsFromZip(zipBytes)
	if err != nil {
		return fmt.Errorf("derive bus corridors: %w", err)
	}

	frequentCount := 0
	for _, c := range corridors {
		if c.meanHeadway <= 10.0 {
			frequentCount++
		}
	}

	slog.Info("bus_corridors: derived all bus corridors",
		slog.Int("total_routes", len(corridors)),
		slog.Int("frequent_routes", frequentCount),
	)

	// 3. Zero-downtime staging recompute pipeline (ARCHITECTURE §11.3)
	// Create an UNLOGGED staging table to build rows without WAL overhead
	_, err = pool.Exec(ctx, `
		CREATE UNLOGGED TABLE IF NOT EXISTS bus_corridors_staging (
			id BIGSERIAL PRIMARY KEY,
			route_id TEXT NOT NULL,
			route_name TEXT NOT NULL,
			mean_headway_minutes NUMERIC(5, 2) NOT NULL,
			geom GEOGRAPHY(MULTILINESTRING, 4326) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create bus_corridors_staging: %w", err)
	}
	defer func() {
		// Clean up staging table if left behind
		_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS bus_corridors_staging")
	}()

	_, err = pool.Exec(ctx, "TRUNCATE bus_corridors_staging")
	if err != nil {
		return fmt.Errorf("truncate bus_corridors_staging: %w", err)
	}

	// 4. Batch insert into staging table
	batch := &pgx.Batch{}
	for _, corr := range corridors {
		batch.Queue(`
			INSERT INTO bus_corridors_staging (route_id, route_name, mean_headway_minutes, geom)
			VALUES ($1, $2, $3, ST_GeomFromText($4, 4326)::geography)
		`, corr.routeID, corr.routeName, corr.meanHeadway, corr.wkt)
	}

	br := pool.SendBatch(ctx, batch)
	for i := 0; i < len(corridors); i++ {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("insert staging corridor row %d: %w", i, err)
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("close staging insert batch: %w", err)
	}

	computeDuration := time.Since(startTime)

	// 5. Atomic swap into active bus_corridors table
	swapStart := time.Now()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin swap transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "TRUNCATE bus_corridors"); err != nil {
		return fmt.Errorf("truncate active bus_corridors: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO bus_corridors (route_id, route_name, mean_headway_minutes, geom, created_at, updated_at)
		SELECT route_id, route_name, mean_headway_minutes, geom, created_at, updated_at
		FROM bus_corridors_staging
	`); err != nil {
		return fmt.Errorf("swap rows from staging into bus_corridors: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit swap transaction: %w", err)
	}

	lockDuration := time.Since(swapStart)
	totalDuration := time.Since(startTime)

	slog.Info("bus_corridors: atomic derivation swap complete",
		slog.Int("corridors_populated", len(corridors)),
		slog.Int("frequent_corridors", frequentCount),
		slog.Duration("compute_duration", computeDuration),
		slog.Duration("lock_duration", lockDuration),
		slog.Duration("total_duration", totalDuration),
	)

	return nil
}
