import { useEffect, useRef } from "react";
import { Map as MapLibreMap, NavigationControl } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { BASEMAP_STYLE_URL, CHICAGO_CENTER, CHICAGO_DEFAULT_ZOOM } from "./constants";

/**
 * GPU-accelerated MapLibre GL JS canvas. Spatial layers (CTA rail, bike
 * routes, POIs, Divvy docks, solar ray) mount on top of this in later
 * Phase 4 PRs — see docs/IMPLEMENTATION_PHASES.md Phase 4.
 */
export function MapCanvas() {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<MapLibreMap | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container || mapRef.current) return;

    const map = new MapLibreMap({
      container,
      style: BASEMAP_STYLE_URL,
      center: CHICAGO_CENTER,
      zoom: CHICAGO_DEFAULT_ZOOM,
    });
    map.addControl(new NavigationControl({ showCompass: true }), "top-right");
    mapRef.current = map;

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  return (
    <div
      ref={containerRef}
      className="h-full w-full"
      role="application"
      aria-label="Chicago livability map"
    />
  );
}
