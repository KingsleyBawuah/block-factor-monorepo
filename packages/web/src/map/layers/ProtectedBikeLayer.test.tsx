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
  return {
    addSource: mock(() => {}),
    addLayer: mock(() => {}),
    removeLayer: mock(() => {}),
    removeSource: mock(() => {}),
    getLayer: mock(() => undefined),
    getSource: mock(() => undefined),
    on: mock(() => {}),
    off: mock(() => {}),
    getCanvas: mock(() => ({ style: {} })),
  } as unknown as MapLibreMap;
}

function renderWithClient(map: MapLibreMap) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ProtectedBikeLayer map={map} />
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
    const layerOptions = routesLayerCall[0] as { type: string };
    expect(layerOptions.type).toBe("line");
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
    expect((fillLayerCall?.[0] as { type: string } | undefined)?.type).toBe(
      "fill",
    );

    const outlineLayerCall = (
      map.addLayer as ReturnType<typeof mock>
    ).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "trail-access-outline-layer",
    );
    expect((outlineLayerCall?.[0] as { type: string } | undefined)?.type).toBe(
      "line",
    );

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
