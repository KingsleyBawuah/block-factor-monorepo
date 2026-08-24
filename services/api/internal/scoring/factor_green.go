package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// Green space scoring thresholds and bonus constants (PRD §5.5).
const (
	// ParkMaxQueryRadiusMeters is 0.75 miles (1207.0 m).
	ParkMaxQueryRadiusMeters = 1207.008

	// ParkCloseThresholdMeters is 0.1 miles (160.9 m) for 100 pt score.
	ParkCloseThresholdMeters = 160.934
	// ParkMidThresholdMeters is 0.3 miles (482.8 m) for 50 pt score.
	ParkMidThresholdMeters = 482.803

	// ParkTrailBonus is the +15 pt boost for signature linear greenways.
	ParkTrailBonus = 15.0
	// ParkTrailThresholdMeters is 0.25 miles (402.3 m).
	ParkTrailThresholdMeters = 402.336
)

// ParkProximity represents a nearby municipal park or off-street greenway trail.
type ParkProximity struct {
	Name           string
	ParkClass      string
	Acres          *float64
	DistanceMeters float64
}

// GreenQuerier defines spatial database queries for parks.
type GreenQuerier interface {
	GetNearbyParks(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]ParkProximity, error)
}

// DBGreenQuerier queries the parks table in PostGIS.
type DBGreenQuerier struct {
	db spatial.QueryProvider
}

// NewDBGreenQuerier creates a new DBGreenQuerier instance.
func NewDBGreenQuerier(db spatial.QueryProvider) *DBGreenQuerier {
	return &DBGreenQuerier{db: db}
}

// GetNearbyParks returns public park polygons within maxDistanceMeters of the coordinate.
func (q *DBGreenQuerier) GetNearbyParks(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]ParkProximity, error) {
	if q.db == nil {
		return nil, errors.New("scoring: nil database connection")
	}

	query := `
		SELECT
			park_name,
			park_class,
			acres,
			ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance_meters
		FROM parks
		WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance_meters ASC;
	`

	rows, err := q.db.Query(ctx, query, lon, lat, maxDistanceMeters)
	if err != nil {
		return nil, fmt.Errorf("scoring: query nearby parks: %w", err)
	}
	defer rows.Close()

	var parks []ParkProximity
	for rows.Next() {
		var p ParkProximity
		if err := rows.Scan(&p.Name, &p.ParkClass, &p.Acres, &p.DistanceMeters); err != nil {
			return nil, fmt.Errorf("scoring: scan park row: %w", err)
		}
		parks = append(parks, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scoring: iterate park rows: %w", err)
	}

	return parks, nil
}

// GreenCalculator calculates the S_green livability factor.
type GreenCalculator struct {
	querier GreenQuerier
}

// NewGreenCalculator creates a new GreenCalculator instance.
func NewGreenCalculator(querier GreenQuerier) *GreenCalculator {
	return &GreenCalculator{querier: querier}
}

// Name returns FactorGreen.
func (c *GreenCalculator) Name() FactorName {
	return FactorGreen
}

// Calculate computes S_green for the given origin coordinate.
func (c *GreenCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.querier == nil {
		return FactorResult{Name: FactorGreen, Degraded: true}, errors.New("scoring: green querier not configured")
	}

	parks, err := c.querier.GetNearbyParks(ctx, params.Origin.Lat, params.Origin.Lon, ParkMaxQueryRadiusMeters)
	if err != nil {
		return FactorResult{Name: FactorGreen, Degraded: true}, err
	}

	score, minDistance := CalculateGreenScore(parks)
	return FactorResult{
		Name:           FactorGreen,
		Score:          &score,
		DistanceMeters: minDistance,
		Degraded:       false,
	}, nil
}

// CalculateGreenScore computes the green space factor based on PRD §5.5:
//   - Park Polygon Boundary Distance:
//     <= 0.1 mi (160.9 m): 100 pts
//     0.1 – 0.3 mi (160.9 – 482.8 m): linear decay from 100 to 50 pts
//     0.3 – 0.75 mi (482.8 – 1207.0 m): linear decay from 50 to 0 pts
//     > 0.75 mi: 0 pts
//   - Linear Greenway / Trail Bonus:
//     <= 0.25 mi (402.3 m) to an off-street trail or linear park (The 606, Lakefront Trail, LINEAR PARK) adds +15 pts.
//   - Formula: clamp(BaseScore + TrailBonus, 0, 100).
func CalculateGreenScore(parks []ParkProximity) (score float32, minDistance *float32) {
	if len(parks) == 0 {
		return 0.0, nil
	}

	var closestPark *ParkProximity
	hasTrailBonus := false

	for i := range parks {
		p := &parks[i]
		if closestPark == nil || p.DistanceMeters < closestPark.DistanceMeters {
			closestPark = p
		}

		if p.DistanceMeters <= ParkTrailThresholdMeters && isLinearTrail(p) {
			hasTrailBonus = true
		}
	}

	d := closestPark.DistanceMeters
	baseScore := 0.0

	switch {
	case d <= ParkCloseThresholdMeters:
		baseScore = 100.0
	case d <= ParkMidThresholdMeters:
		// Linear decay from 100 to 50 between 0.1 mi and 0.3 mi
		ratio := (d - ParkCloseThresholdMeters) / (ParkMidThresholdMeters - ParkCloseThresholdMeters)
		baseScore = 100.0 - (50.0 * ratio)
	case d <= ParkMaxQueryRadiusMeters:
		// Linear decay from 50 to 0 between 0.3 mi and 0.75 mi
		ratio := (ParkMaxQueryRadiusMeters - d) / (ParkMaxQueryRadiusMeters - ParkMidThresholdMeters)
		baseScore = 50.0 * math.Max(0.0, ratio)
	default:
		baseScore = 0.0
	}

	bonus := 0.0
	if hasTrailBonus {
		bonus = ParkTrailBonus
	}

	finalScore := math.Min(100.0, math.Max(0.0, baseScore+bonus))
	score = float32(math.Round(finalScore*10) / 10)

	distVal := float32(math.Round(d*10) / 10)
	minDistance = &distVal

	return score, minDistance
}

func isLinearTrail(p *ParkProximity) bool {
	if strings.ToUpper(p.ParkClass) == "LINEAR PARK" {
		return true
	}

	name := strings.ToLower(p.Name)
	return strings.Contains(name, "lakefront") ||
		strings.Contains(name, "606") ||
		strings.Contains(name, "bloomingdale")
}
