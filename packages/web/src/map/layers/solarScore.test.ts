import { describe, it, expect } from "bun:test";
import { solarBaseScore, solarScoreTier } from "./solarScore";

describe("solarBaseScore", () => {
  it("matches factor_solar.go's exact orientation base-score table", () => {
    expect(solarBaseScore("S")).toBe(100);
    expect(solarBaseScore("SE")).toBe(92.5);
    expect(solarBaseScore("SW")).toBe(87.5);
    expect(solarBaseScore("E")).toBe(85);
    expect(solarBaseScore("W")).toBe(75);
    expect(solarBaseScore("NE")).toBe(62.5);
    expect(solarBaseScore("NW")).toBe(57.5);
    expect(solarBaseScore("N")).toBe(40);
  });

  it("is case-insensitive", () => {
    expect(solarBaseScore("w")).toBe(75);
  });

  it("falls back to 100, matching factor_solar.go's own baseScore default", () => {
    expect(solarBaseScore("nonsense")).toBe(100);
  });

  it("trims surrounding whitespace before matching", () => {
    expect(solarBaseScore("  sw  ")).toBe(87.5);
  });
});

describe("solarScoreTier", () => {
  it("maps each score boundary to its exact tier name, the sidebar Badge variant name depends on", () => {
    expect(solarScoreTier(100)).toBe("emerald");
    expect(solarScoreTier(80)).toBe("emerald");
    expect(solarScoreTier(79)).toBe("lime");
    expect(solarScoreTier(65)).toBe("lime");
    expect(solarScoreTier(64)).toBe("amber");
    expect(solarScoreTier(45)).toBe("amber");
    expect(solarScoreTier(44)).toBe("rose");
    expect(solarScoreTier(0)).toBe("rose");
  });

  it("treats out-of-range scores as clamped to their nearest real boundary", () => {
    expect(solarScoreTier(-10)).toBe("rose");
    expect(solarScoreTier(150)).toBe("emerald");
  });

  it("resolves fractional scores just below each boundary to the lower tier", () => {
    expect(solarScoreTier(79.9)).toBe("lime");
    expect(solarScoreTier(64.9)).toBe("amber");
    expect(solarScoreTier(44.9)).toBe("rose");
  });

  it("maps every real S_solar base score to the tier PRD §5.8's own ranking implies", () => {
    expect(solarScoreTier(solarBaseScore("S"))).toBe("emerald");
    expect(solarScoreTier(solarBaseScore("SE"))).toBe("emerald");
    expect(solarScoreTier(solarBaseScore("SW"))).toBe("emerald");
    expect(solarScoreTier(solarBaseScore("E"))).toBe("emerald");
    expect(solarScoreTier(solarBaseScore("W"))).toBe("lime");
    expect(solarScoreTier(solarBaseScore("NE"))).toBe("amber");
    expect(solarScoreTier(solarBaseScore("NW"))).toBe("amber");
    expect(solarScoreTier(solarBaseScore("N"))).toBe("rose");
  });
});
