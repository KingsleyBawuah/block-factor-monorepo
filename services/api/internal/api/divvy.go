package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// DivvyDock is a single Divvy/Lyft bike-share station resolved for
// GET /api/v1/layers/divvy-docks (docs/IMPLEMENTATION_PHASES.md Phase 4
// deliverable 8): live location, capacity, and availability. Display-only
// -- Divvy is not a scored factor.
type DivvyDock struct {
	Name            string
	Lat             float64
	Lon             float64
	Capacity        int
	BikesAvailable  int
	EBikesAvailable int
	DocksAvailable  int
	IsRenting       bool
	IsReturning     bool
}

// DivvyDocksReader resolves the live Divvy dock list for
// GET /api/v1/layers/divvy-docks. Declared here (api-owned types only)
// rather than referencing internal/clients directly, for the same
// import-cycle reason as ScoreEngine -- see its doc comment.
type DivvyDocksReader interface {
	GetDivvyDocks(ctx context.Context) ([]DivvyDock, error)
}

// WithDivvyDocksReader sets the Divvy dock data source for the server.
func WithDivvyDocksReader(r DivvyDocksReader) ServerOption {
	return func(s *Server) {
		s.divvyDocksReader = r
	}
}

// GetDivvyDocksLayer handles GET /api/v1/layers/divvy-docks.
func (s *Server) GetDivvyDocksLayer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.divvyDocksReader == nil {
		s.logger.Error("divvy docks reader is not configured")
		s.writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Divvy dock data is unavailable")
		return
	}

	docks, err := s.divvyDocksReader.GetDivvyDocks(r.Context())
	if err != nil {
		s.logger.Error("divvy docks lookup failed", slog.String("error", err.Error()))
		s.writeError(w, http.StatusBadGateway, "DIVVY_UNAVAILABLE", "Failed to load live Divvy dock data")
		return
	}

	features := make([]DivvyDockFeature, len(docks))
	for i, d := range docks {
		f := DivvyDockFeature{Type: DivvyDockFeatureTypeFeature}
		f.Properties.Name = d.Name
		f.Properties.Capacity = d.Capacity
		f.Properties.BikesAvailable = d.BikesAvailable
		f.Properties.EbikesAvailable = d.EBikesAvailable
		f.Properties.DocksAvailable = d.DocksAvailable
		f.Properties.IsRenting = d.IsRenting
		f.Properties.IsReturning = d.IsReturning
		f.Geometry.Type = DivvyDockFeatureGeometryTypePoint
		f.Geometry.Coordinates = []interface{}{d.Lon, d.Lat}
		features[i] = f
	}

	// Live data refreshed on its own 5-minute cache cycle
	// (docs/ARCHITECTURE.md §5.4) -- a short browser cache, not the
	// 1-day one the bulk/static layer endpoints use.
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=240")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(DivvyDocksResponse{
		Type:     DivvyDocksResponseTypeFeatureCollection,
		Features: features,
	}); err != nil {
		s.logger.Error("failed to encode divvy docks response", slog.String("error", err.Error()))
	}
}
