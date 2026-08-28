import { FACTOR_ORDER, type FactorKey } from "./factor-metadata";

export type WeightFactorKey = Exclude<FactorKey, "commute">;

export type Weights = Record<WeightFactorKey, number>;

/**
 * The 9 factors the Weight Customization Slider Drawer exposes --
 * `FACTOR_ORDER` minus `commute`. `S_commute` only ever activates when a
 * scoring request supplies a destination (docs/PRD.md §3.2), and no
 * destination-entry UI exists anywhere in the app yet (a separate,
 * not-yet-built deliverable) -- a commute slider would let a user "weight"
 * a factor that can never actually score. Once that UI lands, this list
 * (and PRESETS below) is the only place that needs to grow to include it.
 */
export const WEIGHT_FACTOR_ORDER = FACTOR_ORDER.filter((key): key is WeightFactorKey => key !== "commute");

/**
 * The two persona presets from docs/PRD.md §3.4, exactly as documented
 * there (including `commute`, even though the drawer itself never surfaces
 * it -- see WEIGHT_FACTOR_ORDER above). Keeping the full 10-key table
 * rather than pre-stripping `commute` means these numbers stay a literal,
 * checkable transcription of the PRD table; weightsForPreset below is what
 * actually drops and renormalizes it for the drawer's own use.
 */
export const PRESETS = {
  remoteWorker: {
    transit: 25,
    commute: 0,
    bike: 12,
    grocery: 18,
    thirdPlace: 13,
    green: 9,
    noise: 9,
    safety: 9,
    flood: 3,
    solar: 2,
  },
  hybridCommuter: {
    transit: 17,
    commute: 17,
    bike: 10,
    grocery: 13,
    thirdPlace: 9,
    green: 9,
    noise: 10,
    safety: 10,
    flood: 3,
    solar: 2,
  },
} as const satisfies Record<string, Record<FactorKey, number>>;

export type PresetName = keyof typeof PRESETS;

export const PRESET_LABELS: Record<PresetName, string> = {
  remoteWorker: "Remote Worker",
  hybridCommuter: "Hybrid Commuter",
};

const WEIGHT_SUM = 100;

function clampWeight(value: number): number {
  if (Number.isNaN(value)) return 0;
  return Math.min(WEIGHT_SUM, Math.max(0, Math.round(value)));
}

/**
 * Scales a non-negative weight distribution to sum to exactly `targetSum`,
 * preserving relative proportions, via largest-remainder (Hamilton)
 * apportionment: floor every proportional share, then hand the leftover
 * whole points to the shares with the largest fractional remainder --
 * plain integer division would round every share down and lose points off
 * the total, and naive round-half-up can overshoot it. Falls back to an
 * even split of `targetSum` when every input is 0 (nothing to scale
 * proportionally from).
 */
function scaleToSum(raw: Record<WeightFactorKey, number>, keys: readonly WeightFactorKey[], targetSum: number): Weights {
  const result = {} as Weights;

  if (targetSum <= 0) {
    keys.forEach((k) => {
      result[k] = 0;
    });
    return result;
  }

  const nonNegative = keys.map((k) => Math.max(0, raw[k] ?? 0));
  const total = nonNegative.reduce((sum, v) => sum + v, 0);

  if (total === 0) {
    const even = Math.floor(targetSum / keys.length);
    let remainder = targetSum - even * keys.length;
    keys.forEach((k) => {
      result[k] = even + (remainder > 0 ? 1 : 0);
      if (remainder > 0) remainder -= 1;
    });
    return result;
  }

  const scaled = nonNegative.map((v) => (v / total) * targetSum);
  const floors = scaled.map(Math.floor);
  const remainder = targetSum - floors.reduce((sum, v) => sum + v, 0);
  const byRemainingFractionDesc = keys
    .map((_, i) => i)
    .sort((a, b) => scaled[b] - floors[b] - (scaled[a] - floors[a]));

  keys.forEach((k, i) => {
    result[k] = floors[i];
  });
  for (let i = 0; i < remainder; i++) {
    result[keys[byRemainingFractionDesc[i]]] += 1;
  }
  return result;
}

/** Scales an arbitrary weight distribution (a preset with `commute` dropped, or values parsed from the URL) so `keys` sum to exactly 100. */
export function normalizeToSum100(raw: Record<WeightFactorKey, number>, keys: readonly WeightFactorKey[] = WEIGHT_FACTOR_ORDER): Weights {
  return scaleToSum(raw, keys, WEIGHT_SUM);
}

/** `PRESETS[name]` with `commute` dropped and the remaining 9 factors renormalized to sum to 100 (docs/PRD.md's own "Dynamic Normalization" rule). */
export function weightsForPreset(name: PresetName): Weights {
  const preset = PRESETS[name];
  const raw = {} as Weights;
  WEIGHT_FACTOR_ORDER.forEach((k) => {
    raw[k] = preset[k];
  });
  return normalizeToSum100(raw);
}

/**
 * Applies a user's slider drag: `key` moves to exactly `rawNewValue`
 * (clamped 0-100) and is never itself rescaled -- it's the one slider the
 * user just dragged to a specific spot. The other 8 factors are rescaled
 * (via scaleToSum, keeping their existing relative proportions) to whatever
 * total is left over, so the full set keeps summing to 100. Assumes
 * `current` already sums to 100 (every caller in this app only ever passes
 * a previous redistributeWeights/weightsForPreset/normalizeToSum100
 * result), in which case the other 8 always collectively hold exactly
 * enough to let `key` reach 100 -- the `Math.min` below is a defensive
 * floor for that invariant, not reachable through a real slider drag.
 */
export function redistributeWeights(current: Weights, key: WeightFactorKey, rawNewValue: number): Weights {
  const clampedNew = clampWeight(rawNewValue);
  const oldValue = current[key];
  const delta = clampedNew - oldValue;
  if (delta === 0) return current;

  const otherKeys = WEIGHT_FACTOR_ORDER.filter((k) => k !== key);
  const othersOldTotal = otherKeys.reduce((sum, k) => sum + current[k], 0);

  const actualDelta = delta > 0 ? Math.min(delta, othersOldTotal) : delta;
  const newKeyValue = oldValue + actualDelta;
  const othersNewTotal = WEIGHT_SUM - newKeyValue;

  const others = scaleToSum(current, otherKeys, othersNewTotal);
  return { ...others, [key]: newKeyValue };
}

/** The preset `weights` exactly matches (every factor equal), or `null` once the user has dragged a slider away from both. Drives which preset chip, if any, renders as active. */
export function matchPreset(weights: Weights): PresetName | null {
  for (const name of Object.keys(PRESETS) as PresetName[]) {
    const preset = weightsForPreset(name);
    if (WEIGHT_FACTOR_ORDER.every((k) => weights[k] === preset[k])) return name;
  }
  return null;
}
