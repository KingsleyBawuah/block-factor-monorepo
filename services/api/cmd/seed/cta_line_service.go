package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/cache"
)

const (
	// overnightWindowStartSecs and overnightWindowEndSecs define the "1:00-4:00 AM,
	// any day" overnight window from PRD 5.2, in same-calendar-day seconds-from-midnight.
	overnightWindowStartSecs = 1 * 3600
	overnightWindowEndSecs   = 4 * 3600

	// overnightWindowStartSecsNextDay and overnightWindowEndSecsNextDay are the same
	// wall-clock 1-4 AM window expressed in GTFS's after-midnight hour>=24 convention,
	// which CTA's real feed does use for trips whose service_id represents the
	// previous calendar day (confirmed live: e.g. the Red Line has thousands of
	// stop_times rows in this 25:00-28:00 range). Both representations describe the
	// same 3-clock-hour span, so they're never double-counted against each other --
	// a departure matches at most one of the two ranges.
	overnightWindowStartSecsNextDay = 25 * 3600
	overnightWindowEndSecsNextDay   = 28 * 3600

	overnightWindowMinutes = 180.0

	// expectedCTALineCount is the current real count of CTA 'L' lines (matching
	// cta_line_service's line_name CHECK constraint). Guards against silently
	// truncating and replacing good data with a partial set if a future GTFS
	// feed change (e.g. a route naming/type change) causes derivation to find
	// fewer lines than expected.
	expectedCTALineCount = 8
)

type derivedLineService struct {
	lineName                string
	amPeakHeadwayMinutes    float64
	overnightHeadwayMinutes *float64
}

// canonicalLineName derives the line_name value matching cta_stops.lines exactly
// (Red, Blue, Brown, Green, Orange, Pink, Purple, Yellow) from a GTFS route_long_name
// like "Red Line" -- route_id itself is inconsistent (Red, Blue, Pink use the full
// word; Purple, Yellow, Green, Orange, Brown use single/short letters: P, Y, G, Org,
// Brn), so route_long_name is the reliable join key, confirmed against the live feed.
func canonicalLineName(routeLongName string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(routeLongName), "Line"))
}

func parseRailRoutes(files map[string]*zip.File) (map[string]gtfsRoute, error) {
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
	rIDIdx, longIdx, typeIdx := -1, -1, -1
	for i, h := range routesHeader {
		switch strings.TrimSpace(h) {
		case "route_id":
			rIDIdx = i
		case "route_long_name":
			longIdx = i
		case "route_type":
			typeIdx = i
		}
	}
	if rIDIdx < 0 || longIdx < 0 || typeIdx < 0 {
		return nil, fmt.Errorf("routes.txt missing required columns")
	}

	railRoutes := make(map[string]gtfsRoute)
	for {
		row, err := routesReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if strings.TrimSpace(row[typeIdx]) != "1" {
			continue
		}
		rID := strings.TrimSpace(row[rIDIdx])
		railRoutes[rID] = gtfsRoute{
			id:        rID,
			longName:  strings.TrimSpace(row[longIdx]),
			routeType: "1",
		}
	}
	return railRoutes, nil
}

// isInOvernightWindow reports whether secs (seconds from midnight, GTFS-style so
// values >= 24*3600 are valid) falls in the 1-4 AM window under either GTFS
// same-day or after-midnight representation.
func isInOvernightWindow(secs int) bool {
	return (secs >= overnightWindowStartSecs && secs < overnightWindowEndSecs) ||
		(secs >= overnightWindowStartSecsNextDay && secs < overnightWindowEndSecsNextDay)
}

// deriveLineServiceFromZip parses a GTFS zip archive and computes, per CTA 'L' line as
// of referenceDate: mean AM-peak headway (weekday 7-9 AM, PRD 5.2) and mean overnight
// headway (1-4 AM, any day of week -- but still validity-window-filtered; see
// ARCHITECTURE.md Decision Log row 28 for why the date-window check matters and why
// "any day" here means "no day-of-week filter", not "no date filter").
func deriveLineServiceFromZip(zipBytes []byte, referenceDate time.Time) ([]derivedLineService, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("open zip reader: %w", err)
	}

	files := make(map[string]*zip.File)
	for _, f := range zr.File {
		files[f.Name] = f
	}

	calendarServices, err := parseCalendarServices(files)
	if err != nil {
		return nil, err
	}
	weekdayServices := weekdayServiceIDs(calendarServices, referenceDate)

	anyDayServices := make(map[string]bool)
	for id, svc := range calendarServices {
		if svc.isActiveOn(referenceDate) {
			anyDayServices[id] = true
		}
	}

	railRoutes, err := parseRailRoutes(files)
	if err != nil {
		return nil, err
	}

	amPeakTrips, _, err := parseTrips(files, weekdayServices, railRoutes)
	if err != nil {
		return nil, err
	}
	overnightTrips, _, err := parseTrips(files, anyDayServices, railRoutes)
	if err != nil {
		return nil, err
	}

	amPeakDep, err := parseFirstDepartures(files, amPeakTrips)
	if err != nil {
		return nil, err
	}
	overnightDep, err := parseFirstDepartures(files, overnightTrips)
	if err != nil {
		return nil, err
	}

	amPeakHeadway := computeHeadwaysByLine(railRoutes, amPeakTrips, amPeakDep, func(secs int) bool {
		return secs >= amPeakStartSecs && secs < amPeakEndSecs
	}, amPeakWindowMinutes)

	overnightHeadway := computeOvernightHeadwaysByLine(railRoutes, overnightTrips, overnightDep, calendarServices)

	var results []derivedLineService
	for routeID, route := range railRoutes {
		lineName := canonicalLineName(route.longName)
		amHeadway, ok := amPeakHeadway[routeID]
		if !ok {
			amHeadway = noAMPeakHeadwaySentinel
		}

		var overnight *float64
		if h, ok := overnightHeadway[routeID]; ok {
			overnight = &h
		}

		results = append(results, derivedLineService{
			lineName:                lineName,
			amPeakHeadwayMinutes:    amHeadway,
			overnightHeadwayMinutes: overnight,
		})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].lineName < results[j].lineName })
	return results, nil
}

// computeHeadwaysByLine counts, per route, first-departures matching inWindow and
// converts to a mean headway (windowMinutes / avg trips per active direction) --
// same trip-count-based approximation already used for bus_corridors, kept
// consistent across both derivations rather than introducing a second methodology.
func computeHeadwaysByLine(routes map[string]gtfsRoute, trips map[string]gtfsTrip, firstDepartures map[string]string, inWindow func(secs int) bool, windowMinutes float64) map[string]float64 {
	routeDirTrips := make(map[string]map[string]int)
	for tripID, depTime := range firstDepartures {
		secs, ok := parseTimeSeconds(depTime)
		if !ok || !inWindow(secs) {
			continue
		}
		meta := trips[tripID]
		if routeDirTrips[meta.routeID] == nil {
			routeDirTrips[meta.routeID] = make(map[string]int)
		}
		routeDirTrips[meta.routeID][meta.directionID]++
	}

	headways := make(map[string]float64)
	for routeID := range routes {
		dirCounts := routeDirTrips[routeID]
		if dirCounts == nil {
			continue
		}
		dir0, dir1 := dirCounts["0"], dirCounts["1"]
		total := dir0 + dir1
		if total == 0 {
			continue
		}
		numDirs := 0
		if dir0 > 0 {
			numDirs++
		}
		if dir1 > 0 {
			numDirs++
		}
		avgTripsPerDir := float64(total) / float64(numDirs)
		headways[routeID] = windowMinutes / avgTripsPerDir
	}
	return headways
}

// computeOvernightHeadwaysByLine is computeHeadwaysByLine's overnight counterpart --
// NOT a simple trip count, because the overnight trip set spans multiple day-of-week
// service_id patterns (weekday, Saturday, Sunday, and CTA's real feed splits these
// even more finely -- e.g. separate "Monday only" and "Tuesday-Saturday" templates
// for the late-night portion of overnight service). Naively summing first-departures
// across all of them and dividing by the 3-hour window, like computeHeadwaysByLine
// does, treats every day-type's trips as if they all ran on the SAME single night --
// e.g. pooling a Mon-Fri service_id (5 nights/week) with a Saturday-only and a
// Sunday-only service_id (1 night/week each) inflates the trip count for any single
// night by up to ~3x, deflating the computed headway by the same factor (confirmed
// against live data: Red Line computed as a 5.0 min overnight headway, actual
// per-night pattern is a consistent 12 trips/direction = 15.0 min every single night).
//
// The fix: weight each qualifying trip's contribution by its service_id's
// daysPerWeek (how many nights per week that trip actually runs), sum those weights
// per route/direction, and divide by 7 to get the average trips-per-night across a
// full week -- which correctly reflects "what headway should a rider expect on a
// typical night" regardless of how many separate service_id templates the feed
// happens to split that week's schedule into.
func computeOvernightHeadwaysByLine(routes map[string]gtfsRoute, trips map[string]gtfsTrip, firstDepartures map[string]string, calendarServices map[string]gtfsService) map[string]float64 {
	const daysPerWeekTotal = 7.0

	routeDirWeightedTrips := make(map[string]map[string]int)
	for tripID, depTime := range firstDepartures {
		secs, ok := parseTimeSeconds(depTime)
		if !ok || !isInOvernightWindow(secs) {
			continue
		}
		meta := trips[tripID]
		svc, ok := calendarServices[meta.serviceID]
		if !ok {
			continue
		}
		if routeDirWeightedTrips[meta.routeID] == nil {
			routeDirWeightedTrips[meta.routeID] = make(map[string]int)
		}
		routeDirWeightedTrips[meta.routeID][meta.directionID] += svc.daysPerWeek
	}

	headways := make(map[string]float64)
	for routeID := range routes {
		dirWeighted := routeDirWeightedTrips[routeID]
		if dirWeighted == nil {
			continue
		}
		dir0, dir1 := dirWeighted["0"], dirWeighted["1"]
		totalWeighted := dir0 + dir1
		if totalWeighted == 0 {
			continue
		}
		numDirs := 0
		if dir0 > 0 {
			numDirs++
		}
		if dir1 > 0 {
			numDirs++
		}
		avgWeightedPerDir := float64(totalWeighted) / float64(numDirs)
		avgTripsPerNightPerDir := avgWeightedPerDir / daysPerWeekTotal
		headways[routeID] = overnightWindowMinutes / avgTripsPerNightPerDir
	}
	return headways
}

// ingestLineService derives per-line AM-peak and overnight headway from the CTA GTFS
// feed and loads them into cta_line_service. No staging-table swap (unlike
// bus_corridors/safety_grid_percentiles): 8 rows with no geometry already gives a
// plain transaction ~zero lock duration, per the migration's own design note.
func ingestLineService(ctx context.Context, c *cache.Cache, pool *pgxpool.Pool) error {
	zipBytes, err := fetchOrLoadGTFS(ctx, c)
	if err != nil {
		return fmt.Errorf("fetch GTFS archive: %w", err)
	}

	lines, err := deriveLineServiceFromZip(zipBytes, time.Now())
	if err != nil {
		return fmt.Errorf("derive line service: %w", err)
	}
	if len(lines) != expectedCTALineCount {
		return fmt.Errorf("cta_line_service: expected %d CTA rail lines, derived %d -- refusing to truncate/replace existing data with a partial set", expectedCTALineCount, len(lines))
	}

	overnightCount := 0
	for _, l := range lines {
		if l.overnightHeadwayMinutes != nil {
			overnightCount++
		}
	}
	slog.Info("cta_line_service: derived all rail lines",
		slog.Int("total_lines", len(lines)),
		slog.Int("lines_with_overnight_service", overnightCount),
	)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "TRUNCATE cta_line_service"); err != nil {
		return fmt.Errorf("truncate cta_line_service: %w", err)
	}

	batch := &pgx.Batch{}
	for _, l := range lines {
		batch.Queue(`
			INSERT INTO cta_line_service (line_name, am_peak_headway_minutes, overnight_headway_minutes)
			VALUES ($1, $2, $3)
		`, l.lineName, l.amPeakHeadwayMinutes, l.overnightHeadwayMinutes)
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < len(lines); i++ {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("insert line service row %d: %w", i, err)
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("close insert batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	slog.Info("cta_line_service: ingestion complete", slog.Int("lines_populated", len(lines)))
	return nil
}
