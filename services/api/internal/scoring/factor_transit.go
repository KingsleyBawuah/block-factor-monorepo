package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// Transit scoring thresholds and bonus constants (PRD §5.2).
const (
	// TransitMaxQueryRadiusMeters is 1.0 mile (1609.344 m).
	TransitMaxQueryRadiusMeters = 1609.344

	// TransitStationCloseThresholdMeters is 0.25 miles (402.336 m) for 100 pt score.
	TransitStationCloseThresholdMeters = 402.336

	// TransitBonusRadiusThresholdMeters is 0.2 miles (321.869 m) for high-frequency bus proximity.
	TransitBonusRadiusThresholdMeters = 321.869

	// TransitBonusPoints is the +10 pt boost for dual-line access or high-frequency bus service.
	TransitBonusPoints = 10.0

	// TransitBusHeadwayThresholdMinutes defines the <= 10 min headway benchmark for high frequency.
	TransitBusHeadwayThresholdMinutes = 10.0
)

// CtaStation represents a nearby CTA 'L' rapid-transit station.
type CtaStation struct {
	StopName       string
	Lines          []string
	DistanceMeters float64
}

// BusCorridor represents a high-frequency CTA bus route corridor.
type BusCorridor struct {
	RouteID            string
	RouteName          string
	MeanHeadwayMinutes float64
	DistanceMeters     float64
}

// TransitQuerier defines spatial database queries for CTA stops and bus corridors.
type TransitQuerier interface {
	GetNearbyCtaStations(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]CtaStation, error)
	GetNearbyBusCorridors(ctx context.Context, lat, lon float64, maxDistanceMeters float64, maxHeadwayMinutes float64) ([]BusCorridor, error)
}

// DBTransitQuerier queries cta_stops and bus_corridors in PostGIS.
type DBTransitQuerier struct {
	db spatial.QueryProvider
}

// NewDBTransitQuerier creates a new DBTransitQuerier instance.
func NewDBTransitQuerier(db spatial.QueryProvider) *DBTransitQuerier {
	return &DBTransitQuerier{db: db}
}

// GetNearbyCtaStations returns CTA 'L' stations within maxDistanceMeters of the coordinate.
func (q *DBTransitQuerier) GetNearbyCtaStations(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]CtaStation, error) {
	if q.db == nil {
		return nil, errors.New("scoring: nil database connection")
	}

	query := `
		SELECT
			stop_name,
			lines,
			ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance_meters
		FROM cta_stops
		WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance_meters ASC;
	`

	rows, err := q.db.Query(ctx, query, lon, lat, maxDistanceMeters)
	if err != nil {
		return nil, fmt.Errorf("scoring: query nearby cta stations: %w", err)
	}
	defer rows.Close()

	var stations []CtaStation
	for rows.Next() {
		var s CtaStation
		if err := rows.Scan(&s.StopName, &s.Lines, &s.DistanceMeters); err != nil {
			return nil, fmt.Errorf("scoring: scan cta station row: %w", err)
		}
		stations = append(stations, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scoring: iterate cta station rows: %w", err)
	}

	return stations, nil
}

// GetNearbyBusCorridors returns bus corridors with mean headway <= maxHeadwayMinutes within maxDistanceMeters.
func (q *DBTransitQuerier) GetNearbyBusCorridors(ctx context.Context, lat, lon float64, maxDistanceMeters float64, maxHeadwayMinutes float64) ([]BusCorridor, error) {
	if q.db == nil {
		return nil, errors.New("scoring: nil database connection")
	}

	query := `
		SELECT
			route_id,
			route_name,
			mean_headway_minutes,
			ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance_meters
		FROM bus_corridors
		WHERE mean_headway_minutes <= $4
		  AND ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance_meters ASC;
	`

	rows, err := q.db.Query(ctx, query, lon, lat, maxDistanceMeters, maxHeadwayMinutes)
	if err != nil {
		return nil, fmt.Errorf("scoring: query nearby bus corridors: %w", err)
	}
	defer rows.Close()

	var corridors []BusCorridor
	for rows.Next() {
		var b BusCorridor
		if err := rows.Scan(&b.RouteID, &b.RouteName, &b.MeanHeadwayMinutes, &b.DistanceMeters); err != nil {
			return nil, fmt.Errorf("scoring: scan bus corridor row: %w", err)
		}
		corridors = append(corridors, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scoring: iterate bus corridor rows: %w", err)
	}

	return corridors, nil
}

// TransitCalculator calculates the S_transit livability factor.
type TransitCalculator struct {
	querier TransitQuerier
}

// NewTransitCalculator creates a new TransitCalculator instance.
func NewTransitCalculator(querier TransitQuerier) *TransitCalculator {
	return &TransitCalculator{querier: querier}
}

// Name returns FactorTransit.
func (c *TransitCalculator) Name() FactorName {
	return FactorTransit
}

// Calculate computes S_transit for the given origin coordinate.
func (c *TransitCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.querier == nil {
		return FactorResult{Name: FactorTransit, Degraded: true}, errors.New("scoring: transit querier not configured")
	}

	stations, err := c.querier.GetNearbyCtaStations(ctx, params.Origin.Lat, params.Origin.Lon, TransitMaxQueryRadiusMeters)
	if err != nil {
		return FactorResult{Name: FactorTransit, Degraded: true}, err
	}

	corridors, err := c.querier.GetNearbyBusCorridors(ctx, params.Origin.Lat, params.Origin.Lon, TransitBonusRadiusThresholdMeters, TransitBusHeadwayThresholdMinutes)
	if err != nil {
		return FactorResult{Name: FactorTransit, Degraded: true}, err
	}

	score, minDistance, contributing := CalculateTransitScore(stations, corridors)
	return FactorResult{
		Name:             FactorTransit,
		Score:            &score,
		DistanceMeters:   minDistance,
		ContributingPois: contributing,
		Degraded:         false,
	}, nil
}

// CalculateTransitScore computes the transit access score based on PRD §5.2:
//   - CTA 'L' Station Distance:
//     <= 0.25 mi (402.336 m): 100 pts
//     0.25 – 1.0 mi (402.336 – 1609.344 m): linear decay from 100 to 0 pts
//     > 1.0 mi: 0 pts
//   - Bonus (+10 pts, capped at 100 total):
//     Dual-line access (2+ distinct CTA lines across accessible stations within 1.0 mi)
//     OR high-frequency bus corridor (mean headway <= 10 min) within <= 0.2 mi (321.869 m).
//   - Formula: clamp(BaseScore + Bonus, 0, 100).
func CalculateTransitScore(stations []CtaStation, corridors []BusCorridor) (score float32, minDistance *float32, contributing []api.ContributingPoi) {
	var closestStation *CtaStation
	uniqueLines := make(map[string]struct{})

	for i := range stations {
		s := &stations[i]
		if closestStation == nil || s.DistanceMeters < closestStation.DistanceMeters {
			closestStation = s
		}
		for _, line := range s.Lines {
			cleanLine := strings.TrimSpace(line)
			if cleanLine != "" {
				uniqueLines[cleanLine] = struct{}{}
			}
		}
	}

	baseScore := 0.0
	if closestStation != nil {
		d := closestStation.DistanceMeters
		switch {
		case d <= TransitStationCloseThresholdMeters:
			baseScore = 100.0
		case d <= TransitMaxQueryRadiusMeters:
			ratio := (TransitMaxQueryRadiusMeters - d) / (TransitMaxQueryRadiusMeters - TransitStationCloseThresholdMeters)
			baseScore = 100.0 * math.Max(0.0, ratio)
		default:
			baseScore = 0.0
		}
	}

	hasDualLineBonus := len(uniqueLines) >= 2
	var qualifyingBusCorridor *BusCorridor
	for i := range corridors {
		c := &corridors[i]
		if c.DistanceMeters <= TransitBonusRadiusThresholdMeters && c.MeanHeadwayMinutes <= TransitBusHeadwayThresholdMinutes {
			if qualifyingBusCorridor == nil || c.DistanceMeters < qualifyingBusCorridor.DistanceMeters {
				qualifyingBusCorridor = c
			}
		}
	}
	hasBusBonus := qualifyingBusCorridor != nil

	bonus := 0.0
	if hasDualLineBonus || hasBusBonus {
		bonus = TransitBonusPoints
	}

	finalScore := math.Min(100.0, math.Max(0.0, baseScore+bonus))
	score = float32(math.Round(finalScore*10) / 10)

	// Determine minDistance and contributing POIs
	if closestStation != nil {
		distVal := float32(math.Round(closestStation.DistanceMeters*10) / 10)
		minDistance = &distVal

		var name string
		if len(closestStation.Lines) > 0 {
			name = fmt.Sprintf("%s ('L' %s)", closestStation.StopName, strings.Join(closestStation.Lines, ", "))
		} else {
			name = fmt.Sprintf("%s ('L' Station)", closestStation.StopName)
		}

		contributing = append(contributing, api.ContributingPoi{
			Name:           name,
			DistanceMeters: distVal,
		})
	}

	if qualifyingBusCorridor != nil {
		busDistVal := float32(math.Round(qualifyingBusCorridor.DistanceMeters*10) / 10)
		if minDistance == nil || busDistVal < *minDistance {
			minDistance = &busDistVal
		}

		busName := qualifyingBusCorridor.RouteName
		if busName == "" {
			busName = fmt.Sprintf("Route %s Bus", qualifyingBusCorridor.RouteID)
		} else {
			busName = fmt.Sprintf("Route %s - %s (Bus)", qualifyingBusCorridor.RouteID, qualifyingBusCorridor.RouteName)
		}

		contributing = append(contributing, api.ContributingPoi{
			Name:           busName,
			DistanceMeters: busDistVal,
		})
	}

	return score, minDistance, contributing
}
