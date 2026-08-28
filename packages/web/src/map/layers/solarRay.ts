import type { FeatureCollection, GeoPosition, PolygonGeometry } from "../geojson";

const METERS_PER_DEGREE_LAT = 111_320;

/**
 * 8-point compass -> bearing in degrees, matching the exact vocabulary
 * docs/openapi.yaml's UnitMetadata.facing enum and
 * services/api/internal/scoring/factor_solar.go both use.
 */
const COMPASS_BEARINGS: Record<string, number> = {
  N: 0,
  NE: 45,
  E: 90,
  SE: 135,
  S: 180,
  SW: 225,
  W: 270,
  NW: 315,
};

/**
 * Maps an 8-point compass facing to a bearing in degrees. An unrecognized
 * value falls back to South (180°) -- the same default
 * factor_solar.go/InferStreetFacingOrientation itself falls back to
 * (api.S) when no facing can be determined.
 */
export function compassBearingDegrees(facing: string): number {
  return COMPASS_BEARINGS[facing.toUpperCase()] ?? 180;
}

/**
 * S_solar's exact orientation base-score table (PRD §5.8,
 * services/api/internal/scoring/factor_solar.go CalculateSolarScore) --
 * kept in sync by hand since this is presentation-only (a widget visualizing
 * what the score means), not the scoring computation itself, which stays
 * server-side.
 */
const SOLAR_BASE_SCORES: Record<string, number> = {
  S: 100,
  SE: 92.5,
  SW: 87.5,
  E: 85,
  W: 75,
  NE: 62.5,
  NW: 57.5,
  N: 40,
};

/**
 * Maps an 8-point compass facing to its S_solar orientation base score.
 * An unrecognized value falls back to 100 -- factor_solar.go's own
 * baseScore starts at 100.0 before its switch statement narrows it for a
 * recognized facing, so an unrecognized one keeps that same default.
 */
export function solarBaseScore(facing: string): number {
  return SOLAR_BASE_SCORES[facing.toUpperCase()] ?? 100;
}

// The exact score-tier hexes DESIGN.md's "Score Indicators" already define
// (Optimal/Emerald 80-100, Good/Lime 65-79, Moderate/Amber 45-64,
// Alert/Rose 0-44) -- reused here deliberately: unlike the POI/Divvy
// category markers, this widget genuinely represents a score, so the score
// palette is the *correct* semantic fit, not a borrowed one.
const SCORE_EMERALD = "#10b981";
const SCORE_LIME = "#84cc16";
const SCORE_AMBER = "#f59e0b";
const SCORE_ROSE = "#f43f5e";

/** Maps an S_solar base score to its DESIGN.md score-tier color. */
export function solarScoreColor(score: number): string {
  if (score >= 80) return SCORE_EMERALD;
  if (score >= 65) return SCORE_LIME;
  if (score >= 45) return SCORE_AMBER;
  return SCORE_ROSE;
}

const MIN_RAY_HEIGHT_METERS = 10;
const MAX_RAY_HEIGHT_METERS = 45;

/**
 * Maps an S_solar base score (0-100) to a ray extrusion height -- a
 * stronger orientation stands taller, so quality reads viscerally from the
 * ray's silhouette, not just its color. Never fully flat (MIN_RAY_HEIGHT_METERS
 * keeps even North, the worst case, visible as a real 3D shape).
 */
export function solarRayHeightMeters(score: number): number {
  const clamped = Math.min(100, Math.max(0, score));
  return MIN_RAY_HEIGHT_METERS + (clamped / 100) * (MAX_RAY_HEIGHT_METERS - MIN_RAY_HEIGHT_METERS);
}

/**
 * Plain-language description of what each facing direction means for
 * natural light, shown in the widget's hover card -- the compass letters
 * and a bare score number mean nothing without this to a user who's never
 * read PRD.md §5.8.
 */
const SOLAR_FACING_DESCRIPTIONS: Record<string, string> = {
  S: "All-day direct sun",
  SE: "Strong morning-to-midday sun",
  SW: "Strong midday-to-evening sun",
  E: "Morning light",
  W: "Afternoon light",
  NE: "Limited morning light",
  NW: "Limited evening light",
  N: "Minimal direct sunlight",
};

export function solarFacingDescription(facing: string): string {
  return SOLAR_FACING_DESCRIPTIONS[facing.toUpperCase()] ?? SOLAR_FACING_DESCRIPTIONS.S;
}

/**
 * Returns the [lon, lat] point distanceMeters from (lat, lon) along
 * bearingDegrees (0 = north, clockwise). Same flat equirectangular
 * approximation distanceRing.ts and services/api/internal/scoring/factor_third_places.go
 * both already use -- accurate enough at the scale of one short ray.
 */
function destinationPoint(lat: number, lon: number, bearingDegrees: number, distanceMeters: number): GeoPosition {
  const metersPerDegreeLon = METERS_PER_DEGREE_LAT * Math.cos((lat * Math.PI) / 180);
  const bearingRad = (bearingDegrees * Math.PI) / 180;
  const dLat = (distanceMeters * Math.cos(bearingRad)) / METERS_PER_DEGREE_LAT;
  const dLon = (distanceMeters * Math.sin(bearingRad)) / metersPerDegreeLon;
  return [lon + dLon, lat + dLat];
}

export type SolarRayProperties = {
  height: number;
};

/**
 * Builds a single tapering-wedge Polygon feature pointing from (lat, lon)
 * along bearingDegrees -- a narrow base at the origin widening the visual
 * footprint just enough to extrude into a recognizable 3D blade
 * (SolarRayWidget.tsx renders this via a `fill-extrusion` layer), tapering
 * to a point at the tip.
 */
export function solarRayGeoJSON(
  lat: number,
  lon: number,
  bearingDegrees: number,
  lengthMeters = 200,
  baseHalfWidthMeters = 8,
  heightMeters = 35
): FeatureCollection<PolygonGeometry, SolarRayProperties> {
  const tip = destinationPoint(lat, lon, bearingDegrees, lengthMeters);
  const baseLeft = destinationPoint(lat, lon, bearingDegrees - 90, baseHalfWidthMeters);
  const baseRight = destinationPoint(lat, lon, bearingDegrees + 90, baseHalfWidthMeters);

  return {
    type: "FeatureCollection",
    features: [
      {
        type: "Feature",
        properties: { height: heightMeters },
        geometry: {
          type: "Polygon",
          coordinates: [[baseLeft, tip, baseRight, baseLeft]],
        },
      },
    ],
  };
}

/**
 * Returns the ray's tip point, for positioning the facing/score label
 * (SolarRayWidget.tsx) -- lengthMeters must match the value passed to
 * solarRayGeoJSON so the label actually sits at the rendered ray's tip.
 */
export function solarRayTipPoint(lat: number, lon: number, bearingDegrees: number, lengthMeters = 200): GeoPosition {
  return destinationPoint(lat, lon, bearingDegrees, lengthMeters);
}
