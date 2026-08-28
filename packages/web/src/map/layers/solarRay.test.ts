import { describe, it, expect } from "bun:test";
import {
  compassBearingDegrees,
  solarBaseScore,
  solarFacingDescription,
  solarRayGeoJSON,
  solarRayHeightMeters,
  solarRayTipPoint,
  solarScoreColor,
  solarScoreTier,
} from "./solarRay";

describe("compassBearingDegrees", () => {
  it("maps every point of the 8-point compass to its bearing", () => {
    expect(compassBearingDegrees("N")).toBe(0);
    expect(compassBearingDegrees("NE")).toBe(45);
    expect(compassBearingDegrees("E")).toBe(90);
    expect(compassBearingDegrees("SE")).toBe(135);
    expect(compassBearingDegrees("S")).toBe(180);
    expect(compassBearingDegrees("SW")).toBe(225);
    expect(compassBearingDegrees("W")).toBe(270);
    expect(compassBearingDegrees("NW")).toBe(315);
  });

  it("is case-insensitive", () => {
    expect(compassBearingDegrees("w")).toBe(270);
  });

  it("falls back to South for an unrecognized value, matching factor_solar.go's own default", () => {
    expect(compassBearingDegrees("nonsense")).toBe(180);
  });
});

const EARTH_RADIUS_METERS = 6_371_000;

function haversineDistanceMeters(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a =
    Math.sin(dLat / 2) ** 2 + Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2;
  return EARTH_RADIUS_METERS * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

describe("solarRayGeoJSON", () => {
  const lat = 41.8781;
  const lon = -87.6298;

  it("returns one closed-ring Polygon feature carrying the requested height", () => {
    const fc = solarRayGeoJSON(lat, lon, 270, 200, 8, 35);
    expect(fc.type).toBe("FeatureCollection");
    expect(fc.features).toHaveLength(1);

    const feature = fc.features[0];
    expect(feature.geometry.type).toBe("Polygon");
    expect(feature.properties.height).toBe(35);

    const ring = feature.geometry.coordinates[0];
    expect(ring).toHaveLength(4);
    expect(ring[0]).toEqual(ring[3]); // closed ring
  });

  it("places the tip approximately lengthMeters from the origin, in the requested bearing", () => {
    const lengthMeters = 200;
    const fc = solarRayGeoJSON(lat, lon, 270, lengthMeters, 8, 35); // due West
    const [baseLeft, tip] = fc.features[0].geometry.coordinates[0];

    const distance = haversineDistanceMeters(lat, lon, tip[1], tip[0]);
    expect(Math.abs(distance - lengthMeters) / lengthMeters).toBeLessThan(0.01);

    // Due West means the tip's longitude decreases with (approximately)
    // unchanged latitude.
    expect(tip[0]).toBeLessThan(lon);
    expect(Math.abs(tip[1] - lat)).toBeLessThan(0.001);

    // The base corners sit close to the origin, not out near the tip.
    const baseDistance = haversineDistanceMeters(lat, lon, baseLeft[1], baseLeft[0]);
    expect(baseDistance).toBeLessThan(20);
  });
});

describe("solarRayTipPoint", () => {
  it("matches the tip solarRayGeoJSON itself places, given the same inputs", () => {
    const lat = 41.8781;
    const lon = -87.6298;
    const fc = solarRayGeoJSON(lat, lon, 270, 200, 8, 35);
    const [, tipFromPolygon] = fc.features[0].geometry.coordinates[0];

    expect(solarRayTipPoint(lat, lon, 270, 200)).toEqual(tipFromPolygon);
  });
});

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

  it("falls back to 100, matching factor_solar.go's own baseScore default", () => {
    expect(solarBaseScore("nonsense")).toBe(100);
  });
});

describe("solarScoreColor", () => {
  it("maps each DESIGN.md score tier to its exact hex", () => {
    expect(solarScoreColor(100)).toBe("#10b981"); // emerald
    expect(solarScoreColor(80)).toBe("#10b981"); // emerald lower bound
    expect(solarScoreColor(79)).toBe("#84cc16"); // lime
    expect(solarScoreColor(65)).toBe("#84cc16"); // lime lower bound
    expect(solarScoreColor(64)).toBe("#f59e0b"); // amber
    expect(solarScoreColor(45)).toBe("#f59e0b"); // amber lower bound
    expect(solarScoreColor(44)).toBe("#f43f5e"); // rose
    expect(solarScoreColor(0)).toBe("#f43f5e"); // rose
  });

  it("maps every real S_solar base score to the tier PRD §5.8's own ranking implies", () => {
    expect(solarScoreColor(solarBaseScore("S"))).toBe("#10b981");
    expect(solarScoreColor(solarBaseScore("SE"))).toBe("#10b981");
    expect(solarScoreColor(solarBaseScore("SW"))).toBe("#10b981");
    expect(solarScoreColor(solarBaseScore("E"))).toBe("#10b981");
    expect(solarScoreColor(solarBaseScore("W"))).toBe("#84cc16");
    expect(solarScoreColor(solarBaseScore("NE"))).toBe("#f59e0b");
    expect(solarScoreColor(solarBaseScore("NW"))).toBe("#f59e0b");
    expect(solarScoreColor(solarBaseScore("N"))).toBe("#f43f5e");
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

  it("agrees with solarScoreColor for every real S_solar base score", () => {
    const tierHexes: Record<string, string> = {
      emerald: "#10b981",
      lime: "#84cc16",
      amber: "#f59e0b",
      rose: "#f43f5e",
    };
    for (const facing of ["N", "NE", "E", "SE", "S", "SW", "W", "NW"]) {
      const score = solarBaseScore(facing);
      expect(tierHexes[solarScoreTier(score)]).toBe(solarScoreColor(score));
    }
  });
});

describe("solarRayHeightMeters", () => {
  it("scales linearly between the min and max height across the 0-100 score range", () => {
    expect(solarRayHeightMeters(0)).toBe(10);
    expect(solarRayHeightMeters(100)).toBe(45);
    expect(solarRayHeightMeters(40)).toBeCloseTo(24, 5);
  });

  it("clamps out-of-range scores instead of producing a nonsensical height", () => {
    expect(solarRayHeightMeters(-50)).toBe(10);
    expect(solarRayHeightMeters(150)).toBe(45);
  });
});

describe("solarFacingDescription", () => {
  it("returns a distinct, non-empty description for every real facing", () => {
    const facings = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"];
    const descriptions = new Set(facings.map((f) => solarFacingDescription(f)));
    expect(descriptions.size).toBe(facings.length);
    for (const d of descriptions) {
      expect(d.length).toBeGreaterThan(0);
    }
  });

  it("falls back to South's description for an unrecognized value", () => {
    expect(solarFacingDescription("nonsense")).toBe(solarFacingDescription("S"));
  });
});
