package main

import (
	"context"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

// divvyDocksReaderAdapter satisfies api.DivvyDocksReader by delegating to a
// real *clients.DivvyStationsCache, translating between api-owned types
// (the HTTP layer) and clients-owned types at this one boundary -- see
// api.ScoreEngine's doc comment for why those two packages can't import
// each other directly.
type divvyDocksReaderAdapter struct {
	cache *clients.DivvyStationsCache
}

func (a *divvyDocksReaderAdapter) GetDivvyDocks(ctx context.Context) ([]api.DivvyDock, error) {
	stations, err := a.cache.Get(ctx)
	if err != nil {
		return nil, err
	}

	docks := make([]api.DivvyDock, len(stations))
	for i, s := range stations {
		docks[i] = api.DivvyDock{
			Name:            s.Name,
			Lat:             s.Lat,
			Lon:             s.Lon,
			Capacity:        s.Capacity,
			BikesAvailable:  s.BikesAvailable,
			EBikesAvailable: s.EBikesAvailable,
			DocksAvailable:  s.DocksAvailable,
			IsRenting:       s.IsRenting,
			IsReturning:     s.IsReturning,
		}
	}
	return docks, nil
}
