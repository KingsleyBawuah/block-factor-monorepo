/**
 * The exact font stack CARTO's dark-matter-gl-style basemap itself already
 * uses for its own labels (verified live against the style's own JSON,
 * https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json) --
 * a symbol layer's `text-font` must reference a stack the style's glyphs
 * endpoint actually serves, or the label silently fails to render.
 */
export const LABEL_FONT_STACK = [
  "Montserrat Regular",
  "Open Sans Regular",
  "Noto Sans Regular",
  "HanWangHeiLight Regular",
  "NanumBarunGothic Regular",
];
