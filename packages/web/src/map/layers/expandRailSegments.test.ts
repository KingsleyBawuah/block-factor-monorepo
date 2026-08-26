import { describe, it, expect } from "bun:test";
import { expandRailSegments } from "./expandRailSegments";
import { ctaLineColor } from "./cta-colors";
import type { components } from "../../lib/api/schema";

type CtaRailLinesResponse = components["schemas"]["CtaRailLinesResponse"];

function railResponse(
  segments: Array<{ lines: string[]; express: boolean; grade: "subway" | "elevated_or_at_grade" }>
): CtaRailLinesResponse {
  return {
    type: "FeatureCollection",
    features: segments.map((s) => ({
      type: "Feature",
      properties: s,
      geometry: {
        type: "MultiLineString",
        coordinates: [
          [
            [0, 0],
            [1, 1],
          ],
        ],
      },
    })),
  };
}

describe("expandRailSegments", () => {
  it("keeps a single-line segment as one centered (zero-offset) feature", () => {
    const result = expandRailSegments(
      railResponse([{ lines: ["Yellow"], express: false, grade: "elevated_or_at_grade" }])
    );

    expect(result.features).toHaveLength(1);
    expect(result.features[0].properties).toEqual({
      line: "Yellow",
      color: ctaLineColor("Yellow"),
      offset: 0,
      grade: "elevated_or_at_grade",
      express: false,
    });
  });

  it("splits a 2-line segment into two symmetric, oppositely-offset features", () => {
    const result = expandRailSegments(
      railResponse([{ lines: ["Brown", "Purple"], express: true, grade: "elevated_or_at_grade" }])
    );

    expect(result.features).toHaveLength(2);
    const [a, b] = result.features.map((f) => f.properties);
    expect(a.line).toBe("Brown");
    expect(b.line).toBe("Purple");
    expect(a.color).toBe(ctaLineColor("Brown"));
    expect(b.color).toBe(ctaLineColor("Purple"));
    expect(a.offset).toBeCloseTo(-b.offset);
    expect(a.offset).not.toBe(0);
    // express/grade are per-segment, not per-line -- both expanded features carry them.
    expect(a.express).toBe(true);
    expect(b.express).toBe(true);
  });

  it("splits a real 5-line segment into 5 distinct offsets symmetric around zero", () => {
    const result = expandRailSegments(
      railResponse([
        { lines: ["Brown", "Green", "Orange", "Pink", "Purple"], express: true, grade: "elevated_or_at_grade" },
      ])
    );

    expect(result.features).toHaveLength(5);
    const offsets = result.features.map((f) => f.properties.offset);
    expect(offsets.reduce((sum, o) => sum + o, 0)).toBeCloseTo(0);
    expect(new Set(offsets).size).toBe(5);
  });

  it("preserves geometry coordinates unchanged", () => {
    const result = expandRailSegments(railResponse([{ lines: ["Red"], express: false, grade: "subway" }]));
    expect(result.features[0].geometry.coordinates).toEqual([
      [
        [0, 0],
        [1, 1],
      ],
    ]);
  });

  it("expands multiple source segments independently", () => {
    const result = expandRailSegments(
      railResponse([
        { lines: ["Red"], express: false, grade: "subway" },
        { lines: ["Blue", "Pink"], express: false, grade: "elevated_or_at_grade" },
      ])
    );
    expect(result.features).toHaveLength(1 + 2);
  });
});
