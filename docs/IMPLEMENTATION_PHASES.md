# BlockFactor: Implementation Phases & Roadmap

**Status:** Draft / Active Planning  
**See also:**
* [PRD.md](./PRD.md) — Product Requirements & Scoring Logic
* [ARCHITECTURE.md](./ARCHITECTURE.md) — Technical Architecture & Decision Log
* [AGENTS.md](../AGENTS.md) — Engineering Standards & Pair Programming Rules

---

## Overview

This document outlines the phased engineering roadmap for BlockFactor, spanning local development on the home server through Phase 2 public cloud deployment. Each phase is scoped with clear deliverables, architecture alignment, and acceptance criteria.

---

## Phase 1: Monorepo Foundation, Design System & Contracts

### Objectives
Establish the monorepo foundation, Docker development environment, shared OpenAPI contract, and front-end design system primitives.

### Deliverables
1. **Repository Tooling:**
   - Root `Makefile` with universal targets (`dev`, `test`, `lint`, `migrate-up`, `seed`, `generate`, `generate-check`).
   - Root `docker-compose.yml` with PostgreSQL 18 + PostGIS and Docker Compose Profiles (`profiles: ["transit"]` for OTP2). Named volumes for local dev; the home server binds the NVMe SSD via `docker-compose.override.yml`.
   - Toolchain pins recorded during scaffolding: `go.mod` `toolchain` directive, `.nvmrc` + `package.json` `packageManager` (Corepack/pnpm), exact Docker image tags (PostGIS, OTP2), pinned `golangci-lint`.
   - GitHub Actions CI (GitHub-hosted runners) running `make lint`, `make test`, and `make generate-check` on every PR, with branch protection on `main`.
2. **Go API Backend Skeleton (`services/api/`):**
   - Standard library `net/http` server with Go 1.22+ routing patterns.
   - `log/slog` JSON logging configured for Promtail/Loki.
   - Prometheus `/metrics` endpoint.
   - Domain-layered package structure (`internal/api`, `internal/scoring`, `internal/spatial`, `internal/clients`, `internal/cache`).
3. **OpenAPI 3.0 Contract (`docs/openapi.yaml`):**
   - Strict REST request/response schemas for `POST /api/v1/score` (accepts a raw address or pre-resolved coordinates), `GET /api/v1/geocode`, `GET /api/v1/isochrones`, and `GET /api/v1/health`.
   - Score response contract: factor sub-scores as objects (`score`, `degraded` flag, distances / contributing POIs) plus benchmark commute pills; isochrones are excluded and served only by `GET /api/v1/isochrones` (faster first paint, independent caching). Out-of-coverage requests return 422 `OUT_OF_COVERAGE`.
   - Code generation scripts: Go server types via `oapi-codegen`, TypeScript client types via `openapi-typescript`.
4. **React 19 + Vite Frontend Skeleton (`packages/web/`):**
   - Tailwind CSS v4 configured with sharp, disciplined design tokens (Slate/Zinc, CTA rail colors, zero violet glow).
   - `shadcn/ui` (Radix UI) component primitives.
   - **Lineicons** icon library integration.
   - TanStack Router with type-safe search param validation schema.
   - TanStack Query client setup.
   - Vite dev proxy (`/api` → `http://localhost:4001`) — no CORS in development.

### Validation Criteria
- `make test` and `make lint` pass across both Go and TypeScript workspaces.
- `make generate` regenerates contract types without manual intervention.

---

## Phase 2: PostGIS Schema & Open Data Ingestion

### Objectives
Create database tables with spatial indexes and build the standalone CLI tool to seed Chicago municipal datasets.

### Deliverables
1. **`goose` Database Migrations (`db/migrations/`):**
   - Spatial tables with `GIST` indexes on `geography(Point, 4326)`:
     - `cta_stops` ('L' stations).
     - `bus_corridors` (High-frequency bus segments derived from GTFS `stop_times` AM-peak headway analysis, powering the `S_transit` bus bonus).
     - `bike_routes` (Protected lanes and off-street trails).
     - `parks` (City parks, forest preserves, community gardens).
     - `osm_pois` (Supermarkets, convenience stores, and pharmacies from the Illinois OSM extract, powering `S_grocery`).
     - `cpd_crimes` (Trailing 12-month crime records).
     - `safety_grid_percentiles` (~500m citywide penalty distribution powering `S_safety` normalization).
     - `fema_flood_zones` (NFHL hazard polygons) and `sewer_311_incidents` (24-month water/sewer backup points), powering `S_flood`.
     - `noise_sources` (Grade-tagged CTA track segments + expressway/arterial linestrings, powering `S_noise`).
     - `city_boundary` (City of Chicago boundary polygon powering the `OUT_OF_COVERAGE` hard-reject).
     - `spatial_grid_cache` (400m bounding cells for Google Places POIs).
     - `unlogged_cache_entries` (Postgres cache table with indexed `expires_at`).
2. **Data Ingestion CLI Tool (`services/api/cmd/seed/main.go`):**
   - SODA API client connecting to the Chicago Data Portal (authenticated via `CHICAGO_APP_TOKEN`).
   - Automated parsers for CTA 'L' stops, bike routes, parks, crime incidents, FEMA/311 flood data, OSM POIs, noise geometry, and the city boundary polygon.
   - Post-ingest derivation steps: GTFS `stop_times` headway analysis → `bus_corridors`; crime-grid distribution → `safety_grid_percentiles` (staging-table + rename swap per ARCHITECTURE 11.3).
   - Ingestion tooling per dataset (GDAL `ogr2ogr` vs. Go shapefile lib, PBF parsing, GTFS parsing) resolved at phase start per ARCHITECTURE 13.2.
   - Selective CLI flags: `--dataset=cta`, `--dataset=crimes`, `--dataset=osm_pois`, `--dataset=flood`, `--dataset=noise`, `--dataset=derive`, `--limit=1000`, `--all`.

### Validation Criteria
- `make migrate-up` creates all tables and spatial indexes cleanly.
- `make seed` successfully populates PostGIS with real Chicago open data (SODA datasets in < 60 seconds; the one-time OSM PBF import and derivation steps are exempt from this budget).
- Spatial radius queries (`ST_DWithin`) execute in < 5 ms.

---

## Phase 3: Go Scoring Engine & Core API

### Objectives
Implement the deterministic and spatial scoring algorithms with full unit test coverage and wire them to HTTP endpoints.

### Deliverables
1. **Factor Calculators (`services/api/internal/scoring/`):**
   - `factor_transit.go` (`S_transit`: 'L' station distance decay + bus bonus).
   - `factor_commute.go` (`S_commute`: Rush-hour OTP2 point-to-point transit duration + transfer friction).
   - `factor_grocery.go` (`S_grocery`: Supermarket distance + pharmacy boost).
   - `factor_third_places.go` (`S_third_place`: Cached POI density).
   - `factor_green.go` (`S_green`: Park proximity + Lakefront/606 trail bonus).
   - `factor_noise.go` (`S_noise`: Elevated CTA track & arterial road proximity penalties).
   - `factor_flood.go` (`S_flood`: FEMA zones + 311 sewer backup penalty).
   - `factor_safety.go` (`S_safety`: 12-month violent/property penalty normalized to city percentiles).
   - `factor_solar.go` (`S_solar`: orientation base lookup × exposure modifier + floor bonus).
2. **Outbound Clients (`services/api/internal/clients/`):**
   - `places.go` — Google Places (New) Nearby Search client with 400m grid-cache fill (30-day TTL), powering `S_third_place` and the Phase 4 POI map layer.
     - **TODO (prerequisite):** provision the Google Cloud project + Places API (New) key — restricted, hard daily quota cap, billing alert — before this client lands.
   - `nominatim.go` — one-shot geocoding client returning ranked candidates (public endpoint, ≤ 1 req/sec, custom `User-Agent`; no typeahead in Phase 1 per Nominatim policy).
   - `otp.go` — **interface definition only**: `factor_commute.go` is built and unit-tested against fixture-based fakes shaped as **GTFS GraphQL API** responses (OTP 2.x removed the legacy REST plan API); the live client lands in Phase 6.
3. **Concurrency Orchestrator (`engine.go`):**
   - `errgroup` fan-out with a hard 8-second parent context deadline.
   - Dynamic weight normalization ($\sum w_i = 100\%$).
   - Graceful degradation: timed-out or data-less factors return `null`, are excluded from the weight sum, and remaining weights renormalize; the response marks degraded factors.
4. **HTTP Handlers (`services/api/internal/api/`):**
   - `POST /api/v1/score`
   - `GET /api/v1/geocode` (one-shot geocode returning coordinates; typeahead autocomplete deferred until self-hosted Photon per Nominatim policy)
   - `GET /api/v1/health`

### Validation Criteria
- 100% unit test coverage on scoring mathematical formulas and edge cases (boundary distances, zero weights, all-null factors).
- Live API response latency < 150 ms for warm cache hits (excluding `S_commute`, which activates with OTP2 in Phase 6).

---

## Phase 4: Frontend WebGL Map & Vector Visualization

### Objectives
Build the GPU-accelerated interactive map workspace with custom Chicago vector styles and spatial layer overlays.

### Deliverables
1. **MapLibre GL JS Map Component (`packages/web/src/map/`):**
   - High-contrast, clean Chicago basemap style (CartoCDN / OpenFreeMap).
   - Responsive split-screen desktop & mobile viewport handling.
2. **Interactive Spatial Layers:**
   - **CTA Rail Layer:** Official line colors with interactive station hover cards.
   - **Protected Bike Layer:** High-visibility green linestring overlay with trail access markers.
   - **POI Density Markers:** Cafes, fitness studios, and supermarkets with distance radius rings.
   - **Divvy Dock Layer:** Station markers with live capacity badges (display-only — Divvy is not a scored factor). Delivers `internal/clients/divvy.go` + the in-memory GBFS cache (5m TTL).
   - **Solar Ray Widget:** 3D compass orientation ray projected from the address marker.

### Validation Criteria
- 60fps GPU-accelerated panning and zooming on desktop and mobile browsers.
- Smooth layer toggle transitions with zero WebGL context leaks.

---

## Phase 5: Scored Report UI, Sliders & Affiliate Integration

### Objectives
Assemble the high-density Split-Screen Analytics Workspace, reactive weight customization drawer, and contextual monetization cards.

### Deliverables
1. **Left Scored Analytics Panel:**
   - Hero Composite Score Ring (0–100). (Neighborhood archetype badge deferred to v2.)
   - Collapsible Weight Customization Slider Drawer with instant reactive recalculation and URL search param sync.
   - 8–9 Dense Factor Breakdown Cards (the commute card renders only when a destination is set) with distance badges and sub-score progress bars.
2. **Contextual Affiliate Cards:**
   - Broadband / Fiber card with available ISP speed badges (AT&T Fiber, Astound). Delivers `internal/clients/fcc.go` + per-address cache; the card hides gracefully on FCC API failure.
   - Lemonade instant renters quote modal trigger.
   - Dolly / Unpakt moving quote comparison card.
   - ClassPass fitness studio trial card.
3. **Bottom Multi-Address Comparison Tray:**
   - Sticky dock to pin and compare up to 3 addresses side-by-side.
4. **Error & Empty States:**
   - Address-not-found, ambiguous-address candidate picker, out-of-coverage (422 `OUT_OF_COVERAGE`), geocoding failure, OTP2-unavailable, and degraded-factor (`null`) states — styled per AGENTS.md (high-density copy, no emojis).

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
   - Dockerfile and build script pulling `chicago-gtfs.zip` and `illinois.osm.pbf`.
   - Pre-compiled graph persistence (`graph.obj`) with `-Xmx5G` JVM cap.
2. **Go Backend OTP2 Client (`services/api/internal/clients/otp.go`):**
   - Implements the client interface stubbed in Phase 3, activating `S_commute` and the benchmark commute pills end-to-end.
   - Speaks the **GTFS GraphQL API**; isochrones via the travel-time API (no legacy REST plan calls).
   - Multimodal travel time calculations to Loop, O'Hare, Midway, and Fulton Market.
   - GeoJSON isochrone polygon endpoint proxy (15-min walk, 30-min transit).
3. **Frontend Isochrone Map Rendering:**
   - Semi-transparent reachable area polygons rendered directly on MapLibre.

### Validation Criteria
- OTP2 boots in < 15 seconds from pre-compiled `graph.obj`.
- Isochrone polygons render accurately over the Chicago street grid.

---

## Phase 7: Deployment, Observability & Home Server Integration

### Objectives
Deploy BlockFactor to the Beelink home server behind Caddy with automated monitoring and Tailscale ingress.

### Deliverables
1. **Caddy Reverse Proxy & Production Frontend Serving:**
   - Path-based routing with automatic Tailscale TLS: `/blockfactor/api/*` → Go API (`:4001`); all other `/blockfactor/*` paths serve the static `vite build` output (the Vite dev server is not a deployment target).
   - Sub-path hosting configured end-to-end: Vite `base`, TanStack Router `basepath`, and static asset URLs.
2. **Monitoring Dashboards:**
   - Grafana dashboard importing BlockFactor Prometheus metrics and Loki logs.
3. **CI/CD Self-Hosted Runner:**
   - Migrates the Phase 1 GitHub-hosted workflows (`make lint` / `make test` / `make generate-check`) to the local home-server runner.
4. **Data Durability:**
   - Seeded datasets and caches are reproducible via `make seed` — no backup required in Phase 1. A nightly `pg_dump` is introduced only once user-generated state (saved reports / share links) exists in Phase 2.
