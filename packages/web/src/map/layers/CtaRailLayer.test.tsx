import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Map as MapLibreMap, MapLayerMouseEvent } from "maplibre-gl";
// Same mocked constructor src/test/setup.ts installs for every test file --
// imported here (not re-mocked) so this file can inspect the Popup
// instance CtaRailLayer actually constructs and calls.
import { Popup } from "maplibre-gl";

const getMock = mock(async (path: string) => {
  if (path === "/api/v1/layers/cta-rail") {
    return {
      data: {
        type: "FeatureCollection" as const,
        features: [
          {
            type: "Feature" as const,
            properties: { lines: ["Brown", "Purple"], express: true, grade: "elevated_or_at_grade" as const },
            geometry: { type: "MultiLineString" as const, coordinates: [[[0, 0], [1, 1]]] },
          },
        ],
      },
      error: undefined,
    };
  }
  if (path === "/api/v1/layers/cta-stations") {
    return {
      data: {
        type: "FeatureCollection" as const,
        features: [
          {
            type: "Feature" as const,
            properties: { name: "Fullerton", lines: ["Red", "Brown", "Purple"], ada: true },
            geometry: { type: "Point" as const, coordinates: [-87.65, 41.93] },
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

const { CtaRailLayer } = await import("./CtaRailLayer");

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
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <CtaRailLayer map={map} />
    </QueryClientProvider>
  );
}

describe("CtaRailLayer", () => {
  beforeEach(() => {
    getMock.mockClear();
    (Popup as unknown as ReturnType<typeof mock>).mockClear();
  });

  it("adds the rail line source and layer once the query resolves", async () => {
    const map = fakeMap();
    renderWithClient(map);

    // Both queries (rail + stations) resolve around the same tick, so
    // addSource/addLayer each fire twice in total (once per layer) -- find
    // the rail-specific calls rather than asserting a raw total count.
    await waitFor(() =>
      expect(
        (map.addSource as ReturnType<typeof mock>).mock.calls.some((call) => call[0] === "cta-rail-lines")
      ).toBe(true)
    );

    const railSourceCall = (map.addSource as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "cta-rail-lines"
    );
    if (!railSourceCall) throw new Error("expected an addSource call for cta-rail-lines");
    const sourceOptions = railSourceCall[1] as { data: { features: unknown[] } };
    // The one 2-line source feature (Brown+Purple) expands into 2 rendered features.
    expect(sourceOptions.data.features).toHaveLength(2);

    const railLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "cta-rail-lines-layer"
    );
    if (!railLayerCall) throw new Error("expected an addLayer call for cta-rail-lines-layer");
    const layerOptions = railLayerCall[0] as { type: string };
    expect(layerOptions.type).toBe("line");
  });

  it("adds the stations source, circle layer, and hover listeners", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() => expect(map.addSource).toHaveBeenCalledTimes(2));

    const stationsCall = (map.addSource as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "cta-stations"
    );
    expect(stationsCall).toBeTruthy();

    const stationsLayerCall = (map.addLayer as ReturnType<typeof mock>).mock.calls.find(
      (call) => (call[0] as { id: string }).id === "cta-stations-layer"
    );
    expect(stationsLayerCall).toBeDefined();
    expect((stationsLayerCall?.[0] as { type: string } | undefined)?.type).toBe("circle");

    expect(map.on).toHaveBeenCalledWith("mouseenter", "cta-stations-layer", expect.any(Function));
    expect(map.on).toHaveBeenCalledWith("mouseleave", "cta-stations-layer", expect.any(Function));
  });

  it("renders a popup with the station name, colored+escaped line badges, and ADA status on hover", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() =>
      expect(map.on).toHaveBeenCalledWith("mouseenter", "cta-stations-layer", expect.any(Function))
    );

    const enterCall = (map.on as ReturnType<typeof mock>).mock.calls.find(
      (call) => call[0] === "mouseenter" && call[1] === "cta-stations-layer"
    );
    if (!enterCall) throw new Error("expected a mouseenter listener for cta-stations-layer");
    const handleMouseEnter = enterCall[2] as (e: MapLayerMouseEvent) => void;

    handleMouseEnter({
      features: [
        {
          geometry: { type: "Point", coordinates: [-87.65, 41.93] },
          // Simulates the pinned maplibre-gl v5 behavior confirmed against
          // its own v5-to-v6 migration guide: array/boolean properties can
          // come back JSON-stringified from a real hover event.
          properties: { name: "O'Hare", lines: '["Blue"]', ada: "true" },
        },
      ],
    } as unknown as MapLayerMouseEvent);

    const popupInstance = (Popup as unknown as ReturnType<typeof mock>).mock.results[0]?.value as {
      setHTML: ReturnType<typeof mock>;
    };
    expect(popupInstance.setHTML).toHaveBeenCalledTimes(1);
    const html = popupInstance.setHTML.mock.calls[0][0] as string;

    // Name is HTML-escaped (finding: unescaped interpolation into setHTML).
    expect(html).toContain("O&#39;Hare");
    // Stringified `lines`/`ada` were correctly normalized back to a real
    // array/boolean, not left as literal JSON text in the card.
    expect(html).toContain(">Blue<");
    expect(html).not.toContain("[&quot;Blue&quot;]");
    expect(html).toContain("ADA accessible");
    // Blue fails WCAG AA 4.5:1 with white text -- badge must use dark text.
    expect(html).toContain("background-color:#00a1de;color:#09090b");
  });

  it("removes both layers/sources and hover listeners on unmount", async () => {
    const map = fakeMap();
    (map.getLayer as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);
    (map.getSource as ReturnType<typeof mock>).mockImplementation(() => ({}) as never);

    const { unmount } = renderWithClient(map);
    await waitFor(() => expect(map.addSource).toHaveBeenCalledTimes(2));

    unmount();

    expect(map.removeLayer).toHaveBeenCalledWith("cta-rail-lines-layer");
    expect(map.removeSource).toHaveBeenCalledWith("cta-rail-lines");
    expect(map.removeLayer).toHaveBeenCalledWith("cta-stations-layer");
    expect(map.removeSource).toHaveBeenCalledWith("cta-stations");
    expect(map.off).toHaveBeenCalledWith("mouseenter", "cta-stations-layer", expect.any(Function));
    expect(map.off).toHaveBeenCalledWith("mouseleave", "cta-stations-layer", expect.any(Function));
  });

  it("fetches from the exact live endpoint paths", async () => {
    const map = fakeMap();
    renderWithClient(map);

    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(2));
    const calledPaths = getMock.mock.calls.map((call) => call[0]).sort();
    expect(calledPaths).toEqual(["/api/v1/layers/cta-rail", "/api/v1/layers/cta-stations"]);
  });
});
