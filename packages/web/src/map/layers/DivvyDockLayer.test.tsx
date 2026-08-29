import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
import { Popup } from "maplibre-gl";

const fakeStationFeature = {
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
};

type DivvyDocksResult =
  | { data: { type: "FeatureCollection"; features: (typeof fakeStationFeature)[] }; error: undefined }
  | { data: undefined; error: { message: string } };

const getMock = mock(async (path: string): Promise<DivvyDocksResult> => {
  if (path === "/api/v1/layers/divvy-docks") {
    return {
      data: { type: "FeatureCollection" as const, features: [fakeStationFeature] },
      error: undefined,
    };
  }
  throw new Error(`unexpected path: ${path}`);
});

mock.module("../../lib/api/client", () => ({
  apiClient: { GET: getMock },
}));

// happy-dom (this test environment) has no canvas 2D implementation, so
// createDivvyMarkerIcon() would return null here -- mocked to return a
// real (fake) ImageData so DivvyDockLayer's addImage/hasImage/removeImage
// lifecycle around it can actually be exercised. The drawing math itself
// is covered separately in divvyMarkerIcon.test.ts (via a mocked canvas
// context), not here.
const fakeIcon = { width: 24, height: 24, data: new Uint8ClampedArray(24 * 24 * 4) } as ImageData;
const createDivvyMarkerIconMock = mock(() => fakeIcon);

mock.module("./divvyMarkerIcon", () => ({
  DIVVY_MARKER_ICON_ID: "divvy-dock-marker",
  createDivvyMarkerIcon: createDivvyMarkerIconMock,
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
    setLayoutProperty: mock(() => {}),
    on: mock(() => {}),
    off: mock(() => {}),
    getCanvas: mock(() => canvas),
  } as unknown as MapLibreMap;
}

function renderWithClient(map: MapLibreMap, visible?: boolean) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <DivvyDockLayer map={map} visible={visible} />
    </QueryClientProvider>
  );
}

describe("DivvyDockLayer", () => {
  beforeEach(() => {
    getMock.mockClear();
    createDivvyMarkerIconMock.mockClear();
    (Popup as unknown as ReturnType<typeof mock>).mockClear();
  });

  it("registers the marker icon and adds the source, symbol layer, and hover listeners", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some((call) => call[0] === "divvy-docks")
      ).toBe(true)
    );

    expect(map.addImage).toHaveBeenCalledWith("divvy-dock-marker", fakeIcon);

    const layerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "divvy-docks-layer"
    );
    // Full layout/paint assertion -- a mutated icon-size, text-font,
    // icon-allow-overlap, or text-opacity would otherwise pass every other
    // test in this file silently.
    expect(layerCall?.[0]).toEqual({
      id: "divvy-docks-layer",
      type: "symbol",
      source: "divvy-docks",
      layout: {
        "icon-image": "divvy-dock-marker",
        "icon-size": 0.5,
        visibility: "visible",
        "icon-allow-overlap": true,
        "text-field": ["to-string", ["get", "bikesAvailable"]],
        "text-font": [
          "Montserrat Regular",
          "Open Sans Regular",
          "Noto Sans Regular",
          "HanWangHeiLight Regular",
          "NanumBarunGothic Regular",
        ],
        "text-size": 10,
        "text-offset": [0, 1.1],
        "text-anchor": "top",
      },
      paint: {
        "icon-opacity": ["case", ["all", ["get", "isRenting"], ["get", "isReturning"]], 1, 0.4],
        "text-opacity": ["case", ["all", ["get", "isRenting"], ["get", "isReturning"]], 1, 0.4],
        "text-color": "#f4f4f5",
        "text-halo-color": "#09090b",
        "text-halo-width": 1,
      },
    });

    expect(map.on).toHaveBeenCalledWith("mouseenter", "divvy-docks-layer", expect.any(Function));
    expect(map.on).toHaveBeenCalledWith("mouseleave", "divvy-docks-layer", expect.any(Function));
  });

  it("does not re-register the icon if it's already registered", async () => {
    const map = fakeMap();
    (map.hasImage as ReturnType<typeof mock>).mockReturnValue(true);
    renderWithClient(map);

    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some((call) => call[0] === "divvy-docks")
      ).toBe(true)
    );

    expect(createDivvyMarkerIconMock).not.toHaveBeenCalled();
    expect(map.addImage).not.toHaveBeenCalled();
  });

  it("renders a popup positioned at the feature and with the escaped name, bike/dock counts, and open status on hover", async () => {
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
      setLngLat: ReturnType<typeof mock>;
      setHTML: ReturnType<typeof mock>;
      addTo: ReturnType<typeof mock>;
    };
    expect(popupInstance.setLngLat).toHaveBeenCalledWith([-87.69413, 41.84024]);
    expect(popupInstance.setHTML).toHaveBeenCalledTimes(1);
    expect(popupInstance.addTo).toHaveBeenCalledWith(map);

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

  it("shows closed-for-rebalancing for an asymmetric status (renting but not returning)", async () => {
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

    handleMouseEnter({
      features: [
        {
          geometry: { type: "Point", coordinates: [-87.69413, 41.84024] },
          properties: {
            name: "Full Dock, Not Accepting Returns",
            bikesAvailable: 6,
            ebikesAvailable: 0,
            docksAvailable: 0,
            isRenting: true,
            isReturning: false,
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

  it("falls back to 0 for missing/non-numeric counts instead of rendering NaN", async () => {
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

    handleMouseEnter({
      features: [
        {
          geometry: { type: "Point", coordinates: [-87.69413, 41.84024] },
          properties: {},
        },
      ],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      setHTML: ReturnType<typeof mock>;
    };
    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    expect(html).toContain("0 bikes / 0 e-bikes");
    expect(html).toContain("0 docks open");
    expect(html).not.toContain("NaN");
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

  it("removes the layer, source, icon, and hover listeners on unmount", async () => {
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
    expect(map.removeImage).toHaveBeenCalledWith("divvy-dock-marker");
    expect(map.off).toHaveBeenCalledWith("mouseenter", "divvy-docks-layer", expect.any(Function));
    expect(map.off).toHaveBeenCalledWith("mouseleave", "divvy-docks-layer", expect.any(Function));
  });

  it("creates the layer hidden when visible=false from the start (MapLegend, Phase 5 deliverable 5)", async () => {
    const map = fakeMap();
    renderWithClient(map, false);

    await waitFor(() =>
      expect(
        (map.addLayer as ReturnType<typeof mock>).mock.calls.some(
          (call) => (call[0] as { id: string }).id === "divvy-docks-layer"
        )
      ).toBe(true)
    );
    const layerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "divvy-docks-layer"
    );
    expect((layerCall?.[0] as { layout?: { visibility?: string } })?.layout?.visibility).toBe("none");
  });

  it("toggles the layer's visibility via setLayoutProperty when the visible prop flips after mount", async () => {
    const map = fakeMap();
    (map.getLayer as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { rerender } = render(
      <QueryClientProvider client={queryClient}>
        <DivvyDockLayer map={map} visible={true} />
      </QueryClientProvider>
    );
    await waitFor(() =>
      expect(
        (map.addLayer as ReturnType<typeof mock>).mock.calls.some(
          (call) => (call[0] as { id: string }).id === "divvy-docks-layer"
        )
      ).toBe(true)
    );
    (map.setLayoutProperty as ReturnType<typeof mock>).mockClear();

    rerender(
      <QueryClientProvider client={queryClient}>
        <DivvyDockLayer map={map} visible={false} />
      </QueryClientProvider>
    );

    expect(map.setLayoutProperty).toHaveBeenCalledWith("divvy-docks-layer", "visibility", "none");
  });

  it("fetches from the exact live endpoint path", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(1));
    expect(getMock).toHaveBeenCalledWith("/api/v1/layers/divvy-docks");
  });

  it("adds no source or layer when the query fails", async () => {
    const map = fakeMap();
    getMock.mockImplementationOnce(async () => ({
      data: undefined,
      error: { message: "upstream unavailable" },
    }));
    renderWithClient(map);

    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(1));
    // Give the failed query's effect a tick to (not) run.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(map.addSource).not.toHaveBeenCalled();
    expect(map.addLayer).not.toHaveBeenCalled();
  });
});
