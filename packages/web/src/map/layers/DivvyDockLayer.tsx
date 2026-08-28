import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";
import "lineicons/dist/lineicons.css";
import { apiClient } from "../../lib/api/client";
import { escapeHtml } from "./escapeHtml";
import { createDivvyMarkerIcon, DIVVY_MARKER_ICON_ID } from "./divvyMarkerIcon";
import type { FeatureCollection, PointGeometry } from "../geojson";

const DOCKS_SOURCE_ID = "divvy-docks";
const DOCKS_LAYER_ID = "divvy-docks-layer";

// The exact font stack CARTO's dark-matter-gl-style basemap itself already
// uses for its own labels (verified live against the style's own JSON,
// https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json) --
// text-font must reference a stack the style's glyphs endpoint actually
// serves, or the label silently fails to render.
const LABEL_FONT_STACK = [
  "Montserrat Regular",
  "Open Sans Regular",
  "Noto Sans Regular",
  "HanWangHeiLight Regular",
  "NanumBarunGothic Regular",
];

type DivvyDockProperties = {
  name: string;
  capacity: number;
  bikesAvailable: number;
  ebikesAvailable: number;
  docksAvailable: number;
  isRenting: boolean;
  isReturning: boolean;
};

export type DivvyDockLayerProps = {
  map: MapLibreMap;
};

/**
 * Adds the Divvy Dock Layer (docs/IMPLEMENTATION_PHASES.md Phase 4
 * deliverable 9) to the given map: live bike-share station markers with a
 * live bikes-available count badge, from GET /api/v1/layers/divvy-docks.
 * Display-only -- Divvy is not a scored factor. Renders nothing itself --
 * all output is imperative MapLibre sources/layers/popups on `map`.
 */
export function DivvyDockLayer({ map }: DivvyDockLayerProps) {
  const docksQuery = useQuery({
    queryKey: ["layers", "divvy-docks"],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/api/v1/layers/divvy-docks");
      if (error) throw new Error("Failed to load the Divvy dock layer");
      return data;
    },
  });

  useEffect(() => {
    if (!docksQuery.data) return;

    if (!map.hasImage(DIVVY_MARKER_ICON_ID)) {
      const icon = createDivvyMarkerIcon();
      if (icon) map.addImage(DIVVY_MARKER_ICON_ID, icon);
    }

    const data = docksQuery.data as unknown as FeatureCollection<PointGeometry, DivvyDockProperties>;
    map.addSource(DOCKS_SOURCE_ID, { type: "geojson", data });
    map.addLayer({
      id: DOCKS_LAYER_ID,
      type: "symbol",
      source: DOCKS_SOURCE_ID,
      layout: {
        "icon-image": DIVVY_MARKER_ICON_ID,
        "icon-size": 0.5,
        // Every dock renders regardless of density (matches how CtaRailLayer's
        // circle-layer station markers behave -- circle layers have no
        // collision system to opt into in the first place) -- only the text
        // labels below use MapLibre's default collision-based hiding, so a
        // dense cluster still shows every marker, just fewer numeric badges
        // at low zoom.
        "icon-allow-overlap": true,
        "text-field": ["to-string", ["get", "bikesAvailable"]],
        "text-font": LABEL_FONT_STACK,
        "text-size": 10,
        "text-offset": [0, 1.1],
        "text-anchor": "top",
      },
      paint: {
        // A station that's installed but not currently renting/returning
        // (closed for rebalancing) dims rather than disappears -- it's
        // still a real dock, just temporarily unusable.
        "icon-opacity": ["case", ["all", ["get", "isRenting"], ["get", "isReturning"]], 1, 0.4],
        // Dims in lockstep with the icon -- otherwise a closed station's
        // bikes-available number stayed fully opaque while its marker
        // faded, looking like two contradictory signals for one station.
        "text-opacity": ["case", ["all", ["get", "isRenting"], ["get", "isReturning"]], 1, 0.4],
        "text-color": "#f4f4f5", // zinc-100, matches DESIGN.md text-primary
        "text-halo-color": "#09090b", // zinc-950, matches DESIGN.md bg-canvas
        "text-halo-width": 1,
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
      const props = (feature.properties ?? {}) as Record<string, unknown>;
      const name = typeof props.name === "string" ? props.name : "";
      const toCount = (value: unknown) => {
        const n = Number(value);
        return Number.isFinite(n) ? n : 0;
      };
      const bikesAvailable = toCount(props.bikesAvailable);
      const ebikesAvailable = toCount(props.ebikesAvailable);
      const docksAvailable = toCount(props.docksAvailable);
      const isRenting = props.isRenting === true || props.isRenting === "true";
      const isReturning = props.isReturning === true || props.isReturning === "true";

      popup
        .setLngLat(coords)
        .setHTML(
          `<div class="bf-poi-card">` +
            `<div class="bf-poi-name">${escapeHtml(name)}</div>` +
            `<div class="bf-poi-meta"><i class="lni lni-bike"></i> ${bikesAvailable} bikes / ${ebikesAvailable} e-bikes</div>` +
            `<div class="bf-poi-meta">${docksAvailable} docks open</div>` +
            `<div class="bf-poi-meta">${isRenting && isReturning ? "Open" : "Closed for rebalancing"}</div>` +
            `</div>`
        )
        .addTo(map);
    }

    function handleMouseLeave() {
      map.getCanvas().style.cursor = "";
      popup.remove();
    }

    map.on("mouseenter", DOCKS_LAYER_ID, handleMouseEnter);
    map.on("mouseleave", DOCKS_LAYER_ID, handleMouseLeave);

    return () => {
      map.off("mouseenter", DOCKS_LAYER_ID, handleMouseEnter);
      map.off("mouseleave", DOCKS_LAYER_ID, handleMouseLeave);
      map.getCanvas().style.cursor = "";
      popup.remove();
      if (map.getLayer(DOCKS_LAYER_ID)) map.removeLayer(DOCKS_LAYER_ID);
      if (map.getSource(DOCKS_SOURCE_ID)) map.removeSource(DOCKS_SOURCE_ID);
      if (map.hasImage(DIVVY_MARKER_ICON_ID)) map.removeImage(DIVVY_MARKER_ICON_ID);
    };
  }, [map, docksQuery.data]);

  return null;
}
