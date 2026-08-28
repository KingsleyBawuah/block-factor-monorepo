import { describe, it, expect } from "bun:test";
import { poiIcon, poiCircleColorExpression } from "./poi-icons";

describe("poiIcon", () => {
  it("maps each known category to a distinct icon class and color", () => {
    const cafe = poiIcon("cafe");
    const fitness = poiIcon("fitness");
    const supermarket = poiIcon("supermarket");

    expect(cafe.icon).toBe("lni-coffee-cup-2");
    expect(fitness.icon).toBe("lni-dumbbell-1");
    expect(supermarket.icon).toBe("lni-basket-shopping-3");

    const icons = new Set([cafe.icon, fitness.icon, supermarket.icon]);
    expect(icons.size).toBe(3);

    // Colors must be distinct from each other and never reuse a CTA line
    // or score-* hex (DESIGN.md "Category Color Independence Rule").
    const reservedHexes = new Set([
      "#c60c30",
      "#00a1de",
      "#62361b",
      "#009b3a",
      "#e27ea6",
      "#f9461c",
      "#522398",
      "#f9e300", // CTA lines
      "#10b981",
      "#84cc16",
      "#f59e0b",
      "#f43f5e", // score-*
    ]);
    const colors = new Set([cafe.color, fitness.color, supermarket.color]);
    expect(colors.size).toBe(3);
    for (const c of colors) {
      expect(reservedHexes.has(c)).toBe(false);
    }
  });

  it("falls back to a generic marker for an unrecognized category", () => {
    const unknown = poiIcon("something-unexpected");
    expect(unknown.icon).toBe("lni-map-marker-1");
    expect(unknown.label).toBe("Place");
    expect(unknown.color).toBe("#71717a");
  });
});

describe("poiCircleColorExpression", () => {
  it("builds a match expression whose colors agree with poiIcon for every known category", () => {
    const expr = poiCircleColorExpression();
    expect(expr[0]).toBe("match");

    for (const category of ["cafe", "fitness", "supermarket"]) {
      const idx = expr.indexOf(category);
      expect(idx).toBeGreaterThan(0);
      expect(expr[idx + 1]).toBe(poiIcon(category).color);
    }
  });
});
