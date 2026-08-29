import { useId, useState } from "react";
import { cn } from "../lib/utils";
import { MAP_LEGEND_ENTRIES, type MapLayerKey, type MapLegendLineDetail } from "./layers/legend-entries";

export type MapLegendProps = {
  visibility: Record<MapLayerKey, boolean>;
  onToggle: (key: MapLayerKey) => void;
};

/**
 * Map Legend & Layer Visibility Controls (docs/IMPLEMENTATION_PHASES.md
 * Phase 5 deliverable 5): a collapsible panel anchored on the map itself
 * (top-left, mirroring MapCanvas's NavigationControl at top-right) rather
 * than the left analytics panel -- map display is a map-chrome concern, not
 * a score concern. One entry per Phase 4 layer: a color swatch doubling as
 * the legend, plus a checkbox toggling that layer's visibility. Manual
 * toggle only -- every layer stays on by default, no smart/auto-hide.
 */
export function MapLegend({ visibility, onToggle }: MapLegendProps) {
  const [expanded, setExpanded] = useState(false);
  const panelId = useId();

  return (
    <div className="absolute top-3 left-3 z-10 w-64 rounded-lg border border-zinc-800 bg-zinc-950/90 text-zinc-100">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center gap-2 p-2 text-left"
        aria-expanded={expanded}
        aria-controls={expanded ? panelId : undefined}
      >
        <i className="lni lni-layers-1 text-sm text-zinc-400" aria-hidden="true" />
        <span className="flex-1 text-xs font-medium tracking-wide text-zinc-300 uppercase">Layers</span>
        <i
          className={cn("lni lni-chevron-down text-xs text-zinc-500 transition-transform", expanded && "rotate-180")}
          aria-hidden="true"
        />
      </button>

      {expanded && (
        <ul id={panelId} className="space-y-1.5 border-t border-zinc-800 p-2">
          {MAP_LEGEND_ENTRIES.map((entry) => {
            const checkboxId = `${panelId}-${entry.key}`;
            return (
              <li key={entry.key}>
                <div className="flex items-center gap-2">
                  <input
                    id={checkboxId}
                    type="checkbox"
                    checked={visibility[entry.key]}
                    onChange={() => onToggle(entry.key)}
                    className="h-3.5 w-3.5 shrink-0 accent-zinc-100"
                  />
                  <Swatch colors={entry.swatchColors} splitCircle={entry.splitCircle} />
                  <label htmlFor={checkboxId} className="truncate text-xs text-zinc-300">
                    {entry.label}
                  </label>
                </div>

                {entry.lineDetail && (
                  <ul className="mt-1 space-y-1 pl-[1.375rem]">
                    {entry.lineDetail.map((detail) => (
                      <li key={detail.label} className="flex items-center gap-1.5">
                        <LineSample detail={detail} color={entry.swatchColors[0]} />
                        <span className="truncate text-[11px] text-zinc-400">{detail.label}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

/**
 * A layer's color key: a single square for 1 color, a small dot cluster for
 * 2-3 (CTA Rail's lines, POI Density's categories), or -- when
 * `splitCircle` is set (Divvy Docks only) -- a left/right split circle
 * matching divvyMarkerIcon.ts's real two-tone marker exactly, rather than
 * collapsing it to one flat color and losing the split pattern that marker
 * exists to convey.
 */
function Swatch({ colors, splitCircle }: { colors: string[]; splitCircle?: boolean }) {
  if (splitCircle && colors.length === 2) {
    return (
      <span
        aria-hidden="true"
        className="inline-block h-3 w-3 shrink-0 rounded-full border border-zinc-900"
        style={{ background: `linear-gradient(to right, ${colors[0]} 50%, ${colors[1]} 50%)` }}
      />
    );
  }

  return (
    <span className="flex shrink-0 items-center gap-0.5" aria-hidden="true">
      {colors.map((color) => (
        // Swatch colors are a fixed, ordered tuple per entry
        // (legend-entries.ts), never reordered/filtered at render time --
        // and are already the natural unique key.
        <span
          key={color}
          className={cn("inline-block", colors.length === 1 ? "h-3 w-3 rounded-sm" : "h-2 w-2 rounded-full")}
          style={{ backgroundColor: color }}
        />
      ))}
    </span>
  );
}

/** A short horizontal line sample (weight + solid/dashed) explaining one of a line layer's real width/dash tiers -- e.g. ProtectedBikeLayer.tsx's protected-lane-vs-shared-lane encoding, which a flat swatch color can't convey on its own. */
function LineSample({ detail, color }: { detail: MapLegendLineDetail; color: string }) {
  return (
    <span
      aria-hidden="true"
      className="inline-block w-4"
      style={{
        borderTopWidth: detail.weight === "thick" ? 2 : 1,
        borderTopStyle: detail.dashed ? "dashed" : "solid",
        borderTopColor: color,
      }}
    />
  );
}
