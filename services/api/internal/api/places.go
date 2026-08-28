package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Place is a single categorized nearby POI resolved for GET /api/v1/places
// (docs/IMPLEMENTATION_PHASES.md Phase 4 deliverable 6): a cafe, fitness
// venue, or supermarket within the fixed per-address search radius
// (docs/ARCHITECTURE.md Decision Log row 30).
type Place struct {
	Name           string
	Category       string // "cafe", "fitness", or "supermarket"
	Lat            float64
	Lon            float64
	DistanceMeters float64
}

// PlacesFinder resolves nearby places for GET /api/v1/places. Declared here
// (api-owned types only) rather than referencing internal/scoring directly,
// for the same import-cycle reason as ScoreEngine -- see its doc comment.
type PlacesFinder interface {
	FindNearby(ctx context.Context, lat, lon float64) ([]Place, error)
}

// WithPlacesFinder sets the nearby-places data source for the server.
func WithPlacesFinder(f PlacesFinder) ServerOption {
	return func(s *Server) {
		s.placesFinder = f
	}
}

// placeCategoryToWire maps a scoring-owned category string to its
// generated OpenAPI enum value. An unrecognized category (which shouldn't
// happen -- PlacesFinder implementations only ever emit the 3 known
// values) falls back to Supermarket rather than panicking or silently
// emitting an empty string the schema doesn't allow.
func placeCategoryToWire(category string) PlaceFeaturePropertiesCategory {
	switch category {
	case "cafe":
		return Cafe
	case "fitness":
		return Fitness
	default:
		return Supermarket
	}
}

// GetPlaces handles GET /api/v1/places.
func (s *Server) GetPlaces(w http.ResponseWriter, r *http.Request, params GetPlacesParams) {
	w.Header().Set("Content-Type", "application/json")

	if params.Lat < -90 || params.Lat > 90 || params.Lon < -180 || params.Lon > 180 {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "'lat' must be between -90 and 90, and 'lon' between -180 and 180")
		return
	}

	if s.placesFinder == nil || s.boundaryChecker == nil {
		s.logger.Error("places finder or boundary checker is not configured")
		s.writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Nearby places service is unavailable")
		return
	}

	ctx := r.Context()

	within, err := s.boundaryChecker.IsWithinCityBoundary(ctx, params.Lat, params.Lon)
	if err != nil {
		s.logger.Error("boundary check failed", slog.String("error", err.Error()))
		s.writeError(w, http.StatusInternalServerError, "PLACES_ERROR", "Failed to verify service coverage")
		return
	}
	if !within {
		s.writeError(w, http.StatusUnprocessableEntity, "OUT_OF_COVERAGE", "The requested coordinate falls outside the City of Chicago service area")
		return
	}

	places, err := s.placesFinder.FindNearby(ctx, params.Lat, params.Lon)
	if err != nil {
		s.logger.Error("nearby places lookup failed", slog.String("error", err.Error()))
		s.writeError(w, http.StatusInternalServerError, "PLACES_ERROR", "Failed to load nearby places")
		return
	}

	features := make([]PlaceFeature, len(places))
	for i, p := range places {
		f := PlaceFeature{Type: PlaceFeatureTypeFeature}
		f.Properties.Name = p.Name
		f.Properties.Category = placeCategoryToWire(p.Category)
		f.Properties.DistanceMeters = float32(p.DistanceMeters)
		f.Geometry.Type = PlaceFeatureGeometryTypePoint
		f.Geometry.Coordinates = []interface{}{p.Lon, p.Lat}
		features[i] = f
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(PlacesResponse{
		Type:     PlacesResponseTypeFeatureCollection,
		Features: features,
	}); err != nil {
		s.logger.Error("failed to encode places response", slog.String("error", err.Error()))
	}
}
