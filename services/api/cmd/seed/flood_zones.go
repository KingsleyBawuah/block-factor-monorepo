package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/cache"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

const (
	// floodZonesCacheKey/TTL: the assembled (all-pages) FEMA response is
	// cached in cache_entries, not a local file -- same reasoning and same
	// mechanism as OSM POIs/noise sources (see docs/DATA_SOURCES.md "OSM
	// POIs" "Caching"). The 24h TTL isn't tracking how often FEMA's real
	// data changes (official flood map revisions are years apart, far
	// slower than OSM) -- it exists purely to stop repeated local
	// `make seed` runs during development from re-pulling ~46MB across 13
	// requests every time. Production only syncs quarterly regardless,
	// always past any reasonable TTL, so the cache never risks serving
	// stale data there -- the real-world update cadence doesn't change
	// that math either way, hence reusing the same 24h value as the other
	// datasets rather than picking a bespoke one.
	floodZonesCacheKey = "fema:flood_zones:chicago"
	floodZonesCacheTTL = 24 * time.Hour

	// floodHazardZonesLayerID is NFHL MapServer layer 28, "Flood Hazard
	// Zones" -- confirmed live against the service's own layer list
	// (layer 3 "FIRM Panels" is just the map-sheet index, not hazard
	// data; layer 1/34 are Letters of Map Revision/Amendment, not zones).
	floodHazardZonesLayerID = 28

	// floodZonesPageSize is a starting point for QueryLayer's adaptive
	// pagination, not a fixed size -- some individual NFHL polygons are
	// large/complex enough that even this window occasionally needs to
	// shrink further for a specific offset (confirmed live: page size 150
	// failed at one particular offset while succeeding everywhere else,
	// because of one unusually dense polygon in that window, not the page
	// size in general). See FEMAClient.QueryLayer's doc comment.
	floodZonesPageSize = 150
)

// floodZoneProperties is the subset of NFHL layer 28's real fields (see
// docs/DATA_SOURCES.md "Flood Zones") this ingestion needs.
type floodZoneProperties struct {
	FldArID string `json:"FLD_AR_ID"`
	FldZone string `json:"FLD_ZONE"`
}

// knownFloodZones mirrors the fema_flood_zones.zone_designation CHECK
// constraint -- FEMA's fixed, published national standard (confirmed
// against FEMA's own glossary before the migration was written).
// Anything outside this set is logged and skipped rather than guessed or
// left to fail the CHECK constraint mid-batch.
var knownFloodZones = map[string]bool{
	"A": true, "AE": true, "AH": true, "AO": true, "AR": true,
	"A99": true, "V": true, "VE": true, "X": true, "D": true,
}

// ingestFloodZones replaces fema_flood_zones with FEMA's live National
// Flood Hazard Layer data for Chicago -- see docs/DATA_SOURCES.md "Flood
// Zones" for why this uses FEMA's ArcGIS REST service directly instead of
// a downloaded shapefile/GDB extract.
func ingestFloodZones(ctx context.Context, fema *clients.FEMAClient, c *cache.Cache, pool *pgxpool.Pool) error {
	var features []clients.FEMAFeature
	cached, err := c.Get(ctx, floodZonesCacheKey, &features)
	if err != nil {
		return fmt.Errorf("check flood_zones cache: %w", err)
	}
	if cached {
		slog.Info("flood_zones: using cached FEMA response", slog.Int("features", len(features)))
	} else {
		features, err = fema.QueryLayer(ctx, floodHazardZonesLayerID,
			chicagoBBoxSouth, chicagoBBoxWest, chicagoBBoxNorth, chicagoBBoxEast,
			"FLD_AR_ID,FLD_ZONE", floodZonesPageSize)
		if err != nil {
			return fmt.Errorf("query FEMA for flood zones: %w", err)
		}
		if err := c.Set(ctx, floodZonesCacheKey, features, floodZonesCacheTTL); err != nil {
			return fmt.Errorf("cache flood_zones response: %w", err)
		}
		slog.Info("flood_zones: fetched fresh FEMA response", slog.Int("features", len(features)))
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "TRUNCATE fema_flood_zones"); err != nil {
		return fmt.Errorf("truncate existing fema_flood_zones: %w", err)
	}

	// Flood zones follow floodplain/waterway geography, not city limits --
	// unlike osm_pois' single points, a real zone polygon can legitimately
	// straddle the city boundary. ST_Intersects (any overlap) is the right
	// predicate here, not ST_Covers (fully contained), which would wrongly
	// drop zones that meaningfully affect Chicago addresses near the edge.
	//
	// ST_MakeValid() repairs self-intersecting rings, same as parks.go --
	// confirmed live that this dataset has the same class of real-world
	// topology defect (1 of 291 Chicago-area zones failed ST_IsValid).
	// Applied before the ST_Intersects check too, not just the stored
	// value, since an invalid input can make that predicate unreliable.
	const insertSQL = `
		INSERT INTO fema_flood_zones (source_id, zone_designation, geom)
		SELECT $1, $2, ST_Multi(ST_MakeValid(ST_GeomFromGeoJSON($3)))::geography
		WHERE ST_Intersects(
			(SELECT geom FROM city_boundary LIMIT 1)::geometry,
			ST_MakeValid(ST_GeomFromGeoJSON($3))
		)
	`

	batch := &pgx.Batch{}
	queued := 0
	var unparseableProps, unknownZone int
	for _, f := range features {
		var props floodZoneProperties
		if err := json.Unmarshal(f.Properties, &props); err != nil {
			unparseableProps++
			continue
		}
		if !knownFloodZones[props.FldZone] {
			unknownZone++
			slog.Warn("unrecognized FEMA flood zone designation", slog.String("fld_zone", props.FldZone), slog.String("fld_ar_id", props.FldArID))
			continue
		}

		batch.Queue(insertSQL, props.FldArID, props.FldZone, string(f.Geometry))
		queued++
	}

	var inserted int64
	if queued > 0 {
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < queued; i++ {
			tag, err := results.Exec()
			if err != nil {
				results.Close() //nolint:errcheck // already returning the real error
				return fmt.Errorf("insert flood zone batch (item %d of %d): %w", i+1, queued, err)
			}
			inserted += tag.RowsAffected()
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("close flood zone batch results: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	slog.Info("flood_zones ingestion summary",
		slog.Int("candidates", len(features)),
		slog.Int64("inserted", inserted),
		slog.Int64("outside_chicago", int64(queued)-inserted),
		slog.Int("unparseable_properties", unparseableProps),
		slog.Int("unknown_zone", unknownZone),
	)

	return nil
}
