package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

// cityBoundaryResourceID is the Chicago Data Portal's "Boundaries - City"
// dataset. Its the_geom field is a GeoJSON MultiPolygon — see
// docs/DATA_SOURCES.md "City Boundary".
const cityBoundaryResourceID = "qqq8-j68g"

type cityBoundaryRow struct {
	TheGeom json.RawMessage `json:"the_geom"`
}

// ingestBoundary replaces the single-row city_boundary table with the
// current Chicago Data Portal polygon. The table is treated as fully
// replaceable rather than upserted: it holds exactly one static row, and a
// delete-then-insert is simpler to reason about than a unique constraint +
// ON CONFLICT for a dataset that changes only when Chicago's city limits
// legally change.
func ingestBoundary(ctx context.Context, soda *clients.SODAClient, pool *pgxpool.Pool) error {
	var rows []cityBoundaryRow
	if err := soda.GetJSON(ctx, cityBoundaryResourceID, nil, &rows); err != nil {
		return fmt.Errorf("fetch city boundary: %w", err)
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected exactly 1 city boundary row, got %d", len(rows))
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "DELETE FROM city_boundary"); err != nil {
		return fmt.Errorf("clear existing city boundary: %w", err)
	}

	const insertSQL = `
		INSERT INTO city_boundary (name, geom)
		VALUES ('Chicago', ST_GeomFromGeoJSON($1)::geography)
	`
	if _, err := tx.Exec(ctx, insertSQL, rows[0].TheGeom); err != nil {
		return fmt.Errorf("insert city boundary: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
