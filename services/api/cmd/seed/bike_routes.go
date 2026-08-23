package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

// bikeRoutesResourceID is the Chicago Data Portal's current "Bike Routes"
// dataset -- see docs/DATA_SOURCES.md "Bike Routes". displayrou maps
// directly onto bike_routes.route_type's 5 allowed values. This dataset has
// no published unique-ID field of its own, so source_id uses Socrata's
// internal :id row identifier (requested via $select) instead.
const bikeRoutesResourceID = "hvv9-38ut"

type bikeRouteRow struct {
	SourceID     string          `json:"id"` // aliased from Socrata's :id via $select
	TheGeom      json.RawMessage `json:"the_geom"`
	DisplayRoute string          `json:"displayrou"`
	Street       string          `json:"street"`
}

// ingestBikeRoutes replaces bike_routes with the current Chicago Data
// Portal bike infrastructure segments.
func ingestBikeRoutes(ctx context.Context, soda *clients.SODAClient, pool *pgxpool.Pool) error {
	var rows []bikeRouteRow
	params := url.Values{
		"$select": {":id AS id, the_geom, displayrou, street"},
		"$limit":  {"5000"}, // source has 1008 rows; well above SODA's default 1000-row cap
	}
	if err := soda.GetJSON(ctx, bikeRoutesResourceID, params, &rows); err != nil {
		return fmt.Errorf("fetch bike routes: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "DELETE FROM bike_routes"); err != nil {
		return fmt.Errorf("clear existing bike routes: %w", err)
	}

	const insertSQL = `
		INSERT INTO bike_routes (source_id, route_type, street_name, geom)
		VALUES ($1, $2, $3, ST_GeomFromGeoJSON($4)::geography)
	`
	for _, row := range rows {
		if _, err := tx.Exec(ctx, insertSQL, row.SourceID, row.DisplayRoute, row.Street, row.TheGeom); err != nil {
			return fmt.Errorf("insert bike route %s: %w", row.SourceID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
