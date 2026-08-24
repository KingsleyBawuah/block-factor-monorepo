package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

const (
	// Sewer311BufferMeters represents 0.25 miles in meters (PRD §5.7).
	Sewer311BufferMeters = 402.336
	// Sewer311IncidentTrailingMonths defines the 24-month incident evaluation window.
	Sewer311IncidentTrailingMonths = 24
	// Sewer311PenaltyPerIncident is the point deduction per reported sewer backup.
	Sewer311PenaltyPerIncident = 5.0
)

// FloodQuerier defines spatial database queries required for the flood factor.
type FloodQuerier interface {
	GetFEMAFloodZone(ctx context.Context, lat, lon float64) (string, error)
	GetSewer311IncidentCount(ctx context.Context, lat, lon float64, radiusMeters float64, months int) (int, error)
}

// DBFloodQuerier executes flood factor queries against PostGIS.
type DBFloodQuerier struct {
	db spatial.QueryRowProvider
}

// NewDBFloodQuerier creates a DBFloodQuerier with a database query provider.
func NewDBFloodQuerier(db spatial.QueryRowProvider) *DBFloodQuerier {
	return &DBFloodQuerier{db: db}
}

// GetFEMAFloodZone finds the FEMA flood hazard zone polygon covering the given coordinate.
// If the location is outside all SFHA polygons, it defaults to Zone X (Minimal Flood Hazard).
func (q *DBFloodQuerier) GetFEMAFloodZone(ctx context.Context, lat, lon float64) (string, error) {
	if q.db == nil {
		return "", errors.New("scoring: nil database connection")
	}

	query := `
		SELECT zone_designation
		FROM fema_flood_zones
		WHERE ST_Covers(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
		LIMIT 1;
	`

	var zone string
	err := q.db.QueryRow(ctx, query, lon, lat).Scan(&zone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "X", nil
		}
		return "", fmt.Errorf("scoring: query FEMA flood zone: %w", err)
	}

	return zone, nil
}

// GetSewer311IncidentCount counts water in basement complaints within radiusMeters over trailing months.
func (q *DBFloodQuerier) GetSewer311IncidentCount(ctx context.Context, lat, lon float64, radiusMeters float64, months int) (int, error) {
	if q.db == nil {
		return 0, errors.New("scoring: nil database connection")
	}

	query := `
		SELECT count(*)
		FROM sewer_311_incidents
		WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		  AND incident_date >= now() - ($4 * INTERVAL '1 month');
	`

	var count int
	err := q.db.QueryRow(ctx, query, lon, lat, radiusMeters, months).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("scoring: query 311 sewer incidents: %w", err)
	}

	return count, nil
}

// FloodCalculator calculates the S_flood livability factor.
type FloodCalculator struct {
	querier FloodQuerier
}

// NewFloodCalculator creates a new FloodCalculator instance.
func NewFloodCalculator(querier FloodQuerier) *FloodCalculator {
	return &FloodCalculator{querier: querier}
}

// Name returns FactorFlood.
func (c *FloodCalculator) Name() FactorName {
	return FactorFlood
}

// Calculate computes S_flood for the given coordinates.
func (c *FloodCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.querier == nil {
		return FactorResult{Name: FactorFlood, Degraded: true}, errors.New("scoring: flood querier not configured")
	}

	zone, err := c.querier.GetFEMAFloodZone(ctx, params.Origin.Lat, params.Origin.Lon)
	if err != nil {
		return FactorResult{Name: FactorFlood, Degraded: true}, err
	}

	incidentCount, err := c.querier.GetSewer311IncidentCount(
		ctx,
		params.Origin.Lat,
		params.Origin.Lon,
		Sewer311BufferMeters,
		Sewer311IncidentTrailingMonths,
	)
	if err != nil {
		return FactorResult{Name: FactorFlood, Degraded: true}, err
	}

	score := CalculateFloodScore(zone, incidentCount)
	return FactorResult{
		Name:     FactorFlood,
		Score:    &score,
		Degraded: false,
	}, nil
}

// CalculateFloodScore computes the flood score from the FEMA zone designation and 311 incident count.
// Base score:
//   - Zone X (Minimal Hazard): 100 pts
//   - Zone A/AE/AH/AO/AR/A99/V/VE (100-Year SFHA): 40 pts
//   - Zone D (Undetermined): 70 pts
//
// Penalty:
//   - -5 pts per 311 basement backup incident within 0.25 miles over trailing 24 months.
//
// Formula: max(0, BaseScore - (5 × IncidentCount)).
func CalculateFloodScore(zoneDesignation string, sewerIncidentCount int) float32 {
	var baseScore float64
	switch strings.ToUpper(strings.TrimSpace(zoneDesignation)) {
	case "A", "AE", "AH", "AO", "AR", "A99", "V", "VE":
		baseScore = 40.0
	case "D":
		baseScore = 70.0
	case "X":
		baseScore = 100.0
	default:
		baseScore = 100.0
	}

	penalty := float64(sewerIncidentCount) * Sewer311PenaltyPerIncident
	finalScore := math.Max(0.0, baseScore-penalty)
	return float32(math.Round(finalScore*10) / 10)
}
