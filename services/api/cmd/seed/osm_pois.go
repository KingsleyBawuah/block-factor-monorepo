package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/cache"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

const (
	// osmPOIsCacheKey/TTL: the raw Overpass response is cached in
	// cache_entries (not a local file -- see docs/DATA_SOURCES.md "OSM
	// POIs" for why) so repeated local `make seed` runs during
	// development reuse it instead of re-querying the live API every
	// time. In production this dataset only syncs quarterly, so any
	// real run is always past this TTL and always fetches live -- the
	// cache never risks serving stale data there.
	osmPOIsCacheKey = "overpass:osm_pois:chicago"
	osmPOIsCacheTTL = 24 * time.Hour

	// Chicago bounding box (south, west, north, east), derived from our
	// own city_boundary geometry -- covers the whole city including
	// O'Hare's detached exclave, plus margin. This is deliberately a
	// rough over-fetch: Overpass's bbox filter can't express our exact
	// (two-ring) boundary shape, and there's no need for it to --
	// precise Chicago-limits filtering happens at insert time via
	// ST_Covers against city_boundary instead.
	chicagoBBoxSouth = 41.6445
	chicagoBBoxWest  = -87.9401
	chicagoBBoxNorth = 42.0230
	chicagoBBoxEast  = -87.5241
)

// osmPOIsQuery fetches every node and way tagged as a supermarket,
// convenience store, or pharmacy within the Chicago bounding box, in one
// request -- one combined query rather than one per category keeps this as
// light on Overpass as possible. "out center" computes a representable
// point for way results server-side, so no centroid math is needed here.
var osmPOIsQuery = fmt.Sprintf(`
[out:json][timeout:90];
(
  node["shop"~"^(supermarket|convenience)$"](%[1]f,%[2]f,%[3]f,%[4]f);
  node["amenity"="pharmacy"](%[1]f,%[2]f,%[3]f,%[4]f);
  way["shop"~"^(supermarket|convenience)$"](%[1]f,%[2]f,%[3]f,%[4]f);
  way["amenity"="pharmacy"](%[1]f,%[2]f,%[3]f,%[4]f);
);
out tags center;
`, chicagoBBoxSouth, chicagoBBoxWest, chicagoBBoxNorth, chicagoBBoxEast)

// classifyOSMPOI returns the osm_pois.poi_type for a set of OSM tags, and
// false if none of the target tags are present -- shouldn't happen given
// osmPOIsQuery's own tag filter, but defensive against a future query
// change. amenity=pharmacy is checked first, though in practice no element
// carries both a target shop tag and amenity=pharmacy simultaneously
// (confirmed against a real Chicago-wide query result).
func classifyOSMPOI(tags map[string]string) (poiType string, ok bool) {
	if tags["amenity"] == "pharmacy" {
		return "pharmacy", true
	}
	switch tags["shop"] {
	case "supermarket":
		return "supermarket", true
	case "convenience":
		return "convenience", true
	}
	return "", false
}

// ingestOSMPOIs replaces osm_pois with supermarkets, convenience stores,
// and pharmacies from the live Overpass API -- see docs/DATA_SOURCES.md
// "OSM POIs" for why this replaced the originally-planned local PBF file +
// ogr2ogr approach (way-mapped features came back with unusable/missing IDs
// through that path; Overpass returns real IDs for both nodes and ways).
func ingestOSMPOIs(ctx context.Context, overpass *clients.OverpassClient, c *cache.Cache, pool *pgxpool.Pool) error {
	// The insert below filters candidates against city_boundary via
	// ST_Covers; if that table is empty, ST_Covers(NULL, point) evaluates
	// to NULL (not true), silently filtering out every row rather than
	// erroring. Fail loudly instead of shipping an empty osm_pois table
	// with no explanation.
	var boundaryCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM city_boundary").Scan(&boundaryCount); err != nil {
		return fmt.Errorf("check city_boundary is populated: %w", err)
	}
	if boundaryCount == 0 {
		return fmt.Errorf("city_boundary is empty; run --dataset=boundary first")
	}

	var elements []clients.OverpassElement
	cached, err := c.Get(ctx, osmPOIsCacheKey, &elements)
	if err != nil {
		return fmt.Errorf("check osm_pois cache: %w", err)
	}
	if cached {
		slog.Info("osm_pois: using cached Overpass response", slog.Int("elements", len(elements)))
	} else {
		elements, err = overpass.Query(ctx, osmPOIsQuery)
		if err != nil {
			return fmt.Errorf("query Overpass for OSM POIs: %w", err)
		}
		if err := c.Set(ctx, osmPOIsCacheKey, elements, osmPOIsCacheTTL); err != nil {
			return fmt.Errorf("cache osm_pois response: %w", err)
		}
		slog.Info("osm_pois: fetched fresh Overpass response", slog.Int("elements", len(elements)))
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "TRUNCATE osm_pois"); err != nil {
		return fmt.Errorf("truncate existing osm_pois: %w", err)
	}

	// WHERE ST_Covers(...) filters each candidate to Chicago's real
	// (two-ring) boundary at insert time; a candidate outside it is
	// simply not inserted by this statement rather than needing a
	// separate app-level check.
	const insertSQL = `
		INSERT INTO osm_pois (source_id, poi_name, poi_type, geom)
		SELECT $1, NULLIF($2, ''), $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography
		WHERE ST_Covers(
			(SELECT geom FROM city_boundary LIMIT 1)::geometry,
			ST_SetSRID(ST_MakePoint($4, $5), 4326)
		)
	`

	batch := &pgx.Batch{}
	queued := 0
	var noPoint, unclassifiable int
	for _, el := range elements {
		lat, lon, ok := el.Point()
		if !ok {
			noPoint++
			continue
		}
		poiType, ok := classifyOSMPOI(el.Tags)
		if !ok {
			unclassifiable++
			continue
		}

		sourceID := el.Type + "/" + strconv.FormatInt(el.ID, 10)
		batch.Queue(insertSQL, sourceID, el.Tags["name"], poiType, lon, lat)
		queued++
	}

	var inserted int64
	if queued > 0 {
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < queued; i++ {
			tag, err := results.Exec()
			if err != nil {
				results.Close() //nolint:errcheck // already returning the real error
				return fmt.Errorf("insert OSM POI batch (item %d of %d): %w", i+1, queued, err)
			}
			inserted += tag.RowsAffected()
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("close OSM POI batch results: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	slog.Info("osm_pois ingestion summary",
		slog.Int("candidates", len(elements)),
		slog.Int64("inserted", inserted),
		slog.Int64("outside_chicago", int64(queued)-inserted),
		slog.Int("no_point", noPoint),
		slog.Int("unclassifiable", unclassifiable),
	)

	return nil
}
