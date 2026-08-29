import { useId, useState } from "react";
import "lineicons/dist/lineicons.css";
import { Badge, type BadgeProps } from "./ui/badge";
import { cn } from "../lib/utils";
import { FACTOR_METADATA, FACTOR_ORDER } from "../lib/factor-metadata";
import { scoreTier, type ScoreTier } from "../map";
import type { PinnedAddress } from "../lib/pinned-addresses";
import { MAX_PINNED_ADDRESSES } from "../lib/pinned-addresses";

export type ComparisonTrayProps = {
  pinned: PinnedAddress[];
  onUnpin: (index: number) => void;
};

const TIER_BADGE_VARIANT: Record<ScoreTier, NonNullable<BadgeProps["variant"]>> = {
  emerald: "score-emerald",
  lime: "score-lime",
  amber: "score-amber",
  rose: "score-rose",
};

const TIER_TEXT_CLASS: Record<ScoreTier, string> = {
  emerald: "text-emerald-400",
  lime: "text-lime-400",
  amber: "text-amber-400",
  rose: "text-rose-400",
};

function roundedTier(score: number): { rounded: number; tier: ScoreTier } {
  const rounded = Math.round(Math.min(100, Math.max(0, score)));
  return { rounded, tier: scoreTier(rounded) };
}

/**
 * Bottom Multi-Address Comparison Tray (docs/IMPLEMENTATION_PHASES.md Phase
 * 5 deliverable 3): pin up to 3 resolved addresses and compare their
 * composite and per-factor sub-scores side by side. Renders nothing until
 * at least one address is pinned, and the factor comparison table only
 * becomes reachable once there are 2+ to actually compare -- comparing a
 * single pinned address against nothing is just its own factor breakdown,
 * already visible in the main sidebar.
 */
export function ComparisonTray({ pinned, onUnpin }: ComparisonTrayProps) {
  const [expanded, setExpanded] = useState(false);
  const canExpand = pinned.length >= 2;
  const panelId = useId();

  if (pinned.length === 0) return null;

  return (
    <div className="sticky bottom-0 z-20 shrink-0 border-t border-zinc-800 bg-zinc-900/95">
      <div className="flex items-center gap-3 px-6 py-2">
        <span className="shrink-0 font-mono text-xs font-medium tracking-wide text-zinc-500 uppercase">
          Comparing {pinned.length}/{MAX_PINNED_ADDRESSES}
        </span>

        <ul className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto">
          {pinned.map((p, index) => {
            const { rounded, tier } = roundedTier(p.score.compositeScore);
            return (
              <li
                key={`${p.lat}-${p.lon}`}
                className="flex shrink-0 items-center gap-1.5 rounded-sm border border-zinc-800 bg-zinc-950 py-1 pr-1 pl-2"
              >
                <span className="max-w-[10rem] truncate font-mono text-xs text-zinc-300">{p.formattedAddress}</span>
                <Badge variant={TIER_BADGE_VARIANT[tier]}>{rounded}</Badge>
                <button
                  type="button"
                  onClick={() => onUnpin(index)}
                  aria-label={`Unpin ${p.formattedAddress}`}
                  className="rounded-sm p-0.5 text-zinc-500 hover:text-zinc-300"
                >
                  <i className="lni lni-xmark text-xs" aria-hidden="true" />
                </button>
              </li>
            );
          })}
        </ul>

        {canExpand && (
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            aria-expanded={expanded}
            aria-controls={panelId}
            className="flex shrink-0 items-center gap-1.5 rounded-sm border border-zinc-700 bg-zinc-800 px-2 py-1 font-mono text-xs font-medium tracking-wide text-zinc-200 uppercase hover:bg-zinc-700"
          >
            <i className="lni lni-layers-1 text-xs" aria-hidden="true" />
            {expanded ? "Hide comparison" : "Compare"}
          </button>
        )}
      </div>

      {expanded && canExpand && (
        <div id={panelId} className="max-h-64 overflow-y-auto border-t border-zinc-800 px-6 py-2">
          <table className="w-full border-collapse font-mono text-xs">
            <thead>
              <tr>
                <th scope="col" className="px-2 py-1 text-left tracking-wide text-zinc-500 uppercase">
                  Factor
                </th>
                {pinned.map((p) => (
                  <th
                    key={`${p.lat}-${p.lon}`}
                    scope="col"
                    className="max-w-[8rem] truncate px-2 py-1 text-left font-normal tracking-wide text-zinc-500 uppercase"
                  >
                    {p.formattedAddress}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {FACTOR_ORDER.map((key) => {
                if (!pinned.some((p) => p.score.factors[key])) return null;
                const { label, icon } = FACTOR_METADATA[key];
                return (
                  <tr key={key} className="border-t border-zinc-900">
                    <th scope="row" className="px-2 py-1.5 text-left font-normal text-zinc-300">
                      <span className="inline-flex items-center gap-1.5">
                        <i className={cn("lni", icon, "text-zinc-500")} aria-hidden="true" />
                        {label}
                      </span>
                    </th>
                    {pinned.map((p) => {
                      const factor = p.score.factors[key];
                      if (!factor || factor.degraded || factor.score == null) {
                        return (
                          <td key={`${p.lat}-${p.lon}`} className="px-2 py-1.5 text-zinc-400">
                            Unavailable
                          </td>
                        );
                      }
                      const { rounded, tier } = roundedTier(factor.score);
                      return (
                        <td key={`${p.lat}-${p.lon}`} className={cn("px-2 py-1.5 font-semibold", TIER_TEXT_CLASS[tier])}>
                          {rounded}
                        </td>
                      );
                    })}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
