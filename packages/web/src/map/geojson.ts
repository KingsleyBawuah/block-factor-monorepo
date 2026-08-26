/**
 * Minimal GeoJSON types for exactly what the map layer code needs.
 * Deliberately not the `@types/geojson` package's ambient `GeoJSON`
 * namespace: it resolves as a transitive dependency (of maplibre-gl's own
 * types) but isn't hoisted where tsconfig.json's explicit `types: ["@types/bun"]`
 * restriction can see it, and pulling it in properly would mean loosening
 * that restriction project-wide for one dependency's sake.
 */

export type GeoPosition = number[];

export type MultiLineStringGeometry = {
  type: "MultiLineString";
  coordinates: GeoPosition[][];
};

export type PointGeometry = {
  type: "Point";
  coordinates: GeoPosition;
};

export type Feature<Geometry, Properties> = {
  type: "Feature";
  geometry: Geometry;
  properties: Properties;
};

export type FeatureCollection<Geometry, Properties> = {
  type: "FeatureCollection";
  features: Feature<Geometry, Properties>[];
};
