import type { FeatureCollection, GeoPosition, MultiLineStringGeometry } from "../geojson";

const METERS_PER_DEGREE_LAT = 111_320;

/**
 * Returns a closed ring of [lon, lat] points approximating a circle of
 * radiusMeters around (centerLat, centerLon). Flat equirectangular
 * approximation (same one services/api/internal/scoring/factor_third_places.go
 * uses for its own grid math) -- accurate enough at city scale, no need for
 * a full geodesic library for a visual reference ring.
 */
function circleRingCoordinates(centerLat: number, centerLon: number, radiusMeters: number, segments = 64): GeoPosition[] {
  const metersPerDegreeLon = METERS_PER_DEGREE_LAT * Math.cos((centerLat * Math.PI) / 180);
  const points: GeoPosition[] = [];
  for (let i = 0; i <= segments; i++) {
    const angle = (i / segments) * 2 * Math.PI;
    const dLat = (radiusMeters * Math.sin(angle)) / METERS_PER_DEGREE_LAT;
    const dLon = (radiusMeters * Math.cos(angle)) / metersPerDegreeLon;
    points.push([centerLon + dLon, centerLat + dLat]);
  }
  return points;
}

export type DistanceRingProperties = {
  radiusMeters: number;
};

/**
 * Builds a FeatureCollection of concentric distance-reference rings around
 * (centerLat, centerLon), one MultiLineString feature per radius in
 * radiiMeters -- spatial reference guides for the POI Density Layer
 * (docs/IMPLEMENTATION_PHASES.md Phase 4 deliverable 7), not scored data,
 * so they render as neutral/muted rather than an accent color.
 */
export function distanceRingsGeoJSON(
  centerLat: number,
  centerLon: number,
  radiiMeters: number[]
): FeatureCollection<MultiLineStringGeometry, DistanceRingProperties> {
  return {
    type: "FeatureCollection",
    features: radiiMeters.map((radiusMeters) => ({
      type: "Feature",
      properties: { radiusMeters },
      geometry: {
        type: "MultiLineString",
        coordinates: [circleRingCoordinates(centerLat, centerLon, radiusMeters)],
      },
    })),
  };
}
