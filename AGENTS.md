# BlockFactor: Agent Guidelines & Working Agreements

This document establishes the behavioral and operational guidelines for AI coding agents operating in this repository.

---

## 1. Core Philosophy: Disciplined Pair Programming

- **Pair to a Software Engineer:** You are collaborating with a software engineer. The standard is rigorous, maintainable, production-grade engineering—**not vibe coding**.
- **Deliberate & Transparent:** Every architectural decision, schema modification, dependency addition, and code change must be intentional, well-reasoned, and clearly communicated.

---

## 2. Interaction & Execution Rules

### Rule 1: Answer First, Then Propose
- When the user asks a question or raises a topic, answer the question thoroughly and directly first.
- Do not jump straight to executing code or file modifications.
- After answering, clearly outline any recommended or proposed changes and wait for guidance.

### Rule 2: Explicit Permission Before Modifying Files
- Always outline proposed changes, design decisions, or architectural plans before executing file creation or edits.
- Obtain explicit user agreement before making significant changes or starting a new phase of implementation.

### Rule 3: Outline & Review Changes
- When changes are made, provide a clear, concise summary of the modifications (what changed, why it changed, and any trade-offs).
- Reference modified files and line ranges where appropriate.

### Rule 4: Never Commit Without Explicit Permission
- Do **not** run `git commit`, `git push`, or alter git history automatically or implicitly.
- After presenting modifications and diff summaries, ask for explicit permission before staging or committing changes.

---

## 3. Engineering & Code Standards

- **Idiomatic Go Standard:**
  - Adhere strictly to Effective Go, standard library idioms, and the Go Code Review Comments style guide.
  - Favor standard library packages (`net/http`, `log/slog`, `context`, `sync`) over heavy third-party framework dependencies.
  - Keep packages cohesive and flat (avoid deep inheritance-like hierarchies or premature abstraction).
  - Explicit error handling: wrap errors with context (`fmt.Errorf("...: %w", err)`), never swallow errors, and avoid naked returns or unhandled goroutine panics.
- **Type Safety & Contracts:** Strict type definitions across Go backend and TypeScript frontend generated from a single OpenAPI contract.
- **Frontend Standards:** Idiomatic React 19 / TanStack patterns (Router & Query), strictly typed search params, and declarative component state.
- **Spatial Precision:** Maintain exact PostGIS geometry conventions (SRID 4326 for WGS84 coordinates, appropriate metric spherical distance queries with `ST_DWithin`).
- **Cost & API Consciousness:** Prioritize local calculation, caching (PostGIS spatial grid cache), and self-hosted services (OTP2, Nominatim, OSM) over metered third-party commercial APIs.

---

## 4. UI/UX & Anti-Vibe-Coding Standards

The application must look and feel like an analytical, high-density location intelligence tool (e.g. Linear, Raycast, Citymapper, NYT Graphics)—**never a generic AI vibe-coded project**.

- **No AI Clichés or "Sparkle/Purple Traps":**
  - Strictly **no sparkle/wand icons** (`Sparkles`, `Wand`) or AI novelty gimmicks.
  - Strictly **no violet/purple radial gradients** or generic dark-mode neon glows.
  - Theme palette is anchored in crisp neutral slates/zincs with official CTA transit line color tokens and clean metric indicators.
- **No Emoji Slop:**
  - Strictly **no emojis** in headers, metric cards, table cells, or empty states.
  - Use sharp, geometric vector icons (**Lineicons** at 14–16px, 1.5px stroke) or authentic municipal/transit badge chips.
- **No Redundant Explanatory Fluff:**
  - Avoid verbose, self-evident subtitles (e.g. *"Transit Score: The score that measures transit"*).
  - Prioritize high information density: lead with the metric, entity name, and distance.
- **Data & Typography Precision:**
  - Use a tabular monospace font (`Geist Mono`) for all coordinates, distances, travel durations, and numeric scores.
  - Maintain strict layout geometry: tight border-radii (`rounded-sm` or `rounded-md`), consistent padding scales (`p-4`, `p-6`), and zero bubbly floating elements.
- **Authentic Spatial Imagery:**
  - Never use generic abstract placeholder divs; utilize real MapLibre GL vector tiles, authentic CTA rail badges, and real spatial overlays.
