import { WEIGHT_FACTOR_ORDER, type Weights, type WeightFactorKey, normalizeToSum100, weightsForPreset } from "./weights";

/**
 * Flat, human-readable URL query keys for each weight (docs/ARCHITECTURE.md
 * §7.1's own example: `?w_transit=35&w_grocery=25`) -- deliberately not a
 * single JSON-blob search param, so the URL stays shareable/readable and a
 * single weight can be tweaked by hand.
 */
export type WeightSearch = { [K in WeightFactorKey as `w_${K}`]: number };

function searchKey(key: WeightFactorKey): keyof WeightSearch {
  return `w_${key}`;
}

export function weightsToSearch(weights: Weights): WeightSearch {
  const search = {} as WeightSearch;
  WEIGHT_FACTOR_ORDER.forEach((k) => {
    search[searchKey(k)] = weights[k];
  });
  return search;
}

export function searchToWeights(search: WeightSearch): Weights {
  const weights = {} as Weights;
  WEIGHT_FACTOR_ORDER.forEach((k) => {
    weights[k] = search[searchKey(k)];
  });
  return weights;
}

/**
 * TanStack Router `validateSearch`: turns whatever raw, unvalidated query
 * params showed up in the URL into a complete, sum-to-100 `WeightSearch`.
 * A fresh visit (no `w_*` params at all) gets the Remote Worker preset
 * (docs/PRD.md §3.4's documented default). Once any `w_*` param is present,
 * every factor is read from the URL independently -- an omitted or garbage
 * (non-numeric, negative) value reads as 0, not the preset's value for that
 * factor, and the whole set is renormalized to sum to 100. This keeps the
 * URL a complete, self-contained snapshot rather than a partial overlay on
 * a preset a reader of the link wouldn't know to assume.
 */
export function validateWeightSearch(raw: Record<string, unknown>): WeightSearch {
  const hasAnyWeightParam = WEIGHT_FACTOR_ORDER.some((k) => raw[searchKey(k)] !== undefined);
  if (!hasAnyWeightParam) return weightsToSearch(weightsForPreset("remoteWorker"));

  const rawWeights = {} as Weights;
  WEIGHT_FACTOR_ORDER.forEach((k) => {
    const value = Number(raw[searchKey(k)]);
    rawWeights[k] = Number.isFinite(value) && value >= 0 ? value : 0;
  });
  return weightsToSearch(normalizeToSum100(rawWeights));
}
