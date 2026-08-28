import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render } from "@testing-library/react";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";
import { SolarRayWidget } from "./SolarRayWidget";

function fakeMap(pitch = 0) {
  const canvas = { style: { cursor: "" } };
  return {
    addSource: mock(() => {}),
    addLayer: mock(() => {}),
    removeLayer: mock(() => {}),
    removeSource: mock(() => {}),
    getLayer: mock(() => undefined),
    getSource: mock(() => undefined),
    getPitch: mock(() => pitch),
    easeTo: mock(() => {}),
    on: mock(() => {}),
    off: mock(() => {}),
    getCanvas: mock(() => canvas),
  } as unknown as MapLibreMap;
}

describe("SolarRayWidget", () => {
  beforeEach(() => {
    (Popup as unknown as ReturnType<typeof mock>).mockClear();
  });

  it("tilts the camera when the map starts flat", () => {
    const map = fakeMap(0);
    render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="W" />);

    expect(map.easeTo).toHaveBeenCalledWith(expect.objectContaining({ pitch: 55 }));
  });

  it("does not re-tilt the camera if it's already pitched (e.g. the user already tilted it)", () => {
    const map = fakeMap(40);
    render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="W" />);

    expect(map.easeTo).not.toHaveBeenCalled();
  });

  it("colors and sizes the ray by the S_solar score for a mid-tier direction (West -> lime)", () => {
    const map = fakeMap();
    render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="W" />);

    const sourceCall = (map.addSource as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "solar-ray"
    );
    if (!sourceCall) throw new Error("expected an addSource call for solar-ray");
    const sourceOptions = sourceCall[1] as { data: { features: { properties: { height: number } }[] } };
    // West's base score is 75 -> height = 10 + (75/100) * 35 = 36.25.
    expect(sourceOptions.data.features[0].properties.height).toBeCloseTo(36.25, 5);

    const rayLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "solar-ray-layer"
    );
    const rayLayerOptions = rayLayerCall?.[0] as
      | { type: string; source: string; paint: Record<string, unknown> }
      | undefined;
    expect(rayLayerOptions?.type).toBe("fill-extrusion");
    expect(rayLayerOptions?.source).toBe("solar-ray");
    expect(rayLayerOptions?.paint["fill-extrusion-height"]).toEqual(["get", "height"]);
    // score-lime (65-79 tier) -- 75 falls in this band, not emerald/amber/rose.
    expect(rayLayerOptions?.paint["fill-extrusion-color"]).toBe("#84cc16");

    const groundLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "solar-ray-ground-layer"
    );
    expect((groundLayerCall?.[0] as { type: string; paint: Record<string, unknown> } | undefined)?.type).toBe(
      "fill"
    );
    expect(
      (groundLayerCall?.[0] as { paint: Record<string, unknown> } | undefined)?.paint["fill-color"]
    ).toBe("#84cc16");
  });

  it("colors the ray score-rose for the worst orientation (North)", () => {
    const map = fakeMap();
    render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="N" />);

    const rayLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "solar-ray-layer"
    );
    const paint = (rayLayerCall?.[0] as { paint: Record<string, unknown> } | undefined)?.paint;
    expect(paint?.["fill-extrusion-color"]).toBe("#f43f5e");
  });

  it("colors the ray score-emerald for the best orientation (South)", () => {
    const map = fakeMap();
    render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="S" />);

    const rayLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "solar-ray-layer"
    );
    const paint = (rayLayerCall?.[0] as { paint: Record<string, unknown> } | undefined)?.paint;
    expect(paint?.["fill-extrusion-color"]).toBe("#10b981");
  });

  it("adds a persistent text label at the ray's tip showing the direction and score", () => {
    const map = fakeMap();
    render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="W" />);

    const labelSourceCall = (map.addSource as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "solar-ray-label"
    );
    if (!labelSourceCall) throw new Error("expected an addSource call for solar-ray-label");
    const labelData = (labelSourceCall[1] as { data: { features: { properties: { label: string } }[] } }).data;
    expect(labelData.features[0].properties.label).toBe("W · 75");

    const labelLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "solar-ray-label-layer"
    );
    const labelLayerOptions = labelLayerCall?.[0] as
      | { type: string; layout: Record<string, unknown> }
      | undefined;
    expect(labelLayerOptions?.type).toBe("symbol");
    expect(labelLayerOptions?.layout["text-field"]).toEqual(["get", "label"]);
  });

  it("renders a popup explaining the direction, description, and score on hover", () => {
    const map = fakeMap();
    render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="W" />);

    const enterCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "mouseenter" && call[1] === "solar-ray-layer"
    );
    if (!enterCall) throw new Error("expected a mouseenter listener for solar-ray-layer");
    const handleMouseEnter = enterCall[2] as (e: MapLayerMouseEvent) => void;

    handleMouseEnter({ lngLat: { lng: -87.6298, lat: 41.8781 } } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      setHTML: ReturnType<typeof mock>;
    };
    expect(popupInstance.setHTML).toHaveBeenCalledTimes(1);
    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    expect(html).toContain("W-Facing Windows");
    expect(html).toContain("Afternoon light");
    expect(html).toContain("75 / 100");
  });

  it("resets the cursor and removes the popup on mouse leave", () => {
    const map = fakeMap();
    render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="W" />);

    const leaveCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "mouseleave" && call[1] === "solar-ray-layer"
    );
    if (!leaveCall) throw new Error("expected a mouseleave listener");
    const handleMouseLeave = leaveCall[2] as () => void;

    (map.getCanvas() as unknown as { style: { cursor: string } }).style.cursor = "pointer";
    handleMouseLeave();

    expect((map.getCanvas() as unknown as { style: { cursor: string } }).style.cursor).toBe("");
    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      remove: ReturnType<typeof mock>;
    };
    expect(popupInstance.remove).toHaveBeenCalled();
  });

  it("removes both ray layers, the label layer, and both sources on unmount", () => {
    const map = fakeMap();
    (map.getLayer as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);
    (map.getSource as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);

    const { unmount } = render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="W" />);
    unmount();

    expect(map.removeLayer).toHaveBeenCalledWith("solar-ray-layer");
    expect(map.removeLayer).toHaveBeenCalledWith("solar-ray-ground-layer");
    expect(map.removeLayer).toHaveBeenCalledWith("solar-ray-label-layer");
    expect(map.removeSource).toHaveBeenCalledWith("solar-ray");
    expect(map.removeSource).toHaveBeenCalledWith("solar-ray-label");
  });

  it("rebuilds the ray when facing changes", () => {
    const map = fakeMap();
    const { rerender } = render(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="W" />);
    (map.addSource as ReturnType<typeof mock>).mockClear();

    rerender(<SolarRayWidget map={map} lat={41.8781} lon={-87.6298} facing="S" />);

    expect(map.addSource).toHaveBeenCalledTimes(2); // ray source + label source
  });
});
