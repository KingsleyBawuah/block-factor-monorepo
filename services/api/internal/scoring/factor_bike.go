package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// Bike scoring thresholds and network bonus constants (PRD §5.11).
const (
	// BikeMaxQueryRadiusMeters is 0.5 miles (804.672 m).
	BikeMaxQueryRadiusMeters = 804.672
	// BikeCloseThresholdMeters is 0.1 miles (160.934 m) for full tier score.
	BikeCloseThresholdMeters = 160.934

	// BikeTierProtectedScore is 100 pts for Protected Bike Lanes and Neighborhood Greenways.
	BikeTierProtectedScore = 100.0
	// BikeTierBufferedScore is 70 pts for Buffered Bike Lanes.
	BikeTierBufferedScore = 70.0
	// BikeTierStandardScore is 40 pts for painted on-street Bike Lanes.
	BikeTierStandardScore = 40.0
	// BikeTierSharedScore is 15 pts for Marked Shared Lanes / Sharrows.
	BikeTierSharedScore = 15.0

	// BikeNetworkBonus is the +15 pt boost for dual-axis (N-S and E-W) connectivity.
	BikeNetworkBonus = 15.0
)

// BikeRouteProximity represents a nearby bike route segment and its bearing.
type BikeRouteProximity struct {
	RouteType      string // 'Protected Bike Lane', 'Neighborhood Greenway', etc.
	StreetName     *string
	DistanceMeters float64
	AzimuthDegrees float64
}

// BikeQuerier defines spatial database queries for bike infrastructure.
type BikeQuerier interface {
	GetNearbyBikeRoutes(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]BikeRouteProximity, error)
}

// DBBikeQuerier queries bike_routes in PostGIS.
type DBBikeQuerier struct {
	db spatial.QueryProvider
}

// NewDBBikeQuerier creates a new DBBikeQuerier instance.
func NewDBBikeQuerier(db spatial.QueryProvider) *DBBikeQuerier {
	return &DBBikeQuerier{db: db}
}

// GetNearbyBikeRoutes returns bike route segments within maxDistanceMeters of the coordinate.
func (q *DBBikeQuerier) GetNearbyBikeRoutes(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]BikeRouteProximity, error) {
	if q.db == nil {
		return nil, errors.New("scoring: nil database connection")
	}

	query := `
		SELECT
			route_type,
			street_name,
			ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance_meters,
			COALESCE(degrees(ST_Azimuth(
				ST_StartPoint(ST_GeometryN(geom::geometry, 1)),
				ST_EndPoint(ST_GeometryN(geom::geometry, 1))
			)), 0.0) AS azimuth_deg
		FROM bike_routes
		WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance_meters ASC;
	`

	rows, err := q.db.Query(ctx, query, lon, lat, maxDistanceMeters)
	if err != nil {
		return nil, fmt.Errorf("scoring: query nearby bike routes: %w", err)
	}
	defer rows.Close()

	var routes []BikeRouteProximity
	for rows.Next() {
		var r BikeRouteProximity
		if err := rows.Scan(&r.RouteType, &r.StreetName, &r.DistanceMeters, &r.AzimuthDegrees); err != nil {
			return nil, fmt.Errorf("scoring: scan bike route row: %w", err)
		}
		routes = append(routes, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scoring: iterate bike route rows: %w", err)
	}

	return routes, nil
}

// BikeCalculator calculates the S_bike livability factor.
type BikeCalculator struct {
	querier BikeQuerier
}

// NewBikeCalculator creates a new BikeCalculator instance.
func NewBikeCalculator(querier BikeQuerier) *BikeCalculator {
	return &BikeCalculator{querier: querier}
}

// Name returns FactorBike.
func (c *BikeCalculator) Name() FactorName {
	return FactorBike
}

// Calculate computes S_bike for the given origin coordinate.
func (c *BikeCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.querier == nil {
		return FactorResult{Name: FactorBike, Degraded: true}, errors.New("scoring: bike querier not configured")
	}

	routes, err := c.querier.GetNearbyBikeRoutes(ctx, params.Origin.Lat, params.Origin.Lon, BikeMaxQueryRadiusMeters)
	if err != nil {
		return FactorResult{Name: FactorBike, Degraded: true}, err
	}

	score, minDistance := CalculateBikeScore(routes)
	return FactorResult{
		Name:           FactorBike,
		Score:          &score,
		DistanceMeters: minDistance,
		Degraded:       false,
	}, nil
}

// CalculateBikeScore computes the cycling infrastructure score based on PRD §5.11:
//   - Tier Base Scores (at <= 0.1 mi / 160.9 m):
//     Protected Bike Lane / Neighborhood Greenway: 100 pts
//     Buffered Bike Lane: 70 pts
//     Bike Lane (paint only): 40 pts
//     Marked Shared Lane / Sharrow: 15 pts
//   - Linear distance decay to 0 pts at 0.5 mi (804.7 m).
//   - Network Bonus (+15 pts):
//     Qualifying infrastructure (Buffered Bike Lane or better) exists in both North-South
//     and East-West directions within 0.5 mi.
//   - Formula: clamp(BestSegmentScore + NetworkBonus, 0, 100).
func CalculateBikeScore(routes []BikeRouteProximity) (score float32, minDistance *float32) {
	if len(routes) == 0 {
		return 0.0, nil
	}

	bestBaseScore := 0.0
	var closestDist *float64
	hasNorthSouth := false
	hasEastWest := false

	for i := range routes {
		r := &routes[i]
		if closestDist == nil || r.DistanceMeters < *closestDist {
			d := r.DistanceMeters
			closestDist = &d
		}

		tierScore := getBikeTierScore(r.RouteType)
		segScore := computeDecayedScore(tierScore, r.DistanceMeters)
		if segScore > bestBaseScore {
			bestBaseScore = segScore
		}

		// Qualifying network connectivity: Buffered Bike Lane or better within 0.5 mi
		if tierScore >= BikeTierBufferedScore && r.DistanceMeters <= BikeMaxQueryRadiusMeters {
			if isNorthSouthAzimuth(r.AzimuthDegrees) {
				hasNorthSouth = true
			} else {
				hasEastWest = true
			}
		}
	}

	bonus := 0.0
	if hasNorthSouth && hasEastWest {
		bonus = BikeNetworkBonus
	}

	finalScore := math.Min(100.0, math.Max(0.0, bestBaseScore+bonus))
	score = float32(math.Round(finalScore*10) / 10)

	if closestDist != nil {
		distVal := float32(math.Round(*closestDist*10) / 10)
		minDistance = &distVal
	}

	return score, minDistance
}

func getBikeTierScore(routeType string) float64 {
	clean := strings.ToLower(strings.TrimSpace(routeType))
	switch clean {
	case "protected bike lane", "neighborhood greenway":
		return BikeTierProtectedScore
	case "buffered bike lane":
		return BikeTierBufferedScore
	case "bike lane":
		return BikeTierStandardScore
	case "marked shared lane":
		return BikeTierSharedScore
	default:
		return 0.0
	}
}

func computeDecayedScore(baseScore float64, distanceMeters float64) float64 {
	if distanceMeters <= BikeCloseThresholdMeters {
		return baseScore
	}
	if distanceMeters >= BikeMaxQueryRadiusMeters {
		return 0.0
	}

	// Linear decay from baseScore at 0.1 mi to 0 at 0.5 mi
	ratio := (BikeMaxQueryRadiusMeters - distanceMeters) / (BikeMaxQueryRadiusMeters - BikeCloseThresholdMeters)
	return baseScore * math.Max(0.0, ratio)
}

func isNorthSouthAzimuth(deg float64) bool {
	// Normalize to [0, 360)
	d := math.Mod(deg, 360)
	if d < 0 {
		d += 360
	}

	// North-South is within ±45° of 0° (due North) or 180° (due South)
	return (d >= 315 || d <= 45) || (d >= 135 && d <= 225)
}
