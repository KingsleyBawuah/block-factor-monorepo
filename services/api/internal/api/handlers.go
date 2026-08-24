// Package api provides HTTP request handlers, routing, and generated OpenAPI contracts.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// Geocoder resolves a raw query string into ranked candidate locations.
type Geocoder interface {
	Geocode(ctx context.Context, query string) ([]clients.GeocodeCandidate, error)
}

// Server implements the generated ServerInterface.
type Server struct {
	logger          *slog.Logger
	geocoder        Geocoder
	boundaryChecker spatial.BoundaryChecker
}

// ServerOption configures a Server instance.
type ServerOption func(*Server)

// WithGeocoder sets the geocoding client for the server.
func WithGeocoder(g Geocoder) ServerOption {
	return func(s *Server) {
		s.geocoder = g
	}
}

// WithBoundaryChecker sets the spatial boundary checker for the server.
func WithBoundaryChecker(b spatial.BoundaryChecker) ServerOption {
	return func(s *Server) {
		s.boundaryChecker = b
	}
}

// NewServer constructs a new Server instance.
func NewServer(logger *slog.Logger, opts ...ServerOption) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		logger: logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
func (s *Server) GeocodeAddress(w http.ResponseWriter, r *http.Request, params GeocodeAddressParams) {
	w.Header().Set("Content-Type", "application/json")

	query := strings.TrimSpace(params.Q)
	if query == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(Error{
			Code:    "VALIDATION_ERROR",
			Message: "Query parameter 'q' is required and cannot be empty",
		})
		return
	}

	if s.geocoder == nil {
		s.logger.Error("geocoder client is not configured")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(Error{
			Code:    "SERVICE_UNAVAILABLE",
			Message: "Geocoding service is unavailable",
		})
		return
	}

	results, err := s.geocoder.Geocode(r.Context(), query)
	if err != nil {
		s.logger.Error("geocoding failed", slog.String("query", query), slog.String("error", err.Error()))
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(Error{
			Code:    "GEOCODING_ERROR",
			Message: "Failed to resolve address coordinates",
		})
		return
	}

	candidates := make([]GeocodeCandidate, len(results))
	for i, res := range results {
		candidates[i] = GeocodeCandidate{
			FormattedAddress: res.FormattedAddress,
			Coordinates: Coordinates{
				Lat: res.Lat,
				Lon: res.Lon,
			},
			Rank: res.Rank,
		}
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(GeocodeResponse{
		Candidates: candidates,
	}); err != nil {
		s.logger.Error("failed to encode geocode response", slog.String("error", err.Error()))
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
