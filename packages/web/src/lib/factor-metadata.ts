/**
 * The 10 sub-factor keys ScoreResponse.factors can carry (schema.d.ts's
 * ScoreResponse["factors"]), in the fixed display order docs/PRD.md's
 * weighting table (§4) lists them.
 */
export const FACTOR_ORDER = [
  "transit",
  "commute",
  "bike",
  "grocery",
  "thirdPlace",
  "green",
  "noise",
  "safety",
  "flood",
  "solar",
] as const;

export type FactorKey = (typeof FACTOR_ORDER)[number];

export type FactorMetadata = {
  label: string;
  /** Lineicons class name, e.g. "lni-train-1" (docs/ARCHITECTURE.md §7.2: 14-16px, 1.5px stroke). */
  icon: string;
};

export const FACTOR_METADATA: Record<FactorKey, FactorMetadata> = {
  transit: { label: "Transit", icon: "lni-train-1" },
  commute: { label: "Commute", icon: "lni-route-1" },
  bike: { label: "Bike Infrastructure", icon: "lni-bike" },
  grocery: { label: "Groceries", icon: "lni-basket-shopping-3" },
  thirdPlace: { label: "Third Places", icon: "lni-coffee-cup-2" },
  green: { label: "Green Space", icon: "lni-tree-2" },
  noise: { label: "Noise Environment", icon: "lni-volume-high" },
  safety: { label: "Public Safety", icon: "lni-shield-2" },
  flood: { label: "Flood Risk", icon: "lni-water-drop-1" },
  solar: { label: "Solar Exposure", icon: "lni-sun-1" },
};
