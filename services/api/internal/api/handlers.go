// Package api provides HTTP request handlers, routing, and generated OpenAPI contracts.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server implements the generated ServerInterface.
type Server struct {
	logger *slog.Logger
}

// NewServer constructs a new Server instance.
func NewServer(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		logger: logger,
	}
}

// GetHealth handles GET /api/v1/health.
func (s *Server) GetHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(HealthResponse{
		Status: "ok",
	}); err != nil {
		s.logger.Error("failed to encode health response", slog.String("error", err.Error()))
	}
}

// ScoreAddress handles POST /api/v1/score.
func (s *Server) ScoreAddress(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	if err := json.NewEncoder(w).Encode(Error{
		Code:    "NOT_IMPLEMENTED",
		Message: "Scoring engine will be implemented in Phase 3",
	}); err != nil {
		s.logger.Error("failed to encode score error response", slog.String("error", err.Error()))
	}
}

// GeocodeAddress handles GET /api/v1/geocode.
func (s *Server) GeocodeAddress(w http.ResponseWriter, _ *http.Request, _ GeocodeAddressParams) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	if err := json.NewEncoder(w).Encode(Error{
		Code:    "NOT_IMPLEMENTED",
		Message: "Geocoding client will be implemented in Phase 3",
	}); err != nil {
		s.logger.Error("failed to encode geocode error response", slog.String("error", err.Error()))
	}
}

// GetIsochrones handles GET /api/v1/isochrones.
func (s *Server) GetIsochrones(w http.ResponseWriter, _ *http.Request, _ GetIsochronesParams) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	if err := json.NewEncoder(w).Encode(Error{
		Code:    "NOT_IMPLEMENTED",
		Message: "Isochrone service will be implemented in Phase 6",
	}); err != nil {
		s.logger.Error("failed to encode isochrones error response", slog.String("error", err.Error()))
	}
}

// NewRouter registers the OpenAPI handlers and /metrics on a standard http.ServeMux.
func NewRouter(srv ServerInterface) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return HandlerFromMux(srv, mux)
}
