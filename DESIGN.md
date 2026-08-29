---
name: BlockFactor
description: High-density urban location intelligence and livability analytics for Chicago
colors:
  bg-canvas: "#09090b"
  surface-card: "#18181b"
  surface-header: "#18181b"
  border-subtle: "#27272a"
  border-active: "#3f3f46"
  text-primary: "#f4f4f5"
  text-secondary: "#a1a1aa"
  text-muted: "#71717a"
  cta-red: "#c60c30"
  cta-blue: "#00a1de"
  cta-brown: "#62361b"
  cta-green: "#009b3a"
  cta-pink: "#e27ea6"
  cta-orange: "#f9461c"
  cta-purple: "#522398"
  cta-yellow: "#f9e300"
  score-emerald: "#10b981"
  score-lime: "#84cc16"
  score-amber: "#f59e0b"
  score-rose: "#f43f5e"
  poi-cafe: "#c2703d"
  poi-fitness: "#6366f1"
  poi-supermarket: "#0d9488"
  divvy-blue: "#41b6e6"
typography:
  display:
    fontFamily: "Geist Mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
    fontSize: "2.25rem"
    fontWeight: 700
    lineHeight: 1.1
    letterSpacing: "-0.02em"
  headline:
    fontFamily: "Inter, Geist, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.01em"
  title:
    fontFamily: "Inter, Geist, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: "0.05em"
  body:
    fontFamily: "Inter, Geist, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "normal"
  label:
    fontFamily: "Geist Mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
    fontSize: "0.75rem"
    fontWeight: 500
    lineHeight: 1.2
    letterSpacing: "0.05em"
rounded:
  sm: "2px"
  md: "4px"
  lg: "6px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  card:
    backgroundColor: "{colors.surface-card}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.sm}"
    padding: "16px"
  badge-default:
    backgroundColor: "{colors.border-subtle}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.sm}"
    padding: "2px 8px"
  badge-cta-blue:
    backgroundColor: "{colors.cta-blue}"
    textColor: "#ffffff"
    rounded: "{rounded.sm}"
    padding: "2px 8px"
  badge-cta-red:
    backgroundColor: "{colors.cta-red}"
    textColor: "#ffffff"
    rounded: "{rounded.sm}"
    padding: "2px 8px"
  badge-score-emerald:
    backgroundColor: "#022c22"
    textColor: "{colors.score-emerald}"
    rounded: "{rounded.sm}"
    padding: "2px 8px"
---

# Design System: BlockFactor

## Overview

**Creative North Star: "The Urban Control Room"**

BlockFactor is an analytical, high-density location intelligence system engineered to deliver objective residential livability assessments for Chicago. The interface models a tactical urban control room: crisp dark zinc surfaces, strict 1px border delineation, authentic municipal CTA line badges, and monospace tabular telemetry. It rejects generic marketing froth in favor of raw informational density and mathematical clarity.

The visual language balances utilitarian cartography with modern software craft (in the lineage of Linear, Raycast, and Citymapper). Every pixel serves to communicate hyper-local environmental and spatial reality—distances, travel durations, acoustic buffers, and percentile scores—with absolute fidelity.

**Key Characteristics:**
- Tactical high-density information architecture prioritizing metrics, entity names, and exact distances
- Flat tonal layering (deep zinc-950 canvas, zinc-900 surfaces, zinc-800 borders, zero drop shadows)
- Strict data typography: `Geist Mono` for all numeric scores, coordinates, distances, and transit durations
- Authentic municipal color palette anchored in official CTA transit line hues
- Zero AI novelty clichés (no purple neon gradients, no sparkle icons, no emoji slop)

## Colors

The palette is anchored in deep dark neutrals with official municipal Chicago Transit Authority line hues and clear score health indicators.

### Primary (CTA Rail Lines)
- **CTA Red Line** (`#c60c30`): North-south spine transit line indicator.
- **CTA Blue Line** (`#00a1de`): O'Hare / Northwest / West transit line indicator.
- **CTA Brown Line** (`#62361b`): Ravenswood / North Side loop transit indicator.
- **CTA Green Line** (`#009b3a`): West / South Side transit line indicator.
- **CTA Pink Line** (`#e27ea6`): Pilsen / Southwest transit line indicator.
- **CTA Orange Line** (`#f9461c`): Midway / Southwest transit line indicator.
- **CTA Purple Line** (`#522398`): Evanston express transit indicator.
- **CTA Yellow Line** (`#f9e300`): Skokie Swift transit indicator.

### Score Indicators
- **Score Optimal / Emerald** (`#10b981`): High livability scores (80–100) and top percentiles.
- **Score Good / Lime** (`#84cc16`): Solid livability scores (65–79).
- **Score Moderate / Amber** (`#f59e0b`): Average or moderate caution scores (45–64).
- **Score Alert / Rose** (`#f43f5e`): High friction, severe noise, flood risk, or low scores (0–44).

### POI Category Markers
Map-overlay-only accent colors for the POI Density Layer (`GET /api/v1/places`), distinguishing cafe/fitness/supermarket points from each other and from CTA station markers. Deliberately never reused from the CTA or Score Indicator sets above — see "The Category Color Independence Rule" below.
- **POI Cafe** (`#c2703d`): Cafe/coffee shop markers.
- **POI Fitness** (`#6366f1`): Gym/fitness studio/yoga markers.
- **POI Supermarket** (`#0d9488`): Supermarket markers.

### Divvy Dock Markers
The Divvy Dock Layer (`GET /api/v1/layers/divvy-docks`) uses Divvy's real brand blue (`#41b6e6`, Pantone 298 C, sourced from Divvy's public identity guide — distinct from, but close in hue to, CTA Blue Line's `#00a1de`/Pantone 299 C). Because the hue alone is close enough to CTA Blue to risk the same "can't tell these two layers apart" problem the POI Category Markers were introduced to fix, the Divvy marker is a two-tone split icon (Divvy Blue + `text-secondary` `#a1a1aa`/zinc-400, standing in for the grey used on Divvy's retroreflective e-bike fleet — no verified digital hex exists for that physical coating) rather than a flat single-color dot: the split *pattern* itself is what reads as distinct at a glance, not the hue in isolation.

### Transit Factor Card Dots
The Transit factor card's contributing-POI rows use small colored dots, not text or badges, to carry line/category meaning inline next to a plain-text POI name — chosen after an earlier badge-pill iteration (colored `<Badge>` chips) was live-reviewed and found to visually compete with the name text itself for attention on a dense list.
- **CTA station line dots** reuse the Primary (CTA Rail Lines) hex codes above exactly — one dot per line serving that station, in station order, so a multi-line stop (e.g. Fullerton: Red, Brown, Purple) shows one dot per line rather than one blended color. An unrecognized line name falls back to Text Muted (`#71717a`) rather than a blind color lookup.
- **CTA bus category dots** get their own dedicated tokens, verified against CTA's live published GTFS feed (`routes.txt`) rather than assumed: **Bus Local** (`#99999c`, grey) for standard routes, **Bus Express** (`#b71234`, maroon) for X-prefixed arterial routes and any route whose real name contains "Express", and **Bus Jump** (`#0065bd`, royal blue) for the single Jeffery Jump route (J14). These intentionally are NOT reused from the CTA rail palette (per the Category Color Independence Rule below) since a bus route and a rail line are different service types even when a hue happens to be nearby. "Night Owl" and "Arterial Rapid Transit" were investigated as possible additional categories and found not to be real, GTFS-derivable CTA distinctions, so neither exists as a token or category here.
- Every dot carries both a hover `title` and an `aria-label` naming the line or category in plain text (e.g. "Red Line", "Express bus") — since a dot's color is otherwise the *only* signal it carries, and color alone fails both color-blind viewers and screen readers.

### Solar Exposure Badge
The scored unit's window-facing orientation (PRD §5.8 `S_solar`) is shown as a plain sidebar `<Badge>` (e.g. "W-Facing · 75"), colored by its actual score tier (Emerald for South/SE/SW/E, Lime for W, Amber for NE/NW, Rose for N) — reusing the Score Indicator tiers above is the correct semantic fit here, not a borrowed one, since this badge genuinely represents a score. An earlier version instead drew a 3D `fill-extrusion` ray on the map itself, colored/sized the same way; live review found that a novel 3D shape, a color-tier meaning, and a compass direction all at once was still too much for a viewer to parse at a glance with no on-screen legend yet, so the score moved to the sidebar's plain badge language (`DESIGN.md` "Layout": the sidebar is the analytical-truth surface, the map is the spatial-context surface) instead of asking the map scene to carry it alone.

### Map Legend
The Map Legend & Layer Visibility Controls (`packages/web/src/map/MapLegend.tsx`, `docs/IMPLEMENTATION_PHASES.md` Phase 5 deliverable 5) — a collapsible panel anchored top-left on the map itself, mirroring `NavigationControl`'s top-right position — reuses every color above rather than declaring its own (`packages/web/src/map/layers/legend-entries.ts` imports each value from its layer's own single source of truth: `cta-colors.ts`, `ProtectedBikeLayer.tsx`, `poi-icons.ts`, `divvyMarkerIcon.ts`). One entry per Phase 4 layer, each with a checkbox toggling that layer's visibility (manual only — every layer stays on by default, no smart/auto-hide):
- **CTA Rail:** a 2-dot cluster (Red + Blue, the two most recognizable lines) rather than all 8 — a full-line swatch would be too small/busy to read at legend size, and individual line identity isn't this entry's job; `CtaRailLayer`'s own station hover cards already show real per-line badges for that.
- **Bike / Trail:** a single green swatch, plus a small width/dash mini-key underneath (`MapLegendLineDetail`) reading "Protected" (thick solid) / "Shared lane" (thin dashed) — `ProtectedBikeLayer.tsx` actually draws 4 real tiers by `routeType` (Protected 3px down to Bike Lane/Marked Shared Lane 1.5px dashed), collapsed here to the 2 that matter most at a glance rather than an exhaustive breakdown.
- **Cafes / Fitness / Groceries (POI Density):** one combined entry/checkbox, not 3 independent ones — there's only one underlying MapLibre layer under the hood (`POIS_LAYER_ID`, category-colored via a single `circle-color` match expression), so 3 rows would mean either 3 checkboxes secretly controlling the same one thing, or real per-category filter-layers (a bigger change than a legend tweak). A 3-dot cluster (cafe/fitness/supermarket, `poi-*` tokens) stands in for the swatch. *Future consideration:* if more POI categories get added later, this one label keeps growing sideways and eventually stops being readable — splitting into genuinely independent per-category layers (own MapLibre layer + filter + visibility state per category) would be the real fix at that point.
- **Divvy Docks:** a left/right split circle (Divvy Blue + `text-secondary` grey), not a flat swatch — matching `divvyMarkerIcon.ts`'s real two-tone marker exactly. That marker exists specifically because Divvy's brand blue sits under 1° apart in hue from CTA Blue Line's, so the split *pattern*, not the hue alone, is what disambiguates it; a flat blue square here would have silently undone that reasoning.

### Neutral
- **Canvas Deep Zinc** (`#09090b` / `zinc-950`): Root viewport and map background.
- **Surface Card Zinc** (`#18181b` / `zinc-900`): Analytical cards, containers, and table rows.
- **Border Subtle** (`#27272a` / `zinc-800`): 1px structural container and divider borders.
- **Border Active** (`#3f3f46` / `zinc-700`): Focused inputs, interactive borders, and selected chips.
- **Text Primary** (`#f4f4f5` / `zinc-100`): Primary headings, numbers, and active labels.
- **Text Secondary** (`#a1a1aa` / `zinc-400`): Factor names, descriptions, and metric subtitles.
- **Text Muted** (`#71717a` / `zinc-500`): Max score denominators (`/ 100`), telemetry labels, timestamps.

### Named Rules
**The Authentic Transit Palette Rule.** Transit badges and line indicators must use exact Chicago CTA hex codes (Red, Blue, Brown, Green, Pink, Orange, Purple, Yellow); never approximate or theme-shift official municipal lines.

**The One Voice Rule.** Accent colors appear strictly on badges, score tiers, and map overlays. Surfaces and containers remain strictly neutral zinc to preserve high contrast and scanability.

**The Category Color Independence Rule.** A color that carries meaning in one part of the app (a CTA line, a `score-*` livability tier) must never be reused for an unrelated category elsewhere, even on a different surface (e.g. a map marker) — a POI marker rendered in `score-amber` would misread as "this place scored 45–64," which isn't a thing a single POI has. POI category markers (cafe/fitness/supermarket) get their own dedicated `poi-*` tokens instead of borrowing from the CTA or Score Indicator sets.

## Typography

**Display Font:** `Geist Mono` (with `ui-monospace`, `SFMono-Regular`, `Menlo`, `monospace` fallbacks)  
**Body Font:** `Inter` / `Geist` (with `-apple-system`, `BlinkMacSystemFont`, `Segoe UI`, `Roboto`, `sans-serif` fallbacks)  
**Label/Mono Font:** `Geist Mono`  

**Character:** Technical, crisp, and tabular. Monospace powers all data points and telemetry to guarantee columnar alignment; clean sans-serif powers labels and descriptive copy.

### Hierarchy
- **Display** (Bold 700, `2.25rem` / `36px`, `line-height: 1.1`): Primary composite livability score readouts.
- **Headline** (SemiBold 600, `1.25rem` / `20px`, `line-height: 1.3`): Address titles, major modal headers.
- **Title** (SemiBold 600, `0.875rem` / `14px`, `line-height: 1.4`, uppercase tracking `0.05em`): Card titles, factor category headers.
- **Body** (Regular 400, `0.875rem` / `14px`, `line-height: 1.5`): Explanatory analysis notes, factor descriptions.
- **Label** (Medium 500, `0.75rem` / `12px`, `line-height: 1.2`, uppercase tracking `0.05em`): Monospace distance pills, coordinates, status badges, travel durations.

### Named Rules
**The Tabular Monospace Rule.** Every coordinate, distance, travel duration, and calculated score must render in `Geist Mono` with tabular lining figures to ensure zero visual jitter during live updates.

**The Uppercase Header Rule.** Section titles, card titles, and badge chips must use uppercase tracking (`uppercase tracking-wider` / `tracking-wide`) to maintain the tactical control room feel.

## Layout

The UI uses an asymmetrical split-pane app shell designed for concurrent visual and spatial analysis. On `lg+` screens the page itself never scrolls (`h-screen`/`overflow-hidden` on the root) -- the header, map, and Comparison Tray all stay in fixed position, and only the sidebar's own content scrolls internally:
- **Header:** `sticky top-0`, persistent at the top of the viewport at every breakpoint.
- **Left Sidebar (Desktop fixed `w-[400px]`):** Independently `overflow-y-auto` analytical column housing the address input, composite score card, Weight Customization Drawer, and the 10-factor breakdowns -- scrolls on its own without moving the header, map, or tray.
- **Right Map Canvas (Desktop `flex-1` / `h-full`):** Fills the exact remaining viewport height next to the sidebar (not a hand-tuned `vh` percentage, which could leave a gap beneath it depending on actual header height) -- MapLibre GL vector canvas displaying the candidate address, CTA rail network, high-frequency bus corridors, bike routes, and spatial heatmaps.
- **Bottom Comparison Tray:** Sits at the bottom of the same fixed viewport, below the map/sidebar row, only once at least one address is pinned.
- **Below `lg`:** Falls back to the original full-page-scroll stacked layout (a fixed-height split-screen doesn't fit a narrow single column) -- the header and tray stay `sticky` so they remain pinned during that scroll, but the sidebar and map scroll together with the page.
- **Spacing Scale:** Built on strict 4px/8px modular units (`p-4` / 16px cards, `gap-6` / 24px flex gutters, `py-0.5 px-2` badge chips).

## Elevation & Depth

BlockFactor uses **flat tonal layering**. Surfaces are flat at rest with zero drop shadows (`shadow-none`). Visual hierarchy and depth are conveyed entirely through tonal stepping and crisp 1px borders:
- Canvas: `zinc-950`
- Surface / Cards: `zinc-900` (80% opacity overlay over canvas)
- Headers / Floating Bars: `zinc-900/60` with backdrop blur
- Borders: 1px solid `zinc-800`

### Named Rules
**The Flat Surface Rule.** Surfaces are strictly flat with no drop shadows; visual hierarchy and elevation are conveyed exclusively through tonal stepping (`zinc-950` → `zinc-900` → `zinc-800`) and 1px border strokes.

## Shapes

- **Form Language:** Strict geometric rectangles with tight corner radii.
- **Radius Scale:**
  - `sm` (`2px` / `0.125rem`): Badges and chips only.
  - `md` (`4px` / `0.25rem`): Modal containers, dropdown menus.
  - `lg` (`6px` / `0.375rem`): Cards, buttons, inputs, and outer application wrappers — the app-wide default (AGENTS.md §4).
- **Borders:** 1px solid `zinc-800` on all cards and dividers; 1px colored strokes on score badges.

## Components

### Cards
- **Corner Style:** `rounded-lg` (6px).
- **Background:** `zinc-900/80` (`#18181b`).
- **Border:** 1px solid `zinc-800` (`#27272a`).
- **Internal Padding:** `p-4` (16px).
- **Header:** Uppercase `text-sm font-semibold tracking-tight text-zinc-100`.

### Badges & Chips
- **Corner Style:** `rounded-sm` (2px).
- **Typography:** `text-xs font-mono font-medium tracking-wide uppercase`.
- **Padding:** `px-2 py-0.5` (8px horizontal, 2px vertical).
- **Transit Variants:** Solid CTA line backgrounds (`#c60c30`, `#00a1de`, etc.) with white or dark text.
- **Score Variants:** 40% tint background with 60% border stroke (`emerald-950/40 border-emerald-900/60 text-emerald-400`).

### Buttons
- **Shape:** `rounded-lg` (6px).
- **Primary:** `bg-zinc-100 text-zinc-950 hover:bg-zinc-200` font-mono font-medium text-xs uppercase.
- **Secondary / Ghost:** `bg-zinc-800 text-zinc-100 hover:bg-zinc-700` with 1px `zinc-700` border.

### Comparison Tray
Sticky bottom dock (`sticky bottom-0`, `border-t border-zinc-800`, `bg-zinc-900/95`), hidden entirely until at least one address is pinned. Pinned addresses render as compact chips (`rounded-sm border border-zinc-800 bg-zinc-950`) carrying the truncated address, a Score Indicator badge for the composite score, and an unpin control -- the same visual language as everywhere else a score gets a badge, not a bespoke tray-only style. The factor-by-factor comparison table (reachable only once 2+ are pinned) reuses the Tabular Monospace Rule and Score Indicator tier colors per cell, with "Unavailable" text (not a blank cell or `NaN`) for a degraded or missing factor.

### Inputs & Address Search
- **Style:** `bg-zinc-900 border border-zinc-800 rounded-lg px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-500 font-sans`.
- **Focus:** 1px solid `zinc-500`, zero outer ring glow.

## Do's and Don'ts

### Do:
- **Do** use `Geist Mono` for all numeric scores, distances, coordinates, and travel durations.
- **Do** use exact CTA transit line colors for route badges, transit chips, and map layers.
- **Do** maintain tight border radii (`rounded-lg` 6px for cards/buttons/inputs, `rounded-sm` 2px for badges/chips).
- **Do** lead component copy with metrics, entity names, and straight-line distances (e.g. `0.2 mi · CTA Blue Line`).
- **Do** maintain 1px `zinc-800` borders between tonal surfaces for crisp visual delineation.

### Don't:
- **Don't** use drop shadows, glow filters, or floating elevated surfaces.
- **Don't** use emojis in metric cards, table headers, scorecards, or empty states.
- **Don't** use purple or violet radial background glows or AI novelty tropes.
- **Don't** write redundant explanatory fluff subtitles under score headers.
- **Don't** approximate CTA transit line colors with generic Tailwind accents.
