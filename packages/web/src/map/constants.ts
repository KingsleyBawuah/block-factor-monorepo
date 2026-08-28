import type { LngLatLike } from "maplibre-gl";

/**
 * CARTO's free, no-API-key vector basemap (dark-matter): high-contrast dark
 * style matching the anti-vibe-coding neutral zinc/slate palette (AGENTS.md
 * §4), rather than a default colorful basemap. Verified live and reachable
 * at build time — see docs/ARCHITECTURE.md Decision Log row 10 (Map Engine).
 */
export const BASEMAP_STYLE_URL =
  "https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json";

/**
 * Overrides dark-matter-gl-style's own water fill color (its `water` layer,
 * source-layer `water` -- Lake Michigan and the Chicago River -- defaults to
 * #2C353C, verified by fetching the live style JSON) with a lighter,
 * more legibly "water" blue-gray, applied in MapCanvas once the style loads.
 */
export const WATER_FILL_COLOR = "#5c8da0";

/** Chicago Loop, used as the default map center before an address is scored. */
export const CHICAGO_CENTER: LngLatLike = [-87.6298, 41.8781];

export const CHICAGO_DEFAULT_ZOOM = 11;

/** Map zoom level App.tsx flies to once an address search resolves. */
export const RESOLVED_ADDRESS_ZOOM = 15;
