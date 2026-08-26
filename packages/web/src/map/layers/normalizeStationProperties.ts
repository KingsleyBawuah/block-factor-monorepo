export type CtaStationProperties = { name: string; lines: string[]; ada: boolean };

/**
 * The project's pinned maplibre-gl major version (^5.19.0) serializes
 * nested arrays/objects in GeoJSON source feature properties to JSON
 * strings when they cross the tiling worker boundary, returning them as
 * real arrays/objects only from v6 onward (confirmed against MapLibre's
 * own v5-to-v6 migration guide, "Remove JSON.parse from GeoJSON
 * Properties"). `feature.properties` from a MapLibre mouse event on this
 * version can therefore hand back `lines` as either a real `string[]` or a
 * JSON-encoded string, depending on the exact code path -- this normalizes
 * either shape into the real array CtaRailLayer expects.
 */
export function normalizeStationProperties(raw: Record<string, unknown>): CtaStationProperties {
  let lines: string[] = [];
  if (Array.isArray(raw.lines)) {
    lines = raw.lines as string[];
  } else if (typeof raw.lines === "string") {
    try {
      const parsed = JSON.parse(raw.lines);
      if (Array.isArray(parsed)) lines = parsed;
    } catch {
      lines = [];
    }
  }

  const ada = raw.ada === true || raw.ada === "true";
  const name = typeof raw.name === "string" ? raw.name : "";

  return { name, lines, ada };
}
