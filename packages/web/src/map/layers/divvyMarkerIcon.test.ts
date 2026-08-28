import { describe, it, expect } from "bun:test";
import { createDivvyMarkerIcon } from "./divvyMarkerIcon";

describe("createDivvyMarkerIcon", () => {
  it("returns null when no 2D canvas context is available", () => {
    // happy-dom (this project's test environment) has no canvas 2D
    // implementation -- getContext("2d") returns null there, confirmed
    // live. This is exactly the fallback path DivvyDockLayer relies on to
    // skip map.addImage gracefully rather than throw; every real browser
    // supports canvas 2D, so the drawing path itself isn't exercised here.
    expect(createDivvyMarkerIcon()).toBeNull();
  });
});
