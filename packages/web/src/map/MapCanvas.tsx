import { useEffect, useRef } from "react";
import { Map as MapLibreMap, NavigationControl } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { BASEMAP_STYLE_URL, CHICAGO_CENTER, CHICAGO_DEFAULT_ZOOM } from "./constants";

export type MapCanvasProps = {
  /**
   * Called once the MapLibre instance's `load` event fires (all initial
   * resources, including the basemap style, have downloaded), handing back
   * the live map so later Phase 4 layer components (CTA rail, bike routes,
   * POIs, Divvy docks, solar ray) can call addSource/addLayer/flyTo on it
   * rather than each reimplementing map setup — see
   * docs/IMPLEMENTATION_PHASES.md Phase 4. Deliberately not fired right
   * after construction: addSource/addLayer throw ("Style is not done
   * loading") if called before the style has finished loading, and the
   * remote CARTO style JSON fetch is real, unavoidable async latency.
   */
  onMapReady?: (map: MapLibreMap) => void;
};

/** GPU-accelerated MapLibre GL JS canvas rendering the Chicago basemap. */
export function MapCanvas({ onMapReady }: MapCanvasProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  // Read via a ref inside the effect (below) rather than depending on
  // `onMapReady` directly: it's only needed at construction time, and
  // callers typically pass an inline function, which would otherwise
  // re-trigger the effect (tearing down and rebuilding the map) on every
  // render.
  const onMapReadyRef = useRef(onMapReady);
  onMapReadyRef.current = onMapReady;

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
    map.on("load", () => onMapReadyRef.current?.(map));

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
