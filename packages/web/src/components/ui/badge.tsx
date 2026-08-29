import type * as React from "react";
import { cn } from "../../lib/utils";

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?:
    | "default"
    | "outline"
    | "cta-red"
    | "cta-blue"
    | "cta-brown"
    | "cta-green"
    | "cta-pink"
    | "cta-orange"
    | "cta-purple"
    | "cta-yellow"
    | "score-emerald"
    | "score-lime"
    | "score-amber"
    | "score-rose";
}

const CTA_BADGE_LINE_NAMES = new Set(["red", "blue", "brown", "green", "pink", "orange", "purple", "yellow"]);

/**
 * Maps a real CTA line name (e.g. "Red", as returned by `GET /api/v1/score`'s
 * `ContributingPoi.lines` -- `factor_transit.go`) to this Badge's own
 * matching `cta-*` variant. Falls back to `"outline"` for anything
 * unrecognized: `lines` comes from the API over the wire, not a closed
 * local constant, so an unexpected value (a future 9th CTA line, a data
 * hiccup) should degrade to a plain badge rather than the variant lookup
 * silently returning `undefined` styling.
 */
export function ctaLineBadgeVariant(line: string): BadgeProps["variant"] {
  const key = line.toLowerCase();
  return CTA_BADGE_LINE_NAMES.has(key) ? (`cta-${key}` as BadgeProps["variant"]) : "outline";
}

export function Badge({
  className,
  variant = "default",
  ...props
}: BadgeProps) {
  const variantStyles = {
    default: "border-transparent bg-zinc-800 text-zinc-100",
    outline: "border-zinc-700 bg-transparent text-zinc-300",
    "cta-red": "border-transparent bg-[#c60c30] text-white",
    "cta-blue": "border-transparent bg-[#00a1de] text-white",
    "cta-brown": "border-transparent bg-[#62361b] text-white",
    "cta-green": "border-transparent bg-[#009b3a] text-white",
    "cta-pink": "border-transparent bg-[#e27ea6] text-zinc-950",
    "cta-orange": "border-transparent bg-[#f9461c] text-white",
    "cta-purple": "border-transparent bg-[#522398] text-white",
    "cta-yellow": "border-transparent bg-[#f9e300] text-zinc-950",
    "score-emerald": "border-emerald-900/60 bg-emerald-950/40 text-emerald-400",
    "score-lime": "border-lime-900/60 bg-lime-950/40 text-lime-400",
    "score-amber": "border-amber-900/60 bg-amber-950/40 text-amber-400",
    "score-rose": "border-rose-900/60 bg-rose-950/40 text-rose-400",
  }[variant];

  return (
    <div
      className={cn(
        "inline-flex items-center rounded-sm border px-2 py-0.5 text-xs font-mono font-medium tracking-wide uppercase transition-colors",
        variantStyles,
        className
      )}
      {...props}
    />
  );
}
