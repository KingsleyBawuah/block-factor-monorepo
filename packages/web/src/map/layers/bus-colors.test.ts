import { describe, it, expect } from "bun:test";
import { busCategoryColor, busCategoryLabel } from "./bus-colors";

describe("busCategoryColor", () => {
  it.each([
    ["local", "#99999c"],
    ["express", "#b71234"],
    ["jump", "#0065bd"],
  ])("maps %s to its real CTA GTFS color %s", (category, expectedColor) => {
    expect(busCategoryColor(category)).toBe(expectedColor);
  });

  it("falls back to the local/grey color for an unrecognized category", () => {
    expect(busCategoryColor("nightOwl")).toBe("#99999c");
    expect(busCategoryColor("")).toBe("#99999c");
  });
});

describe("busCategoryLabel", () => {
  it.each([
    ["local", "Local bus"],
    ["express", "Express bus"],
    ["jump", "Jeffery Jump"],
  ])("labels %s as %s", (category, expectedLabel) => {
    expect(busCategoryLabel(category)).toBe(expectedLabel);
  });

  it("falls back to a generic label for an unrecognized category", () => {
    expect(busCategoryLabel("nightOwl")).toBe("Bus");
  });
});
