export type PoiCategoryStyle = { icon: string; label: string; color: string };

/**
 * Category -> Lineicons class, label, and marker color for the POI Density
 * Layer (docs/IMPLEMENTATION_PHASES.md Phase 4 deliverable 7). Class names
 * verified against the installed lineicons package
 * (node_modules/lineicons/dist/lineicons.css), not guessed. Colors are the
 * dedicated `poi-*` tokens (DESIGN.md "POI Category Markers") -- never a
 * CTA line color or a `score-*` tier color, per DESIGN.md's "Category
 * Color Independence Rule": CTA station markers already render as neutral
 * dots on the map, so without a real color of their own here, POI markers
 * were visually indistinguishable from stations.
 */
const POI_STYLE_BY_CATEGORY: Record<string, PoiCategoryStyle> = {
  cafe: { icon: "lni-coffee-cup-2", label: "Cafe", color: "#c2703d" },
  fitness: { icon: "lni-dumbbell-1", label: "Fitness", color: "#6366f1" },
  supermarket: { icon: "lni-basket-shopping-3", label: "Supermarket", color: "#0d9488" },
};

const DEFAULT_POI_STYLE: PoiCategoryStyle = { icon: "lni-map-marker-1", label: "Place", color: "#71717a" };

export function poiIcon(category: string): PoiCategoryStyle {
  return POI_STYLE_BY_CATEGORY[category] ?? DEFAULT_POI_STYLE;
}

/**
 * Builds a MapLibre `match` expression mapping the `category` feature
 * property to its poi-* color, generated from the same
 * POI_STYLE_BY_CATEGORY table poiIcon reads -- so the marker fill and the
 * hover card's icon color can never drift out of sync with each other.
 */
export function poiCircleColorExpression(): unknown[] {
  const expression: unknown[] = ["match", ["get", "category"]];
  for (const [category, style] of Object.entries(POI_STYLE_BY_CATEGORY)) {
    expression.push(category, style.color);
  }
  expression.push(DEFAULT_POI_STYLE.color);
  return expression;
}
