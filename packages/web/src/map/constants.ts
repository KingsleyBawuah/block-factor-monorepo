import type { LngLatLike } from "maplibre-gl";

/**
 * CARTO's free, no-API-key vector basemap (dark-matter): high-contrast dark
 * style matching the anti-vibe-coding neutral zinc/slate palette (AGENTS.md
 * §4), rather than a default colorful basemap. Verified live and reachable
 * at build time — see docs/ARCHITECTURE.md Decision Log row 10 (Map Engine).
 */
export const BASEMAP_STYLE_URL =
  "https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json";

/** Chicago Loop, used as the default map center before an address is scored. */
export const CHICAGO_CENTER: LngLatLike = [-87.6298, 41.8781];

export const CHICAGO_DEFAULT_ZOOM = 11;

/**
 * 233 S Wacker Dr, Chicago, IL -- the same example address docs/openapi.yaml
 * itself uses for POST /api/v1/score, and the same coordinate the Go test
 * suite uses throughout. Stands in for a real selected address until
 * address search exists (docs/IMPLEMENTATION_PHASES.md Phase 5 deliverable
 * 1); PoiDensityLayer queries GET /api/v1/places around this point.
 */
export const DEMO_ADDRESS_LAT = 41.8781;
export const DEMO_ADDRESS_LON = -87.6298;
