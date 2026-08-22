package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
)

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
