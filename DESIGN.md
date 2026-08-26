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

The UI uses an asymmetrical split-pane layout designed for concurrent visual and spatial analysis:
- **Left Sidebar (Desktop `col-span-4` / `min-w-[380px]`):** Scrollable analytical column housing the address input, composite score card, individual 10-factor breakdowns, and commute duration pills.
- **Right Map Canvas (Desktop `col-span-8` / sticky `h-[calc(100vh-7rem)]`):** Fixed-viewport MapLibre GL vector canvas displaying the candidate address, CTA rail network, high-frequency bus corridors, bike routes, and spatial heatmaps.
- **Spacing Scale:** Built on strict 4px/8px modular units (`p-4` / 16px cards, `gap-6` / 24px grid gutters, `py-0.5 px-2` badge chips).

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
  - `sm` (`2px` / `0.125rem`): Cards, badges, chips, buttons, inputs.
  - `md` (`4px` / `0.25rem`): Modal containers, dropdown menus.
  - `lg` (`6px` / `0.375rem`): Outer application wrappers.
- **Borders:** 1px solid `zinc-800` on all cards and dividers; 1px colored strokes on score badges.

## Components

### Cards
- **Corner Style:** `rounded-sm` (2px).
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
- **Shape:** `rounded-sm` (2px).
- **Primary:** `bg-zinc-100 text-zinc-950 hover:bg-zinc-200` font-mono font-medium text-xs uppercase.
- **Secondary / Ghost:** `bg-zinc-800 text-zinc-100 hover:bg-zinc-700` with 1px `zinc-700` border.

### Inputs & Address Search
- **Style:** `bg-zinc-900 border border-zinc-800 rounded-sm px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-500 font-sans`.
- **Focus:** 1px solid `zinc-500`, zero outer ring glow.

## Do's and Don'ts

### Do:
- **Do** use `Geist Mono` for all numeric scores, distances, coordinates, and travel durations.
- **Do** use exact CTA transit line colors for route badges, transit chips, and map layers.
- **Do** maintain tight border radii (`rounded-sm` 2px for cards and badges).
- **Do** lead component copy with metrics, entity names, and straight-line distances (e.g. `0.2 mi · CTA Blue Line`).
- **Do** maintain 1px `zinc-800` borders between tonal surfaces for crisp visual delineation.

### Don't:
- **Don't** use drop shadows, glow filters, or floating elevated surfaces.
- **Don't** use emojis in metric cards, table headers, scorecards, or empty states.
- **Don't** use purple or violet radial background glows or AI novelty tropes.
- **Don't** write redundant explanatory fluff subtitles under score headers.
- **Don't** approximate CTA transit line colors with generic Tailwind accents.
