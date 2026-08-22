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
   - Root `Makefile` with universal targets (`dev`, `test`, `lint`, `migrate-up`, `seed`, `generate`).
   - Root `docker-compose.yml` with PostgreSQL 18 + PostGIS and Docker Compose Profiles (`profiles: ["transit"]` for OTP2).
2. **Go API Backend Skeleton (`services/api/`):**
   - Standard library `net/http` server with Go 1.22+ routing patterns.
   - `log/slog` JSON logging configured for Promtail/Loki.
   - Prometheus `/metrics` endpoint.
   - Domain-layered package structure (`internal/api`, `internal/scoring`, `internal/spatial`, `internal/clients`, `internal/cache`).
3. **OpenAPI 3.0 Contract (`docs/openapi.yaml`):**
   - Strict REST request/response schemas for `/api/v1/score`, `/api/v1/geocode`, and `/api/v1/health`.
   - Code generation scripts: Go server types via `oapi-codegen`, TypeScript client types via `openapi-typescript`.
4. **React 19 + Vite Frontend Skeleton (`packages/web/`):**
   - Tailwind CSS v4 configured with sharp, disciplined design tokens (Slate/Zinc, CTA rail colors, zero violet glow).
   - `shadcn/ui` (Radix UI) component primitives.
   - **Lineicons** icon library integration.
   - TanStack Router with type-safe search param validation schema.
   - TanStack Query client setup.

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
     - `cta_stops` ('L' stations and bus corridors).
     - `bike_routes` (Protected lanes and off-street trails).
     - `parks` (City parks, forest preserves, community gardens).
     - `cpd_crimes` (Trailing 12-month crime records).
     - `spatial_grid_cache` (400m bounding cells for Google Places POIs).
     - `unlogged_cache_entries` (Postgres cache table with indexed `expires_at`).
2. **Data Ingestion CLI Tool (`services/api/cmd/seed/main.go`):**
   - SODA API client connecting to the Chicago Data Portal.
   - Automated parser for CTA 'L' stops, bike routes, parks, and crime incident streams.
   - Selective CLI flags: `--dataset=cta`, `--dataset=crimes`, `--limit=1000`, `--all`.

### Validation Criteria
- `make migrate-up` creates all tables and spatial indexes cleanly.
- `make seed` successfully populates PostGIS with real Chicago open data in < 60 seconds.
- Spatial radius queries (`ST_DWithin`) execute in < 5 ms.

---

## Phase 3: Go Scoring Engine & Core API

### Objectives
Implement the deterministic and spatial scoring algorithms with full unit test coverage and wire them to HTTP endpoints.

### Deliverables
1. **Factor Calculators (`services/api/internal/scoring/`):**
   - `factor_transit.go` (`S_transit`: 'L' station distance decay + bus bonus).
   - `factor_grocery.go` (`S_grocery`: Supermarket distance + pharmacy boost).
   - `factor_third_places.go` (`S_third_place`: Cached POI density).
   - `factor_green.go` (`S_green`: Park proximity + Lakefront/606 trail bonus).
   - `factor_noise.go` (`S_noise`: Elevated CTA track & arterial road proximity penalties).
   - `factor_flood.go` (`S_flood`: FEMA zones + 311 sewer backup penalty).
   - `factor_safety.go` (`S_safety`: 12-month violent/property penalty normalized to city percentiles).
   - `factor_solar.go` (`S_solar`: SunCalc azimuth + window orientation + floor level).
2. **Concurrency Orchestrator (`engine.go`):**
   - `errgroup` fan-out with a hard 8-second parent context deadline.
   - Dynamic weight normalization ($\sum w_i = 100\%$).
   - Graceful degradation: timed-out factors return `null` without failing the composite score.
3. **HTTP Handlers (`services/api/internal/api/`):**
   - `POST /api/v1/score`
   - `GET /api/v1/geocode/autocomplete`
   - `GET /api/v1/health`

### Validation Criteria
- 100% unit test coverage on scoring mathematical formulas and edge cases (boundary distances, zero weights).
- Live API response latency < 150 ms for warm cache hits.

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
   - Hero Composite Score Ring (0–100) with archetype badge.
   - Collapsible Weight Customization Slider Drawer with instant reactive recalculation and URL search param sync.
   - 8 Dense Factor Breakdown Cards with distance badges and sub-score progress bars.
2. **Contextual Affiliate Cards:**
   - Broadband / Fiber card with available ISP speed badges (AT&T Fiber, Astound).
   - Lemonade instant renters quote modal trigger.
   - Dolly / Unpakt moving quote comparison card.
   - ClassPass fitness studio trial card.
3. **Bottom Multi-Address Comparison Tray:**
   - Sticky dock to pin and compare up to 3 addresses side-by-side.

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
1. **Caddy Reverse Proxy Configuration:**
   - Path-based routing (`handle_path /blockfactor/*`) with automatic Tailscale TLS.
2. **Monitoring Dashboards:**
   - Grafana dashboard importing BlockFactor Prometheus metrics and Loki logs.
3. **CI/CD Self-Hosted Runner:**
   - GitHub Actions workflow running `make test` and `make lint` on local home server runner.
