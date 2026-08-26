import { describe, it, expect } from "bun:test";
import { ctaLineColor, CTA_LINE_COLORS } from "./cta-colors";

describe("ctaLineColor", () => {
  it("returns the official hex color for each real CTA line", () => {
    expect(ctaLineColor("Red")).toBe("#c60c30");
    expect(ctaLineColor("Blue")).toBe("#00a1de");
    expect(ctaLineColor("Brown")).toBe("#62361b");
    expect(ctaLineColor("Green")).toBe("#009b3a");
    expect(ctaLineColor("Pink")).toBe("#e27ea6");
    expect(ctaLineColor("Orange")).toBe("#f9461c");
    expect(ctaLineColor("Purple")).toBe("#522398");
    expect(ctaLineColor("Yellow")).toBe("#f9e300");
  });

  it("falls back to zinc-500 for an unrecognized line name", () => {
    expect(ctaLineColor("Not A Real Line")).toBe("#71717a");
  });

  it("covers every line CTA_LINE_COLORS declares", () => {
    for (const [line, color] of Object.entries(CTA_LINE_COLORS)) {
      expect(ctaLineColor(line)).toBe(color);
    }
  });
});
