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
- **Testing Discipline (TDD where appropriate):** Favor writing the test before or alongside the implementation for logic with a clear, checkable contract — pure functions, parsers, classifiers/mapping tables, retry/backoff behavior, and similar. Bias toward maximizing what's covered by fast, deterministic unit tests rather than relying on manual or live-data verification for anything that can be isolated and tested in-process — a data taxonomy or retry policy is exactly the kind of logic a live spot-check won't keep catching regressions on later, a unit test will. Not every change warrants strict test-first: exploratory spikes, thin wiring/glue code, and genuinely DB-dependent behavior are reasonable exceptions — the latter is a separate, larger investment gated on `ARCHITECTURE.md` §13.2's still-open integration-test-harness decision (`testcontainers-go` vs. compose-managed test DB), so pure logic shouldn't wait on that to get covered.
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
  - Maintain strict layout geometry: a consistent `rounded-lg` corner radius across cards and interactive controls (small chips/badges may use `rounded-sm`), consistent padding scales (`p-4`, `p-6`), and zero bubbly floating elements.
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

All agent work is done by Claude Code sessions, coordinated through Orca workspaces. One session pairs with the user as the **coordinator**; it launches **workers** for well-scoped, one-PR-sized tasks.

### Coordinator — Role: Architect, Planner & Reviewer
* Primary Focus: Architecture, complex SQL/PostGIS query design, mathematical formula validation, and OpenAPI specification authoring.
* Responsibilities:
  * Writing and updating `docs/openapi.yaml`.
  * Drafting `db/migrations/*.sql` for spatial indexes and staging table swaps.
  * Writing the math logic for scoring factors (`internal/scoring/factor_*.go`).
  * Splitting approved work into one-PR-sized tasks and writing each worker's brief.
  * Reviewing every worker's diff against `ARCHITECTURE.md` and `PRD.md` before the user commits or merges.

### Workers — Role: Execution & Tooling
* Launched by the coordinator into their own Orca workspace (own branch, own checkout):
  `orca worktree create --name <task> --no-parent --agent claude --model <model-id> --effort <level> --prompt "<brief>" --json`
* Primary Focus: Implementing a specified task, file scaffolding, dependency wiring, test execution, and container diagnostics.
* Responsibilities:
  * Running build and code-generation pipelines (`make generate`, `make generate-check`).
  * Applying database migrations (`make migrate-up`) and ingestion CLI tasks (`make seed`).
  * Running linters and unit tests (`make test`, `make lint`) and resolving compilation/import errors.
  * Inspecting Docker service logs (`blockfactor-db`, `blockfactor-otp`).
* Workers follow every rule in this file, including Rule 4: no commit, push, or PR without the user's explicit approval.

### Handoff Protocol
* Workers start with no shared context. A worker brief is a self-contained prompt: restate the relevant constraints, file paths, the branch name (`type/short-kebab-description`), and the no-commit rule rather than referring back to prior conversation.
* When execution fails against an artifact the worker didn't author (OpenAPI spec, migration, scoring logic), the worker reports the failure back to the coordinator rather than patching that file directly.
* **PR Scope:** Prefer multiple small, reviewable PRs over one large one. A single implementation-phase deliverable item (per `IMPLEMENTATION_PHASES.md`'s checklist) is usually the right PR size — not a whole phase. Exception: initial scaffolding where files are inherently interdependent (e.g. Phase 1) may reasonably span one PR, but should still be called out explicitly as a deliberate exception, not the default.

### Choosing a Model
Pick the cheapest model that can do the task well, and set effort to match. Adjust effort before switching model tiers; if a worker fails the same task twice, relaunch it one tier up rather than retrying as-is.

| Model | ID | Use for | Price (input / output per 1M tokens) |
|---|---|---|---|
| Claude Haiku 5.5 | `claude-haiku-5-5` | Mechanical, fully specified work: renames, link/typo fixes, read-only code searches, log triage. | $0.10 / $0.50 |
| Claude Sonnet 5.5 | `claude-sonnet-5-5` | **Default worker.** Implementing a specified one-PR task: Go handlers and clients, React components, tests, wiring, Docker/Makefile changes. | $2 / $10 |
| Claude Opus 5.5 | `claude-opus-5-5` | **Coordinator.** PostGIS/SQL design, scoring math, OpenAPI and migration design, multi-file review, hard debugging. Workers only when the task needs design judgment. | $4 / $20 |
| Claude Fable 5.1 | `claude-fable-5-1` | Rare: long-horizon or unusually hard problems where Opus has already fallen short. | $10 / $50 |

* **Effort:** `low` for mechanical tasks, `medium` for typical worker tasks, `high`/`xhigh` for design, review, and debugging. Use `max` only when correctness clearly outweighs cost.
* Prices are Anthropic API list prices as of 2026-10; models and prices change, so verify current ones per Rule 7 before relying on this table.

### Workspace Isolation
* **Why:** Agents run real `git`/`gh` commands. Two agents sharing one checkout means one agent's `git checkout` or `gh pr checkout` silently switches the branch out from under the other mid-task — confirmed live during PR #39's review.
* **The fix — separate working directories, one shared `.git`:** Orca workspaces are `git worktree`s: independent working directories attached to the same repository (shared object store and refs — a commit made in one is immediately visible in the others), each with its own branch checkout.
* **Convention:**
  * `~/orca/block-factor-monorepo/` (the primary clone) stays on `main` at all times — kept up to date via `git pull`, and not used for feature-branch work or PR reviews.
  * Every task, coordinator or worker, gets its own Orca workspace under `~/orca/workspaces/block-factor-monorepo/<name>`, created with `orca worktree create` and removed with `orca worktree rm` once its branch is merged.
  * `git worktree list` shows all registered worktrees and what each has checked out.
* **The constraint that makes this safe, not just tidy:** git refuses to check out the same branch in two worktrees at once, so an agent trying to check out a branch another agent is actively working on fails loudly instead of clobbering anything.
* **Per-workspace setup, since only tracked files are shared:** Orca's repo setup hook runs `bun install` in each new workspace; gitignored local config (`.env`) is not shared and must be copied in manually before anything that needs it will run.

### PostGIS Queries
* For ad-hoc queries that call PostGIS functions (`ST_Area`, `ST_DWithin`, ...), use `docker exec blockfactor-db psql -U blockfactor -d blockfactor -c "..."`.

---

## Agent skills

### Issue tracker

Issues live in this repo's GitHub Issues, managed with the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: root `GLOSSARY.md` + `docs/adr/`, plus `docs/GIS_GLOSSARY.md` for GIS terms. See `docs/agents/domain.md`.
