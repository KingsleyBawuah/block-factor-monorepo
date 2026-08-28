package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

func TestDivvyDocksReaderAdapter_GetDivvyDocks_TranslatesResults(t *testing.T) {
	fetcher := &mockDivvyFetcherForAdapter{stations: []clients.DivvyStation{
		{
			StationID: "1", Name: "California Blvd & 29th St", Lat: 41.84024, Lon: -87.69413,
			Capacity: 8, BikesAvailable: 2, EBikesAvailable: 1, DocksAvailable: 5,
			IsRenting: true, IsReturning: true,
		},
	}}
	cache := clients.NewDivvyStationsCache(fetcher, time.Hour, nil)
	adapter := &divvyDocksReaderAdapter{cache: cache}

	docks, err := adapter.GetDivvyDocks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docks) != 1 {
		t.Fatalf("expected 1 dock, got %d", len(docks))
	}

	d := docks[0]
	if d.Name != "California Blvd & 29th St" || d.Lat != 41.84024 || d.Lon != -87.69413 {
		t.Errorf("unexpected translated dock: %+v", d)
	}
	if d.Capacity != 8 || d.BikesAvailable != 2 || d.EBikesAvailable != 1 || d.DocksAvailable != 5 {
		t.Errorf("unexpected translated dock counts: %+v", d)
	}
	if !d.IsRenting || !d.IsReturning {
		t.Errorf("expected IsRenting=true IsReturning=true, got %+v", d)
	}
}

func TestDivvyDocksReaderAdapter_GetDivvyDocks_PropagatesError(t *testing.T) {
	fetcher := &mockDivvyFetcherForAdapter{err: errors.New("upstream GBFS feed unreachable")}
	cache := clients.NewDivvyStationsCache(fetcher, time.Hour, nil)
	adapter := &divvyDocksReaderAdapter{cache: cache}

	if _, err := adapter.GetDivvyDocks(context.Background()); err == nil {
		t.Fatal("expected the underlying cache error to propagate, got nil")
	}
}

type mockDivvyFetcherForAdapter struct {
	stations []clients.DivvyStation
	err      error
}

func (m *mockDivvyFetcherForAdapter) FetchStations(_ context.Context) ([]clients.DivvyStation, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stations, nil
}
