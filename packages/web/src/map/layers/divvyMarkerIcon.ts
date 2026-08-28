export const DIVVY_MARKER_ICON_ID = "divvy-dock-marker";
const ICON_SIZE = 24;

// divvy-blue (DESIGN.md) -- Divvy's real brand blue, close in hue to CTA
// Blue Line's #00a1de but split against DIVVY_GRAY below rather than used
// as a flat dot specifically so the *pattern*, not the hue alone,
// disambiguates this layer from CTA stations at a glance.
const DIVVY_BLUE = "#41b6e6";
// text-secondary (DESIGN.md) -- stands in for the grey used on Divvy's
// retroreflective e-bike fleet; no verified digital hex exists for that
// physical coating.
const DIVVY_GRAY = "#a1a1aa";
// zinc-900 -- an outline the icon needs (unlike a flat circle-color dot)
// since two bright, light colors meeting the dark basemap directly would
// have no defined edge.
const STROKE_COLOR = "#18181b";

/**
 * Renders the two-tone Divvy dock marker (DESIGN.md "Divvy Dock Markers")
 * to an ImageData for registration via `map.addImage` (DivvyDockLayer.tsx)
 * -- a `circle` layer's `circle-color` paint property can only fill one
 * solid color per feature, so a genuine half-and-half marker needs a
 * custom icon rendered via the Canvas API instead.
 *
 * Returns null when no 2D canvas context is available (this project's own
 * test environment, happy-dom, has no canvas 2D implementation --
 * confirmed live, `getContext("2d")` returns null there; every real
 * browser supports this).
 */
export function createDivvyMarkerIcon(): ImageData | null {
  const canvas = document.createElement("canvas");
  canvas.width = ICON_SIZE;
  canvas.height = ICON_SIZE;
  const ctx = canvas.getContext("2d");
  if (!ctx) return null;

  const cx = ICON_SIZE / 2;
  const cy = ICON_SIZE / 2;
  const r = ICON_SIZE / 2 - 1; // leaves room for the 1.5px outline stroke

  ctx.beginPath();
  ctx.moveTo(cx, cy);
  ctx.arc(cx, cy, r, Math.PI / 2, (3 * Math.PI) / 2);
  ctx.closePath();
  ctx.fillStyle = DIVVY_BLUE;
  ctx.fill();

  ctx.beginPath();
  ctx.moveTo(cx, cy);
  ctx.arc(cx, cy, r, -Math.PI / 2, Math.PI / 2);
  ctx.closePath();
  ctx.fillStyle = DIVVY_GRAY;
  ctx.fill();

  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.lineWidth = 1.5;
  ctx.strokeStyle = STROKE_COLOR;
  ctx.stroke();

  return ctx.getImageData(0, 0, ICON_SIZE, ICON_SIZE);
}
