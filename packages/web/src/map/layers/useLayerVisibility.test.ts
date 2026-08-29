import { describe, it, expect, mock } from "bun:test";
import { renderHook } from "@testing-library/react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { useLayerVisibility } from "./useLayerVisibility";

function fakeMap(existingLayerIds: string[] = []) {
  return {
    getLayer: mock((id: string) => (existingLayerIds.includes(id) ? {} : undefined)),
    setLayoutProperty: mock(() => {}),
  } as unknown as MapLibreMap;
}

describe("useLayerVisibility", () => {
  it("sets visibility to 'visible' for every existing layer id when visible is true", () => {
    const map = fakeMap(["layer-a", "layer-b"]);
    renderHook(() => useLayerVisibility(map, ["layer-a", "layer-b"], true));

    expect(map.setLayoutProperty).toHaveBeenCalledWith("layer-a", "visibility", "visible");
    expect(map.setLayoutProperty).toHaveBeenCalledWith("layer-b", "visibility", "visible");
  });

  it("sets visibility to 'none' for every existing layer id when visible is false", () => {
    const map = fakeMap(["layer-a", "layer-b"]);
    renderHook(() => useLayerVisibility(map, ["layer-a", "layer-b"], false));

    expect(map.setLayoutProperty).toHaveBeenCalledWith("layer-a", "visibility", "none");
    expect(map.setLayoutProperty).toHaveBeenCalledWith("layer-b", "visibility", "none");
  });

  it("re-syncs every layer id when visible toggles after they already exist", () => {
    const map = fakeMap(["layer-a"]);
    const { rerender } = renderHook(({ visible }) => useLayerVisibility(map, ["layer-a"], visible), {
      initialProps: { visible: true },
    });
    (map.setLayoutProperty as ReturnType<typeof mock>).mockClear();

    rerender({ visible: false });

    expect(map.setLayoutProperty).toHaveBeenCalledWith("layer-a", "visibility", "none");
  });

  it("skips a layer id that doesn't exist yet, without throwing", () => {
    const map = fakeMap([]); // no layers added yet
    renderHook(() => useLayerVisibility(map, ["not-added-yet"], true));

    expect(map.setLayoutProperty).not.toHaveBeenCalled();
  });
});
