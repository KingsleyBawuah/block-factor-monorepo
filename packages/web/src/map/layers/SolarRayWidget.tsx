import { useEffect } from "react";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";
import { escapeHtml } from "./escapeHtml";
import { LABEL_FONT_STACK } from "./mapFonts";
import {
  compassBearingDegrees,
  solarBaseScore,
  solarFacingDescription,
  solarRayGeoJSON,
  solarRayHeightMeters,
  solarRayTipPoint,
  solarScoreColor,
} from "./solarRay";
import type { FeatureCollection, PointGeometry } from "../geojson";

const RAY_SOURCE_ID = "solar-ray";
const RAY_LAYER_ID = "solar-ray-layer";
const RAY_GROUND_LAYER_ID = "solar-ray-ground-layer";
const LABEL_SOURCE_ID = "solar-ray-label";
const LABEL_LAYER_ID = "solar-ray-label-layer";

const RAY_LENGTH_METERS = 200;

// A fill-extrusion viewed from directly overhead (pitch 0, the map's
// default) renders indistinguishably from a flat 2D polygon -- the camera
// needs real tilt for "3D" to mean anything visually.
const WIDGET_PITCH = 55;

export type SolarRayWidgetProps = {
  map: MapLibreMap;
  lat: number;
  lon: number;
  /** 8-point compass direction (N/NE/E/SE/S/SW/W/NW), matching the vocabulary
   * docs/openapi.yaml's UnitMetadata.facing and factor_solar.go both use. */
  facing: string;
};

/**
 * Adds the Solar Ray Widget (docs/IMPLEMENTATION_PHASES.md Phase 4
 * deliverable 10) to the given map: a 3D compass orientation ray extruded
 * from the address marker in the unit's window-facing direction. Unlike a
 * plain direction pointer, the ray's color and height both encode the
 * actual S_solar orientation base score (PRD §5.8) -- a bare compass
 * direction means nothing to a viewer who hasn't read the scoring formula,
 * so the ray uses the same score-emerald/lime/amber/rose tiers the rest of
 * the app already uses for scores, and stands taller for a stronger
 * orientation. A persistent label and hover card spell out the direction,
 * score, and what it practically means. Frontend-only -- no backend call,
 * no TanStack Query; the geometry and score-tier math are pure functions
 * in solarRay.ts. Renders nothing itself -- all output is imperative
 * MapLibre sources/layers/popups on `map`, plus a one-time camera tilt so
 * the extrusion is actually visible as 3D.
 */
export function SolarRayWidget({ map, lat, lon, facing }: SolarRayWidgetProps) {
  useEffect(() => {
    if (map.getPitch() === 0) {
      map.easeTo({ pitch: WIDGET_PITCH, duration: 800 });
    }

    const bearing = compassBearingDegrees(facing);
    const score = solarBaseScore(facing);
    const color = solarScoreColor(score);
    const height = solarRayHeightMeters(score);
    const description = solarFacingDescription(facing);

    const rayData = solarRayGeoJSON(lat, lon, bearing, RAY_LENGTH_METERS, 8, height);
    map.addSource(RAY_SOURCE_ID, { type: "geojson", data: rayData });

    // A flat, translucent ground footprint under the extrusion -- a common
    // cartographic trick that helps an extruded shape read as "an object
    // sitting on the map" rather than floating.
    map.addLayer({
      id: RAY_GROUND_LAYER_ID,
      type: "fill",
      source: RAY_SOURCE_ID,
      paint: {
        "fill-color": color,
        "fill-opacity": 0.15,
      },
    });

    map.addLayer({
      id: RAY_LAYER_ID,
      type: "fill-extrusion",
      source: RAY_SOURCE_ID,
      paint: {
        "fill-extrusion-color": color,
        "fill-extrusion-height": ["get", "height"],
        "fill-extrusion-opacity": 0.75,
        "fill-extrusion-vertical-gradient": true,
      },
    });

    // A persistent label (not hover-only, unlike every other layer's
    // popups) -- this is a single one-off widget, not a dense set of many
    // markers needing collision management, so there's no density cost to
    // always showing what the ray means.
    const tip = solarRayTipPoint(lat, lon, bearing, RAY_LENGTH_METERS);
    const labelData: FeatureCollection<PointGeometry, { label: string }> = {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: { label: `${facing.toUpperCase()} · ${score}` },
          geometry: { type: "Point", coordinates: tip },
        },
      ],
    };
    map.addSource(LABEL_SOURCE_ID, { type: "geojson", data: labelData });
    map.addLayer({
      id: LABEL_LAYER_ID,
      type: "symbol",
      source: LABEL_SOURCE_ID,
      layout: {
        "text-field": ["get", "label"],
        "text-font": LABEL_FONT_STACK,
        "text-size": 11,
        "text-anchor": "left",
        "text-offset": [0.6, 0],
      },
      paint: {
        "text-color": color,
        "text-halo-color": "#09090b", // zinc-950, matches DESIGN.md bg-canvas
        "text-halo-width": 1.2,
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
      popup
        .setLngLat(e.lngLat)
        .setHTML(
          `<div class="bf-poi-card">` +
            `<div class="bf-poi-name">${escapeHtml(facing.toUpperCase())}-Facing Windows</div>` +
            `<div class="bf-poi-meta">${escapeHtml(description)}</div>` +
            `<div class="bf-poi-meta">S_solar base score: ${score} / 100</div>` +
            `</div>`
        )
        .addTo(map);
    }

    function handleMouseLeave() {
      map.getCanvas().style.cursor = "";
      popup.remove();
    }

    map.on("mouseenter", RAY_LAYER_ID, handleMouseEnter);
    map.on("mouseleave", RAY_LAYER_ID, handleMouseLeave);

    return () => {
      map.off("mouseenter", RAY_LAYER_ID, handleMouseEnter);
      map.off("mouseleave", RAY_LAYER_ID, handleMouseLeave);
      map.getCanvas().style.cursor = "";
      popup.remove();
      if (map.getLayer(LABEL_LAYER_ID)) map.removeLayer(LABEL_LAYER_ID);
      if (map.getSource(LABEL_SOURCE_ID)) map.removeSource(LABEL_SOURCE_ID);
      if (map.getLayer(RAY_LAYER_ID)) map.removeLayer(RAY_LAYER_ID);
      if (map.getLayer(RAY_GROUND_LAYER_ID)) map.removeLayer(RAY_GROUND_LAYER_ID);
      if (map.getSource(RAY_SOURCE_ID)) map.removeSource(RAY_SOURCE_ID);
    };
  }, [map, lat, lon, facing]);

  return null;
}
