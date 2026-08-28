package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

type stubPlacesQuerier struct {
	places []clients.Place
	err    error
}

func (s *stubPlacesQuerier) SearchNearby(_ context.Context, _, _, _ float64, _ []string) ([]clients.Place, error) {
	return s.places, s.err
}

type stubThirdPlaceCache struct {
	data map[string]any
}

func (c *stubThirdPlaceCache) Get(_ context.Context, key string, dest any) (bool, error) {
	v, ok := c.data[key]
	if !ok {
		return false, nil
	}
	*dest.(*[]scoring.ThirdPlacePoi) = v.([]scoring.ThirdPlacePoi)
	return true, nil
}

func (c *stubThirdPlaceCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	if c.data == nil {
		c.data = make(map[string]any)
	}
	c.data[key] = value
	return nil
}

type stubPlacesBudget struct {
	withinBudget bool
}

func (b *stubPlacesBudget) IncrementAndCheck(_ context.Context, _ int) (bool, error) {
	return b.withinBudget, nil
}

type stubSupermarketQuerier struct {
	pois []scoring.OsmPoi
	err  error
}

func (q *stubSupermarketQuerier) GetNearbyGroceryPois(_ context.Context, _, _ float64, _ float64) ([]scoring.OsmPoi, error) {
	return q.pois, q.err
}

func TestPlacesFinderAdapter_FindNearby_TranslatesResults(t *testing.T) {
	thirdPlaces := scoring.NewThirdPlacePoiResolver(
		&stubPlacesQuerier{places: []clients.Place{
			{Name: "Sip Coffee", PrimaryType: "cafe", Lat: 41.8999, Lon: -87.6506},
		}},
		&stubThirdPlaceCache{},
		&stubPlacesBudget{withinBudget: true},
		100,
	)
	supermarkets := &stubSupermarketQuerier{pois: []scoring.OsmPoi{
		{Name: "Jewel-Osco", PoiType: "supermarket", Lat: 41.9, Lon: -87.65, DistanceMeters: 300.0},
	}}
	resolver := scoring.NewNearbyPlacesResolver(thirdPlaces, supermarkets)
	adapter := &placesFinderAdapter{resolver: resolver}

	places, err := adapter.FindNearby(context.Background(), 41.8998, -87.6505)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(places) != 2 {
		t.Fatalf("expected 2 places, got %d: %+v", len(places), places)
	}

	byName := map[string]int{}
	for _, p := range places {
		byName[p.Name]++
	}
	if byName["Sip Coffee"] != 1 || byName["Jewel-Osco"] != 1 {
		t.Errorf("expected both Sip Coffee and Jewel-Osco, got %+v", places)
	}

	for _, p := range places {
		if p.Name == "Sip Coffee" && p.Category != "cafe" {
			t.Errorf("expected category %q for Sip Coffee, got %q", "cafe", p.Category)
		}
		if p.Name == "Jewel-Osco" && p.Category != "supermarket" {
			t.Errorf("expected category %q for Jewel-Osco, got %q", "supermarket", p.Category)
		}
	}
}

func TestPlacesFinderAdapter_FindNearby_PropagatesError(t *testing.T) {
	thirdPlaces := scoring.NewThirdPlacePoiResolver(
		&stubPlacesQuerier{},
		&stubThirdPlaceCache{},
		&stubPlacesBudget{withinBudget: true},
		100,
	)
	supermarkets := &stubSupermarketQuerier{err: errors.New("db unavailable")}
	resolver := scoring.NewNearbyPlacesResolver(thirdPlaces, supermarkets)
	adapter := &placesFinderAdapter{resolver: resolver}

	if _, err := adapter.FindNearby(context.Background(), 41.8998, -87.6505); err == nil {
		t.Fatal("expected the underlying resolver error to propagate, got nil")
	}
}
