import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render, cleanup } from "@testing-library/react";

// happy-dom has no WebGL context, so the real maplibre-gl Map constructor
// would throw. Mocked here rather than via --preload since only this file
// (and App.test.tsx) render map-bearing components.
const removeMock = mock(() => {});
const addControlMock = mock(() => {});
const MapConstructorMock = mock((options: unknown) => ({
  options,
  addControl: addControlMock,
  remove: removeMock,
}));

mock.module("maplibre-gl", () => ({
  Map: MapConstructorMock,
  NavigationControl: mock(() => ({})),
}));

const { MapCanvas } = await import("./MapCanvas");
const { BASEMAP_STYLE_URL, CHICAGO_CENTER, CHICAGO_DEFAULT_ZOOM } = await import("./constants");

describe("MapCanvas", () => {
  beforeEach(() => {
    MapConstructorMock.mockClear();
    addControlMock.mockClear();
    removeMock.mockClear();
  });

  it("renders a container with an accessible label", () => {
    const { getByRole } = render(<MapCanvas />);
    expect(getByRole("application", { name: "Chicago livability map" })).toBeTruthy();
    cleanup();
  });

  it("initializes MapLibre with the Chicago basemap, center, and zoom", () => {
    render(<MapCanvas />);
    expect(MapConstructorMock).toHaveBeenCalledTimes(1);
    const options = MapConstructorMock.mock.calls[0][0] as {
      style: string;
      center: unknown;
      zoom: number;
    };
    expect(options.style).toBe(BASEMAP_STYLE_URL);
    expect(options.center).toEqual(CHICAGO_CENTER);
    expect(options.zoom).toBe(CHICAGO_DEFAULT_ZOOM);
    cleanup();
  });

  it("adds navigation controls", () => {
    render(<MapCanvas />);
    expect(addControlMock).toHaveBeenCalledTimes(1);
    cleanup();
  });

  it("removes the map instance on unmount to avoid a WebGL context leak", () => {
    const { unmount } = render(<MapCanvas />);
    unmount();
    expect(removeMock).toHaveBeenCalledTimes(1);
  });
});
