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
  const canvas = { style: { cursor: "" } };
  return {
    addSource: mock(() => {}),
    addLayer: mock(() => {}),
    removeLayer: mock(() => {}),
    removeSource: mock(() => {}),
    getLayer: mock(() => undefined),
    getSource: mock(() => undefined),
    setLayoutProperty: mock(() => {}),
    on: mock(() => {}),
    off: mock(() => {}),
    getCanvas: mock(() => canvas),
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
    // Asserts the full paint spec, not just the layer type -- a mutated
    // ring color/width/dasharray would otherwise pass every other test in
    // this file silently.
    expect(ringsLayerCall?.[0]).toEqual({
      id: "poi-density-rings-layer",
      type: "line",
      source: "poi-density-rings",
      layout: { visibility: "visible" },
      paint: {
        "line-color": "#71717a",
        "line-width": 1,
        "line-dasharray": ["literal", [2, 2]],
      },
    });
  });

  describe("distance ring z-order", () => {
    it("places rings below bike routes when both bike routes and CTA rail already loaded", () => {
      const map = fakeMap();
      (map.getLayer as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);
      renderWithClient(map);

      const ringsLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
        (call) => (call[0] as { id: string }).id === "poi-density-rings-layer"
      );
      expect(ringsLayerCall?.[1]).toBe("bike-routes-layer");
    });

    it("places rings below CTA rail when only CTA rail has loaded", () => {
      const map = fakeMap();
      (map.getLayer as ReturnType<typeof mock>).mockImplementation((id: string) =>
        id === "cta-rail-lines-layer" ? ({} as never) : undefined
      );
      renderWithClient(map);

      const ringsLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
        (call) => (call[0] as { id: string }).id === "poi-density-rings-layer"
      );
      expect(ringsLayerCall?.[1]).toBe("cta-rail-lines-layer");
    });

    it("passes no beforeId when neither transit nor bike layers have loaded yet", () => {
      const map = fakeMap(); // getLayer stubbed to always return undefined
      renderWithClient(map);

      const ringsLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
        (call) => (call[0] as { id: string }).id === "poi-density-rings-layer"
      );
      expect(ringsLayerCall?.[1]).toBeUndefined();
    });
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
    // Full paint assertion -- markers must be color-coded per category, not
    // the neutral zinc dot CtaRailLayer's station markers use (that reuse
    // was the original bug report: POI markers indistinguishable from CTA
    // stations on the map).
    expect(poisLayerCall?.[0]).toEqual({
      id: "poi-density-pois-layer",
      type: "circle",
      source: "poi-density-pois",
      layout: { visibility: "visible" },
      paint: {
        "circle-radius": 4,
        "circle-color": [
          "match",
          ["get", "category"],
          "cafe",
          "#c2703d",
          "fitness",
          "#6366f1",
          "supermarket",
          "#0d9488",
          "#71717a",
        ],
        "circle-stroke-width": 1,
        "circle-stroke-color": "#f4f4f5",
      },
    });

    expect(map.on).toHaveBeenCalledWith("mouseenter", "poi-density-pois-layer", expect.any(Function));
    expect(map.on).toHaveBeenCalledWith("mouseleave", "poi-density-pois-layer", expect.any(Function));
  });

  it("renders a popup with the escaped name, category icon+label+color, and rounded distance on hover", async () => {
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
          properties: { name: "Intelligentsia & Co.", category: "cafe", distanceMeters: 120.5 },
        },
      ],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      setLngLat: ReturnType<typeof mock>;
      setHTML: ReturnType<typeof mock>;
      addTo: ReturnType<typeof mock>;
    };
    expect(popupInstance.setLngLat).toHaveBeenCalledWith([-87.6506, 41.8999]);
    expect(popupInstance.setHTML).toHaveBeenCalledTimes(1);
    expect(popupInstance.addTo).toHaveBeenCalledWith(map);

    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    // Name is HTML-escaped -- POI names come from Google Places/OSM data,
    // not user input, but this is cheap insurance against building the
    // habit of interpolating unescaped strings into innerHTML-equivalent
    // APIs (same rationale as escapeHtml.ts itself).
    expect(html).toContain("Intelligentsia &amp; Co.");
    expect(html).not.toContain("Intelligentsia & Co.<");
    expect(html).toContain("lni-coffee-cup-2");
    expect(html).toContain("color:#c2703d"); // poi-cafe -- distinct from any CTA/score-* hex
    expect(html).toContain("Cafe");
    expect(html).toContain("121 m"); // Math.round(120.5) -- banker's-adjacent JS round -> 121, not 120
  });

  it("falls back to the generic marker and coerces a stringified distance for an unrecognized category", async () => {
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
          // Simulates a real hover event where distanceMeters comes back
          // stringified (the same maplibre-gl v5 quirk CtaRailLayer's own
          // tests document for array/boolean properties -- see
          // normalizeStationProperties.ts) and an unrecognized category.
          properties: { name: "", category: "unknown-category", distanceMeters: "120.5" },
        },
      ],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      setHTML: ReturnType<typeof mock>;
    };
    const html = popupInstance.setHTML.mock.calls[0][0] as string;
    expect(html).toContain("lni-map-marker-1");
    expect(html).toContain("color:#71717a");
    expect(html).toContain("Place");
    expect(html).toContain("121 m");
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

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      remove: ReturnType<typeof mock>;
    };
    expect(popupInstance.remove).toHaveBeenCalled();
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
