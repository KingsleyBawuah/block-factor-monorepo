package scoring

import (
	"context"
	"errors"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

type mockSupermarketQuerier struct {
	pois []OsmPoi
	err  error
}

func (m *mockSupermarketQuerier) GetNearbyGroceryPois(_ context.Context, _, _ float64, _ float64) ([]OsmPoi, error) {
	return m.pois, m.err
}

func TestNearbyPlacesResolver_FindNearby_BlendsCafesFitnessAndSupermarkets(t *testing.T) {
	origin := struct{ lat, lon float64 }{41.8998, -87.6505}

	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{places: []clients.Place{
		{Name: "Sip Coffee", PrimaryType: "cafe", Lat: 41.8999, Lon: -87.6506},
		{Name: "CrossFit Loop", PrimaryType: "gym", Lat: 41.8997, Lon: -87.6504},
	}}
	budget := &mockPlacesBudget{withinBudget: true}
	resolver := NewThirdPlacePoiResolver(places, cache, budget, 100)

	supermarkets := &mockSupermarketQuerier{pois: []OsmPoi{
		{Name: "Jewel-Osco", PoiType: "supermarket", Lat: 41.9, Lon: -87.65, DistanceMeters: 120.0},
		// A pharmacy/convenience row must never leak into the places
		// result -- GET /api/v1/places only ever advertises supermarkets
		// from osm_pois, not the full grocery POI taxonomy.
		{Name: "Walgreens", PoiType: "pharmacy", Lat: 41.9, Lon: -87.65, DistanceMeters: 80.0},
	}}

	resolver2 := NewNearbyPlacesResolver(resolver, supermarkets)
	got, err := resolver2.FindNearby(context.Background(), origin.lat, origin.lon)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byCategory := map[NearbyPlaceCategory]int{}
	for _, p := range got {
		byCategory[p.Category]++
	}
	if byCategory[NearbyPlaceCafe] != 1 {
		t.Errorf("expected 1 cafe, got %d", byCategory[NearbyPlaceCafe])
	}
	if byCategory[NearbyPlaceFitness] != 1 {
		t.Errorf("expected 1 fitness venue, got %d", byCategory[NearbyPlaceFitness])
	}
	if byCategory[NearbyPlaceSupermarket] != 1 {
		t.Errorf("expected 1 supermarket, got %d", byCategory[NearbyPlaceSupermarket])
	}
	if byCategory["pharmacy"] != 0 {
		t.Errorf("expected pharmacy POIs to be excluded, got %d", byCategory["pharmacy"])
	}
}

// TestNearbyPlacesResolver_FindNearby_ExcludesUnrecognizedPrimaryTypes
// covers a real bug found via live verification: Google Places (New) can
// return a result whose primaryType isn't actually one of the requested
// includedTypes (a hotel with a gym amenity, a fast-food restaurant with a
// coffee counter) -- that must be excluded, not defaulted into "fitness".
func TestNearbyPlacesResolver_FindNearby_ExcludesUnrecognizedPrimaryTypes(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{places: []clients.Place{
		{Name: "Holiday Inn & Suites", PrimaryType: "hotel", Lat: 41.8999, Lon: -87.6506},
		{Name: "Sip Coffee", PrimaryType: "cafe", Lat: 41.8999, Lon: -87.6506},
	}}
	budget := &mockPlacesBudget{withinBudget: true}
	resolver := NewThirdPlacePoiResolver(places, cache, budget, 100)
	supermarkets := &mockSupermarketQuerier{}

	resolver2 := NewNearbyPlacesResolver(resolver, supermarkets)
	got, err := resolver2.FindNearby(context.Background(), 41.8998, -87.6505)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Sip Coffee" {
		t.Fatalf("expected only the cafe to survive, got %+v", got)
	}
}

func TestNearbyPlacesResolver_FindNearby_ExcludesPoisBeyondRadius(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{places: []clients.Place{
		// thirdPlaceFillRadiusMeters searches wider than
		// NearbyPlacesRadiusMeters, so a fetched POI can legitimately fall
		// just outside the endpoint's own display radius.
		{Name: "Far Cafe", PrimaryType: "cafe", Lat: 41.95, Lon: -87.6505},
	}}
	budget := &mockPlacesBudget{withinBudget: true}
	resolver := NewThirdPlacePoiResolver(places, cache, budget, 100)
	supermarkets := &mockSupermarketQuerier{}

	resolver2 := NewNearbyPlacesResolver(resolver, supermarkets)
	got, err := resolver2.FindNearby(context.Background(), 41.8998, -87.6505)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected the out-of-radius cafe to be excluded, got %d places", len(got))
	}
}

func TestNearbyPlacesResolver_FindNearby_DegradedThirdPlacesStillReturnsSupermarkets(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{places: []clients.Place{{Name: "Sip Coffee", PrimaryType: "cafe"}}}
	budget := &mockPlacesBudget{withinBudget: false} // budget exhausted -> degraded
	resolver := NewThirdPlacePoiResolver(places, cache, budget, 0)

	supermarkets := &mockSupermarketQuerier{pois: []OsmPoi{
		{Name: "Jewel-Osco", PoiType: "supermarket", Lat: 41.9, Lon: -87.65, DistanceMeters: 120.0},
	}}

	resolver2 := NewNearbyPlacesResolver(resolver, supermarkets)
	got, err := resolver2.FindNearby(context.Background(), 41.8998, -87.6505)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Category != NearbyPlaceSupermarket {
		t.Fatalf("expected only the supermarket to survive a degraded third-place lookup, got %+v", got)
	}
}

func TestNearbyPlacesResolver_FindNearby_SupermarketQueryErrorPropagates(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{}
	budget := &mockPlacesBudget{withinBudget: true}
	resolver := NewThirdPlacePoiResolver(places, cache, budget, 100)

	supermarkets := &mockSupermarketQuerier{err: errors.New("db unavailable")}

	resolver2 := NewNearbyPlacesResolver(resolver, supermarkets)
	if _, err := resolver2.FindNearby(context.Background(), 41.8998, -87.6505); err == nil {
		t.Fatal("expected the supermarket query error to propagate, got nil")
	}
}

func TestNearbyPlacesResolver_FindNearby_NotConfigured(t *testing.T) {
	resolver2 := &NearbyPlacesResolver{}
	if _, err := resolver2.FindNearby(context.Background(), 41.8998, -87.6505); err == nil {
		t.Fatal("expected an error for an unconfigured resolver, got nil")
	}
}
