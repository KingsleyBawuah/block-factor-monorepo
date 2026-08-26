package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// WithLayerReader sets the static map layer data source for the server.
func WithLayerReader(r spatial.LayerReader) ServerOption {
	return func(s *Server) {
		s.layerReader = r
	}
}

// writeLayerGeoJSON runs get and writes its result as the raw response
// body. get returns the already-fully-assembled FeatureCollection JSON
// (built server-side in Postgres -- see spatial.LayerReader), so there's
// nothing to unmarshal/remarshal here.
func (s *Server) writeLayerGeoJSON(w http.ResponseWriter, layerName string, get func() ([]byte, error)) {
	w.Header().Set("Content-Type", "application/json")

	if s.layerReader == nil {
		s.logger.Error("layer reader is not configured", slog.String("layer", layerName))
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(Error{
			Code:    "SERVICE_UNAVAILABLE",
			Message: "Map layer data is unavailable",
		})
		return
	}

	raw, err := get()
	if err != nil {
		s.logger.Error("layer query failed", slog.String("layer", layerName), slog.String("error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(Error{
			Code:    "LAYER_QUERY_ERROR",
			Message: "Failed to load map layer data",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(raw); err != nil {
		s.logger.Error("failed to write layer response", slog.String("layer", layerName), slog.String("error", err.Error()))
	}
}

// GetCtaRailLinesLayer handles GET /api/v1/layers/cta-rail.
func (s *Server) GetCtaRailLinesLayer(w http.ResponseWriter, r *http.Request) {
	s.writeLayerGeoJSON(w, "cta-rail", func() ([]byte, error) {
		return s.layerReader.CTARailLinesGeoJSON(r.Context())
	})
}

// GetCtaStationsLayer handles GET /api/v1/layers/cta-stations.
func (s *Server) GetCtaStationsLayer(w http.ResponseWriter, r *http.Request) {
	s.writeLayerGeoJSON(w, "cta-stations", func() ([]byte, error) {
		return s.layerReader.CTAStationsGeoJSON(r.Context())
	})
}

// GetBikeRoutesLayer handles GET /api/v1/layers/bike-routes.
func (s *Server) GetBikeRoutesLayer(w http.ResponseWriter, r *http.Request) {
	s.writeLayerGeoJSON(w, "bike-routes", func() ([]byte, error) {
		return s.layerReader.BikeRoutesGeoJSON(r.Context())
	})
}

// GetTrailAccessLayer handles GET /api/v1/layers/trail-access.
func (s *Server) GetTrailAccessLayer(w http.ResponseWriter, r *http.Request) {
	s.writeLayerGeoJSON(w, "trail-access", func() ([]byte, error) {
		return s.layerReader.TrailAccessGeoJSON(r.Context())
	})
}

// GetSupermarketsLayer handles GET /api/v1/layers/supermarkets.
func (s *Server) GetSupermarketsLayer(w http.ResponseWriter, r *http.Request) {
	s.writeLayerGeoJSON(w, "supermarkets", func() ([]byte, error) {
		return s.layerReader.SupermarketsGeoJSON(r.Context())
	})
}
