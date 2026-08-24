package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// Grocery scoring thresholds and bonus constants (PRD §5.3).
const (
	// GroceryMaxQueryRadiusMeters represents 1.0 mile (1609.344 m).
	GroceryMaxQueryRadiusMeters = 1609.344

	// GrocerySupermarketCloseThresholdMeters is 0.3 miles (482.8 m) for 100 pt score.
	GrocerySupermarketCloseThresholdMeters = 482.803
	// GrocerySupermarketMidThresholdMeters is 0.75 miles (1207.0 m) for 75 pt score.
	GrocerySupermarketMidThresholdMeters = 1207.008

	// GroceryConvenienceMaxScore is the maximum score cap for corner stores / bodegas.
	GroceryConvenienceMaxScore = 40.0
	// GroceryConvenienceThresholdMeters is 0.3 miles (482.8 m).
	GroceryConvenienceThresholdMeters = 482.803

	// GroceryPharmacyBonus is the +10 pt boost for a nearby drugstore / pharmacy.
	GroceryPharmacyBonus = 10.0
	// GroceryPharmacyThresholdMeters is 0.2 miles (321.87 m).
	GroceryPharmacyThresholdMeters = 321.869
)

// OsmPoi represents a retail grocery, convenience, or pharmacy location.
type OsmPoi struct {
	Name           string
	PoiType        string // 'supermarket', 'convenience', 'pharmacy'
	DistanceMeters float64
}

// GroceryQuerier defines spatial database queries for grocery POIs.
type GroceryQuerier interface {
	GetNearbyGroceryPois(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]OsmPoi, error)
}

// DBGroceryQuerier queries osm_pois in PostGIS.
type DBGroceryQuerier struct {
	db spatial.QueryProvider
}

// NewDBGroceryQuerier creates a new DBGroceryQuerier instance.
func NewDBGroceryQuerier(db spatial.QueryProvider) *DBGroceryQuerier {
	return &DBGroceryQuerier{db: db}
}

// GetNearbyGroceryPois returns grocery, convenience, and pharmacy POIs within maxDistanceMeters.
func (q *DBGroceryQuerier) GetNearbyGroceryPois(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]OsmPoi, error) {
	if q.db == nil {
		return nil, errors.New("scoring: nil database connection")
	}

	query := `
		SELECT
			COALESCE(poi_name, ''),
			poi_type,
			ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance_meters
		FROM osm_pois
		WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance_meters ASC;
	`

	rows, err := q.db.Query(ctx, query, lon, lat, maxDistanceMeters)
	if err != nil {
		return nil, fmt.Errorf("scoring: query grocery pois: %w", err)
	}
	defer rows.Close()

	var pois []OsmPoi
	for rows.Next() {
		var p OsmPoi
		if err := rows.Scan(&p.Name, &p.PoiType, &p.DistanceMeters); err != nil {
			return nil, fmt.Errorf("scoring: scan grocery poi row: %w", err)
		}
		pois = append(pois, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scoring: iterate grocery poi rows: %w", err)
	}

	return pois, nil
}

// GroceryCalculator calculates the S_grocery livability factor.
type GroceryCalculator struct {
	querier GroceryQuerier
}

// NewGroceryCalculator creates a new GroceryCalculator.
func NewGroceryCalculator(querier GroceryQuerier) *GroceryCalculator {
	return &GroceryCalculator{querier: querier}
}

// Name returns FactorGrocery.
func (c *GroceryCalculator) Name() FactorName {
	return FactorGrocery
}

// Calculate computes S_grocery for the given origin coordinate.
func (c *GroceryCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.querier == nil {
		return FactorResult{Name: FactorGrocery, Degraded: true}, errors.New("scoring: grocery querier not configured")
	}

	pois, err := c.querier.GetNearbyGroceryPois(ctx, params.Origin.Lat, params.Origin.Lon, GroceryMaxQueryRadiusMeters)
	if err != nil {
		return FactorResult{Name: FactorGrocery, Degraded: true}, err
	}

	score, minDistance, contributing := CalculateGroceryScore(pois)
	return FactorResult{
		Name:             FactorGrocery,
		Score:            &score,
		DistanceMeters:   minDistance,
		ContributingPois: contributing,
		Degraded:         false,
	}, nil
}

// CalculateGroceryScore computes the grocery livability factor based on PRD §5.3:
//   - Supermarket Distance:
//     <= 0.3 mi (482.8 m): 100 pts
//     0.3 – 0.75 mi (482.8 – 1207.0 m): 75 pts
//     0.75 – 1.0 mi (1207.0 – 1609.3 m): linear decay from 75 to 0 pts
//     > 1.0 mi: 0 pts
//   - Convenience / Corner Store fallback: <= 0.3 mi capped at 40 pts.
//   - Pharmacy Bonus: Pharmacy within <= 0.2 mi (321.87 m) adds +10 pts boost.
//   - Formula: clamp(BaseScore + PharmacyBonus, 0, 100).
func CalculateGroceryScore(pois []OsmPoi) (score float32, minDistance *float32, contributing []api.ContributingPoi) {
	var closestSupermarket *OsmPoi
	var closestConvenience *OsmPoi
	var closestPharmacy *OsmPoi

	for i := range pois {
		p := &pois[i]
		switch p.PoiType {
		case "supermarket":
			if closestSupermarket == nil || p.DistanceMeters < closestSupermarket.DistanceMeters {
				closestSupermarket = p
			}
		case "convenience":
			if closestConvenience == nil || p.DistanceMeters < closestConvenience.DistanceMeters {
				closestConvenience = p
			}
		case "pharmacy":
			if closestPharmacy == nil || p.DistanceMeters < closestPharmacy.DistanceMeters {
				closestPharmacy = p
			}
		}
	}

	supermarketScore := 0.0
	if closestSupermarket != nil {
		d := closestSupermarket.DistanceMeters
		switch {
		case d <= GrocerySupermarketCloseThresholdMeters:
			supermarketScore = 100.0
		case d <= GrocerySupermarketMidThresholdMeters:
			supermarketScore = 75.0
		case d <= GroceryMaxQueryRadiusMeters:
			// Linear decay from 75 to 0 between 0.75 mi and 1.0 mi
			decayRatio := (GroceryMaxQueryRadiusMeters - d) / (GroceryMaxQueryRadiusMeters - GrocerySupermarketMidThresholdMeters)
			supermarketScore = 75.0 * math.Max(0.0, decayRatio)
		}
	}

	convenienceScore := 0.0
	if closestConvenience != nil && closestConvenience.DistanceMeters <= GroceryConvenienceThresholdMeters {
		convenienceScore = GroceryConvenienceMaxScore
	}

	// The convenience-store cap is an independent rule (PRD §5.3), not a
	// fallback gated on the total absence of a supermarket: a very close
	// bodega can outscore a distant, heavily-decayed supermarket.
	baseScore := supermarketScore
	primaryPoi := closestSupermarket
	if convenienceScore > baseScore {
		baseScore = convenienceScore
		primaryPoi = closestConvenience
	}
	if baseScore == 0 {
		primaryPoi = nil
	}

	pharmacyBoost := 0.0
	if closestPharmacy != nil && closestPharmacy.DistanceMeters <= GroceryPharmacyThresholdMeters {
		pharmacyBoost = GroceryPharmacyBonus
	}

	finalScore := math.Min(100.0, math.Max(0.0, baseScore+pharmacyBoost))
	score = float32(math.Round(finalScore*10) / 10)

	var primaryDist *float64
	if primaryPoi != nil {
		primaryDist = &primaryPoi.DistanceMeters
	} else if pharmacyBoost > 0 {
		primaryDist = &closestPharmacy.DistanceMeters
	}
	if primaryDist != nil {
		distVal := float32(math.Round(*primaryDist*10) / 10)
		minDistance = &distVal
	}

	// Only list a POI as contributing if it actually moved the score —
	// a convenience store or pharmacy outside its threshold contributed 0.
	if primaryPoi != nil {
		name := primaryPoi.Name
		if name == "" {
			if primaryPoi == closestSupermarket {
				name = "Supermarket"
			} else {
				name = "Corner Store"
			}
		}
		contributing = append(contributing, api.ContributingPoi{
			Name:           name,
			DistanceMeters: float32(math.Round(primaryPoi.DistanceMeters*10) / 10),
		})
	}

	if pharmacyBoost > 0 {
		name := closestPharmacy.Name
		if name == "" {
			name = "Pharmacy"
		}
		contributing = append(contributing, api.ContributingPoi{
			Name:           name,
			DistanceMeters: float32(math.Round(closestPharmacy.DistanceMeters*10) / 10),
		})
	}

	return score, minDistance, contributing
}
