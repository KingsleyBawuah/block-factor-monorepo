package scoring

import (
	"context"
	"errors"
)

// NearbyPlacesRadiusMeters is the fixed per-address search radius for
// GET /api/v1/places (docs/ARCHITECTURE.md Decision Log row 30). Reuses
// ThirdPlaceQueryRadiusMeters -- the same 0.5 mi window S_third_place
// itself scores against -- so the cafes/fitness venues shown on the map
// are exactly the POIs that can move S_third_place, not some second,
// inconsistent radius.
const NearbyPlacesRadiusMeters = ThirdPlaceQueryRadiusMeters

// NearbyPlaceCategory is the map-display category of a NearbyPlace.
type NearbyPlaceCategory string

// The 3 categories GET /api/v1/places distinguishes between.
const (
	NearbyPlaceCafe        NearbyPlaceCategory = "cafe"
	NearbyPlaceFitness     NearbyPlaceCategory = "fitness"
	NearbyPlaceSupermarket NearbyPlaceCategory = "supermarket"
)

// NearbyPlace is a single categorized POI for the Phase 4 POI Density map
// layer (docs/IMPLEMENTATION_PHASES.md Phase 4 deliverables 6/7).
type NearbyPlace struct {
	Name           string
	Category       NearbyPlaceCategory
	Lat            float64
	Lon            float64
	DistanceMeters float64
}

// SupermarketQuerier is the subset of GroceryQuerier the nearby-places
// resolver depends on -- kept narrow so it's obvious this only ever reads,
// never scores, grocery POIs.
type SupermarketQuerier interface {
	GetNearbyGroceryPois(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]OsmPoi, error)
}

// NearbyPlacesResolver blends cafes/fitness venues -- via the shared
// ThirdPlacePoiResolver, the same Google Places grid cache S_third_place
// itself reads/fills, so this costs no additional API spend beyond what
// scoring already needs -- with supermarkets, free and already
// bulk-ingested into osm_pois, into one combined nearby-places list.
type NearbyPlacesResolver struct {
	thirdPlaces  *ThirdPlacePoiResolver
	supermarkets SupermarketQuerier
}

// NewNearbyPlacesResolver creates a new NearbyPlacesResolver.
func NewNearbyPlacesResolver(thirdPlaces *ThirdPlacePoiResolver, supermarkets SupermarketQuerier) *NearbyPlacesResolver {
	return &NearbyPlacesResolver{thirdPlaces: thirdPlaces, supermarkets: supermarkets}
}

// FindNearby returns cafes, fitness venues, and supermarkets within
// NearbyPlacesRadiusMeters of (lat, lon). A Places API budget exhaustion
// degrades to omitting cafes/fitness from the result (same soft-fail
// posture S_third_place itself takes) rather than erroring the whole
// request -- supermarkets, sourced independently from osm_pois, are
// unaffected either way.
func (r *NearbyPlacesResolver) FindNearby(ctx context.Context, lat, lon float64) ([]NearbyPlace, error) {
	if r.thirdPlaces == nil || r.supermarkets == nil {
		return nil, errors.New("scoring: nearby places resolver not configured")
	}

	var places []NearbyPlace

	pois, degraded, err := r.thirdPlaces.Resolve(ctx, lat, lon)
	if err != nil {
		return nil, err
	}
	if !degraded {
		for _, p := range pois {
			d := haversineDistanceMeters(lat, lon, p.Lat, p.Lon)
			if d > NearbyPlacesRadiusMeters {
				continue
			}
			// Google Places (New) can return a result whose primaryType
			// isn't actually one of the requested includedTypes (e.g. a
			// hotel or fast-food restaurant with a gym/cafe amenity) --
			// exclude anything that's neither, the same as
			// CalculateThirdPlaceScore's own switch (no default branch)
			// already does for scoring.
			var category NearbyPlaceCategory
			switch {
			case isCafeType(p.PrimaryType):
				category = NearbyPlaceCafe
			case isFitnessType(p.PrimaryType):
				category = NearbyPlaceFitness
			default:
				continue
			}
			places = append(places, NearbyPlace{
				Name:           p.Name,
				Category:       category,
				Lat:            p.Lat,
				Lon:            p.Lon,
				DistanceMeters: d,
			})
		}
	}

	supermarkets, err := r.supermarkets.GetNearbyGroceryPois(ctx, lat, lon, NearbyPlacesRadiusMeters)
	if err != nil {
		return nil, err
	}
	for _, p := range supermarkets {
		if p.PoiType != "supermarket" {
			continue
		}
		places = append(places, NearbyPlace{
			Name:           p.Name,
			Category:       NearbyPlaceSupermarket,
			Lat:            p.Lat,
			Lon:            p.Lon,
			DistanceMeters: p.DistanceMeters,
		})
	}

	return places, nil
}
