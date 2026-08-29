import { describe, it, expect } from "bun:test";
import { MAP_LEGEND_ENTRIES } from "./legend-entries";
import { ctaLineColor } from "./cta-colors";
import { BIKE_GREEN } from "./ProtectedBikeLayer";
import { DIVVY_BLUE, DIVVY_GRAY } from "./divvyMarkerIcon";
import { poiIcon } from "./poi-icons";

describe("MAP_LEGEND_ENTRIES", () => {
  it("has exactly one entry per Phase 4 layer, in a stable order", () => {
    expect(MAP_LEGEND_ENTRIES.map((e) => e.key)).toEqual(["ctaRail", "bikeTrail", "poiDensity", "divvyDocks"]);
  });

  it("pulls every swatch color from its layer's own single source of truth, never a re-declared literal", () => {
    const byKey = Object.fromEntries(MAP_LEGEND_ENTRIES.map((e) => [e.key, e]));

    expect(byKey.ctaRail.swatchColors).toEqual([ctaLineColor("Red"), ctaLineColor("Blue")]);
    expect(byKey.bikeTrail.swatchColors).toEqual([BIKE_GREEN]);
    expect(byKey.poiDensity.swatchColors).toEqual([
      poiIcon("cafe").color,
      poiIcon("fitness").color,
      poiIcon("supermarket").color,
    ]);
    expect(byKey.divvyDocks.swatchColors).toEqual([DIVVY_BLUE, DIVVY_GRAY]);
  });

  it("marks Divvy Docks (and only Divvy Docks) as a split-circle swatch, matching divvyMarkerIcon.ts's real two-tone marker", () => {
    const byKey = Object.fromEntries(MAP_LEGEND_ENTRIES.map((e) => [e.key, e]));
    expect(byKey.divvyDocks.splitCircle).toBe(true);
    for (const entry of MAP_LEGEND_ENTRIES) {
      if (entry.key !== "divvyDocks") expect(entry.splitCircle).toBeUndefined();
    }
  });

  it("gives every entry a non-empty label", () => {
    for (const entry of MAP_LEGEND_ENTRIES) {
      expect(entry.label.length).toBeGreaterThan(0);
    }
  });

  it("gives Bike / Trail a Protected-vs-Shared-lane mini-key, explaining ProtectedBikeLayer's real width/dash tiers", () => {
    const bikeTrail = MAP_LEGEND_ENTRIES.find((e) => e.key === "bikeTrail");
    expect(bikeTrail?.lineDetail).toEqual([
      { label: "Protected", weight: "thick" },
      { label: "Shared lane", weight: "thin", dashed: true },
    ]);
  });

  it("gives every other entry no lineDetail", () => {
    for (const entry of MAP_LEGEND_ENTRIES) {
      if (entry.key !== "bikeTrail") expect(entry.lineDetail).toBeUndefined();
    }
  });
});
