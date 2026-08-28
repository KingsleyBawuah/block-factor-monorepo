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

type mockDivvyDocksReader struct {
	docks []api.DivvyDock
	err   error
}

func (m *mockDivvyDocksReader) GetDivvyDocks(_ context.Context) ([]api.DivvyDock, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.docks, nil
}

func newDivvyServer(reader *mockDivvyDocksReader) *api.Server {
	logger := slog.New(slog.DiscardHandler)
	return api.NewServer(logger, api.WithDivvyDocksReader(reader))
}

func TestGetDivvyDocksLayer_Success(t *testing.T) {
	reader := &mockDivvyDocksReader{docks: []api.DivvyDock{
		{
			Name: "California Blvd & 29th St", Lat: 41.84024, Lon: -87.69413,
			Capacity: 8, BikesAvailable: 2, EBikesAvailable: 1, DocksAvailable: 5,
			IsRenting: true, IsReturning: true,
		},
		{
			Name: "Kedzie Ave & 54th Pl", Lat: 41.794357, Lon: -87.703467,
			Capacity: 11, BikesAvailable: 0, EBikesAvailable: 0, DocksAvailable: 11,
			IsRenting: false, IsReturning: true,
		},
	}}
	srv := newDivvyServer(reader)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/layers/divvy-docks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=60, stale-while-revalidate=240" {
		t.Errorf("unexpected Cache-Control header: %q", got)
	}

	var resp api.DivvyDocksResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Type != api.DivvyDocksResponseTypeFeatureCollection {
		t.Errorf("expected type FeatureCollection, got %q", resp.Type)
	}
	if len(resp.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(resp.Features))
	}

	byName := map[string]api.DivvyDockFeature{}
	for _, f := range resp.Features {
		byName[f.Properties.Name] = f
	}

	open := byName["California Blvd & 29th St"]
	if open.Properties.Capacity != 8 || open.Properties.BikesAvailable != 2 ||
		open.Properties.EbikesAvailable != 1 || open.Properties.DocksAvailable != 5 {
		t.Errorf("unexpected open station properties: %+v", open.Properties)
	}
	if !open.Properties.IsRenting || !open.Properties.IsReturning {
		t.Errorf("expected IsRenting=true IsReturning=true, got %+v", open.Properties)
	}
	if open.Geometry.Type != api.DivvyDockFeatureGeometryTypePoint {
		t.Errorf("expected Point geometry, got %q", open.Geometry.Type)
	}
	wantCoords := []interface{}{-87.69413, 41.84024}
	if len(open.Geometry.Coordinates) != 2 || open.Geometry.Coordinates[0] != wantCoords[0] || open.Geometry.Coordinates[1] != wantCoords[1] {
		t.Errorf("expected coordinates [lon, lat] = %v, got %v", wantCoords, open.Geometry.Coordinates)
	}

	// A station can be installed but temporarily closed for rebalancing --
	// IsRenting=false must round-trip correctly, not just IsRenting=true.
	closed := byName["Kedzie Ave & 54th Pl"]
	if closed.Properties.IsRenting {
		t.Errorf("expected IsRenting=false for a station not currently renting, got true")
	}
	if !closed.Properties.IsReturning {
		t.Errorf("expected IsReturning=true, got false")
	}
}

func TestGetDivvyDocksLayer_UpstreamError(t *testing.T) {
	reader := &mockDivvyDocksReader{err: errors.New("upstream GBFS feed unreachable")}
	srv := newDivvyServer(reader)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/layers/divvy-docks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "DIVVY_UNAVAILABLE" {
		t.Errorf("expected DIVVY_UNAVAILABLE, got %q", errResp.Code)
	}
}

func TestGetDivvyDocksLayer_NotConfigured(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/layers/divvy-docks", nil)
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
