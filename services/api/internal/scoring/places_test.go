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
	if len(got) != 3 {
		t.Fatalf("expected 3 places (pharmacy excluded), got %d: %+v", len(got), got)
	}

	// Also asserts sort order: the two Places-sourced POIs sit ~15m from
	// origin, well inside Jewel-Osco's 120m -- ascending DistanceMeters
	// order must put them first.
	byName := map[string]NearbyPlace{}
	for i, p := range got {
		byName[p.Name] = p
		if i > 0 && p.DistanceMeters < got[i-1].DistanceMeters {
			t.Errorf("expected places sorted ascending by DistanceMeters, got %+v", got)
		}
	}

	cafe, ok := byName["Sip Coffee"]
	if !ok || cafe.Category != NearbyPlaceCafe || cafe.Lat != 41.8999 || cafe.Lon != -87.6506 {
		t.Errorf("unexpected cafe: %+v", cafe)
	}

	fitness, ok := byName["CrossFit Loop"]
	if !ok || fitness.Category != NearbyPlaceFitness || fitness.Lat != 41.8997 || fitness.Lon != -87.6504 {
		t.Errorf("unexpected fitness venue: %+v", fitness)
	}

	supermarket, ok := byName["Jewel-Osco"]
	if !ok || supermarket.Category != NearbyPlaceSupermarket || supermarket.Lat != 41.9 || supermarket.Lon != -87.65 || supermarket.DistanceMeters != 120.0 {
		t.Errorf("unexpected supermarket: %+v", supermarket)
	}

	if _, ok := byName["Walgreens"]; ok {
		t.Error("expected the pharmacy POI to be excluded -- GET /api/v1/places only advertises supermarkets from osm_pois, not the full grocery taxonomy")
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

func TestNearbyPlacesResolver_FindNearby_ThirdPlaceResolverErrorPropagates(t *testing.T) {
	cache := &mockThirdPlaceCache{err: errors.New("cache unavailable")}
	places := &mockPlacesQuerier{}
	budget := &mockPlacesBudget{withinBudget: true}
	resolver := NewThirdPlacePoiResolver(places, cache, budget, 100)
	supermarkets := &mockSupermarketQuerier{}

	resolver2 := NewNearbyPlacesResolver(resolver, supermarkets)
	if _, err := resolver2.FindNearby(context.Background(), 41.8998, -87.6505); err == nil {
		t.Fatal("expected the third-place resolver's own error to propagate, got nil")
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
