package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// safetyGridCellSizeMeters defines the edge length of each square grid cell
	// in meters (500m x 500m, ~0.25 km²), per PRD §5.9.
	safetyGridCellSizeMeters = 500

	// sridUTM16N is the Spatial Reference Identifier for UTM Zone 16N (WGS84),
	// a conformal projection centered on Chicago where coordinate units are in
	// exact meters.
	sridUTM16N = 32616
)

// calculatePenaltyScore computes the raw crime penalty score for a block per
// PRD §5.9:
//
//	Penalty = (5 × Violent) + (5 × Sexual) + (1 × Property) + (2 × Nuisance)
func calculatePenaltyScore(violent, sexual, property, nuisance int) int {
	return (5 * violent) + (5 * sexual) + (1 * property) + (2 * nuisance)
}

// calculatePercentileRank computes the percentile rank (0.00 to 100.00) of each
// penalty score against the population using standard rank / (N - 1) * 100,
// matching PostgreSQL's PERCENT_RANK() window function.
func calculatePercentileRank(scores []float64) []float64 {
	n := len(scores)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return []float64{0.0}
	}

	type scoreItem struct {
		index int
		value float64
	}

	items := make([]scoreItem, n)
	for i, s := range scores {
		items[i] = scoreItem{index: i, value: s}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].value < items[j].value
	})

	results := make([]float64, n)
	for i := 0; i < n; {
		// Find all items with identical score to share the same rank (tie handling)
		j := i
		for j < n && items[j].value == items[i].value {
			j++
		}

		// Rank for percent_rank is 0-indexed position of the first element in tie group
		rank := float64(i)
		percentile := math.Round((rank/float64(n-1))*10000) / 100 // 2 decimal places

		for k := i; k < j; k++ {
			results[items[k].index] = percentile
		}
		i = j
	}

	return results
}

// deriveSafetyGrid computes the citywide ~500m safety grid penalty distribution
// from cpd_crimes and city_boundary, populating safety_grid_percentiles.
//
// Zero-Downtime Recompute:
//  1. Derivations and spatial aggregations are computed into an UNLOGGED staging table
//     (safety_grid_percentiles_staging) without taking locks on the active table.
//  2. An atomic transaction truncates safety_grid_percentiles, copies the derived
//     rows from staging, and drops staging in ~20ms, ensuring live queries never read
//     a partially computed distribution (ARCHITECTURE §11.3).
func deriveSafetyGrid(ctx context.Context, pool *pgxpool.Pool) error {
	slog.Info("verifying prerequisite tables for safety grid derivation")

	var boundaryCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM city_boundary WHERE name = 'Chicago'").Scan(&boundaryCount); err != nil {
		return fmt.Errorf("check city_boundary: %w", err)
	}
	if boundaryCount == 0 {
		return fmt.Errorf("city_boundary table is empty; run --dataset=boundary first")
	}

	var crimeCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM cpd_crimes").Scan(&crimeCount); err != nil {
		return fmt.Errorf("check cpd_crimes: %w", err)
	}
	if crimeCount == 0 {
		return fmt.Errorf("cpd_crimes table is empty; run --dataset=crimes first")
	}

	slog.Info("starting safety grid derivation",
		slog.Int("crimes_available", crimeCount),
		slog.Int("grid_size_meters", safetyGridCellSizeMeters),
	)

	start := time.Now()

	// Step 1: Create unlogged staging table
	const createStagingSQL = `
		DROP TABLE IF EXISTS safety_grid_percentiles_staging;
		CREATE UNLOGGED TABLE safety_grid_percentiles_staging (
			cell_geom GEOGRAPHY(POLYGON, 4326) NOT NULL,
			violent_count INTEGER NOT NULL DEFAULT 0,
			sexual_count INTEGER NOT NULL DEFAULT 0,
			property_count INTEGER NOT NULL DEFAULT 0,
			nuisance_count INTEGER NOT NULL DEFAULT 0,
			penalty_score NUMERIC NOT NULL,
			percentile NUMERIC(5, 2) NOT NULL CHECK (percentile BETWEEN 0 AND 100)
		);
	`
	if _, err := pool.Exec(ctx, createStagingSQL); err != nil {
		return fmt.Errorf("create safety_grid_percentiles_staging: %w", err)
	}

	// Step 2: Generate grid, join crimes, compute penalty scores and percentiles into staging
	deriveSQL := fmt.Sprintf(`
		INSERT INTO safety_grid_percentiles_staging (
			cell_geom, violent_count, sexual_count, property_count, nuisance_count, penalty_score, percentile
		)
		WITH boundary AS (
			SELECT ST_Transform(geom::geometry, %[1]d) AS geom_proj
			FROM city_boundary
			WHERE name = 'Chicago'
		),
		grid AS (
			SELECT (ST_SquareGrid(%[2]d, b.geom_proj)).geom AS cell_proj
			FROM boundary b
		),
		valid_cells AS (
			SELECT 
				ST_Transform(g.cell_proj, 4326)::geography AS cell_geom,
				g.cell_proj
			FROM grid g
			JOIN boundary b ON ST_Intersects(g.cell_proj, b.geom_proj)
		),
		cell_counts AS (
			SELECT
				vc.cell_geom,
				COUNT(c.id) FILTER (WHERE c.category = 'violent')::integer AS violent_count,
				COUNT(c.id) FILTER (WHERE c.category = 'sexual')::integer AS sexual_count,
				COUNT(c.id) FILTER (WHERE c.category = 'property')::integer AS property_count,
				COUNT(c.id) FILTER (WHERE c.category = 'nuisance')::integer AS nuisance_count,
				(
					5 * COUNT(c.id) FILTER (WHERE c.category = 'violent') +
					5 * COUNT(c.id) FILTER (WHERE c.category = 'sexual') +
					1 * COUNT(c.id) FILTER (WHERE c.category = 'property') +
					2 * COUNT(c.id) FILTER (WHERE c.category = 'nuisance')
				)::numeric AS penalty_score
			FROM valid_cells vc
			LEFT JOIN cpd_crimes c ON ST_Intersects(vc.cell_geom, c.geom)
			GROUP BY vc.cell_geom, vc.cell_proj
		)
		SELECT
			cell_geom,
			violent_count,
			sexual_count,
			property_count,
			nuisance_count,
			penalty_score,
			ROUND((PERCENT_RANK() OVER (ORDER BY penalty_score ASC) * 100)::numeric, 2) AS percentile
		FROM cell_counts;
	`, sridUTM16N, safetyGridCellSizeMeters)

	tag, err := pool.Exec(ctx, deriveSQL)
	if err != nil {
		return fmt.Errorf("populate safety_grid_percentiles_staging: %w", err)
	}

	cellsDerived := tag.RowsAffected()
	slog.Info("safety grid spatial computation complete",
		slog.Int64("cells_derived", cellsDerived),
		slog.Duration("compute_duration", time.Since(start)),
	)

	// Step 3: Fast Atomic Swap in a single transaction
	swapStart := time.Now()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin swap transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	const swapSQL = `
		TRUNCATE safety_grid_percentiles;
		INSERT INTO safety_grid_percentiles (
			cell_geom, violent_count, sexual_count, property_count, nuisance_count, penalty_score, percentile
		)
		SELECT cell_geom, violent_count, sexual_count, property_count, nuisance_count, penalty_score, percentile
		FROM safety_grid_percentiles_staging;
		DROP TABLE safety_grid_percentiles_staging;
	`
	if _, err := tx.Exec(ctx, swapSQL); err != nil {
		return fmt.Errorf("execute atomic swap: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit swap transaction: %w", err)
	}

	slog.Info("safety grid percentile derivation completed successfully",
		slog.Int64("total_cells", cellsDerived),
		slog.Duration("lock_duration", time.Since(swapStart)),
		slog.Duration("total_duration", time.Since(start)),
	)

	return nil
}
