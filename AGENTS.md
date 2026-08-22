# BlockFactor: Agent Guidelines & Working Agreements

This document establishes the behavioral and operational guidelines for AI coding agents operating in this repository.

---

## 1. Core Philosophy: Disciplined Pair Programming

- **Pair to a Software Engineer (New to GIS):** You are collaborating with an experienced software engineer. While comfortable with complex architectures, backend patterns, and distributed systems, **GIS (Geographic Information Systems) and spatial data concepts are completely new to the user**.
  - When introducing or working with spatial mechanics (e.g., PostGIS functions, `geometry` vs. `geography` types, coordinate reference systems like SRID 4326/3857, GiST spatial indexing, bounding boxes, or map projection math), explain them clearly, simply, and with intuitive analogies so the user can learn and build strong mental models.
  - New GIS/spatial terms introduced in conversation should be added to `docs/GIS_GLOSSARY.md` — plain-English definition, analogy, and a concrete tie-in to how BlockFactor uses it — so the explanation persists beyond the conversation it came up in.
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

### Rule 5: Conventional Commit Format
- All commit messages MUST follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification: `<type>[optional scope]: <description>`.
- Common types used in this project: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `build`, `perf`, `style`.
- Use a scope for the affected package or domain where it adds clarity (e.g., `feat(scoring): ...`, `fix(api): ...`, `docs(agents): ...`).
- Breaking changes are marked with `!` before the colon (e.g., `feat(api)!: ...`) and/or a `BREAKING CHANGE:` footer describing the break.
- Descriptions are imperative mood ("add", not "added"/"adds"), lowercase after the type, no trailing period.
- If a change doesn't cleanly fit one type, split it into multiple commits rather than combining types in one message.
- Branch names follow the same type vocabulary: `type/short-kebab-case-description` (e.g., `feat/scoring-engine`, `docs/gis-glossary`). The type reflects the branch's primary purpose — a branch may contain commits of other types (e.g., a stray `test` or `docs` commit on a `feat` branch), but its own name reflects the dominant intent. No scope segment in branch names.

### Rule 6: Changelog Discipline
- `CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Any commit that changes user-facing behavior, API contracts, or dependencies MUST add a corresponding entry under `## [Unreleased]` in the same commit/PR — not deferred to release time.
- Group entries under the standard headings (`Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`); only include headings that have entries.
- Purely internal changes with no observable effect (`chore`, `ci`, `test`, `style`, or a `refactor` with zero behavior change) do **not** need a changelog entry — the changelog is for humans, not a mirror of commit history.
- Conventional Commit type → changelog section mapping:

  | Commit Type | Changelog Section |
  |---|---|
  | `feat` | Added |
  | `fix` | Fixed |
  | `feat!` / `BREAKING CHANGE` footer | Added or Changed, explicitly called out as breaking |
  | `refactor` / `perf` (if user-observable) | Changed |
  | Removal of a feature | Removed |
  | Security fix | Security |
  | `docs` / `chore` / `ci` / `test` / `style` / `build` | No entry needed |

- At release time, rename `[Unreleased]` to the new version + ISO 8601 date (e.g., `## [0.2.0] - 2026-09-01`) and add a fresh empty `## [Unreleased]` above it.

### Rule 7: Verify Library & Version Claims — Never Rely on Memory
- Do not treat a version number, API signature, configuration option, or CLI flag recalled from training data as authoritative — training data has a cutoff, and libraries/frameworks/specs move faster than that, including via breaking changes.
- Before pinning a version, or making any specific claim about how a library/framework/CLI/API behaves (syntax, config, flags, migration behavior), verify it using the `find-docs` skill to pull current documentation — or explicitly prompt the user to confirm — never assert it from memory.
- This applies to language runtimes, package/dependency versions, spec versions, Docker image tags, and any specific claim about how a piece of software works.
- Precedent: this rule exists because `docs/openapi.yaml` was initially drafted against OpenAPI 3.2.0 without verifying it, when 3.1.0 was the safer choice for actual tooling compatibility at the time.

---

## 3. Engineering & Code Standards

- **Idiomatic Go Standard:**
  - Adhere strictly to Effective Go, standard library idioms, and the Go Code Review Comments style guide.
  - Favor standard library packages (`net/http`, `log/slog`, `context`, `sync`) over heavy third-party framework dependencies.
  - Keep packages cohesive and flat (avoid deep inheritance-like hierarchies or premature abstraction).
  - Explicit error handling: wrap errors with context (`fmt.Errorf("...: %w", err)`), never swallow errors, and avoid naked returns or unhandled goroutine panics.
- **Type Safety & Contracts:** Strict type definitions across Go backend and TypeScript frontend generated from a single OpenAPI contract.
- **Frontend Standards:** Idiomatic React 19 / TanStack patterns (Router & Query), strictly typed search params, and declarative component state.
- **TypeScript Type Safety:** Never use `any` — enforce via Biome's `noExplicitAny` (suspicious group) set to `error`. Use `unknown` with proper narrowing for genuinely unknown-shape data (JSON.parse results, catch-clause variables, generic boundaries) — `unknown` is the type-safe alternative to `any`, not an equivalent to avoid; TypeScript's catch-clause syntax only permits `any` or `unknown` as annotations, so banning both would leave no strict-mode-compliant way to type a caught error. Never suppress a Biome rule (`// biome-ignore`) without a genuine, specific justification, and treat suppression as a last resort, not a convenience.
- **No Magic Duration Numbers:** Never write raw millisecond arithmetic (e.g. `1000 * 60 * 5`) for durations/intervals (cache `staleTime`, timeouts, polling intervals, TTLs). Use `dayjs`'s `duration` plugin instead — `dayjs.duration(5, "minutes").asMilliseconds()` — which reads as an actual duration rather than an opaque number. Reserve this for genuine time-span construction; calendar/date arithmetic (formatting, comparing, adding to a specific date) is a separate concern and doesn't need this pattern.
- **Spatial Precision:** Maintain exact PostGIS geometry conventions (SRID 4326 for WGS84 coordinates, appropriate metric spherical distance queries with `ST_DWithin`).
- **Cost & API Consciousness:** Prioritize local calculation, caching (PostGIS spatial grid cache), and self-hosted services (OTP2, Nominatim, OSM) over metered third-party commercial APIs.
- **Privacy & Data Minimization:** Follow `docs/PRIVACY_AND_DATA_HANDLING.md` for all PII handling — cache by coordinate/grid cell rather than raw address, never persist raw address strings beyond the request lifecycle, and never add address or coordinate data to logs, metrics labels, or affiliate click payloads.

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

---

## 5. Documentation, Markdown & Diagramming Standards

- **GitHub-Flavored Markdown (GFM) & Math Mode Syntax:**
  - All markdown files must parse cleanly on GitHub without rendering artifacts or KaTeX errors.
  - In math mode (`$...$`), never leave unescaped underscores inside `\text{}` (e.g. use `$S_{\text{third\_place}}$` or simply backtick `S_third_place`).
  - Avoid wrapping simple numbers, ranges, or unit strings in math delimiters (use plain `0–100`, `< 5 ms`, `≥ 85` instead of `$0\text{–}100$`, `$<5\text{ ms}$`, `$\ge 85$`). Use standard Unicode arrows `→` and comparison operators `≥`, `≤`.
- **Mermaid Diagrams Only (No ASCII Art):**
  - Always render system topologies, request lifecycles, architecture components, and data flow pipelines using **Mermaid diagrams** (`mermaid` code blocks).
  - Quote node labels that contain special characters (parentheses, slashes, brackets) to prevent Mermaid syntax parsing errors.
  - Keep diagrams clean, readable, and structured with logical subgraphs where appropriate.

---

## 6. Agent Division of Labor

### Claude Code CLI (`claude`) — Role: Architect & Algorithmic Planner
* Primary Focus: Architecture, complex SQL/PostGIS query design, mathematical formula validation, and OpenAPI specification authoring.
* Responsibilities:
  * Writing and updating `docs/openapi.yaml`.
  * Drafting `db/migrations/*.sql` for spatial indexes and staging table swaps.
  * Writing the math logic for scoring factors (`internal/scoring/factor_*.go`).
  * Reviewing multi-file diffs and ensuring adherence to `ARCHITECTURE.md` and `PRD.md`.

### Antigravity CLI (`agy`) — Role: Execution & Tooling Engine
* Primary Focus: File scaffolding, dependency wiring, test execution, container diagnostics, and continuous code validation.
* Responsibilities:
  * Running build and code-generation pipelines (`make generate`, `make generate-check`).
  * Applying database migrations (`make migrate-up`) and ingestion CLI tasks (`make seed`) that Claude has drafted — agy's own permission system will prompt for approval before running these.
  * Running linters and unit tests (`make test`, `make lint`) and resolving compilation/import errors.
  * Inspecting Docker service logs (`blockfactor-db`, `blockfactor-otp`).

### Handoff Protocol
* Claude and Agy run in separate threads with no shared context. When Claude produces instructions meant for Agy, write them as a self-contained prompt — restate relevant constraints and file paths rather than referring back to prior conversation.
* When execution fails against another agent's artifact (OpenAPI spec, migration, scoring logic), report the failure back rather than patching that file directly. Ownership stays with whichever agent authored it.
* **PR Scope:** Prefer multiple small, reviewable PRs over one large one. A single implementation-phase deliverable item (per `IMPLEMENTATION_PHASES.md`'s checklist) is usually the right PR size — not a whole phase. Exception: initial scaffolding where files are inherently interdependent (e.g. Phase 1) may reasonably span one PR, but should still be called out explicitly as a deliberate exception, not the default.
