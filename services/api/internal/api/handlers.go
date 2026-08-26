// Package api provides HTTP request handlers, routing, and generated OpenAPI contracts.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	scoreEngine     ScoreEngine
	layerReader     spatial.LayerReader
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

// WithScoreEngine sets the composite scoring engine for the server.
func WithScoreEngine(e ScoreEngine) ServerOption {
	return func(s *Server) {
		s.scoreEngine = e
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

// writeError writes a JSON Error body with the given status code.
func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(Error{Code: code, Message: message}); err != nil {
		s.logger.Error("failed to encode error response", slog.String("error", err.Error()))
	}
}

// resolveCoordinates returns coords directly if already resolved, otherwise
// geocodes address and returns the top-ranked candidate. Exactly one of
// address/coords is expected to be non-nil -- callers validate that
// mutual-exclusivity rule before calling this.
func (s *Server) resolveCoordinates(ctx context.Context, address *string, coords *Coordinates) (*Coordinates, error) {
	if coords != nil {
		return coords, nil
	}

	if s.geocoder == nil {
		return nil, errGeocoderUnavailable
	}

	candidates, err := s.geocoder.Geocode(ctx, *address)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGeocodingFailed, err)
	}
	if len(candidates) == 0 {
		return nil, errAddressNotFound
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Rank < best.Rank {
			best = c
		}
	}

	return &Coordinates{Lat: best.Lat, Lon: best.Lon}, nil
}

// Sentinel errors resolveCoordinates returns, mapped to specific HTTP
// status codes/error codes by ScoreAddress.
var (
	errGeocoderUnavailable = errors.New("geocoder not configured")
	errGeocodingFailed     = errors.New("geocoding request failed")
	errAddressNotFound     = errors.New("address could not be resolved to any coordinates")
)

// maxScoreRequestBodyBytes caps the POST /api/v1/score request body --
// legitimate payloads (an address string, optional coordinates/weights/unit
// metadata) are at most a few hundred bytes, so this is a generous ceiling
// purely to stop an oversized body from forcing an unbounded json.Decode
// allocation.
const maxScoreRequestBodyBytes = 1 << 20 // 1 MiB

// ScoreAddress handles POST /api/v1/score.
func (s *Server) ScoreAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	r.Body = http.MaxBytesReader(w, r.Body, maxScoreRequestBodyBytes)

	var req ScoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Request body must be valid JSON")
		return
	}

	if (req.Address == nil) == (req.Origin == nil) {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Exactly one of 'address' or 'origin' must be provided")
		return
	}
	if req.Address != nil && strings.TrimSpace(*req.Address) == "" {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "'address' cannot be empty")
		return
	}
	if req.Destination != nil && req.DestinationCoordinates != nil {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "'destination' and 'destinationCoordinates' are mutually exclusive")
		return
	}
	if req.Destination != nil && strings.TrimSpace(*req.Destination) == "" {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "'destination' cannot be empty")
		return
	}

	if s.scoreEngine == nil || s.boundaryChecker == nil {
		s.logger.Error("scoring engine or boundary checker is not configured")
		s.writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Scoring service is unavailable")
		return
	}

	ctx := r.Context()

	origin, err := s.resolveCoordinates(ctx, req.Address, req.Origin)
	if err != nil {
		s.respondCoordinateResolutionError(w, "origin", err)
		return
	}

	within, err := s.boundaryChecker.IsWithinCityBoundary(ctx, origin.Lat, origin.Lon)
	if err != nil {
		// This is our own PostGIS query failing, not an upstream/gateway
		// call -- 500, not 502, and SCORING_ERROR (an internal fault)
		// rather than SERVICE_UNAVAILABLE (which this codebase otherwise
		// reserves for "a dependency was never configured at all").
		s.logger.Error("boundary check failed", slog.String("error", err.Error()))
		s.writeError(w, http.StatusInternalServerError, "SCORING_ERROR", "Failed to verify service coverage")
		return
	}
	if !within {
		s.writeError(w, http.StatusUnprocessableEntity, "OUT_OF_COVERAGE", "The requested address falls outside the City of Chicago service area")
		return
	}

	var destination *Coordinates
	if req.Destination != nil || req.DestinationCoordinates != nil {
		destination, err = s.resolveCoordinates(ctx, req.Destination, req.DestinationCoordinates)
		if err != nil {
			s.respondCoordinateResolutionError(w, "destination", err)
			return
		}
	}

	composite, factors, err := s.scoreEngine.Score(ctx, ScoreEngineParams{
		Address:                req.Address,
		Origin:                 *origin,
		DestinationCoordinates: destination,
		Unit:                   req.Unit,
		Weights:                req.Weights,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidWeights) {
			s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "At least one active factor weight must be greater than zero")
			return
		}
		s.logger.Error("score computation failed", slog.String("error", err.Error()))
		s.writeError(w, http.StatusInternalServerError, "SCORING_ERROR", "Failed to compute score")
		return
	}

	resp := ScoreResponse{
		CompositeScore: composite,
		// Real benchmark commute pills require a live OTP2 instance
		// (Phase 6); returning an empty array here is honest -- there's no
		// way to compute real Loop/Fulton Market/O'Hare/Midway durations
		// without one, and the schema requires the key to exist, not that
		// it's non-empty.
		BenchmarkCommutes: []CommutePill{},
	}
	resp.Factors.Transit = factorScoreFromMap(factors, "transit")
	resp.Factors.Bike = factorScoreFromMap(factors, "bike")
	resp.Factors.Grocery = factorScoreFromMap(factors, "grocery")
	resp.Factors.ThirdPlace = factorScoreFromMap(factors, "thirdPlace")
	resp.Factors.Green = factorScoreFromMap(factors, "green")
	resp.Factors.Noise = factorScoreFromMap(factors, "noise")
	resp.Factors.Safety = factorScoreFromMap(factors, "safety")
	resp.Factors.Flood = factorScoreFromMap(factors, "flood")
	resp.Factors.Solar = factorScoreFromMap(factors, "solar")
	if _, ok := factors["commute"]; ok {
		commute := factorScoreFromMap(factors, "commute")
		resp.Factors.Commute = &commute
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("failed to encode score response", slog.String("error", err.Error()))
	}
}

// respondCoordinateResolutionError maps a resolveCoordinates error to an
// HTTP response. field is "origin" or "destination", used in the message.
func (s *Server) respondCoordinateResolutionError(w http.ResponseWriter, field string, err error) {
	switch {
	case errors.Is(err, errGeocoderUnavailable):
		s.logger.Error("geocoder client is not configured")
		s.writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Geocoding service is unavailable")
	case errors.Is(err, errAddressNotFound):
		s.writeError(w, http.StatusBadRequest, "ADDRESS_NOT_FOUND", fmt.Sprintf("Could not resolve the %s address to any coordinates", field))
	case errors.Is(err, errGeocodingFailed):
		s.logger.Error("geocoding failed", slog.String("field", field), slog.String("error", err.Error()))
		s.writeError(w, http.StatusBadGateway, "GEOCODING_ERROR", "Failed to resolve address coordinates")
	default:
		s.logger.Error("unexpected coordinate resolution error", slog.String("field", field), slog.String("error", err.Error()))
		s.writeError(w, http.StatusInternalServerError, "SCORING_ERROR", "Failed to resolve address coordinates")
	}
}

// factorScoreFromMap converts factors[name] into the generated FactorScore
// wire type. A name genuinely absent from the map (which shouldn't happen
// for any of the 9 always-registered factors, but is distinct from a
// present-and-degraded result) is reported as degraded too, rather than
// the zero-value ScoreFactorResult's misleading Degraded: false.
func factorScoreFromMap(factors map[string]ScoreFactorResult, name string) FactorScore {
	fr, ok := factors[name]
	if !ok {
		return FactorScore{Degraded: true}
	}
	return FactorScore{
		Score:            fr.Score,
		Degraded:         fr.Degraded,
		DistanceMeters:   fr.DistanceMeters,
		ContributingPois: contributingPoisOrOmit(fr.ContributingPois),
	}
}

// contributingPoisOrOmit returns nil (which json's "omitempty" drops from
// the response entirely) rather than a pointer to a nil/empty slice --
// *[]T with "omitempty" only checks the pointer's nilness, so
// &[]ContributingPoi(nil) would otherwise serialize as
// "contributingPois": null, which docs/openapi.yaml's plain (non-nullable)
// `type: array` schema for this field doesn't permit.
func contributingPoisOrOmit(pois []ContributingPoi) *[]ContributingPoi {
	if len(pois) == 0 {
		return nil
	}
	return &pois
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
		// Deliberately excludes the raw query string -- PRIVACY_AND_DATA_HANDLING.md
		// §5: structured logs never carry raw addresses. Pre-existing bug in
		// this handler, fixed here rather than left for a separate PR.
		s.logger.Error("geocoding failed", slog.String("error", err.Error()))
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
