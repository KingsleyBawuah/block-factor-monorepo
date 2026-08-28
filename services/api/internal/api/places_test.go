package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
)

type mockPlacesFinder struct {
	places      []api.Place
	err         error
	capturedLat float64
	capturedLon float64
}

func (m *mockPlacesFinder) FindNearby(_ context.Context, lat, lon float64) ([]api.Place, error) {
	m.capturedLat, m.capturedLon = lat, lon
	if m.err != nil {
		return nil, m.err
	}
	return m.places, nil
}

func newPlacesServer(finder *mockPlacesFinder, boundary *mockBoundaryChecker) *api.Server {
	logger := slog.New(slog.DiscardHandler)
	return api.NewServer(logger, api.WithPlacesFinder(finder), api.WithBoundaryChecker(boundary))
}

func TestGetPlaces_Success(t *testing.T) {
	finder := &mockPlacesFinder{places: []api.Place{
		{Name: "Sip Coffee", Category: "cafe", Lat: 41.8999, Lon: -87.6506, DistanceMeters: 120.5},
		{Name: "Equinox The Loop", Category: "fitness", Lat: 41.8996, Lon: -87.6503, DistanceMeters: 210.0},
		{Name: "Jewel-Osco", Category: "supermarket", Lat: 41.9, Lon: -87.65, DistanceMeters: 300.0},
	}}
	boundary := &mockBoundaryChecker{within: true}
	srv := newPlacesServer(finder, boundary)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/places?lat=41.8998&lon=-87.6505", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if finder.capturedLat != 41.8998 || finder.capturedLon != -87.6505 {
		t.Errorf("expected finder called with request coords, got (%v, %v)", finder.capturedLat, finder.capturedLon)
	}

	var resp api.PlacesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Type != api.PlacesResponseTypeFeatureCollection {
		t.Errorf("expected type FeatureCollection, got %q", resp.Type)
	}
	if len(resp.Features) != 3 {
		t.Fatalf("expected 3 features, got %d", len(resp.Features))
	}

	byName := map[string]api.PlaceFeature{}
	for _, f := range resp.Features {
		byName[f.Properties.Name] = f
	}

	cafe := byName["Sip Coffee"]
	if cafe.Properties.Category != api.Cafe || cafe.Properties.DistanceMeters != 120.5 {
		t.Errorf("unexpected cafe feature: %+v", cafe.Properties)
	}
	if cafe.Geometry.Type != api.PlaceFeatureGeometryTypePoint {
		t.Errorf("expected Point geometry, got %q", cafe.Geometry.Type)
	}
	wantCoords := []interface{}{-87.6506, 41.8999}
	if len(cafe.Geometry.Coordinates) != 2 || cafe.Geometry.Coordinates[0] != wantCoords[0] || cafe.Geometry.Coordinates[1] != wantCoords[1] {
		t.Errorf("expected coordinates [lon, lat] = %v, got %v", wantCoords, cafe.Geometry.Coordinates)
	}

	fitness := byName["Equinox The Loop"]
	if fitness.Properties.Category != api.Fitness || fitness.Properties.DistanceMeters != 210.0 {
		t.Errorf("unexpected fitness feature: %+v", fitness.Properties)
	}

	supermarket := byName["Jewel-Osco"]
	if supermarket.Properties.Category != api.Supermarket || supermarket.Properties.DistanceMeters != 300.0 {
		t.Errorf("unexpected supermarket feature: %+v", supermarket.Properties)
	}
}

func TestGetPlaces_InvalidLatLon(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"lat above 90", "lat=200&lon=-87.6505"},
		{"lat below -90", "lat=-200&lon=-87.6505"},
		{"lon above 180", "lat=41.8998&lon=200"},
		{"lon below -180", "lat=41.8998&lon=-200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := &mockPlacesFinder{}
			boundary := &mockBoundaryChecker{within: true}
			srv := newPlacesServer(finder, boundary)
			router := api.NewRouter(srv)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/places?"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", w.Code)
			}
			var errResp api.Error
			_ = json.NewDecoder(w.Body).Decode(&errResp)
			if errResp.Code != "VALIDATION_ERROR" {
				t.Errorf("expected VALIDATION_ERROR, got %q", errResp.Code)
			}
		})
	}
}

func TestGetPlaces_MissingRequiredParam(t *testing.T) {
	finder := &mockPlacesFinder{}
	boundary := &mockBoundaryChecker{within: true}
	srv := newPlacesServer(finder, boundary)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/places?lat=41.8998", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for a missing 'lon' param, got %d", w.Code)
	}
}

func TestGetPlaces_OutOfCoverage(t *testing.T) {
	finder := &mockPlacesFinder{}
	boundary := &mockBoundaryChecker{within: false}
	srv := newPlacesServer(finder, boundary)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/places?lat=42.5&lon=-88.5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "OUT_OF_COVERAGE" {
		t.Errorf("expected OUT_OF_COVERAGE, got %q", errResp.Code)
	}
}

func TestGetPlaces_BoundaryCheckError(t *testing.T) {
	finder := &mockPlacesFinder{}
	boundary := &mockBoundaryChecker{err: errors.New("db connection lost")}
	srv := newPlacesServer(finder, boundary)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/places?lat=41.8998&lon=-87.6505", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "PLACES_ERROR" {
		t.Errorf("expected PLACES_ERROR, got %q", errResp.Code)
	}
}

func TestGetPlaces_FinderError(t *testing.T) {
	finder := &mockPlacesFinder{err: errors.New("places lookup failed")}
	boundary := &mockBoundaryChecker{within: true}
	srv := newPlacesServer(finder, boundary)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/places?lat=41.8998&lon=-87.6505", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "PLACES_ERROR" {
		t.Errorf("expected PLACES_ERROR, got %q", errResp.Code)
	}
}

func TestGetPlaces_NotConfigured(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/places?lat=41.8998&lon=-87.6505", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected SERVICE_UNAVAILABLE, got %q", errResp.Code)
	}
}
