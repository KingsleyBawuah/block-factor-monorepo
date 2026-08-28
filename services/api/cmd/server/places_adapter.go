package main

import (
	"context"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

// placesFinderAdapter satisfies api.PlacesFinder by delegating to a real
// *scoring.NearbyPlacesResolver, translating between api-owned types (the
// HTTP layer) and scoring-owned types at this one boundary -- see
// api.ScoreEngine's doc comment for why those two packages can't import
// each other directly.
type placesFinderAdapter struct {
	resolver *scoring.NearbyPlacesResolver
}

func (a *placesFinderAdapter) FindNearby(ctx context.Context, lat, lon float64) ([]api.Place, error) {
	found, err := a.resolver.FindNearby(ctx, lat, lon)
	if err != nil {
		return nil, err
	}

	places := make([]api.Place, len(found))
	for i, p := range found {
		places[i] = api.Place{
			Name:           p.Name,
			Category:       string(p.Category),
			Lat:            p.Lat,
			Lon:            p.Lon,
			DistanceMeters: p.DistanceMeters,
		}
	}
	return places, nil
}
