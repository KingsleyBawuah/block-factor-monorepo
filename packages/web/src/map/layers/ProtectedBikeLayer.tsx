import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";
import { apiClient } from "../../lib/api/client";
import { escapeHtml } from "./escapeHtml";
import { RAIL_LAYER_ID as CTA_RAIL_LAYER_ID } from "./CtaRailLayer";
import type {
  FeatureCollection,
  MultiLineStringGeometry,
  MultiPolygonGeometry,
} from "../geojson";

type BikeRouteProperties = {
  routeType:
    | "Protected Bike Lane"
    | "Neighborhood Greenway"
    | "Buffered Bike Lane"
    | "Bike Lane"
    | "Marked Shared Lane";
  streetName: string | null;
};

type TrailAccessProperties = {
  name: string;
};

const ROUTES_SOURCE_ID = "bike-routes";
// Exported so other layers (PoiDensityLayer's distance rings) can position
// themselves relative to this layer -- same pattern as CtaRailLayer's
// exported RAIL_LAYER_ID (docs/ARCHITECTURE.md Decision Log row 33).
export const ROUTES_LAYER_ID = "bike-routes-layer";
const TRAIL_SOURCE_ID = "trail-access";
const TRAIL_FILL_LAYER_ID = "trail-access-fill-layer";
const TRAIL_OUTLINE_LAYER_ID = "trail-access-outline-layer";

// score-emerald (DESIGN.md) -- deliberately not CTA Green Line's #009b3a,
// which would read as a transit line rather than bike infrastructure.
const BIKE_GREEN = "#10b981";

export type ProtectedBikeLayerProps = {
  map: MapLibreMap;
};

/**
 * Adds the Protected Bike Layer (docs/IMPLEMENTATION_PHASES.md Phase 4
 * deliverable 5) to the given map: a high-visibility green linestring
 * overlay for street bike infrastructure, weighted by the real quality
 * gradient across `routeType` (DATA_SOURCES.md "Bike Routes" -- protected/
 * buffered lanes render bolder than painted lanes/marked shared lanes),
 * plus off-street trail (linear park) boundaries with hover cards. Renders
 * nothing itself -- all output is imperative MapLibre sources/layers/popups
 * on `map`.
 */
export function ProtectedBikeLayer({ map }: ProtectedBikeLayerProps) {
  const routesQuery = useQuery({
    queryKey: ["layers", "bike-routes"],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/api/v1/layers/bike-routes");
      if (error) throw new Error("Failed to load the bike routes layer");
      return data;
    },
  });

  const trailQuery = useQuery({
    queryKey: ["layers", "trail-access"],
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        "/api/v1/layers/trail-access",
      );
      if (error) throw new Error("Failed to load the trail access layer");
      return data;
    },
  });

  useEffect(() => {
    if (!routesQuery.data) return;

    const data = routesQuery.data as unknown as FeatureCollection<
      MultiLineStringGeometry,
      BikeRouteProperties
    >;
    map.addSource(ROUTES_SOURCE_ID, { type: "geojson", data });
    map.addLayer({
      id: ROUTES_LAYER_ID,
      type: "line",
      source: ROUTES_SOURCE_ID,
      layout: { "line-join": "round", "line-cap": "round" },
      paint: {
        "line-color": BIKE_GREEN,
        "line-width": [
          "match",
          ["get", "routeType"],
          "Protected Bike Lane",
          3,
          "Buffered Bike Lane",
          2.5,
          "Neighborhood Greenway",
          2,
          1.5, // Bike Lane / Marked Shared Lane -- painted-only, lowest tier
        ],
        "line-opacity": [
          "match",
          ["get", "routeType"],
          "Marked Shared Lane",
          0.6,
          1,
        ],
        // Marked Shared Lane ("sharrow") is just a painted arrow marking a
        // shared travel lane, not a dedicated bike facility -- dashed to
        // distinguish it from the other 4, all of which are real lanes.
        "line-dasharray": [
          "case",
          ["==", ["get", "routeType"], "Marked Shared Lane"],
          ["literal", [2, 1.5]],
          ["literal", [1, 0]],
        ],
      },
    });

    // Real elevated CTA track and street-level bike lanes sometimes
    // physically coincide (parts of the Loop) -- without an explicit
    // z-order here, whichever layer's async query happened to resolve
    // last would render on top, non-deterministically hiding the other.
    // A plain addLayer() always places a new layer at the very top, which
    // already puts CTA rail above bike routes when CTA loads second; this
    // handles the other ordering (CTA already loaded first) by explicitly
    // moving bike routes below it. Bike routes are usually wider (up to
    // 3px vs. CTA's fixed 2px) so some color still peeks out on either
    // side where they coincide -- a real but partial fix, since the
    // narrowest bike tier (1.5px, Bike Lane/Marked Shared Lane) can still
    // be mostly covered. True pixel-perfect separation would need
    // cross-dataset overlap detection to offset one line off the other's
    // centerline (the same technique expandRailSegments.ts already uses
    // for multiple CTA lines sharing one segment, extended to a second,
    // independently-sourced dataset) -- see docs/ARCHITECTURE.md Decision
    // Log row 33 for why that's out of scope here.
    if (map.getLayer(CTA_RAIL_LAYER_ID)) {
      map.moveLayer(ROUTES_LAYER_ID, CTA_RAIL_LAYER_ID);
    }

    return () => {
      if (map.getLayer(ROUTES_LAYER_ID)) map.removeLayer(ROUTES_LAYER_ID);
      if (map.getSource(ROUTES_SOURCE_ID)) map.removeSource(ROUTES_SOURCE_ID);
    };
  }, [map, routesQuery.data]);

  useEffect(() => {
    if (!trailQuery.data) return;

    const data = trailQuery.data as unknown as FeatureCollection<
      MultiPolygonGeometry,
      TrailAccessProperties
    >;
    map.addSource(TRAIL_SOURCE_ID, { type: "geojson", data });
    map.addLayer({
      id: TRAIL_FILL_LAYER_ID,
      type: "fill",
      source: TRAIL_SOURCE_ID,
      paint: { "fill-color": BIKE_GREEN, "fill-opacity": 0.15 },
    });
    map.addLayer({
      id: TRAIL_OUTLINE_LAYER_ID,
      type: "line",
      source: TRAIL_SOURCE_ID,
      paint: { "line-color": BIKE_GREEN, "line-width": 1.5 },
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
      if (!feature) return;

      const name = String(feature.properties?.name ?? "Trail access");
      popup
        .setLngLat(e.lngLat)
        .setHTML(
          `<div class="bf-station-card"><div class="bf-station-name">${escapeHtml(name)}</div></div>`,
        )
        .addTo(map);
    }

    function handleMouseLeave() {
      map.getCanvas().style.cursor = "";
      popup.remove();
    }

    map.on("mouseenter", TRAIL_FILL_LAYER_ID, handleMouseEnter);
    map.on("mouseleave", TRAIL_FILL_LAYER_ID, handleMouseLeave);

    return () => {
      map.off("mouseenter", TRAIL_FILL_LAYER_ID, handleMouseEnter);
      map.off("mouseleave", TRAIL_FILL_LAYER_ID, handleMouseLeave);
      // Unmounting mid-hover skips handleMouseLeave (its mouseleave event
      // never fires), which would otherwise leave the cursor stuck as
      // "pointer" -- reset it here too.
      map.getCanvas().style.cursor = "";
      popup.remove();
      if (map.getLayer(TRAIL_OUTLINE_LAYER_ID))
        map.removeLayer(TRAIL_OUTLINE_LAYER_ID);
      if (map.getLayer(TRAIL_FILL_LAYER_ID))
        map.removeLayer(TRAIL_FILL_LAYER_ID);
      if (map.getSource(TRAIL_SOURCE_ID)) map.removeSource(TRAIL_SOURCE_ID);
    };
  }, [map, trailQuery.data]);

  return null;
}
