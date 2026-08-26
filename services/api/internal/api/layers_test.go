package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
)

// mockLayerReader implements spatial.LayerReader. Each test wires only the
// func field its endpoint actually calls -- the others are never invoked,
// since each handler calls exactly one method.
type mockLayerReader struct {
	ctaRailLinesFunc func(ctx context.Context) ([]byte, error)
	ctaStationsFunc  func(ctx context.Context) ([]byte, error)
	bikeRoutesFunc   func(ctx context.Context) ([]byte, error)
	trailAccessFunc  func(ctx context.Context) ([]byte, error)
	supermarketsFunc func(ctx context.Context) ([]byte, error)
}

func (m *mockLayerReader) CTARailLinesGeoJSON(ctx context.Context) ([]byte, error) {
	return m.ctaRailLinesFunc(ctx)
}
func (m *mockLayerReader) CTAStationsGeoJSON(ctx context.Context) ([]byte, error) {
	return m.ctaStationsFunc(ctx)
}
func (m *mockLayerReader) BikeRoutesGeoJSON(ctx context.Context) ([]byte, error) {
	return m.bikeRoutesFunc(ctx)
}
func (m *mockLayerReader) TrailAccessGeoJSON(ctx context.Context) ([]byte, error) {
	return m.trailAccessFunc(ctx)
}
func (m *mockLayerReader) SupermarketsGeoJSON(ctx context.Context) ([]byte, error) {
	return m.supermarketsFunc(ctx)
}

func TestGetCtaRailLinesLayer_Success(t *testing.T) {
	want := []byte(`{"type":"FeatureCollection","features":[]}`)
	reader := &mockLayerReader{
		ctaRailLinesFunc: func(_ context.Context) ([]byte, error) {
			return want, nil
		},
	}

	srv := api.NewServer(nil, api.WithLayerReader(reader))
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/layers/cta-rail", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	if w.Body.String() != string(want) {
		t.Errorf("expected body to pass through unchanged, got %q", w.Body.String())
	}
}

func TestGetCtaRailLinesLayer_ReaderUnavailable(t *testing.T) {
	srv := api.NewServer(nil) // no WithLayerReader
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/layers/cta-rail", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}

	var res api.Error
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected code SERVICE_UNAVAILABLE, got %q", res.Code)
	}
}

func TestGetCtaRailLinesLayer_QueryError(t *testing.T) {
	reader := &mockLayerReader{
		ctaRailLinesFunc: func(_ context.Context) ([]byte, error) {
			return nil, errors.New("connection reset")
		},
	}

	srv := api.NewServer(nil, api.WithLayerReader(reader))
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/layers/cta-rail", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	var res api.Error
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Code != "LAYER_QUERY_ERROR" {
		t.Errorf("expected code LAYER_QUERY_ERROR, got %q", res.Code)
	}
}

// TestLayerRoutes_AllWired confirms every layer route resolves to its own
// handler and reaches the right LayerReader method -- a regression guard
// against a copy-paste mistake wiring e.g. /bike-routes to
// CTAStationsGeoJSON instead of BikeRoutesGeoJSON.
func TestLayerRoutes_AllWired(t *testing.T) {
	tests := []struct {
		path      string
		wireField func(*mockLayerReader, func(context.Context) ([]byte, error))
	}{
		{"/api/v1/layers/cta-rail", func(m *mockLayerReader, f func(context.Context) ([]byte, error)) { m.ctaRailLinesFunc = f }},
		{"/api/v1/layers/cta-stations", func(m *mockLayerReader, f func(context.Context) ([]byte, error)) { m.ctaStationsFunc = f }},
		{"/api/v1/layers/bike-routes", func(m *mockLayerReader, f func(context.Context) ([]byte, error)) { m.bikeRoutesFunc = f }},
		{"/api/v1/layers/trail-access", func(m *mockLayerReader, f func(context.Context) ([]byte, error)) { m.trailAccessFunc = f }},
		{"/api/v1/layers/supermarkets", func(m *mockLayerReader, f func(context.Context) ([]byte, error)) { m.supermarketsFunc = f }},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			want := []byte(`{"type":"FeatureCollection","features":[]}`)
			called := false
			reader := &mockLayerReader{}
			tt.wireField(reader, func(_ context.Context) ([]byte, error) {
				called = true
				return want, nil
			})

			srv := api.NewServer(nil, api.WithLayerReader(reader))
			router := api.NewRouter(srv)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
			}
			if !called {
				t.Errorf("expected the LayerReader method wired to %s to be called", tt.path)
			}
			if w.Body.String() != string(want) {
				t.Errorf("expected body to pass through unchanged, got %q", w.Body.String())
			}
		})
	}
}
