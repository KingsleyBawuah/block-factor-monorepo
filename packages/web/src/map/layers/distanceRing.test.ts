import { describe, it, expect } from "bun:test";
import { distanceRingsGeoJSON } from "./distanceRing";

const EARTH_RADIUS_METERS = 6_371_000;

function haversineDistanceMeters(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a =
    Math.sin(dLat / 2) ** 2 + Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2;
  return EARTH_RADIUS_METERS * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

describe("distanceRingsGeoJSON", () => {
  it("returns one MultiLineString feature per radius", () => {
    const fc = distanceRingsGeoJSON(41.8781, -87.6298, [400, 800]);
    expect(fc.type).toBe("FeatureCollection");
    expect(fc.features).toHaveLength(2);
    expect(fc.features[0].properties.radiusMeters).toBe(400);
    expect(fc.features[1].properties.radiusMeters).toBe(800);
    for (const f of fc.features) {
      expect(f.geometry.type).toBe("MultiLineString");
    }
  });

  it("produces a closed ring (first point equals last point)", () => {
    const fc = distanceRingsGeoJSON(41.8781, -87.6298, [500]);
    const ring = fc.features[0].geometry.coordinates[0];
    expect(ring[0]).toEqual(ring[ring.length - 1]);
  });

  it("places every ring point at approximately the requested radius from center", () => {
    const centerLat = 41.8781;
    const centerLon = -87.6298;
    const radiusMeters = 804.672;
    const fc = distanceRingsGeoJSON(centerLat, centerLon, [radiusMeters]);
    const ring = fc.features[0].geometry.coordinates[0];

    for (const [lon, lat] of ring) {
      const d = haversineDistanceMeters(centerLat, centerLon, lat, lon);
      // The flat equirectangular approximation is accurate to well within
      // 1% at city scale -- a wide tolerance here just catches a genuinely
      // broken formula (e.g. a missing cos(lat) longitude correction),
      // not floating-point noise.
      expect(Math.abs(d - radiusMeters) / radiusMeters).toBeLessThan(0.01);
    }
  });
});
