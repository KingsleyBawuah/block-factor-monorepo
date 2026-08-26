import { describe, it, expect } from "bun:test";
import { ctaLineColor, ctaLineTextColor, CTA_LINE_COLORS } from "./cta-colors";

// Independent implementation of the WCAG 2.1 relative-luminance/contrast
// formula (https://www.w3.org/TR/WCAG21/#dfn-relative-luminance), so this
// test verifies the actual accessibility property -- "does the chosen text
// color clear 4.5:1 against this background" -- rather than hardcoding
// which color the implementation happens to pick per line today.
function relativeLuminance(hex: string): number {
  const [r, g, b] = [0, 2, 4].map((i) => Number.parseInt(hex.slice(1 + i, 3 + i), 16) / 255);
  const linear = (c: number) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b);
}

function contrastRatio(hexA: string, hexB: string): number {
  const [lA, lB] = [relativeLuminance(hexA), relativeLuminance(hexB)].sort((a, b) => b - a);
  return (lA + 0.05) / (lB + 0.05);
}

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

describe("ctaLineTextColor", () => {
  it("picks a text color that clears WCAG AA's 4.5:1 contrast ratio against every real CTA line color", () => {
    for (const [line, background] of Object.entries(CTA_LINE_COLORS)) {
      const textColor = ctaLineTextColor(line);
      const ratio = contrastRatio(textColor, background);
      expect(ratio).toBeGreaterThanOrEqual(4.5);
    }
  });

  it("uses white text where it already clears 4.5:1 rather than always defaulting to dark", () => {
    // Confirms the function isn't trivially "always return dark text" --
    // Red/Brown/Purple are dark enough that white text is correct for them.
    expect(ctaLineTextColor("Red")).toBe("#ffffff");
    expect(ctaLineTextColor("Brown")).toBe("#ffffff");
    expect(ctaLineTextColor("Purple")).toBe("#ffffff");
  });

  it("uses dark text for every line light enough that white text fails 4.5:1", () => {
    expect(ctaLineTextColor("Blue")).toBe("#09090b");
    expect(ctaLineTextColor("Green")).toBe("#09090b");
    expect(ctaLineTextColor("Pink")).toBe("#09090b");
    expect(ctaLineTextColor("Orange")).toBe("#09090b");
    expect(ctaLineTextColor("Yellow")).toBe("#09090b");
  });
});
