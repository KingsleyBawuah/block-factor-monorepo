import type { components } from "../../lib/api/schema";
import { ctaLineColor } from "./cta-colors";
import type { FeatureCollection, MultiLineStringGeometry } from "../geojson";

type CtaRailLinesResponse = components["schemas"]["CtaRailLinesResponse"];

export type RailSegmentProperties = {
  line: string;
  color: string;
  offset: number;
  grade: "subway" | "elevated_or_at_grade";
  express: boolean;
};

/**
 * Pixel spacing between parallel offset lines on a track segment shared by
 * multiple CTA lines. Chosen deliberately (2026-08-26 design decision) over
 * a single "first line wins" color or a neutral "shared corridor" color --
 * see the CtaRailLayer PR description for the alternatives considered.
 */
export const RAIL_LINE_OFFSET_SPACING_PX = 3;

/**
 * MapLibre paint properties are per-feature, not per-array-element -- a
 * single feature can't render as two different colors at once. Splits each
 * source feature (which can carry 2+ CTA lines sharing one physical track
 * segment -- confirmed live up to 5 lines on one segment, see
 * docs/DATA_SOURCES.md "CTA 'L' Lines") into one feature per line, evenly
 * offset around the segment's centerline via MapLibre's line-offset paint
 * property. Every line sharing a corridor renders as its own thin colored
 * line rather than one line's color hiding the others.
 */
export function expandRailSegments(
  response: CtaRailLinesResponse
): FeatureCollection<MultiLineStringGeometry, RailSegmentProperties> {
  const features: FeatureCollection<MultiLineStringGeometry, RailSegmentProperties>["features"] = [];

  for (const feature of response.features) {
    const { lines, grade, express } = feature.properties;
    const count = lines.length;

    lines.forEach((line, index) => {
      features.push({
        type: "Feature",
        geometry: {
          type: "MultiLineString",
          coordinates: feature.geometry.coordinates as MultiLineStringGeometry["coordinates"],
        },
        properties: {
          line,
          color: ctaLineColor(line),
          offset: (index - (count - 1) / 2) * RAIL_LINE_OFFSET_SPACING_PX,
          grade,
          express,
        },
      });
    });
  }

  return { type: "FeatureCollection", features };
}
