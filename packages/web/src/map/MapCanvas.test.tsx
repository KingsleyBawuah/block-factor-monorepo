import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render } from "@testing-library/react";
import { StrictMode } from "react";

// The shared preload (src/test/setup.ts) already mocks maplibre-gl with a
// safe no-op baseline. This file overrides it with a richer mock (capturing
// constructor options and per-instance spies) since these tests need to
// assert on how MapCanvas actually calls the library, not just that it
// doesn't crash.
const removeMock = mock(() => {});
const addControlMock = mock((_control: unknown, _position?: string) => {});
const onMock = mock((_event: string, _handler: () => void) => {});
const setPaintPropertyMock = mock((_layer: string, _prop: string, _value: unknown) => {});
const MapConstructorMock = mock((options: unknown) => ({
  options,
  addControl: addControlMock,
  on: onMock,
  remove: removeMock,
  setPaintProperty: setPaintPropertyMock,
}));

/** Finds and invokes the handler MapCanvas registered for `event` via `map.on(...)`. */
function fireMapEvent(event: string) {
  const call = onMock.mock.calls.find((c) => c[0] === event);
  if (!call) throw new Error(`expected map.on("${event}", ...) to have been called`);
  (call[1] as () => void)();
}

mock.module("maplibre-gl", () => ({
  Map: MapConstructorMock,
  NavigationControl: mock((options: unknown) => ({ options })),
}));

const { MapCanvas } = await import("./MapCanvas");
const { BASEMAP_STYLE_URL, CHICAGO_CENTER, CHICAGO_DEFAULT_ZOOM, WATER_FILL_COLOR } = await import("./constants");

describe("MapCanvas", () => {
  beforeEach(() => {
    MapConstructorMock.mockClear();
    addControlMock.mockClear();
    onMock.mockClear();
    removeMock.mockClear();
    setPaintPropertyMock.mockClear();
  });

  it("renders a container with an accessible label", () => {
    const { getByRole } = render(<MapCanvas />);
    expect(getByRole("application", { name: "Chicago livability map" })).toBeTruthy();
  });

  it("initializes MapLibre with the container element, Chicago basemap, center, and zoom", () => {
    const { getByRole } = render(<MapCanvas />);
    expect(MapConstructorMock).toHaveBeenCalledTimes(1);
    const options = MapConstructorMock.mock.calls[0][0] as {
      container: HTMLElement;
      style: string;
      center: unknown;
      zoom: number;
    };
    expect(options.container).toBe(getByRole("application", { name: "Chicago livability map" }));
    expect(options.style).toBe(BASEMAP_STYLE_URL);
    expect(options.center).toEqual(CHICAGO_CENTER);
    expect(options.zoom).toBe(CHICAGO_DEFAULT_ZOOM);
  });

  it("adds a top-right navigation control with the compass enabled", () => {
    render(<MapCanvas />);
    expect(addControlMock).toHaveBeenCalledTimes(1);
    const [, position] = addControlMock.mock.calls[0];
    expect(position).toBe("top-right");
  });

  it("removes the map instance on unmount to avoid a WebGL context leak", () => {
    const { unmount } = render(<MapCanvas />);
    unmount();
    expect(removeMock).toHaveBeenCalledTimes(1);
  });

  it("does not call onMapReady until the map's 'load' event fires", () => {
    // Regression guard: addSource/addLayer throw ("Style is not done
    // loading") if called before the style has finished loading, so
    // onMapReady must never fire based on construction alone.
    const onMapReady = mock((_map: unknown) => {});
    render(<MapCanvas onMapReady={onMapReady} />);
    expect(onMapReady).not.toHaveBeenCalled();
  });

  it("hands the live map instance to onMapReady once 'load' fires", () => {
    const onMapReady = mock((_map: unknown) => {});
    render(<MapCanvas onMapReady={onMapReady} />);

    fireMapEvent("load");

    expect(onMapReady).toHaveBeenCalledTimes(1);
    expect(onMapReady.mock.calls[0][0]).toBe(MapConstructorMock.mock.results[0].value);
  });

  it("overrides the basemap's water fill color once 'load' fires", () => {
    render(<MapCanvas />);

    fireMapEvent("load");

    expect(setPaintPropertyMock).toHaveBeenCalledWith("water", "fill-color", WATER_FILL_COLOR);
  });

  it("survives a React StrictMode mount-unmount-remount cycle without leaking", () => {
    const { unmount } = render(
      <StrictMode>
        <MapCanvas />
      </StrictMode>
    );
    // StrictMode double-invokes effects in dev: mount, cleanup, mount again.
    expect(MapConstructorMock).toHaveBeenCalledTimes(2);
    expect(removeMock).toHaveBeenCalledTimes(1);

    unmount();
    expect(removeMock).toHaveBeenCalledTimes(2);
  });
});
