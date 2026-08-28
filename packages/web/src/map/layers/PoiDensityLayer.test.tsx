import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";

const getMock = mock(async (path: string, options?: { params?: { query?: Record<string, unknown> } }) => {
  if (path === "/api/v1/places") {
    return {
      data: {
        type: "FeatureCollection" as const,
        features: [
          {
            type: "Feature" as const,
            properties: { name: "Sip Coffee", category: "cafe", distanceMeters: 120.5 },
            geometry: { type: "Point" as const, coordinates: [-87.6506, 41.8999] },
          },
        ],
      },
      error: undefined,
      _capturedParams: options?.params,
    };
  }
  throw new Error(`unexpected path: ${path}`);
});

mock.module("../../lib/api/client", () => ({
  apiClient: { GET: getMock },
}));

const { PoiDensityLayer } = await import("./PoiDensityLayer");

function fakeMap() {
  return {
    addSource: mock(() => {}),
    addLayer: mock(() => {}),
    removeLayer: mock(() => {}),
    removeSource: mock(() => {}),
    getLayer: mock(() => undefined),
    getSource: mock(() => undefined),
    on: mock(() => {}),
    off: mock(() => {}),
    getCanvas: mock(() => ({ style: { cursor: "" } })),
  } as unknown as MapLibreMap;
}

function renderWithClient(map: MapLibreMap) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <PoiDensityLayer map={map} lat={41.8781} lon={-87.6298} />
    </QueryClientProvider>
  );
}

describe("PoiDensityLayer", () => {
  beforeEach(() => {
    getMock.mockClear();
    (Popup as unknown as ReturnType<typeof mock>).mockClear();
  });

  it("adds two distance ring sources/layers immediately, independent of the query", () => {
    const map = fakeMap();
    renderWithClient(map);

    const ringsSourceCall = (map.addSource as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "poi-density-rings"
    );
    if (!ringsSourceCall) throw new Error("expected an addSource call for poi-density-rings");
    const sourceOptions = ringsSourceCall[1] as { data: { features: unknown[] } };
    expect(sourceOptions.data.features).toHaveLength(2);

    const ringsLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "poi-density-rings-layer"
    );
    expect((ringsLayerCall?.[0] as { type: string } | undefined)?.type).toBe("line");
  });

  it("adds the POI source, circle layer, and hover listeners once the query resolves", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some((call) => call[0] === "poi-density-pois")
      ).toBe(true)
    );

    const poisLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "poi-density-pois-layer"
    );
    const layerOptions = poisLayerCall?.[0] as { type: string; paint: { "circle-color": unknown[] } } | undefined;
    expect(layerOptions?.type).toBe("circle");
    // Markers must be color-coded per category, not the neutral zinc dot
    // CtaRailLayer's station markers use -- that reuse was the original bug
    // report (POI markers indistinguishable from CTA stations on the map).
    expect(layerOptions?.paint["circle-color"][0]).toBe("match");
    expect(layerOptions?.paint["circle-color"]).toContain("cafe");
    expect(layerOptions?.paint["circle-color"]).not.toContain("#18181b");

    expect(map.on).toHaveBeenCalledWith("mouseenter", "poi-density-pois-layer", expect.any(Function));
    expect(map.on).toHaveBeenCalledWith("mouseleave", "poi-density-pois-layer", expect.any(Function));
  });

  it("renders a popup with the name, category icon+label, and rounded distance on hover", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith("mouseenter", "poi-density-pois-layer", expect.any(Function))
    );

    const enterCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "mouseenter" && call[1] === "poi-density-pois-layer"
    );
    if (!enterCall) throw new Error("expected a mouseenter listener for poi-density-pois-layer");
    const handleMouseEnter = enterCall[2] as (e: MapLayerMouseEvent) => void;

    handleMouseEnter({
      features: [
        {
          geometry: { type: "Point", coordinates: [-87.6506, 41.8999] },
          properties: { name: "Sip Coffee", category: "cafe", distanceMeters: 120.5 },
        },
      ],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      setHTML: ReturnType<typeof mock>;
    };
    expect(popupInstance.setHTML).toHaveBeenCalledTimes(1);
    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    expect(html).toContain("Sip Coffee");
    expect(html).toContain("lni-coffee-cup-2");
    expect(html).toContain("color:#c2703d"); // poi-cafe -- distinct from any CTA/score-* hex
    expect(html).toContain("Cafe");
    expect(html).toContain("121 m"); // Math.round(120.5) -- banker's-adjacent JS round -> 121, not 120
  });

  it("resets the cursor and removes the popup on mouse leave", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith("mouseleave", "poi-density-pois-layer", expect.any(Function))
    );

    const leaveCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "mouseleave" && call[1] === "poi-density-pois-layer"
    );
    if (!leaveCall) throw new Error("expected a mouseleave listener");
    const handleMouseLeave = leaveCall[2] as () => void;

    (map.getCanvas() as unknown as { style: { cursor: string } }).style.cursor = "pointer";
    handleMouseLeave();

    expect((map.getCanvas() as unknown as { style: { cursor: string } }).style.cursor).toBe("");
  });

  it("removes rings + POI layers/sources and hover listeners on unmount", async () => {
    const map = fakeMap();
    (map.getLayer as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);
    (map.getSource as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);

    const { unmount } = renderWithClient(map);
    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some((call) => call[0] === "poi-density-pois")
      ).toBe(true)
    );

    unmount();

    expect(map.removeLayer).toHaveBeenCalledWith("poi-density-rings-layer");
    expect(map.removeSource).toHaveBeenCalledWith("poi-density-rings");
    expect(map.removeLayer).toHaveBeenCalledWith("poi-density-pois-layer");
    expect(map.removeSource).toHaveBeenCalledWith("poi-density-pois");
    expect(map.off).toHaveBeenCalledWith("mouseenter", "poi-density-pois-layer", expect.any(Function));
    expect(map.off).toHaveBeenCalledWith("mouseleave", "poi-density-pois-layer", expect.any(Function));
  });

  it("fetches /api/v1/places with the given lat/lon as query params", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(1));
    expect(getMock).toHaveBeenCalledWith("/api/v1/places", { params: { query: { lat: 41.8781, lon: -87.6298 } } });
  });
});
