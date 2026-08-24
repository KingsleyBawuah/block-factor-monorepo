package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// SafetyQuerier defines spatial queries for safety percentile distribution.
type SafetyQuerier interface {
	GetSafetyPercentile(ctx context.Context, lat, lon float64) (float64, error)
}

// DBSafetyQuerier queries safety_grid_percentiles in PostGIS.
type DBSafetyQuerier struct {
	db spatial.QueryRowProvider
}

// NewDBSafetyQuerier creates a new DBSafetyQuerier instance.
func NewDBSafetyQuerier(db spatial.QueryRowProvider) *DBSafetyQuerier {
	return &DBSafetyQuerier{db: db}
}

// GetSafetyPercentile retrieves the precomputed crime penalty percentile for the 500m grid cell covering the coordinate.
func (q *DBSafetyQuerier) GetSafetyPercentile(ctx context.Context, lat, lon float64) (float64, error) {
	if q.db == nil {
		return 0, errors.New("scoring: nil database connection")
	}

	query := `
		SELECT percentile
		FROM safety_grid_percentiles
		WHERE ST_Covers(cell_geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
		LIMIT 1;
	`

	var percentile float64
	err := q.db.QueryRow(ctx, query, lon, lat).Scan(&percentile)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Outside active crime grid bins (e.g. airport/park/zero-crime boundary slice)
			return 0.0, nil
		}
		return 0, fmt.Errorf("scoring: query safety grid percentile: %w", err)
	}

	return percentile, nil
}

// SafetyCalculator calculates the S_safety livability factor.
type SafetyCalculator struct {
	querier SafetyQuerier
}

// NewSafetyCalculator creates a new SafetyCalculator instance.
func NewSafetyCalculator(querier SafetyQuerier) *SafetyCalculator {
	return &SafetyCalculator{querier: querier}
}

// Name returns FactorSafety.
func (c *SafetyCalculator) Name() FactorName {
	return FactorSafety
}

// Calculate computes S_safety for the given origin coordinate.
func (c *SafetyCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.querier == nil {
		return FactorResult{Name: FactorSafety, Degraded: true}, errors.New("scoring: safety querier not configured")
	}

	percentile, err := c.querier.GetSafetyPercentile(ctx, params.Origin.Lat, params.Origin.Lon)
	if err != nil {
		return FactorResult{Name: FactorSafety, Degraded: true}, err
	}

	score := CalculateSafetyScore(percentile)
	return FactorResult{
		Name:     FactorSafety,
		Score:    &score,
		Degraded: false,
	}, nil
}

// CalculateSafetyScore converts a citywide crime penalty percentile into an intuitive 0-100 safety score (PRD §5.9):
//   - 0th percentile (lowest crime in Chicago) -> 100.0 / 100
//   - 25th percentile crime -> 75.0 / 100
//   - 50th percentile crime -> 50.0 / 100
//   - 90th percentile crime -> 10.0 / 100
//   - 100th percentile crime (highest crime in Chicago) -> 0.0 / 100
//   - Formula: clamp(100.0 - Percentile, 0, 100).
func CalculateSafetyScore(percentile float64) float32 {
	rawScore := 100.0 - percentile
	clamped := math.Min(100.0, math.Max(0.0, rawScore))
	return float32(math.Round(clamped*10) / 10)
}
