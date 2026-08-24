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

	// TransitDualLineBonusPoints is the +10 pt boost for dual-line 'L' access.
	TransitDualLineBonusPoints = 10.0

	// TransitBusHeadwayThresholdMinutes defines the <= 10 min headway benchmark for high frequency.
	TransitBusHeadwayThresholdMinutes = 10.0

	// TransitBusOneAxisBonusPoints is the +10 pt boost for a qualifying high-frequency
	// bus corridor in only one direction (N-S or E-W).
	TransitBusOneAxisBonusPoints = 10.0
	// TransitBusBothAxesBonusPoints is the +15 pt boost when qualifying corridors cover
	// both N-S and E-W.
	TransitBusBothAxesBonusPoints = 15.0

	// Rail service quality (0-20 pts total): AMPeakComponent + OvernightComponent.
	// TransitRailAMPeakMaxPoints is awarded at <= TransitRailAMPeakCloseMinutes headway,
	// decaying to 0 at >= TransitRailAMPeakFarMinutes.
	TransitRailAMPeakMaxPoints    = 10.0
	TransitRailAMPeakCloseMinutes = 6.0
	TransitRailAMPeakFarMinutes   = 20.0

	// TransitRailOvernightMaxPoints is awarded at <= TransitRailOvernightCloseMinutes
	// headway, decaying to 0 at >= TransitRailOvernightFarMinutes (matching CTA's own
	// 30-min service standard for non-Key routes at peak, per PRD §5.2).
	TransitRailOvernightMaxPoints    = 10.0
	TransitRailOvernightCloseMinutes = 15.0
	TransitRailOvernightFarMinutes   = 30.0
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
	// AzimuthDegrees is nil when the corridor's start and end points coincide
	// (e.g. a loop), which leaves the bearing undefined.
	AzimuthDegrees *float64
}

// CtaLineService holds a CTA 'L' line's scheduled AM-peak and overnight headway,
// powering the S_transit rail service-quality bonus (PRD §5.2).
type CtaLineService struct {
	LineName                string
	AMPeakHeadwayMinutes    float64
	OvernightHeadwayMinutes *float64
}

// TransitQuerier defines spatial database queries for CTA stops, bus corridors, and
// per-line service quality.
type TransitQuerier interface {
	GetNearbyCtaStations(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]CtaStation, error)
	GetNearbyBusCorridors(ctx context.Context, lat, lon float64, maxDistanceMeters float64, maxHeadwayMinutes float64) ([]BusCorridor, error)
	GetLineService(ctx context.Context) ([]CtaLineService, error)
}

// DBTransitQuerier queries cta_stops, bus_corridors, and cta_line_service in PostGIS.
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
			ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance_meters,
			degrees(ST_Azimuth(
				ST_StartPoint(ST_GeometryN(geom::geometry, 1)),
				ST_EndPoint(ST_GeometryN(geom::geometry, 1))
			)) AS azimuth_deg
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
		if err := rows.Scan(&b.RouteID, &b.RouteName, &b.MeanHeadwayMinutes, &b.DistanceMeters, &b.AzimuthDegrees); err != nil {
			return nil, fmt.Errorf("scoring: scan bus corridor row: %w", err)
		}
		corridors = append(corridors, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scoring: iterate bus corridor rows: %w", err)
	}

	return corridors, nil
}

// GetLineService returns every CTA 'L' line's scheduled AM-peak and overnight headway.
func (q *DBTransitQuerier) GetLineService(ctx context.Context) ([]CtaLineService, error) {
	if q.db == nil {
		return nil, errors.New("scoring: nil database connection")
	}

	query := `SELECT line_name, am_peak_headway_minutes, overnight_headway_minutes FROM cta_line_service;`

	rows, err := q.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("scoring: query cta line service: %w", err)
	}
	defer rows.Close()

	var lines []CtaLineService
	for rows.Next() {
		var l CtaLineService
		if err := rows.Scan(&l.LineName, &l.AMPeakHeadwayMinutes, &l.OvernightHeadwayMinutes); err != nil {
			return nil, fmt.Errorf("scoring: scan cta line service row: %w", err)
		}
		lines = append(lines, l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scoring: iterate cta line service rows: %w", err)
	}

	return lines, nil
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

	// No stations within range means no line is reachable, so the rail quality
	// bonus is unconditionally 0 -- skip the query rather than fetch data that
	// can't affect the result. cta_stops.lines is always non-empty for a real
	// station, so len(stations) > 0 is an exact proxy for "some line is reachable".
	var lineServices []CtaLineService
	if len(stations) > 0 {
		lineServices, err = c.querier.GetLineService(ctx)
		if err != nil {
			return FactorResult{Name: FactorTransit, Degraded: true}, err
		}
	}

	score, minDistance, contributing := CalculateTransitScore(stations, corridors, lineServices)
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
//   - Three independent, additive bonus components:
//     1. Dual-line access: +10 pts if 2+ distinct 'L' lines are reachable across
//     stations within 1.0 mi.
//     2. Bus directional coverage: +10 pts for a qualifying high-frequency
//     (<= 10 min headway) bus corridor within 0.2 mi in one direction only,
//     +15 pts if corridors cover both N-S and E-W.
//     3. Rail service quality (0-20 pts): AMPeakComponent + OvernightComponent for
//     the best-scoring line among all lines reachable within 1.0 mi (not
//     necessarily the closest station).
//   - Formula: clamp(BaseScore + DualLineBonus + BusBonus + RailQualityBonus, 0, 100).
func CalculateTransitScore(stations []CtaStation, corridors []BusCorridor, lineServices []CtaLineService) (score float32, minDistance *float32, contributing []api.ContributingPoi) {
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

	dualLineBonus := 0.0
	if len(uniqueLines) >= 2 {
		dualLineBonus = TransitDualLineBonusPoints
	}

	busBonus, qualifyingBusCorridor := calculateBusDirectionalBonus(corridors)

	railQualityBonus := calculateRailQualityBonus(uniqueLines, lineServices)

	finalScore := math.Min(100.0, math.Max(0.0, baseScore+dualLineBonus+busBonus+railQualityBonus))
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

// calculateBusDirectionalBonus returns the bus bonus points and the closest qualifying
// corridor (for the contributing-POI display), based on how many distinct axes (N-S,
// E-W) the qualifying high-frequency corridors cover. The caller's query already
// filters to the bonus radius/headway threshold, but the distance/headway check is
// re-applied here defensively so this function stays correct independent of the
// caller, matching the original single-bonus implementation's behavior.
func calculateBusDirectionalBonus(corridors []BusCorridor) (bonus float64, closest *BusCorridor) {
	hasNorthSouth := false
	hasEastWest := false

	for i := range corridors {
		c := &corridors[i]
		if c.DistanceMeters > TransitBonusRadiusThresholdMeters || c.MeanHeadwayMinutes > TransitBusHeadwayThresholdMinutes {
			continue
		}
		if closest == nil || c.DistanceMeters < closest.DistanceMeters {
			closest = c
		}
		if c.AzimuthDegrees != nil {
			if isNorthSouthAzimuth(*c.AzimuthDegrees) {
				hasNorthSouth = true
			} else {
				hasEastWest = true
			}
		}
	}

	switch {
	case hasNorthSouth && hasEastWest:
		return TransitBusBothAxesBonusPoints, closest
	case closest != nil:
		return TransitBusOneAxisBonusPoints, closest
	default:
		return 0.0, nil
	}
}

// calculateRailQualityBonus returns the best (AMPeakComponent + OvernightComponent)
// score among lines, per the "best-scoring accessible line" rule in PRD §5.2 --
// not an average across lines, and not tied to the physically closest station.
func calculateRailQualityBonus(lines map[string]struct{}, lineServices []CtaLineService) float64 {
	if len(lines) == 0 || len(lineServices) == 0 {
		return 0.0
	}

	serviceByLine := make(map[string]CtaLineService, len(lineServices))
	for _, ls := range lineServices {
		serviceByLine[ls.LineName] = ls
	}

	best := 0.0
	for line := range lines {
		svc, ok := serviceByLine[line]
		if !ok {
			continue
		}
		quality := railAMPeakComponent(svc.AMPeakHeadwayMinutes) + railOvernightComponent(svc.OvernightHeadwayMinutes)
		if quality > best {
			best = quality
		}
	}
	return best
}

// railAMPeakComponent computes the 0-10 pt AM-peak sub-score for a line's headway.
func railAMPeakComponent(headwayMinutes float64) float64 {
	switch {
	case headwayMinutes <= TransitRailAMPeakCloseMinutes:
		return TransitRailAMPeakMaxPoints
	case headwayMinutes >= TransitRailAMPeakFarMinutes:
		return 0.0
	default:
		ratio := (TransitRailAMPeakFarMinutes - headwayMinutes) / (TransitRailAMPeakFarMinutes - TransitRailAMPeakCloseMinutes)
		return TransitRailAMPeakMaxPoints * ratio
	}
}

// railOvernightComponent computes the 0-10 pt overnight sub-score for a line's
// headway. A nil headway (no scheduled overnight service at all) scores 0.
func railOvernightComponent(headwayMinutes *float64) float64 {
	if headwayMinutes == nil {
		return 0.0
	}
	h := *headwayMinutes
	switch {
	case h <= TransitRailOvernightCloseMinutes:
		return TransitRailOvernightMaxPoints
	case h >= TransitRailOvernightFarMinutes:
		return 0.0
	default:
		ratio := (TransitRailOvernightFarMinutes - h) / (TransitRailOvernightFarMinutes - TransitRailOvernightCloseMinutes)
		return TransitRailOvernightMaxPoints * ratio
	}
}
