import { ctaLineColor } from "./cta-colors";
import { BIKE_GREEN } from "./ProtectedBikeLayer";
import { DIVVY_BLUE, DIVVY_GRAY } from "./divvyMarkerIcon";
import { poiIcon } from "./poi-icons";

/** One entry per Phase 4 map layer (docs/IMPLEMENTATION_PHASES.md Phase 5 deliverable 5) -- keys match the visibility state App.tsx owns and threads down to each layer component's `visible` prop. */
export type MapLayerKey = "ctaRail" | "bikeTrail" | "poiDensity" | "divvyDocks";

export type MapLegendLineDetail = {
  label: string;
  weight: "thick" | "thin";
  dashed?: boolean;
};

export type MapLegendEntry = {
  key: MapLayerKey;
  label: string;
  /**
   * 1 color renders as a single swatch square; 2-3 render as a small
   * cluster of dots -- CTA Rail and POI Density each cover several
   * real, independently-colored map features (8 CTA lines; 3 POI
   * categories), so one flat swatch color would misrepresent them.
   */
  swatchColors: string[];
  /**
   * Optional mini line-weight/dash key rendered under this entry's main
   * row, explaining a real map encoding the swatch color alone can't --
   * ProtectedBikeLayer.tsx already draws 4 real width/dash tiers by
   * routeType (Protected 3px solid down to Bike Lane/Marked Shared Lane
   * 1.5px, the latter dashed), collapsed here to the 2 that matter most at
   * a glance: a real dedicated lane vs. a painted-only shared lane marking.
   */
  lineDetail?: MapLegendLineDetail[];
  /**
   * Renders `swatchColors[0]`/`[1]` as a left/right split circle instead of
   * the default square/dot-cluster rendering. DivvyDockLayer's own
   * two-tone marker (divvyMarkerIcon.ts, DESIGN.md "Divvy Dock Markers")
   * exists specifically because Divvy's brand blue sits under 1deg apart in
   * hue from CTA Blue Line's -- the split *pattern*, not the hue alone, is
   * what disambiguates it at a glance. A flat single-color swatch here
   * would silently undo that reasoning.
   */
  splitCircle?: boolean;
};

/**
 * Every color here is reused from its layer's own single source of truth
 * (cta-colors.ts, ProtectedBikeLayer.tsx, poi-icons.ts, divvyMarkerIcon.ts)
 * rather than re-declared -- the legend must never drift out of sync with
 * what the map itself actually renders.
 */
export const MAP_LEGEND_ENTRIES: MapLegendEntry[] = [
  // Red + Blue -- the two most recognizable CTA lines; a full 8-line dot
  // cluster would be too small/busy to read at legend size, and unlike POI
  // Density's 3 categories, individual line identity isn't this entry's
  // own legend concern -- CtaRailLayer's own station hover cards already
  // show real per-line badges for that.
  { key: "ctaRail", label: "CTA Rail", swatchColors: [ctaLineColor("Red"), ctaLineColor("Blue")] },
  {
    key: "bikeTrail",
    label: "Bike / Trail",
    swatchColors: [BIKE_GREEN],
    lineDetail: [
      { label: "Protected", weight: "thick" },
      { label: "Shared lane", weight: "thin", dashed: true },
    ],
  },
  // One combined entry/checkbox for all 3 POI categories, not 3 independent
  // ones -- there's only one underlying MapLibre layer (POIS_LAYER_ID in
  // PoiDensityLayer.tsx, category-colored via a single circle-color match
  // expression), so 3 legend rows would mean either 3 checkboxes that
  // secretly all control the same one thing, or building real per-category
  // filter-layers. Future consideration: if more POI categories get added
  // later, this list keeps growing sideways in the label ("Cafes / Fitness
  // / Groceries / ..."), which won't stay readable forever -- splitting
  // into genuinely independent per-category layers (its own MapLibre layer
  // + filter per category, its own visibility state) would be the real
  // fix at that point, not just a legend/label change.
  {
    key: "poiDensity",
    label: "Cafes / Fitness / Groceries",
    swatchColors: [poiIcon("cafe").color, poiIcon("fitness").color, poiIcon("supermarket").color],
  },
  { key: "divvyDocks", label: "Divvy Docks", swatchColors: [DIVVY_BLUE, DIVVY_GRAY], splitCircle: true },
];
