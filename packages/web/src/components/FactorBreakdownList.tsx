import { FactorCard } from "./ui/factor-card";
import { FACTOR_METADATA, FACTOR_ORDER } from "../lib/factor-metadata";
import type { components } from "../lib/api/schema";

type ScoreFactors = components["schemas"]["ScoreResponse"]["factors"];

export type FactorBreakdownListProps = {
  factors: ScoreFactors;
};

/**
 * The 9-10 Dense Factor Breakdown Cards (docs/IMPLEMENTATION_PHASES.md
 * Phase 5 deliverable 2) -- one accordion per scored factor, in
 * docs/PRD.md's §4 weighting-table order. `commute` only renders when the
 * API included it, which it does only when a destination was supplied.
 */
export function FactorBreakdownList({ factors }: FactorBreakdownListProps) {
  return (
    <div className="space-y-2">
      {FACTOR_ORDER.map((key) => {
        const factor = factors[key];
        if (!factor) return null;
        const { label, icon } = FACTOR_METADATA[key];
        return <FactorCard key={key} label={label} icon={icon} factor={factor} />;
      })}
    </div>
  );
}
