import { cn } from "../../lib/utils";
import { scoreTier, type ScoreTier } from "../../map";

const VIEWBOX_SIZE = 120;
const RADIUS = 52;
const STROKE_WIDTH = 8;

/** 2πr -- the full circle's stroke length, so a 0-100 score maps to a 0-100% arc. */
export const RING_CIRCUMFERENCE = 2 * Math.PI * RADIUS;

/**
 * Maps a 0-100 score to how much of the ring's stroke-dasharray to hide via
 * stroke-dashoffset, clamping out-of-range input the same way scoreTier's
 * own callers already expect (a raw compositeScore is never validated
 * client-side beyond what the API itself guarantees).
 */
export function ringDashOffset(score: number): number {
  const clamped = Math.min(100, Math.max(0, score));
  return RING_CIRCUMFERENCE * (1 - clamped / 100);
}

// Mirrors Badge's own score-* variant colors (badge.tsx) exactly -- those
// are plain Tailwind emerald/lime/amber/rose-400 classes, not the
// --color-score-* theme tokens (index.css), which nothing currently
// consumes as utility classes. Reusing the same literal classes keeps the
// ring and any score badge visually identical for the same tier.
const TIER_STROKE_CLASS: Record<ScoreTier, string> = {
  emerald: "stroke-emerald-400",
  lime: "stroke-lime-400",
  amber: "stroke-amber-400",
  rose: "stroke-rose-400",
};

export type ScoreRingProps = {
  /** Composite BlockFactor score, 0-100. */
  score: number;
};

/**
 * Hero composite score ring (docs/IMPLEMENTATION_PHASES.md Phase 5
 * deliverable 2): a circular progress ring around the composite score,
 * colored by the same score tier a Badge would use. Hand-rolled SVG rather
 * than a charting library -- a single arc has no need for one.
 */
export function ScoreRing({ score }: ScoreRingProps) {
  const rounded = Math.round(score);
  // Tier is computed from the *rounded* score, not the raw one: scoreTier's
  // boundaries are inclusive integers (>= 80, >= 65, ...), so a raw score
  // like 79.6 would return "lime" while rounding to the displayed "80" --
  // the emerald boundary. Using the rounded value keeps the ring's color
  // and the number it's drawn around always agreeing. ringDashOffset below
  // deliberately still uses the raw `score` for smooth sub-pixel arc
  // progress -- only the discrete tier color needs to match the rounded
  // display value.
  const tier = scoreTier(rounded);
  const center = VIEWBOX_SIZE / 2;

  return (
    <div
      className="relative inline-flex h-32 w-32 shrink-0 items-center justify-center"
      role="img"
      aria-label={`Composite BlockFactor score: ${rounded} out of 100`}
    >
      <svg
        viewBox={`0 0 ${VIEWBOX_SIZE} ${VIEWBOX_SIZE}`}
        className="h-full w-full -rotate-90"
        aria-hidden="true"
      >
        <circle cx={center} cy={center} r={RADIUS} fill="none" strokeWidth={STROKE_WIDTH} className="stroke-zinc-800" />
        <circle
          cx={center}
          cy={center}
          r={RADIUS}
          fill="none"
          strokeWidth={STROKE_WIDTH}
          // A round cap on a zero-length dash (score 0) still paints as a
          // small dot at the 12 o'clock start point in SVG -- round caps
          // render per sub-path regardless of length. "butt" avoids that
          // artifact; it's indistinguishable from "round" for any
          // non-zero-length arc.
          strokeLinecap={rounded > 0 ? "round" : "butt"}
          strokeDasharray={RING_CIRCUMFERENCE}
          strokeDashoffset={ringDashOffset(score)}
          className={cn(
            "transition-[stroke-dashoffset] duration-500 ease-out motion-reduce:transition-none",
            TIER_STROKE_CLASS[tier]
          )}
        />
      </svg>
      {/* role="img" on the outer container already makes this subtree's
          text non-essential to the accessibility tree in most browsers, but
          aria-hidden here is defense-in-depth: some screen reader/browser
          combinations (VoiceOver, NVDA browse mode) can still traverse and
          double-announce unhidden child text nodes inside a role="img"
          ancestor. */}
      <div className="absolute flex flex-col items-center" aria-hidden="true">
        <span className="text-4xl font-mono font-bold text-zinc-100">{rounded}</span>
        <span className="text-xs font-mono text-zinc-400">/ 100</span>
      </div>
    </div>
  );
}
