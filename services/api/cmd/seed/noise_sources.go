package main

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/cache"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

const (
	noiseSourcesCacheKey = "overpass:noise_sources:chicago"
	noiseSourcesCacheTTL = 24 * time.Hour
)

// noiseSourcesQuery fetches every CTA rapid-transit track way, motorway
// (expressway), and primary road (arterial) within the Chicago bounding
// box (shared with osm_pois -- see chicagoBBox* constants), with full line
// geometry via "out geom".
//
// CTA tracks are matched on railway=subway + a CTA operator tag -- verified
// against the real data that every CTA rapid-transit segment uses
// railway=subway regardless of whether it's actually elevated, at-grade, or
// underground (e.g. a way literally named "South Side Elevated" is tagged
// railway=subway with bridge=yes); grade is derived separately from
// tunnel/bridge/layer tags, not from railway=subway itself. See
// docs/DATA_SOURCES.md "Noise Sources".
var noiseSourcesQuery = fmt.Sprintf(`
[out:json][timeout:120];
(
  way["railway"="subway"]["operator"~"Chicago Transit Authority",i](%[1]f,%[2]f,%[3]f,%[4]f);
  way["highway"="motorway"](%[1]f,%[2]f,%[3]f,%[4]f);
  way["highway"="primary"](%[1]f,%[2]f,%[3]f,%[4]f);
);
out tags geom;
`, chicagoBBoxSouth, chicagoBBoxWest, chicagoBBoxNorth, chicagoBBoxEast)

var maxspeedMPHPattern = regexp.MustCompile(`^(\d+)\s*mph$`)

// classifyNoiseSource returns the noise_sources.source_type and (for
// cta_track only) grade for a way's tags, and false if it matches none of
// the three categories -- shouldn't happen given the query's own filters,
// but defensive against a future query change.
func classifyNoiseSource(tags map[string]string) (sourceType, grade string, ok bool) {
	if tags["railway"] == "subway" {
		return "cta_track", classifyTrackGrade(tags), true
	}
	switch tags["highway"] {
	case "motorway":
		return "expressway", "", true
	case "primary":
		return "arterial", "", true
	}
	return "", "", false
}

// classifyTrackGrade derives elevated/at_grade/underground from OSM's
// tunnel/bridge/layer tags, per PRD 5.6's own stated approach -- verified
// against all 1,842 real CTA track ways in Chicago: tunnel/bridge tags
// alone classify 99% of them, with layer sign as a fallback for the
// remainder that carry a layer tag but no explicit tunnel/bridge tag.
func classifyTrackGrade(tags map[string]string) string {
	if tags["tunnel"] != "" {
		return "underground"
	}
	if tags["bridge"] != "" {
		return "elevated"
	}
	if layer, err := strconv.Atoi(tags["layer"]); err == nil {
		switch {
		case layer > 0:
			return "elevated"
		case layer < 0:
			return "underground"
		}
	}
	return "at_grade"
}

// parseLanes and parseMaxspeedMPH return nil when the tag is absent or
// unparseable -- lanes/maxspeed metadata is best-effort (see the
// noise_sources migration comment), not required.
func parseLanes(tags map[string]string) *int16 {
	n, err := strconv.ParseInt(tags["lanes"], 10, 16)
	if err != nil {
		return nil
	}
	v := int16(n)
	return &v
}

func parseMaxspeedMPH(tags map[string]string) *int16 {
	m := maxspeedMPHPattern.FindStringSubmatch(tags["maxspeed"])
	if m == nil {
		return nil
	}
	n, err := strconv.ParseInt(m[1], 10, 16)
	if err != nil {
		return nil
	}
	v := int16(n)
	return &v
}

// hasTurnLane reports whether any turn:lanes* tag on this way mentions a
// left-turn lane -- a real, verified signal (confirmed against live data),
// though it describes this specific way segment near an intersection, not
// a claim about the whole corridor. See PRD 5.6's S_stroad callout.
func hasTurnLane(tags map[string]string) bool {
	for key, value := range tags {
		if strings.HasPrefix(key, "turn:lanes") && strings.Contains(value, "left") {
			return true
		}
	}
	return false
}

// wayGeometryToWKT builds a LINESTRING WKT string from an Overpass way's
// ordered coordinate sequence, for ST_GeomFromText.
func wayGeometryToWKT(geometry []clients.OverpassLatLon) string {
	points := make([]string, len(geometry))
	for i, pt := range geometry {
		points[i] = fmt.Sprintf("%f %f", pt.Lon, pt.Lat)
	}
	return "LINESTRING(" + strings.Join(points, ", ") + ")"
}

// ingestNoiseSources replaces noise_sources with CTA rapid-transit tracks,
// expressways, and arterial roads from the live Overpass API -- see
// docs/DATA_SOURCES.md "Noise Sources".
func ingestNoiseSources(ctx context.Context, overpass *clients.OverpassClient, c *cache.Cache, pool *pgxpool.Pool) error {
	var elements []clients.OverpassElement
	cached, err := c.Get(ctx, noiseSourcesCacheKey, &elements)
	if err != nil {
		return fmt.Errorf("check noise_sources cache: %w", err)
	}
	if cached {
		slog.Info("noise_sources: using cached Overpass response", slog.Int("elements", len(elements)))
	} else {
		elements, err = overpass.Query(ctx, noiseSourcesQuery)
		if err != nil {
			return fmt.Errorf("query Overpass for noise sources: %w", err)
		}
		if err := c.Set(ctx, noiseSourcesCacheKey, elements, noiseSourcesCacheTTL); err != nil {
			return fmt.Errorf("cache noise_sources response: %w", err)
		}
		slog.Info("noise_sources: fetched fresh Overpass response", slog.Int("elements", len(elements)))
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "TRUNCATE noise_sources"); err != nil {
		return fmt.Errorf("truncate existing noise_sources: %w", err)
	}

	const insertSQL = `
		INSERT INTO noise_sources (source_id, source_type, grade, lanes, maxspeed_mph, has_turn_lane, geom)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, ST_Multi(ST_GeomFromText($7, 4326))::geography)
	`

	batch := &pgx.Batch{}
	queued := 0
	var noGeometry, unclassifiable int
	for _, el := range elements {
		if len(el.Geometry) < 2 {
			noGeometry++
			continue
		}
		sourceType, grade, ok := classifyNoiseSource(el.Tags)
		if !ok {
			unclassifiable++
			continue
		}

		sourceID := el.Type + "/" + strconv.FormatInt(el.ID, 10)
		wkt := wayGeometryToWKT(el.Geometry)
		batch.Queue(insertSQL, sourceID, sourceType, grade, parseLanes(el.Tags), parseMaxspeedMPH(el.Tags), hasTurnLane(el.Tags), wkt)
		queued++
	}

	if queued > 0 {
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < queued; i++ {
			if _, err := results.Exec(); err != nil {
				results.Close() //nolint:errcheck // already returning the real error
				return fmt.Errorf("insert noise source batch (item %d of %d): %w", i+1, queued, err)
			}
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("close noise source batch results: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	slog.Info("noise_sources ingestion summary",
		slog.Int("candidates", len(elements)),
		slog.Int("inserted", queued),
		slog.Int("no_geometry", noGeometry),
		slog.Int("unclassifiable", unclassifiable),
	)

	return nil
}
