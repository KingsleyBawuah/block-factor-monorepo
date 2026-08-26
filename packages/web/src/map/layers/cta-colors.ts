/**
 * Official CTA 'L' line hex colors. Mirrors the --color-cta-* Tailwind
 * theme tokens in src/index.css (and DESIGN.md's Colors section) --
 * duplicated here as plain TS constants because MapLibre paint expressions
 * and inline popup HTML need literal color strings, not CSS custom
 * properties. Keep both in sync if either changes.
 */
export const CTA_LINE_COLORS: Record<string, string> = {
  Red: "#c60c30",
  Blue: "#00a1de",
  Brown: "#62361b",
  Green: "#009b3a",
  Pink: "#e27ea6",
  Orange: "#f9461c",
  Purple: "#522398",
  Yellow: "#f9e300",
};

/** zinc-500 (DESIGN.md "Text Muted") -- used for a line name the palette above doesn't recognize. */
const FALLBACK_LINE_COLOR = "#71717a";

export function ctaLineColor(line: string): string {
  return CTA_LINE_COLORS[line] ?? FALLBACK_LINE_COLOR;
}
