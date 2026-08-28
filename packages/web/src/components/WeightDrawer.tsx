import { useId, useState } from "react";
import { Card } from "./ui/card";
import { cn } from "../lib/utils";
import { FACTOR_METADATA } from "../lib/factor-metadata";
import {
  PRESET_LABELS,
  WEIGHT_FACTOR_ORDER,
  type PresetName,
  type WeightFactorKey,
  type Weights,
  matchPreset,
  redistributeWeights,
  weightsForPreset,
} from "../lib/weights";

export type WeightDrawerProps = {
  weights: Weights;
  onChange: (weights: Weights) => void;
};

/**
 * Collapsible Weight Customization Slider Drawer (docs/IMPLEMENTATION_PHASES.md
 * Phase 5 deliverable 2, docs/PRD.md §3.4): one slider per active factor
 * (commute excluded -- see lib/weights.ts), plus the Remote Worker/Hybrid
 * Commuter preset chips. Collapsed by default to match the panel's
 * high-density default state (same reasoning as FactorCard's own
 * collapsed-by-default accordions).
 */
export function WeightDrawer({ weights, onChange }: WeightDrawerProps) {
  const [expanded, setExpanded] = useState(false);
  const panelId = useId();
  const activePreset = matchPreset(weights);

  return (
    <Card>
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center justify-between text-left"
        aria-expanded={expanded}
        aria-controls={panelId}
      >
        <div className="flex items-center gap-2">
          <i className="lni lni-sliders-horizontal-square-2 text-sm text-zinc-400" aria-hidden="true" />
          <span className="text-xs font-medium tracking-wide text-zinc-300 uppercase">Weight Customization</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs text-zinc-500">
            {activePreset ? PRESET_LABELS[activePreset] : "Custom"}
          </span>
          <i
            className={cn("lni lni-chevron-down text-xs text-zinc-500 transition-transform", expanded && "rotate-180")}
            aria-hidden="true"
          />
        </div>
      </button>

      {expanded && (
        <div id={panelId} className="mt-4 space-y-4">
          <div className="flex gap-2">
            {(Object.keys(PRESET_LABELS) as PresetName[]).map((name) => (
              <PresetChip
                key={name}
                label={PRESET_LABELS[name]}
                active={activePreset === name}
                onClick={() => onChange(weightsForPreset(name))}
              />
            ))}
          </div>

          <div className="space-y-3">
            {WEIGHT_FACTOR_ORDER.map((key) => (
              <WeightSlider
                key={key}
                factorKey={key}
                value={weights[key]}
                onChange={(v) => onChange(redistributeWeights(weights, key, v))}
              />
            ))}
          </div>
        </div>
      )}
    </Card>
  );
}

function PresetChip({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "rounded-sm border px-2 py-1 font-mono text-xs tracking-wide uppercase transition-colors",
        active
          ? "border-zinc-500 bg-zinc-800 text-zinc-100"
          : "border-zinc-800 bg-transparent text-zinc-400 hover:bg-zinc-900"
      )}
    >
      {label}
    </button>
  );
}

function WeightSlider({
  factorKey,
  value,
  onChange,
}: {
  factorKey: WeightFactorKey;
  value: number;
  onChange: (value: number) => void;
}) {
  const { label, icon } = FACTOR_METADATA[factorKey];
  const sliderId = useId();

  return (
    <div>
      <div className="flex items-center justify-between gap-2">
        <label htmlFor={sliderId} className="flex items-center gap-1.5 text-xs font-medium tracking-wide text-zinc-300 uppercase">
          <i className={cn("lni", icon, "text-sm text-zinc-400")} aria-hidden="true" />
          {label}
        </label>
        <span className="font-mono text-xs font-semibold text-zinc-100">{value}%</span>
      </div>
      <input
        id={sliderId}
        type="range"
        min={0}
        max={100}
        step={1}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="bf-weight-slider mt-1.5 w-full"
        aria-valuetext={`${value}%`}
      />
    </div>
  );
}
