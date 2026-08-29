/**
 * Real CTA bus route service category colors, verified by downloading and
 * inspecting CTA's own live published GTFS feed (routes.txt) rather than
 * assumed -- not every category the feed's route_color implies is used
 * here: CTA colors "local" and "express" (including every X-prefixed
 * arterial route) consistently, but Night Owl service isn't reliably
 * identifiable from route data (only one of ~18 real Owl routes has its
 * own route id; the rest run as late-night trips on their normal numbered
 * route), so it's deliberately not a category here.
 */
export const BUS_CATEGORY_COLORS = {
  local: "#99999c",
  express: "#b71234",
  jump: "#0065bd",
} as const;

export type BusCategory = keyof typeof BUS_CATEGORY_COLORS;

const BUS_CATEGORY_LABELS: Record<BusCategory, string> = {
  local: "Local bus",
  express: "Express bus",
  jump: "Jeffery Jump",
};

function isBusCategory(value: string): value is BusCategory {
  return value in BUS_CATEGORY_COLORS;
}

/** Falls back to the local/grey color for anything unrecognized -- `category` comes from the API over the wire, not a closed local constant. */
export function busCategoryColor(category: string): string {
  return isBusCategory(category) ? BUS_CATEGORY_COLORS[category] : BUS_CATEGORY_COLORS.local;
}

/** Falls back to a generic "Bus" label for anything unrecognized. */
export function busCategoryLabel(category: string): string {
  return isBusCategory(category) ? BUS_CATEGORY_LABELS[category] : "Bus";
}
