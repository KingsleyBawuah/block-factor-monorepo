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
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

type mockGeocoder struct {
	geocodeFunc func(ctx context.Context, query string) ([]clients.GeocodeCandidate, error)
}

func (m *mockGeocoder) Geocode(ctx context.Context, query string) ([]clients.GeocodeCandidate, error) {
	return m.geocodeFunc(ctx, query)
}

func TestGetHealth(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res api.HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Status != "ok" {
		t.Fatalf("expected status ok, got %q", res.Status)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestGeocodeAddress_Success(t *testing.T) {
	mockGeo := &mockGeocoder{
		geocodeFunc: func(_ context.Context, query string) ([]clients.GeocodeCandidate, error) {
			if query != "233 S Wacker Dr" {
				t.Errorf("expected query '233 S Wacker Dr', got %q", query)
			}
			return []clients.GeocodeCandidate{
				{
					FormattedAddress: "233 S Wacker Dr, Chicago, IL 60606",
					Lat:              41.878876,
					Lon:              -87.635915,
					Rank:             0,
				},
			}, nil
		},
	}

	logger := slog.New(slog.DiscardHandler)
	srv := api.NewServer(logger, api.WithGeocoder(mockGeo))
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=233+S+Wacker+Dr", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res api.GeocodeResponse
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode geocode response: %v", err)
	}

	if len(res.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(res.Candidates))
	}
	candidate := res.Candidates[0]
	if candidate.FormattedAddress != "233 S Wacker Dr, Chicago, IL 60606" || candidate.Coordinates.Lat != 41.878876 || candidate.Coordinates.Lon != -87.635915 || candidate.Rank != 0 {
		t.Errorf("unexpected candidate result: %+v", candidate)
	}
}

func TestGeocodeAddress_EmptyQuery(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=%20%20", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty query, got %d", w.Code)
	}

	var errResp api.Error
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %q", errResp.Code)
	}
}

func TestGeocodeAddress_MissingGeocoder(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	srv := api.NewServer(logger)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=Chicago", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}

	var errResp api.Error
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected error code SERVICE_UNAVAILABLE, got %q", errResp.Code)
	}
}

func TestGeocodeAddress_GeocodeError(t *testing.T) {
	mockGeo := &mockGeocoder{
		geocodeFunc: func(_ context.Context, _ string) ([]clients.GeocodeCandidate, error) {
			return nil, errors.New("upstream timeout")
		},
	}

	logger := slog.New(slog.DiscardHandler)
	srv := api.NewServer(logger, api.WithGeocoder(mockGeo))
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=Chicago", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", w.Code)
	}

	var errResp api.Error
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "GEOCODING_ERROR" {
		t.Errorf("expected error code GEOCODING_ERROR, got %q", errResp.Code)
	}
}

func TestScoreAddress_NotImplemented(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected status 501, got %d", w.Code)
	}
}

func TestGetIsochrones_NotImplemented(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/isochrones?lat=41.8781&lon=-87.6298", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected status 501, got %d", w.Code)
	}
}
