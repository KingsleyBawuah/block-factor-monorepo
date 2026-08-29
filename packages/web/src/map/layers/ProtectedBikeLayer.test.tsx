import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";

const getMock = mock(async (path: string) => {
  if (path === "/api/v1/layers/bike-routes") {
    return {
      data: {
        type: "FeatureCollection" as const,
        features: [
          {
            type: "Feature" as const,
            properties: {
              routeType: "Protected Bike Lane",
              streetName: "Milwaukee Ave",
            },
            geometry: {
              type: "MultiLineString" as const,
              coordinates: [
                [
                  [0, 0],
                  [1, 1],
                ],
              ],
            },
          },
        ],
      },
      error: undefined,
    };
  }
  if (path === "/api/v1/layers/trail-access") {
    return {
      data: {
        type: "FeatureCollection" as const,
        features: [
          {
            type: "Feature" as const,
            properties: { name: "The 606" },
            geometry: {
              type: "MultiPolygon" as const,
              coordinates: [
                [
                  [
                    [0, 0],
                    [1, 0],
                    [1, 1],
                    [0, 0],
                  ],
                ],
              ],
            },
          },
        ],
      },
      error: undefined,
    };
  }
  throw new Error(`unexpected path: ${path}`);
});

mock.module("../../lib/api/client", () => ({
  apiClient: { GET: getMock },
}));

const { ProtectedBikeLayer } = await import("./ProtectedBikeLayer");

function fakeMap() {
  // Real MapLibre canvas is one stable DOM node -- getCanvas() must return
  // the same object on every call so a cursor set by one handler is
  // observable via a later getCanvas() call, same as the real API.
  const canvas = { style: { cursor: "" } };
  return {
    addSource: mock(() => {}),
    addLayer: mock(() => {}),
    removeLayer: mock(() => {}),
    removeSource: mock(() => {}),
    moveLayer: mock(() => {}),
    getLayer: mock(() => undefined),
    getSource: mock(() => undefined),
    setLayoutProperty: mock(() => {}),
    on: mock(() => {}),
    off: mock(() => {}),
    getCanvas: mock(() => canvas),
  } as unknown as MapLibreMap;
}

function renderWithClient(map: MapLibreMap, visible?: boolean) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ProtectedBikeLayer map={map} visible={visible} />
    </QueryClientProvider>,
  );
}

describe("ProtectedBikeLayer", () => {
  beforeEach(() => {
    getMock.mockClear();
    (Popup as unknown as ReturnType<typeof mock>).mockClear();
  });

  it("adds the bike routes source and a line layer once the query resolves", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some(
          (call) => call[0] === "bike-routes",
        ),
      ).toBe(true),
    );

    const routesLayerCall = (
      map.addLayer as ReturnType<typeof mock>
    ).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "bike-routes-layer",
    );
    if (!routesLayerCall)
      throw new Error("expected an addLayer call for bike-routes-layer");
    // Asserts the full paint/layout expressions, not just the layer type --
    // a typo in the routeType match/case keys (wrong width/dash tier) would
    // otherwise pass every other test in this file silently.
    expect(routesLayerCall[0]).toEqual({
      id: "bike-routes-layer",
      type: "line",
      source: "bike-routes",
      layout: { "line-join": "round", "line-cap": "round", visibility: "visible" },
      paint: {
        "line-color": "#10b981",
        "line-width": [
          "match",
          ["get", "routeType"],
          "Protected Bike Lane",
          3,
          "Buffered Bike Lane",
          2.5,
          "Neighborhood Greenway",
          2,
          1.5,
        ],
        "line-opacity": ["match", ["get", "routeType"], "Marked Shared Lane", 0.6, 1],
        "line-dasharray": [
          "case",
          ["==", ["get", "routeType"], "Marked Shared Lane"],
          ["literal", [2, 1.5]],
          ["literal", [1, 0]],
        ],
      },
    });
  });

  it("moves itself below the CTA rail layer when CTA rail already loaded first", async () => {
    const map = fakeMap();
    (map.getLayer as ReturnType<typeof mock>).mockImplementation((id: string) =>
      id === "cta-rail-lines-layer" ? ({} as never) : undefined,
    );
    renderWithClient(map);

    await waitFor(() =>
      expect(
        (map.addLayer as ReturnType<typeof mock>).mock.calls.some(
          (call) => (call[0] as { id: string }).id === "bike-routes-layer",
        ),
      ).toBe(true),
    );

    // Elevated CTA track and street-level bike lanes sometimes physically
    // coincide (parts of the Loop) -- without an explicit z-order, one
    // layer non-deterministically hides the other depending on which
    // async query resolves last.
    expect(map.moveLayer).toHaveBeenCalledWith("bike-routes-layer", "cta-rail-lines-layer");
  });

  it("does not call moveLayer when CTA rail has not loaded yet (a plain addLayer already puts CTA on top once it does)", async () => {
    const map = fakeMap(); // getLayer stubbed to always return undefined
    renderWithClient(map);

    await waitFor(() =>
      expect(
        (map.addLayer as ReturnType<typeof mock>).mock.calls.some(
          (call) => (call[0] as { id: string }).id === "bike-routes-layer",
        ),
      ).toBe(true),
    );

    expect(map.moveLayer).not.toHaveBeenCalled();
  });

  it("adds the trail access source, fill+outline layers, and hover listeners", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some(
          (call) => call[0] === "trail-access",
        ),
      ).toBe(true),
    );

    const fillLayerCall = (
      map.addLayer as ReturnType<typeof mock>
    ).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "trail-access-fill-layer",
    );
    expect(fillLayerCall?.[0]).toEqual({
      id: "trail-access-fill-layer",
      type: "fill",
      source: "trail-access",
      layout: { visibility: "visible" },
      paint: { "fill-color": "#10b981", "fill-opacity": 0.15 },
    });

    const outlineLayerCall = (
      map.addLayer as ReturnType<typeof mock>
    ).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "trail-access-outline-layer",
    );
    expect(outlineLayerCall?.[0]).toEqual({
      id: "trail-access-outline-layer",
      type: "line",
      source: "trail-access",
      layout: { visibility: "visible" },
      paint: { "line-color": "#10b981", "line-width": 1.5 },
    });

    expect(map.on).toHaveBeenCalledWith(
      "mouseenter",
      "trail-access-fill-layer",
      expect.any(Function),
    );
    expect(map.on).toHaveBeenCalledWith(
      "mouseleave",
      "trail-access-fill-layer",
      expect.any(Function),
    );
  });

  it("renders a popup with the escaped trail name on hover", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith(
        "mouseenter",
        "trail-access-fill-layer",
        expect.any(Function),
      ),
    );

    const enterCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) =>
        call[0] === "mouseenter" && call[1] === "trail-access-fill-layer",
    );
    if (!enterCall)
      throw new Error(
        "expected a mouseenter listener for trail-access-fill-layer",
      );
    const handleMouseEnter = enterCall[2] as (e: MapLayerMouseEvent) => void;

    handleMouseEnter({
      lngLat: { lng: -87.68, lat: 41.91 },
      features: [{ properties: { name: "O'Brien Trail" } }],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock
      .results[0]?.value as {
      setHTML: ReturnType<typeof mock>;
    };
    expect(popupInstance.setHTML).toHaveBeenCalledTimes(1);
    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    expect(html).toContain("O&#39;Brien Trail");
  });

  it("falls back to a generic label when a trail feature has no name", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith(
        "mouseenter",
        "trail-access-fill-layer",
        expect.any(Function),
      ),
    );

    const enterCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) =>
        call[0] === "mouseenter" && call[1] === "trail-access-fill-layer",
    );
    if (!enterCall) throw new Error("expected a mouseenter listener");
    const handleMouseEnter = enterCall[2] as (e: MapLayerMouseEvent) => void;

    handleMouseEnter({
      lngLat: { lng: -87.68, lat: 41.91 },
      features: [{ properties: {} }],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock
      .results[0]?.value as { setHTML: ReturnType<typeof mock> };
    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    expect(html).toContain("Trail access");
  });

  it("resets the cursor and removes the popup on mouse leave", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith(
        "mouseleave",
        "trail-access-fill-layer",
        expect.any(Function),
      ),
    );

    const enterCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) =>
        call[0] === "mouseenter" && call[1] === "trail-access-fill-layer",
    );
    const leaveCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) =>
        call[0] === "mouseleave" && call[1] === "trail-access-fill-layer",
    );
    if (!enterCall || !leaveCall)
      throw new Error("expected mouseenter and mouseleave listeners");
    const handleMouseEnter = enterCall[2] as (e: MapLayerMouseEvent) => void;
    const handleMouseLeave = leaveCall[2] as () => void;

    handleMouseEnter({
      lngLat: { lng: -87.68, lat: 41.91 },
      features: [{ properties: { name: "The 606" } }],
    } as unknown as MapLayerMouseEvent);
    expect((map.getCanvas() as unknown as { style: { cursor: string } }).style.cursor).toBe(
      "pointer",
    );

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock
      .results[0]?.value as { remove: ReturnType<typeof mock> };

    handleMouseLeave();

    expect((map.getCanvas() as unknown as { style: { cursor: string } }).style.cursor).toBe("");
    expect(popupInstance.remove).toHaveBeenCalled();
  });

  it("removes both layers/sources and hover listeners on unmount", async () => {
    const map = fakeMap();
    (map.getLayer as ReturnType<typeof mock>).mockImplementation(
      () => ({}) as never,
    );
    (map.getSource as ReturnType<typeof mock>).mockImplementation(
      () => ({}) as never,
    );

    const { unmount } = renderWithClient(map);
    await waitFor(() => expect(map.addSource).toHaveBeenCalledTimes(2));

    unmount();

    expect(map.removeLayer).toHaveBeenCalledWith("bike-routes-layer");
    expect(map.removeSource).toHaveBeenCalledWith("bike-routes");
    expect(map.removeLayer).toHaveBeenCalledWith("trail-access-fill-layer");
    expect(map.removeLayer).toHaveBeenCalledWith("trail-access-outline-layer");
    expect(map.removeSource).toHaveBeenCalledWith("trail-access");
    expect(map.off).toHaveBeenCalledWith(
      "mouseenter",
      "trail-access-fill-layer",
      expect.any(Function),
    );
    expect(map.off).toHaveBeenCalledWith(
      "mouseleave",
      "trail-access-fill-layer",
      expect.any(Function),
    );
  });

  it("creates all three layers hidden when visible=false from the start (MapLegend, Phase 5 deliverable 5)", async () => {
    const map = fakeMap();
    renderWithClient(map, false);

    await waitFor(() => expect(map.addLayer).toHaveBeenCalledTimes(3));
    for (const id of ["bike-routes-layer", "trail-access-fill-layer", "trail-access-outline-layer"]) {
      const call = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
        (call) => (call[0] as { id: string }).id === id,
      );
      expect((call?.[0] as { layout?: { visibility?: string } })?.layout?.visibility).toBe("none");
    }
  });

  it("toggles all three layers' visibility via setLayoutProperty when the visible prop flips after mount", async () => {
    const map = fakeMap();
    (map.getLayer as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { rerender } = render(
      <QueryClientProvider client={queryClient}>
        <ProtectedBikeLayer map={map} visible={true} />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(map.addLayer).toHaveBeenCalledTimes(3));
    (map.setLayoutProperty as ReturnType<typeof mock>).mockClear();

    rerender(
      <QueryClientProvider client={queryClient}>
        <ProtectedBikeLayer map={map} visible={false} />
      </QueryClientProvider>,
    );

    expect(map.setLayoutProperty).toHaveBeenCalledWith("bike-routes-layer", "visibility", "none");
    expect(map.setLayoutProperty).toHaveBeenCalledWith("trail-access-fill-layer", "visibility", "none");
    expect(map.setLayoutProperty).toHaveBeenCalledWith("trail-access-outline-layer", "visibility", "none");
  });

  it("fetches from the exact live endpoint paths", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(2));
    const calledPaths = getMock.mock.calls.map((call) => call[0]).sort();
    expect(calledPaths).toEqual([
      "/api/v1/layers/bike-routes",
      "/api/v1/layers/trail-access",
    ]);
  });
});
