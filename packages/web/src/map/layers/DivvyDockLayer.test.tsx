import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";

const getMock = mock(async (path: string) => {
  if (path === "/api/v1/layers/divvy-docks") {
    return {
      data: {
        type: "FeatureCollection" as const,
        features: [
          {
            type: "Feature" as const,
            properties: {
              name: "California Blvd & 29th St",
              capacity: 8,
              bikesAvailable: 2,
              ebikesAvailable: 1,
              docksAvailable: 5,
              isRenting: true,
              isReturning: true,
            },
            geometry: { type: "Point" as const, coordinates: [-87.69413, 41.84024] },
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

const { DivvyDockLayer } = await import("./DivvyDockLayer");

function fakeMap() {
  const canvas = { style: { cursor: "" } };
  const images = new Set<string>();
  return {
    addSource: mock(() => {}),
    addLayer: mock(() => {}),
    removeLayer: mock(() => {}),
    removeSource: mock(() => {}),
    addImage: mock((id: string) => {
      images.add(id);
    }),
    removeImage: mock((id: string) => {
      images.delete(id);
    }),
    hasImage: mock((id: string) => images.has(id)),
    getLayer: mock(() => undefined),
    getSource: mock(() => undefined),
    on: mock(() => {}),
    off: mock(() => {}),
    getCanvas: mock(() => canvas),
  } as unknown as MapLibreMap;
}

function renderWithClient(map: MapLibreMap) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <DivvyDockLayer map={map} />
    </QueryClientProvider>
  );
}

describe("DivvyDockLayer", () => {
  beforeEach(() => {
    getMock.mockClear();
    (Popup as unknown as ReturnType<typeof mock>).mockClear();
  });

  it("registers the marker icon once and adds the source, symbol layer, and hover listeners", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some((call) => call[0] === "divvy-docks")
      ).toBe(true)
    );

    // happy-dom (this test environment) has no canvas 2D implementation,
    // so createDivvyMarkerIcon() returns null here and addImage is never
    // actually called with real image data -- this only asserts the layer
    // wires icon-image to the expected id, not the icon's pixel content.
    const layerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "divvy-docks-layer"
    );
    const layerOptions = layerCall?.[0] as
      | { type: string; layout: Record<string, unknown>; paint: Record<string, unknown> }
      | undefined;
    expect(layerOptions?.type).toBe("symbol");
    expect(layerOptions?.layout["icon-image"]).toBe("divvy-dock-marker");
    expect(layerOptions?.layout["text-field"]).toEqual(["to-string", ["get", "bikesAvailable"]]);
    expect(layerOptions?.paint["icon-opacity"]).toEqual([
      "case",
      ["all", ["get", "isRenting"], ["get", "isReturning"]],
      1,
      0.4,
    ]);

    expect(map.on).toHaveBeenCalledWith("mouseenter", "divvy-docks-layer", expect.any(Function));
    expect(map.on).toHaveBeenCalledWith("mouseleave", "divvy-docks-layer", expect.any(Function));
  });

  it("renders a popup with the escaped name, bike/dock counts, and open status on hover", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith("mouseenter", "divvy-docks-layer", expect.any(Function))
    );

    const enterCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "mouseenter" && call[1] === "divvy-docks-layer"
    );
    if (!enterCall) throw new Error("expected a mouseenter listener for divvy-docks-layer");
    const handleMouseEnter = enterCall[2] as (e: MapLayerMouseEvent) => void;

    handleMouseEnter({
      features: [
        {
          geometry: { type: "Point", coordinates: [-87.69413, 41.84024] },
          properties: {
            name: "Foster & Sheridan",
            bikesAvailable: 3,
            ebikesAvailable: 2,
            docksAvailable: 4,
            isRenting: true,
            isReturning: true,
          },
        },
      ],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      setHTML: ReturnType<typeof mock>;
    };
    expect(popupInstance.setHTML).toHaveBeenCalledTimes(1);
    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    expect(html).toContain("Foster &amp; Sheridan");
    expect(html).toContain("3 bikes / 2 e-bikes");
    expect(html).toContain("4 docks open");
    expect(html).toContain("Open");
  });

  it("shows a closed-for-rebalancing status when the station isn't renting or returning", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith("mouseenter", "divvy-docks-layer", expect.any(Function))
    );

    const enterCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "mouseenter" && call[1] === "divvy-docks-layer"
    );
    if (!enterCall) throw new Error("expected a mouseenter listener");
    const handleMouseEnter = enterCall[2] as (e: MapLayerMouseEvent) => void;

    // Simulates the real maplibre-gl v5 stringified-boolean quirk
    // (normalizeStationProperties.ts documents the same behavior for
    // CtaRailLayer) alongside a genuine closed station.
    handleMouseEnter({
      features: [
        {
          geometry: { type: "Point", coordinates: [-87.69413, 41.84024] },
          properties: {
            name: "Damen Ave & Ogden Ave",
            bikesAvailable: 0,
            ebikesAvailable: 0,
            docksAvailable: 0,
            isRenting: "false",
            isReturning: "false",
          },
        },
      ],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      setHTML: ReturnType<typeof mock>;
    };
    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    expect(html).toContain("Closed for rebalancing");
  });

  it("resets the cursor and removes the popup on mouse leave", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith("mouseleave", "divvy-docks-layer", expect.any(Function))
    );

    const leaveCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "mouseleave" && call[1] === "divvy-docks-layer"
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

  it("removes the layer, source, and hover listeners on unmount", async () => {
    const map = fakeMap();
    (map.getLayer as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);
    (map.getSource as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);

    const { unmount } = renderWithClient(map);
    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some((call) => call[0] === "divvy-docks")
      ).toBe(true)
    );

    unmount();

    expect(map.removeLayer).toHaveBeenCalledWith("divvy-docks-layer");
    expect(map.removeSource).toHaveBeenCalledWith("divvy-docks");
    expect(map.off).toHaveBeenCalledWith("mouseenter", "divvy-docks-layer", expect.any(Function));
    expect(map.off).toHaveBeenCalledWith("mouseleave", "divvy-docks-layer", expect.any(Function));
  });

  it("fetches from the exact live endpoint path", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(1));
    expect(getMock).toHaveBeenCalledWith("/api/v1/layers/divvy-docks");
  });
});
