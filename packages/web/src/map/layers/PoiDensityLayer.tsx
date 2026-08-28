import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";
import "lineicons/dist/lineicons.css";
import { apiClient } from "../../lib/api/client";
import { escapeHtml } from "./escapeHtml";
import { poiIcon, poiCircleColorExpression } from "./poi-icons";
import { distanceRingsGeoJSON } from "./distanceRing";
import type { FeatureCollection, PointGeometry } from "../geojson";

const RINGS_SOURCE_ID = "poi-density-rings";
const RINGS_LAYER_ID = "poi-density-rings-layer";
const POIS_SOURCE_ID = "poi-density-pois";
const POIS_LAYER_ID = "poi-density-pois-layer";

// The exact per-address search radius GET /api/v1/places itself queries
// (services/api/internal/scoring/places.go NearbyPlacesRadiusMeters) --
// the outer ring marks the true edge of what could possibly appear, not an
// arbitrary round number.
const OUTER_RING_METERS = 804.672;
const INNER_RING_METERS = OUTER_RING_METERS / 2;

export type PoiDensityLayerProps = {
  map: MapLibreMap;
  /**
   * Coordinate to query GET /api/v1/places around. Currently always a
   * fixed demo coordinate (App.tsx) -- there's no real address-search UI
   * yet (docs/IMPLEMENTATION_PHASES.md Phase 5 deliverable 1).
   */
  lat: number;
  lon: number;
};

/**
 * Adds the POI Density Layer (docs/IMPLEMENTATION_PHASES.md Phase 4
 * deliverable 7) to the given map: cafes, fitness venues, and supermarkets
 * around (lat, lon) from GET /api/v1/places, plus two muted distance
 * reference rings at half and full search radius. Renders nothing itself --
 * all output is imperative MapLibre sources/layers/popups on `map`.
 */
export function PoiDensityLayer({ map, lat, lon }: PoiDensityLayerProps) {
  const placesQuery = useQuery({
    queryKey: ["layers", "places", lat, lon],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/api/v1/places", { params: { query: { lat, lon } } });
      if (error) throw new Error("Failed to load the POI density layer");
      return data;
    },
  });

  useEffect(() => {
    const rings = distanceRingsGeoJSON(lat, lon, [INNER_RING_METERS, OUTER_RING_METERS]);
    map.addSource(RINGS_SOURCE_ID, { type: "geojson", data: rings });
    map.addLayer({
      id: RINGS_LAYER_ID,
      type: "line",
      source: RINGS_SOURCE_ID,
      paint: {
        "line-color": "#71717a", // zinc-500, matches DESIGN.md text-muted -- a spatial reference guide, not scored data, so deliberately not an accent color
        "line-width": 1,
        "line-dasharray": ["literal", [2, 2]],
      },
    });

    return () => {
      if (map.getLayer(RINGS_LAYER_ID)) map.removeLayer(RINGS_LAYER_ID);
      if (map.getSource(RINGS_SOURCE_ID)) map.removeSource(RINGS_SOURCE_ID);
    };
  }, [map, lat, lon]);

  useEffect(() => {
    if (!placesQuery.data) return;

    const data = placesQuery.data as unknown as FeatureCollection<
      PointGeometry,
      { name: string; category: string; distanceMeters: number }
    >;
    map.addSource(POIS_SOURCE_ID, { type: "geojson", data });
    map.addLayer({
      id: POIS_LAYER_ID,
      type: "circle",
      source: POIS_SOURCE_ID,
      paint: {
        "circle-radius": 4,
        // Per-category poi-* color (DESIGN.md "POI Category Markers") --
        // previously a neutral zinc-900 dot identical to CtaRailLayer's
        // station markers, which made the two layers indistinguishable on
        // the map itself (only differed in the hover card).
        "circle-color": poiCircleColorExpression() as never,
        "circle-stroke-width": 1,
        "circle-stroke-color": "#f4f4f5", // zinc-100, matches DESIGN.md text-primary
      },
    });

    const popup = new Popup({
      closeButton: false,
      closeOnClick: false,
      offset: 10,
      className: "blockfactor-popup",
    });

    function handleMouseEnter(e: MapLayerMouseEvent) {
      map.getCanvas().style.cursor = "pointer";
      const feature = e.features?.[0];
      if (feature?.geometry.type !== "Point") return;

      const coords = feature.geometry.coordinates.slice(0, 2) as [number, number];
      const props = feature.properties as { name?: unknown; category?: unknown; distanceMeters?: unknown };
      const name = typeof props.name === "string" ? props.name : "";
      const category = typeof props.category === "string" ? props.category : "";
      const distanceMeters = typeof props.distanceMeters === "number" ? props.distanceMeters : Number(props.distanceMeters);
      const { icon, label, color } = poiIcon(category);

      popup
        .setLngLat(coords)
        .setHTML(
          `<div class="bf-poi-card">` +
            `<div class="bf-poi-name">${escapeHtml(name)}</div>` +
            `<div class="bf-poi-meta"><i class="lni ${icon}" style="color:${color}"></i> ${escapeHtml(label)}</div>` +
            `<div class="bf-poi-meta">${Math.round(distanceMeters)} m</div>` +
            `</div>`
        )
        .addTo(map);
    }

    function handleMouseLeave() {
      map.getCanvas().style.cursor = "";
      popup.remove();
    }

    map.on("mouseenter", POIS_LAYER_ID, handleMouseEnter);
    map.on("mouseleave", POIS_LAYER_ID, handleMouseLeave);

    return () => {
      map.off("mouseenter", POIS_LAYER_ID, handleMouseEnter);
      map.off("mouseleave", POIS_LAYER_ID, handleMouseLeave);
      map.getCanvas().style.cursor = "";
      popup.remove();
      if (map.getLayer(POIS_LAYER_ID)) map.removeLayer(POIS_LAYER_ID);
      if (map.getSource(POIS_SOURCE_ID)) map.removeSource(POIS_SOURCE_ID);
    };
  }, [map, placesQuery.data]);

  return null;
}
