const HTML_ESCAPES: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#39;",
};

/**
 * Escapes text for safe interpolation into an HTML string passed to
 * MapLibre's `Popup#setHTML` (map/layers/CtaRailLayer.tsx). Station and
 * line names currently only ever come from our own municipal data
 * ingestion (never user input), so this isn't defending an active
 * exploit path today -- it's cheap insurance against building the habit
 * of interpolating unescaped strings into innerHTML-equivalent APIs.
 */
export function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (char) => HTML_ESCAPES[char]);
}
