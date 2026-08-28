import { useId, useState } from "react";
import { cn } from "../../lib/utils";
import { formatDistanceMiles } from "../../lib/format";
import { scoreTier } from "../../map";
import { Badge } from "./badge";
import type { components } from "../../lib/api/schema";

type FactorScore = components["schemas"]["FactorScore"];

// Mirrors ScoreRing's TIER_STROKE_CLASS -- same tier boundaries, applied to
// a horizontal bar fill instead of a circular stroke.
const TIER_BAR_CLASS: Record<ReturnType<typeof scoreTier>, string> = {
  emerald: "bg-emerald-400",
  lime: "bg-lime-400",
  amber: "bg-amber-400",
  rose: "bg-rose-400",
};

export type FactorCardProps = {
  label: string;
  /** Lineicons class name, e.g. "lni-train-1". */
  icon: string;
  factor: FactorScore;
};

/**
 * One Factor Breakdown accordion (docs/IMPLEMENTATION_PHASES.md Phase 5
 * deliverable 2, docs/ARCHITECTURE.md §7.3): a collapsed-by-default row with
 * the factor's sub-score bar and straight-line distance pill, expandable to
 * reveal its contributing POIs.
 */
export function FactorCard({ label, icon, factor }: FactorCardProps) {
  const [expanded, setExpanded] = useState(false);
  const panelId = useId();
  const hasPois = !!factor.contributingPois?.length;
  // Clamped the same way ScoreRing's own display value is -- aria-valuenow
  // below must stay within the aria-valuemin/aria-valuemax bounds it also
  // declares even if an out-of-range score were ever passed.
  const rounded = factor.score != null ? Math.round(Math.min(100, Math.max(0, factor.score))) : null;
  const tier = rounded != null ? scoreTier(rounded) : null;

  // Non-expandable cards (no contributingPois) render as a plain <div>, not
  // a disabled <button>: a disabled button drops out of the tab order and
  // screen readers announce it as "disabled", implying a broken/inactive
  // factor rather than simply non-expandable informational content.
  const Header = hasPois ? "button" : "div";
  const headerProps = hasPois
    ? {
        type: "button" as const,
        onClick: () => setExpanded((v) => !v),
        "aria-expanded": expanded,
        "aria-controls": panelId,
      }
    : {};

  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900/60">
      <Header
        className={cn(
          "flex w-full items-center gap-3 p-3 text-left",
          hasPois && "cursor-pointer"
        )}
        {...headerProps}
      >
        <i className={cn("lni", icon, "shrink-0 text-sm text-zinc-400")} aria-hidden="true" />

        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-xs font-medium tracking-wide text-zinc-300 uppercase">
              {label}
            </span>
            {factor.degraded || rounded == null ? (
              <Badge variant="outline">Unavailable</Badge>
            ) : (
              <span className="font-mono text-sm font-semibold text-zinc-100">{rounded}</span>
            )}
          </div>

          {!factor.degraded && rounded != null && (
            <div
              className="mt-1.5 h-1.5 w-full overflow-hidden rounded-sm bg-zinc-800"
              role="progressbar"
              aria-valuenow={rounded}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-label={`${label} sub-score`}
            >
              <div
                className={cn(
                  "h-full rounded-sm transition-[width] duration-500 ease-out motion-reduce:transition-none",
                  tier && TIER_BAR_CLASS[tier]
                )}
                style={{ width: `${rounded}%` }}
              />
            </div>
          )}

          {factor.distanceMeters != null && (
            <div className="mt-1.5 font-mono text-[11px] text-zinc-400">
              {formatDistanceMiles(factor.distanceMeters)} <span className="text-zinc-400">straight-line</span>
            </div>
          )}
        </div>

        {hasPois && (
          <i
            className={cn(
              "lni lni-chevron-down shrink-0 text-xs text-zinc-500 transition-transform",
              expanded && "rotate-180"
            )}
            aria-hidden="true"
          />
        )}
      </Header>

      {hasPois && expanded && (
        <ul id={panelId} className="space-y-1 border-t border-zinc-800 px-3 py-2">
          {factor.contributingPois?.map((poi, index) => (
            <li
              key={`${poi.name}-${poi.distanceMeters}-${index}`}
              className="flex items-center justify-between gap-2 font-mono text-[11px] text-zinc-400"
            >
              <span className="truncate">{poi.name}</span>
              <span className="shrink-0 text-zinc-400">{formatDistanceMiles(poi.distanceMeters)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
