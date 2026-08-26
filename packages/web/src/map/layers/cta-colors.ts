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

/**
 * Lines whose official color is too light for white badge text to clear
 * WCAG AA's 4.5:1 text contrast ratio -- computed via the actual WCAG
 * relative-luminance formula against every CTA_LINE_COLORS value (not
 * eyeballed): Blue 3.05:1, Green 3.79:1, Orange 3.54:1, Pink 2.70:1,
 * Yellow 1.31:1 all fail with white text (only Red 6.21:1, Brown 10.25:1,
 * and Purple 10.68:1 clear it). Dark text (zinc-950, DESIGN.md's
 * bg-canvas) clears 4.5:1 against all eight colors, so it's the correct
 * choice for every line in this set, not just the most visibly-illegible
 * ones.
 */
const DARK_TEXT_LINES = new Set(["Blue", "Green", "Orange", "Pink", "Yellow"]);

export function ctaLineTextColor(line: string): string {
  return DARK_TEXT_LINES.has(line) ? "#09090b" : "#ffffff";
}
