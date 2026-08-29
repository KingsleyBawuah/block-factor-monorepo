import { useEffect } from "react";
import type { Map as MapLibreMap } from "maplibre-gl";

/**
 * Shared visibility-toggle sync for every Phase 4 layer component
 * (MapLegend, Phase 5 deliverable 5): keeps every id in `layerIds` synced
 * to `visible` via `setLayoutProperty` whenever it changes. `map.getLayer`
 * guards the case where a layer hasn't been added yet (its own
 * data-loading effect hasn't resolved) -- that effect is expected to set
 * the correct *initial* visibility itself (via a local `useRef(visible)`
 * the caller owns, so reading it there doesn't need `visible` in that
 * effect's own dependency array -- toggling visibility must never tear
 * down and rebuild the layer/source), so this hook only has to handle
 * later toggles against an already-existing layer.
 */
export function useLayerVisibility(map: MapLibreMap, layerIds: readonly string[], visible: boolean) {
  useEffect(() => {
    for (const id of layerIds) {
      if (map.getLayer(id)) {
        map.setLayoutProperty(id, "visibility", visible ? "visible" : "none");
      }
    }
  }, [map, visible, layerIds]);
}
