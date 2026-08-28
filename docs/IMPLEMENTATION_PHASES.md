# BlockFactor: Implementation Phases & Roadmap

**Status:** Draft / Active Planning  
**See also:**
* [PRD.md](./PRD.md) — Product Requirements & Scoring Logic
* [ARCHITECTURE.md](./ARCHITECTURE.md) — Technical Architecture & Decision Log
* [AGENTS.md](../AGENTS.md) — Engineering Standards & Pair Programming Rules

---

## Overview

This document outlines the phased engineering roadmap for BlockFactor, spanning local development on the home server (Phases 1–7) through the public cloud launch (Phase 8) — see ARCHITECTURE.md §1 for the underlying home-server-vs-cloud deployment target each end of that range implements. Each phase is scoped with clear deliverables, architecture alignment, and acceptance criteria.

---

## Phase 1: Monorepo Foundation, Design System & Contracts

### Objectives
Establish the monorepo foundation, Docker development environment, shared OpenAPI contract, and front-end design system primitives.

### Deliverables
1. **Repository Tooling:**
   - [x] Root `Makefile` with universal targets (`dev`, `test`, `lint`, `migrate-up`, `seed`, `generate`, `generate-check`).
   - [x] Root `docker-compose.yml` with PostgreSQL 18 + PostGIS and Docker Compose Profiles (`profiles: ["transit"]` for OTP2). Named volumes for local dev; the home server binds the NVMe SSD via `docker-compose.override.yml`.
   - [x] Toolchain pins recorded during scaffolding: `go.mod` `toolchain` directive, Bun version pinned via `engines.bun` (package.json) + explicit `bun-version` in the `oven-sh/setup-bun` CI step (Corepack does not manage Bun), exact Docker image tags (PostGIS, OTP2), pinned `golangci-lint`.
   - [x] GitHub Actions CI (GitHub-hosted runners) running `make lint`, `make test`, and `make generate-check` on every PR, with branch protection on `main`.
2. **Go API Backend Skeleton (`services/api/`):**
   - [x] Standard library `net/http` server with Go 1.22+ routing patterns.
   - [x] `log/slog` JSON logging configured for Promtail/Loki.
   - [x] Prometheus `/metrics` endpoint.
   - [x] Domain-layered package structure (`internal/api`, `internal/scoring`, `internal/spatial`, `internal/clients`, `internal/cache`).
3. **OpenAPI 3.1 Contract (`docs/openapi.yaml`):**
   - [x] Strict REST request/response schemas for `POST /api/v1/score` (accepts a raw address or pre-resolved coordinates), `GET /api/v1/geocode`, `GET /api/v1/isochrones`, and `GET /api/v1/health`.
   - [x] Score response contract: factor sub-scores as objects (`score`, `degraded` flag, distances / contributing POIs) plus benchmark commute pills; isochrones are excluded and served only by `GET /api/v1/isochrones` (faster first paint, independent caching). Out-of-coverage requests return 422 `OUT_OF_COVERAGE`.
   - [x] Code generation scripts: Go server types via `oapi-codegen`, TypeScript client types via `openapi-typescript`.
4. **React 19 + Vite Frontend Skeleton (`packages/web/`):**
   - [x] Tailwind CSS v4 configured with sharp, disciplined design tokens (Slate/Zinc, CTA rail colors, zero violet glow).
   - [x] `shadcn/ui` (Radix UI) component primitives.
   - [x] **Lineicons** icon library integration.
   - [x] TanStack Router with type-safe search param validation schema.
   - [x] TanStack Query client setup.
   - [x] Vite dev proxy (`/api` → `http://localhost:4001`) — no CORS in development.

### Validation Criteria
- `make test` and `make lint` pass across both Go and TypeScript workspaces.
- `make generate` regenerates contract types without manual intervention.

---

## Phase 2: PostGIS Schema & Open Data Ingestion

### Objectives
Create database tables with spatial indexes and build the standalone CLI tool to seed Chicago municipal datasets.

### Deliverables
1. **`goose` Database Migrations (`db/migrations/`):**
   - [x] Spatial tables with `GIST` indexes on `geography(Point, 4326)`:
     - `cta_stops` ('L' stations).
     - `bus_corridors` (High-frequency bus segments derived from GTFS `stop_times` AM-peak headway analysis, powering the `S_transit` bus bonus).
     - `bike_routes` (Street-based bike infrastructure — protected lanes, buffered lanes, greenways, marked shared lanes — powering `S_bike`; off-street trails are a separate `parks` classification, not part of this table — see `docs/DATA_SOURCES.md`).
     - `parks` (City parks, forest preserves, community gardens; `park_class = 'LINEAR PARK'` is the real source for the `S_green` off-street trail bonus — see `docs/DATA_SOURCES.md`).
     - `osm_pois` (Supermarkets, convenience stores, and pharmacies from the Illinois OSM extract, powering `S_grocery`).
     - `cpd_crimes` (Trailing 12-month crime records).
     - `safety_grid_percentiles` (~500m citywide penalty distribution powering `S_safety` normalization).
     - `fema_flood_zones` (NFHL hazard polygons) and `sewer_311_incidents` (24-month water/sewer backup points), powering `S_flood`.
     - `noise_sources` (Grade-tagged CTA track segments + expressway/arterial linestrings, powering `S_noise`).
     - `city_boundary` (City of Chicago boundary polygon powering the `OUT_OF_COVERAGE` hard-reject).
     - `spatial_grid_cache` (400m bounding cells for Google Places POIs).
     - `cache_entries` (standard LOGGED Postgres cache table with indexed `expires_at`).
2. **Data Ingestion CLI Tool (`services/api/cmd/seed/main.go`):**
   - [x] SODA API client connecting to the Chicago Data Portal (authenticated via `CHICAGO_APP_TOKEN`).
   - [x] Automated parsers for CTA 'L' stops, bike routes, parks, crime incidents, FEMA flood zones, 311 sewer backup incidents, OSM POIs, noise geometry, and the city boundary polygon.
   - [x] Post-ingest derivation steps: Crime-grid distribution → `safety_grid_percentiles` (staging-table atomic swap per ARCHITECTURE 11.3, completed); GTFS `stop_times` headway analysis → `bus_corridors` (completed).
   - [x] Ingestion tooling per dataset (GDAL `ogr2ogr` vs. Go shapefile lib, PBF parsing, GTFS parsing) resolved at phase start per ARCHITECTURE 13.2.
   - [x] Selective CLI flags: `--dataset=boundary`, `--dataset=cta`, `--dataset=bike_routes`, `--dataset=parks`, `--dataset=crimes`, `--dataset=osm_pois`, `--dataset=noise`, `--dataset=flood_zones`, `--dataset=sewer_311`, `--dataset=safety_grid`, `--dataset=bus_corridors`, `--dataset=derive`, `--limit=1000`, `--all`.

### Validation Criteria
- `make migrate-up` creates all tables and spatial indexes cleanly.
- `make seed` successfully populates PostGIS with real Chicago open data (SODA datasets in < 60 seconds; the trailing-12-month crimes dataset — ~225K rows, ~80 seconds in practice — the one-time OSM PBF import, and derivation steps are exempt from this budget).
- Spatial radius queries (`ST_DWithin`) execute in < 5 ms.

---

## Phase 3: Go Scoring Engine & Core API

### Objectives
Implement the deterministic and spatial scoring algorithms with full unit test coverage and wire them to HTTP endpoints.

### Deliverables
1. **Factor Calculators (`services/api/internal/scoring/`):**
   - [x] `factor_transit.go` (`S_transit`: 'L' station distance decay + bus bonus).
   - [x] `factor_commute.go` (`S_commute`: Rush-hour OTP2 point-to-point transit duration + transfer friction).
   - [x] `factor_bike.go` (`S_bike`: quality-tiered proximity decay + N-S/E-W directional network bonus).
   - [x] `factor_grocery.go` (`S_grocery`: Supermarket distance + pharmacy boost).
   - [x] `factor_third_places.go` (`S_third_place`: Cached POI density).
   - [x] `factor_green.go` (`S_green`: Park proximity + Lakefront/606 trail bonus).
   - [x] `factor_noise.go` (`S_noise`: Elevated CTA track & arterial road proximity penalties).
   - [x] `factor_flood.go` (`S_flood`: FEMA zones + 311 sewer backup penalty).
   - [x] `factor_safety.go` (`S_safety`: 12-month violent/property penalty normalized to city percentiles).
     - [ ] **Amendment (post-Phase 3, not yet scheduled):** normalize each grid cell's penalty score by ambient population (residents + workers + a small, hand-maintained visitor-load list for known high-foot-traffic zones, since no free dataset captures tourist/visitor counts at this resolution) before percentile-ranking, instead of raw incident counts alone. Raw density currently over-penalizes dense, high-traffic commercial/entertainment corridors relative to their actual per-resident risk — confirmed live: a single 500m Loop cell logged 365 violent incidents (mostly `BATTERY`/`ASSAULT`, consistent with nightlife/foot-traffic density) against a Lincoln Square cell's 7, landing the Loop at the 100th percentile despite the difference being driven by non-resident foot traffic, not necessarily resident-facing danger. New population data would come from the 2020 Decennial Census (PL 94-171, block-level, requires a free API key) + TIGER/Line 2020 block boundaries (no key) for residents, and LEHD LODES workplace-employment-by-block for workers — Chicago's own open data portal only has 2010-vintage block population, too stale for the post-2010 downtown residential boom this amendment specifically targets. Grid-cell population apportioned from irregular Census blocks via area-weighted interpolation (`ST_Intersection` area fraction × block population, summed per 500m cell). Visitor-load numbers for named tourist zones sourced from real CTA station entry counts where a nearby 'L' station exists (`CTA - Ridership - 'L' Station Entries`, already on Chicago's portal), or cited published figures (e.g. a venue's own reported annual visitor count) elsewhere — never invented without a source. Requires a PRD §5.9 amendment first, per the same pattern as the `S_transit` bonus redesign (PRD update → implementation PR(s)). Full methodology, data sourcing, and citations to live in a new `docs/SAFETY_METHODOLOGY.md` (not yet created) — also intended to back an on-site explanation of how `S_safety` works, not just an internal design doc.
   - [x] `factor_solar.go` (`S_solar`: orientation base lookup × exposure modifier + floor bonus).

2. **Outbound Clients & Cost Defense (`services/api/internal/clients/`):**
   - [x] `places.go` — Google Places (New) Nearby Search client with 400m grid-cache fill (30-day TTL in `cache_entries`), powering `S_third_place` and the Phase 4 POI map layer.
     - Protected by a 4-tier cost defense layer: token-bucket rate limiter (`golang.org/x/time/rate`), daily budget circuit breaker (degrading `S_third_place` to `null`), in-flight `singleflight` deduplication (`golang.org/x/sync/singleflight`), and Google Cloud hard billing quotas.
     - ~~**TODO (prerequisite):** provision the Google Cloud project + Places API (New) key — restricted, hard daily quota cap, billing alert — before this client lands.~~ Done (key provisioned; IP restriction pending — see `docs/PROVISIONING.md`).
   - [x] `nominatim.go` — one-shot geocoding client returning ranked candidates (public endpoint, ≤ 1 req/sec rate-limited, custom `User-Agent`; no typeahead in Phase 1 per Nominatim policy).
   - [x] `otp.go` — **interface definition only**: `factor_commute.go` is built and unit-tested against fixture-based fakes shaped as **GTFS GraphQL API** responses (OTP 2.x removed the legacy REST plan API); the live client lands in Phase 6.
3. **Concurrency Orchestrator (`engine.go`):**
   - [x] `errgroup` / goroutine fan-out with a hard 8-second parent context deadline.
   - [x] Dynamic weight normalization ($\sum w_i = 100\%$).
   - [x] Graceful degradation: timed-out or data-less factors return `null`, are excluded from the weight sum, and remaining weights renormalize; the response marks degraded factors.

4. **HTTP Handlers (`services/api/internal/api/`):**
   - [x] `POST /api/v1/score`
   - [x] `GET /api/v1/geocode` (one-shot geocode returning coordinates; typeahead autocomplete deferred until self-hosted Photon per Nominatim policy)
   - [x] `GET /api/v1/health`


### Validation Criteria
- 100% unit test coverage on scoring mathematical formulas and edge cases (boundary distances, zero weights, all-null factors).
- Golden barrier-adjacent test suite executed against known edge properties (Goose Island, Pilsen along the river/I-55, West of Kennedy trench, East Side rail yards) to verify scores maintain acceptable bounds under physical barriers.
- Live API response latency < 150 ms for warm cache hits (excluding `S_commute`, which activates with OTP2 in Phase 6).

---

## Phase 4: Frontend WebGL Map & Vector Visualization

### Objectives
Build the GPU-accelerated interactive map workspace with custom Chicago vector styles and spatial layer overlays.

### Deliverables

Originally scoped as 5 items (map component + 4 interactive layers + solar widget). Expanded mid-phase into 10 deliverable-sized PRs after discovering no backend endpoint served bulk map geometry at all — `ScoreResponse` only ever returns distance-to-nearest-feature, never raw geometry (see ARCHITECTURE.md Decision Log rows 29–31 for the three resulting architecture decisions, and DATA_SOURCES.md "CTA 'L' Lines" for a new verified dataset this uncovered). Sequenced below so every frontend layer PR lands only after the backend piece it depends on; each PR branches off `origin/main`.

1. [x] **Map foundation** (frontend only, no dependency) — MapLibre GL JS map component (`packages/web/src/map/`), high-contrast Chicago basemap (CartoCDN / OpenFreeMap), responsive split-screen desktop & mobile viewport handling.
2. [x] **CTA rail line ingestion** (backend/DB only) — New migration + `cmd/seed` ingestion of the verified Chicago Data Portal dataset `xbyr-jnvx` ("CTA - 'L' (Rail) Lines") into a new `cta_rail_lines` table. This is the only real per-line rail track geometry available — `noise_sources`'s existing CTA track geometry is line-agnostic. See DATA_SOURCES.md "CTA 'L' Lines" for the verified schema, real `lines`/`type` field values, and parsing quirks (inconsistent `Express`/`Exp` abbreviation).
3. [x] **Map layer read endpoints** (backend only) — New `openapi.yaml` paths + Go handlers, all read-only GeoJSON `FeatureCollection`s over existing/new bulk-ingested tables (no per-request cost or budget concerns):
   - `GET /api/v1/layers/cta-rail` (from deliverable 2's new `cta_rail_lines` table)
   - `GET /api/v1/layers/cta-stations` (from existing `cta_stops`)
   - `GET /api/v1/layers/bike-routes` (from existing `bike_routes`)
   - `GET /api/v1/layers/trail-access` (from `parks WHERE park_class = 'LINEAR PARK'` — per the existing PRD §5.5 schema decision, **not** `bike_routes`)
   - `GET /api/v1/layers/supermarkets` (from existing `osm_pois WHERE poi_type = 'supermarket'`)
4. [x] **CTA Rail Layer** (frontend, consumes 3) — Official line colors with interactive station hover cards.
5. [x] **Protected Bike Layer** (frontend, consumes 3) — High-visibility green linestring overlay with trail access markers.
6. [x] **Nearby-places endpoint** (backend only) — New `GET /api/v1/places?lat=&lon=`, scoped to a per-address radius, not city-wide (ARCHITECTURE.md Decision Log row 30). Blends the existing `S_third_place` Google Places grid-cache lookup (cafes/fitness) with an `osm_pois` supermarket query. Likely needs a small extraction from `factor_third_places.go` so the scoring path and this handler share one cache-fill/lookup implementation rather than duplicating it.
7. [x] **POI Density Markers** (frontend, consumes 6) — Cafes, fitness studios, and supermarkets with distance radius rings.
8. [x] **Divvy client + endpoint** (backend only) — `internal/clients/divvy.go` (GBFS client) + in-memory GBFS cache (5m TTL) + `GET /api/v1/layers/divvy-docks`.
9. [x] **Divvy Dock Layer** (frontend, consumes 8) — Station markers with live capacity badges (display-only — Divvy is not a scored factor).
10. [x] **Solar Exposure Badge** (frontend only, no backend dependency) — Originally scoped as a 3D compass orientation ray projected from the address marker; a `fill-extrusion` ray (first a flat color, then colored/sized by `S_solar` score tier) shipped but live review found it still wasn't clear at a glance — a novel 3D shape, a color-tier meaning, and a compass direction all at once, with no on-screen legend. Replaced with a plain sidebar `<Badge>` (e.g. "W-Facing · 75") showing the same `facing` direction `factor_solar.go` already infers/accepts and its real base score, colored by score tier — the analytical-truth surface (sidebar), not the spatial-context one (map), is where this needed to live.

### Validation Criteria
- 60fps GPU-accelerated panning and zooming on desktop and mobile browsers.
- Smooth layer toggle transitions with zero WebGL context leaks.
- Each layer PR verified live in a browser against real data before merge, not just type-checked or unit-tested.

---

## Phase 5: Scored Report UI & Weight Sliders

### Objectives
Assemble the high-density Split-Screen Analytics Workspace and reactive weight customization drawer.

Affiliate integration (contextual cards + click instrumentation) originally scoped here has moved to Phase 8, bundled with the cloud hosting migration it's meant to help fund — see Phase 8 below. Shipping both together means affiliate links go live at the same time as the public cloud deployment they help pay for, rather than sitting live earlier on the free home-server deployment.

### Deliverables
1. **Address Search & Geocoding Input:**
   - [x] Address input box wired to the existing `GET /api/v1/geocode` endpoint, with a candidate picker for ambiguous results, feeding `POST /api/v1/score` once an address resolves. The first real entry point for a user-supplied address anywhere in the app — every map layer/endpoint before this (Phase 4) works against a fixed demo coordinate or a bulk city-wide dataset.
2. **Left Scored Analytics Panel:**
   - [ ] Hero Composite Score Ring (0–100). (Neighborhood archetype badge deferred to v2.)
   - [ ] Collapsible Weight Customization Slider Drawer with instant reactive recalculation and URL search param sync.
   - [ ] 9–10 Dense Factor Breakdown Cards (the commute card renders only when a destination is set) with distance badges ("straight-line" qualifier) and sub-score progress bars.
3. **Bottom Multi-Address Comparison Tray:**
   - [ ] Sticky dock to pin and compare up to 3 addresses side-by-side.
4. **Error & Empty States:**
   - [ ] Address-not-found, ambiguous-address candidate picker, out-of-coverage (422 `OUT_OF_COVERAGE`), geocoding failure, OTP2-unavailable, and degraded-factor (`null`) states — styled per AGENTS.md (high-density copy, no emojis).
5. **Map Legend & Layer Visibility Controls:**
   - [ ] A collapsible panel anchored on the map itself (top-left, mirroring the zoom/compass control's top-right position) rather than the left analytics panel — map display is a map-chrome concern, not a score concern. Each Phase 4 map layer (CTA rail, protected bike/trail, POI density, Divvy docks) gets one entry: a color swatch + label doubling as the legend, plus a checkbox toggling that layer's visibility. Addresses the map getting visually overwhelming with every layer always on at once — Divvy's ~2,050 stations being the sharpest example — by defaulting the densest layers off rather than requiring a user to discover they can hide them.

### Validation Criteria
- Slider adjustments immediately update scores and URL query parameters without full page reloads.
- Responsive layout transitions smoothly between desktop split-screen and mobile stacked views.
- Strict adherence to anti-vibe-coding rules: no purple gradients, no emoji slop, Lineicons only.

---

## Phase 6: OpenTripPlanner 2 & Live Multimodal Commute Integration

### Objectives
Deploy OpenTripPlanner 2 in Docker, compile the Cook County transit graph, and stream multimodal commute times and isochrone polygons to the UI.

### Deliverables
1. **OTP2 Docker Service (`services/otp/`):**
   - [ ] Dockerfile and build script pulling `chicago-gtfs.zip` and `illinois.osm.pbf`.
   - [ ] Pre-compiled graph persistence (`graph.obj`) with `-Xmx5G` JVM cap.
2. **Go Backend OTP2 Client (`services/api/internal/clients/otp.go`):**
   - [ ] Implements the client interface stubbed in Phase 3, activating `S_commute` and the benchmark commute pills end-to-end.
   - [ ] Speaks the **GTFS GraphQL API**; isochrones via the travel-time API (no legacy REST plan calls).
   - [ ] Multimodal travel time calculations to Loop, O'Hare, Midway, and Fulton Market.
   - [ ] GeoJSON isochrone polygon endpoint proxy (15-min walk, 30-min transit).
3. **Frontend Isochrone Map Rendering:**
   - [ ] Semi-transparent reachable area polygons rendered directly on MapLibre.

### Validation Criteria
- OTP2 boots in < 15 seconds from pre-compiled `graph.obj`.
- Isochrone polygons render accurately over the Chicago street grid.

---

## Phase 7: Deployment, Observability & Home Server Integration

### Objectives
Deploy BlockFactor to the Beelink home server behind Caddy with automated monitoring and Tailscale ingress.

### Deliverables
1. **Caddy Reverse Proxy & Production Frontend Serving:**
   - [ ] Path-based routing with automatic Tailscale TLS: `/blockfactor/api/*` → Go API (`:4001`); all other `/blockfactor/*` paths serve the static `vite build` output (the Vite dev server is not a deployment target).
   - [ ] Sub-path hosting configured end-to-end: Vite `base`, TanStack Router `basepath`, and static asset URLs.
2. **Monitoring Dashboards:**
   - [ ] Grafana dashboard importing BlockFactor Prometheus metrics and Loki logs.
   - [ ] Loki alert rule on the seed CLI's "unrecognized crime classification" `WARN` log line (`cmd/seed/crimes.go`) — the ingestion CLI already logs and skips any `primary_type`/`iucr` combination outside the documented taxonomy (`docs/DATA_SOURCES.md` "Category coverage") rather than guessing, but nothing currently surfaces that beyond the log line itself. Deferred here rather than built in Phase 2 since it needs the Loki/Grafana stack this phase sets up.
3. **CI/CD Self-Hosted Runner:**
   - [ ] Migrates the Phase 1 GitHub-hosted workflows (`make lint` / `make test` / `make generate-check`) to the local home-server runner.
4. **Data Durability:**
   - [ ] Seeded datasets and caches are reproducible via `make seed` — no backup required in Phase 1. A nightly `pg_dump` is introduced only once user-generated state (saved reports / share links) exists in Phase 2.

---

## Phase 8: Production Release — Cloud Deployment & Affiliate Monetization

### Objectives
Migrate BlockFactor off the home server to public cloud hosting and activate affiliate monetization at the same time, so affiliate revenue can begin offsetting the new hosting cost from day one rather than affiliate links going live earlier on a free home-server deployment. This is the last phase deliberately — everything through Phase 7 targets the home server (personal use, dev/testing), and this is the actual "go public" milestone. Corresponds to ARCHITECTURE.md §1's already-defined "Phase 2 — Cloud Deployment (Post-Polish / Monetization)" deployment target, which this phase implements rather than re-designs.

### Deliverables
1. **Cloud Infrastructure Migration** (ARCHITECTURE.md §1 "Phase 2 — Cloud Deployment"):
   - [ ] Provision Hetzner CX/CAX compute + Cloudflare (CDN / DNS / Tunnels) per the already-defined target stack.
   - [ ] Public-launch hardening: rate limiting / abuse protection on `POST /api/v1/score` (protects both the Google Places daily budget and affiliate CPA integrity), CORS lockdown to the public origin, and basic bot filtering at the Cloudflare edge.
   - [ ] Retarget CI/CD: the same GitHub Actions pipelines, deployed to Hetzner via SSH or container registry push.
2. **Link Sharing & Social Previews** (ARCHITECTURE.md §7.5 "Phase 2 (Public Launch)"):
   - [ ] `GET /share/:report_id` Go route handler injecting dynamic OpenGraph meta cards (title, summary score, static preview map snapshot) for iMessage/Slack/Twitter link previews.
3. **Contextual Affiliate Cards (Intent-Gated)** (moved from Phase 5; PRD §6):
   - [ ] Broadband / Fiber card with available ISP speed badges (AT&T Fiber, Astound) — always-on utility feature. Delivers `internal/clients/fcc.go` + per-address cache; hides gracefully on API failure.
   - [ ] Lemonade instant renters quote modal trigger (intent-gated: unit metadata or 2+ pinned comparisons).
   - [ ] Dolly / Unpakt moving quote comparison card (intent-gated: destination address or unit metadata).
   - [ ] ClassPass fitness studio trial card (displayed alongside `S_third_place` gym counts).
4. **Affiliate Click Instrumentation** (moved from Phase 5):
   - [ ] Prometheus counter metric `blockfactor_affiliate_clicks_total{partner, placement, intent_level}` + subid/UTM passthrough to affiliate links for conversion & EPC tracking.

### Validation Criteria
- BlockFactor is reachable at its public domain over HTTPS, served entirely from Hetzner/Cloudflare — no remaining dependency on the home server for production traffic.
- Rate limiting and CORS lockdown verified against real abuse-pattern requests before affiliate links go live.
- Affiliate click-through correctly attributes conversions via subid/UTM passthrough for at least one live partner integration.
