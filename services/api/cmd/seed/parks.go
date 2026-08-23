package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

// parksResourceID is the Chicago Data Portal's "CPD_Parks" dataset -- the
// actual populated one; two other candidate resource IDs return empty
// geometry despite matching titles, see docs/DATA_SOURCES.md "Parks".
const parksResourceID = "ejsh-fztr"

type parkRow struct {
	ParkNo    string          `json:"park_no"`
	Park      string          `json:"park"`
	ParkClass string          `json:"park_class"`
	Acres     string          `json:"acres"`
	TheGeom   json.RawMessage `json:"the_geom"`
}

// ingestParks replaces parks with the current Chicago Data Portal park
// boundaries. Two geometry fixups are applied on the way in:
//   - ST_MakeValid() repairs self-intersecting rings -- a real data-quality
//     issue in the source's largest, most complex boundaries (Washington
//     Park, Grant Park, Lincoln Park, and others), confirmed by auditing
//     the ingested data: 11 of 617 parks failed ST_IsValid before this fix,
//     including 3 LINEAR PARK rows that S_green's off-street trail bonus
//     depends on.
//   - ST_Multi() -- a no-op if the geometry is already a MultiPolygon --
//     handles the separate case of smaller parks coming back as a plain
//     GeoJSON Polygon, to match the parks table's GEOGRAPHY(MULTIPOLYGON,
//     4326) column. Applied after ST_MakeValid since repairing a
//     self-intersecting Polygon can itself produce a MultiPolygon.
func ingestParks(ctx context.Context, soda *clients.SODAClient, pool *pgxpool.Pool) error {
	var rows []parkRow
	params := url.Values{"$limit": {"5000"}} // source has 617 rows; well above that
	if err := soda.GetJSON(ctx, parksResourceID, params, &rows); err != nil {
		return fmt.Errorf("fetch parks: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "DELETE FROM parks"); err != nil {
		return fmt.Errorf("clear existing parks: %w", err)
	}

	const insertSQL = `
		INSERT INTO parks (source_id, park_name, park_class, acres, geom)
		VALUES ($1, $2, $3, NULLIF($4, '')::numeric, ST_Multi(ST_MakeValid(ST_GeomFromGeoJSON($5)))::geography)
	`
	for _, row := range rows {
		if _, err := tx.Exec(ctx, insertSQL, row.ParkNo, row.Park, row.ParkClass, row.Acres, row.TheGeom); err != nil {
			return fmt.Errorf("insert park %s (%s): %w", row.ParkNo, row.Park, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
