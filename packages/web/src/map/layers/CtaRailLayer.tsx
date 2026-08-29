import { useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";
import "lineicons/dist/lineicons.css";
import { apiClient } from "../../lib/api/client";
import { expandRailSegments } from "./expandRailSegments";
import { ctaLineColor, ctaLineTextColor } from "./cta-colors";
import { normalizeStationProperties, type CtaStationProperties } from "./normalizeStationProperties";
import { escapeHtml } from "./escapeHtml";
import { useLayerVisibility } from "./useLayerVisibility";
import type { FeatureCollection, PointGeometry } from "../geojson";

const RAIL_SOURCE_ID = "cta-rail-lines";
// Exported so ProtectedBikeLayer can position itself relative to this layer
// (docs/ARCHITECTURE.md Decision Log row 33) -- real elevated track and
// street-level bike lanes sometimes physically coincide (e.g. parts of the
// Loop), and without an explicit z-order the two layers' paint order was
// whichever async query happened to resolve last.
export const RAIL_LAYER_ID = "cta-rail-lines-layer";
const STATIONS_SOURCE_ID = "cta-stations";
const STATIONS_LAYER_ID = "cta-stations-layer";

export type CtaRailLayerProps = {
  map: MapLibreMap;
  /** MapLegend's checkbox state for this layer (Phase 5 deliverable 5) -- defaults on, matching every other layer's default-visible behavior. */
  visible?: boolean;
};

/**
 * Adds the CTA Rail Layer (docs/IMPLEMENTATION_PHASES.md Phase 4
 * deliverable 4) to the given map: official line colors on the track
 * segments, and interactive station hover cards. Renders nothing itself --
 * all output is imperative MapLibre sources/layers/popups on `map`.
 */
export function CtaRailLayer({ map, visible = true }: CtaRailLayerProps) {
  useLayerVisibility(map, [RAIL_LAYER_ID, STATIONS_LAYER_ID], visible);
  const visibleRef = useRef(visible);
  visibleRef.current = visible;

  const railQuery = useQuery({
    queryKey: ["layers", "cta-rail"],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/api/v1/layers/cta-rail");
      if (error) throw new Error("Failed to load the CTA rail line layer");
      return data;
    },
  });

  const stationsQuery = useQuery({
    queryKey: ["layers", "cta-stations"],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/api/v1/layers/cta-stations");
      if (error) throw new Error("Failed to load the CTA stations layer");
      return data;
    },
  });

  useEffect(() => {
    if (!railQuery.data) return;

    const expanded = expandRailSegments(railQuery.data);
    map.addSource(RAIL_SOURCE_ID, { type: "geojson", data: expanded });
    map.addLayer({
      id: RAIL_LAYER_ID,
      type: "line",
      source: RAIL_SOURCE_ID,
      layout: {
        "line-join": "round",
        "line-cap": "round",
        visibility: visibleRef.current ? "visible" : "none",
      },
      paint: {
        "line-color": ["get", "color"],
        "line-width": 2,
        "line-offset": ["get", "offset"],
        // Subway (underground) segments render dashed; elevated/at-grade
        // segments render solid -- a common transit-map convention for
        // "this track isn't actually visible at street level," distinct
        // from the per-line color encoding. line-dasharray requires an
        // array in every branch (confirmed against the installed style
        // spec's reference/v8.json: cross-faded-data-driven properties can
        // only take literal array values, selected per-feature via case/
        // match -- there's no "omit for solid" option), so the solid
        // branch uses a zero-length gap ([1, 0]) rather than a bare [1],
        // which is actually a fine 1-unit dash / 1-unit gap dotted pattern.
        "line-dasharray": ["case", ["==", ["get", "grade"], "subway"], ["literal", [2, 1.5]], ["literal", [1, 0]]],
      },
    });

    return () => {
      if (map.getLayer(RAIL_LAYER_ID)) map.removeLayer(RAIL_LAYER_ID);
      if (map.getSource(RAIL_SOURCE_ID)) map.removeSource(RAIL_SOURCE_ID);
    };
  }, [map, railQuery.data]);

  useEffect(() => {
    if (!stationsQuery.data) return;

    const data = stationsQuery.data as unknown as FeatureCollection<PointGeometry, CtaStationProperties>;
    map.addSource(STATIONS_SOURCE_ID, { type: "geojson", data });
    map.addLayer({
      id: STATIONS_LAYER_ID,
      type: "circle",
      source: STATIONS_SOURCE_ID,
      layout: { visibility: visibleRef.current ? "visible" : "none" },
      paint: {
        "circle-radius": 2.5,
        "circle-color": "#18181b", // zinc-900, matches DESIGN.md surface-card
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
      const props = normalizeStationProperties(feature.properties as Record<string, unknown>);

      const badges = props.lines
        .map(
          (line) =>
            `<span class="bf-line-badge" style="background-color:${ctaLineColor(line)};color:${ctaLineTextColor(
              line
            )}">${escapeHtml(line)}</span>`
        )
        .join("");

      popup
        .setLngLat(coords)
        .setHTML(
          `<div class="bf-station-card">` +
            `<div class="bf-station-name">${escapeHtml(props.name)}</div>` +
            `<div class="bf-station-lines">${badges}</div>` +
            `<div class="bf-station-ada">${
              props.ada
                ? '<i class="lni lni-wheelchair-1"></i> ADA accessible'
                : "Not ADA accessible"
            }</div>` +
            `</div>`
        )
        .addTo(map);
    }

    function handleMouseLeave() {
      map.getCanvas().style.cursor = "";
      popup.remove();
    }

    map.on("mouseenter", STATIONS_LAYER_ID, handleMouseEnter);
    map.on("mouseleave", STATIONS_LAYER_ID, handleMouseLeave);

    return () => {
      map.off("mouseenter", STATIONS_LAYER_ID, handleMouseEnter);
      map.off("mouseleave", STATIONS_LAYER_ID, handleMouseLeave);
      popup.remove();
      if (map.getLayer(STATIONS_LAYER_ID)) map.removeLayer(STATIONS_LAYER_ID);
      if (map.getSource(STATIONS_SOURCE_ID)) map.removeSource(STATIONS_SOURCE_ID);
    };
  }, [map, stationsQuery.data]);

  return null;
}
