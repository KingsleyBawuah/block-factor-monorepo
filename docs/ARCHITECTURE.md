# BlockFactor: Architecture & Technical Specification

**Version:** 0.1 (In Progress — being designed collaboratively)
**Status:** Draft
**See also:** [PRD.md](./PRD.md) for product requirements and scoring logic.

---

## 1. Deployment Phases

### Phase 1 — Home Server (Current / Development)

BlockFactor is initially self-hosted on a personal home server for active development and personal use. The developer is the first real user with a concrete deadline: **evaluating Chicago neighborhoods ahead of a move from Pilsen, targeting May 2027.**

**Host Hardware:**
- **Device:** Beelink SEi12 Mini PC
- **OS:** Ubuntu 22.04.5 LTS (Jammy Jellyfish)
- **CPU:** Intel Core i7-12650H (10 cores / 16 threads, up to 4.7 GHz)
- **RAM:** 32 GB total — ~24 GB currently free across co-located services
- **Co-located Services:** Plex, Arr stack (Sonarr, Radarr, Bazarr), Usenet clients, Home Assistant, n8n, monitoring stack (Prometheus, Grafana, Loki)

**Dedicated Storage for BlockFactor:**
- A dedicated **~1TB M.2 2230 NVMe SSD** (via USB-C M.2 adapter) will be provisioned as a dedicated mount for BlockFactor's persistent data volumes: PostGIS data, OTP2 graph files, and OSM extracts.
- Rationale: `/media/disk1` (main array, 704GB free) and `/media/disk2` (17TB array, critically full at 100%) are co-located with media. A dedicated SSD avoids I/O contention and keeps BlockFactor's data isolated and easy to manage.
- ⚠️ This SSD is pending setup. Until it is mounted, use the root volume (`/`, 128GB free) for initial development data.

**Key Constraints:**
- OTP2 JVM heap capped at ~5 GB to avoid starving co-located services.
- Aggressive caching is a **first-class requirement**, not a future optimization.
- All services run under Docker Compose — no Kubernetes overhead.
- Development environment only. No production hardening required yet.

**CI/CD:**
- GitHub Actions with a **self-hosted runner** on the home server.
- PRs trigger lint, test, and build pipelines on local hardware (zero cost).

---

### Phase 2 — Cloud Deployment (Post-Polish / Monetization)

Once the application reaches a quality threshold validated through personal use, BlockFactor migrates to cloud infrastructure with monetization activated.

**Target Stack:** Hetzner (compute) + Cloudflare (CDN / DNS / Tunnels)
- Hetzner CX/CAX VMs (~$8–15/month) offer substantially better price/performance than AWS EC2.
- Cloudflare free tier covers DNS, CDN, and DDoS protection. Cloudflare Tunnels can bridge Phase 1's home server to the public internet without port forwarding.
- AWS remains an option for specific managed services (e.g., RDS) but is not the primary compute target.

**CI/CD:**
- Same GitHub Actions pipelines, retargeted to Hetzner via SSH deploy or container registry push.
- Explore Dagger for pipeline portability as a learning opportunity.

---

## 2. Home Server Environment Profile

> Captured via automated server inventory on 2026-08-21. Informs integration decisions.

### 2.1 Reverse Proxy

- **Caddy** is the sole active reverse proxy (`/etc/caddy/Caddyfile`), running since boot.
- Nginx is installed but inactive. Traefik is not present.
- TLS is handled automatically by **Tailscale MagicDNS** — Caddy provisions certs for `*.han-puffin.ts.net` with zero manual management.
- Primary local access domain: `kingsley-home-server.han-puffin.ts.net`
- **Integration pattern for BlockFactor:** Add a `handle_path /blockfactor/*` block in the Caddyfile (or a dedicated port-based virtual host), proxying to `localhost:<port>`. This is the existing pattern for all services.

### 2.2 Existing Infrastructure BlockFactor Can Leverage

| Resource | Location | Relevance |
|---|---|---|
| **Redis** | `localhost:6379` (host-level) | Available for short-TTL in-memory caching (Divvy GBFS, OTP2 results). No sidecar needed. |
| **PostgreSQL 15** | Internal (Mealie) | Do NOT share — spin a dedicated PostGIS container. |
| **PostgreSQL 16** | Internal (n8n) | Do NOT share — spin a dedicated PostGIS container. |
| **Portainer** | `:9443` | Docker management UI — BlockFactor can be managed here or via standalone Compose. |

### 2.3 Port Allocation

Ports in the 4000–4999 range are unoccupied and reserved for BlockFactor services:

| Service | Host Port |
|---|---|
| Go API | `4001` |
| Frontend (Vite dev server) | `4002` |
| OTP2 | `4003` |
| PostgreSQL + PostGIS | `5433` (avoids conflict with host-level `:5432`) |

### 2.4 Compose Project Location

Following the existing manual compose pattern (e.g., `strava-stats`, `readmeabook`):
```
/home/kingsley-swizzin/blockfactor/
├── docker-compose.yml
├── .env
└── data/               # symlink or bind mount → dedicated NVMe SSD mount point
```

---

## 3. High-Level System Topology

## 3. High-Level System Topology

```
┌─────────────────────────────────────────────────────────────┐
│                      Frontend Client                        │
│             React + TanStack (Query / Router)               │
│             MapLibre GL JS (WebGL / Vector Tiles)           │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTPS / JSON REST API
                           ▼
                    [ Caddy Reverse Proxy ]
                    [ Tailscale MagicDNS TLS ]
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      Go Backend API  :4001                  │
│   - Address Geocoding Handler & Pipeline Orchestrator       │
│   - Concurrent errgroup Fan-Out & Rate Semaphores           │
│   - In-Process Memory Cache (Divvy 5m TTL)                  │
│   - Normalization & Weighted Factor Scoring Engine          │
└──────────┬────────────────────────┬─────────────────────────┘
           │                        │
           ▼                        ▼
┌──────────────────────────┐   ┌──────────────────────────────┐
│  PostgreSQL 18 + PostGIS │   │  OpenTripPlanner 2 (OTP2)    │
│  Port: 5433              │   │  Port: 4003 (JVM max 5GB)    │
│  - CTA Stops & Routes    │   │  - CTA GTFS Schedule         │
│  - Bike Lanes & Trails   │   │  - Cook County OSM Graph     │
│  - CPD Crimes (12 mo)    │   │  - Pedestrian Isochrones     │
│  - Parks & Flood Layers  │   │  - Multi-Modal Commute Times │
│  - Spatial Grid Cache    │   └──────────────────────────────┘
│  - Query/Transit Cache   │
└──────────────────────────┘
```

---

## 4. Backend Architecture & Request Orchestration

### 4.1 Core Language & Contracts
* **Language:** **Go**
  * High-performance compiled binary with minimal memory footprint on shared home server hardware.
  * Native goroutine scheduling and channel primitives for concurrent I/O fan-out.
* **Type Contracts:** **OpenAPI 3.0 / REST**
  * Single source-of-truth API specification.
  * Generates strict types for Go backend (`oapi-codegen`) and TypeScript frontend (`openapi-typescript`).
  * Avoids gRPC gateway complexity while maintaining end-to-end type safety.

### 4.2 Concurrency & Pipeline Lifecycle (`errgroup`)
Rather than a generic worker pool, address evaluation uses structured concurrency with Go's `golang.org/x/sync/errgroup`:

```
┌───────────────── http.Request (Address + Unit Meta + Weights) ─────────────────┐
│                                                                                │
│ 1. Geocode Address (Nominatim) ──► Lat / Lng Coordinate Point                  │
│                                                                                │
│ 2. Concurrent Fan-Out via errgroup (Parent Context: 8s Deadline)               │
│    ├── go scoreTransit(ctx, pt)        [PostGIS + GTFS]                        │
│    ├── go scoreGrocery(ctx, pt)        [PostGIS / OSM]                         │
│    ├── go scoreThirdPlaces(ctx, pt)    [PostGIS Grid Cache / Places API]       │
│    ├── go scoreGreenSpace(ctx, pt)     [PostGIS / Parks + Trails]              │
│    ├── go scoreNoise(ctx, pt)          [PostGIS / Track & Arterial Buffers]    │
│    ├── go scoreFloodRisk(ctx, pt)      [PostGIS / FEMA + 311 Sewer History]    │
│    ├── go scorePublicSafety(ctx, pt)   [PostGIS / CPD 12mo Density]            │
│    ├── go scoreBroadband(ctx, pt)      [PostGIS Address Cache]                 │
│    ├── go scoreSolarExposure(meta)     [Pure Math / In-Memory]                 │
│    └── go fetchCommuteIsochrones(ctx)  [Local OTP2 HTTP API]                   │
│                                                                                │
│ 3. Rate-Limiting Semaphores                                                    │
│    External metered calls (e.g. Google Places API on cache miss) use           │
│    buffered channel semaphores to strictly enforce rate limits.                │
│                                                                                │
│ 4. Graceful Degradation & Normalization                                        │
│    If a non-critical factor times out (e.g. OTP2 takes >8s), it returns null   │
│    and is excluded from the denominator. The report renders remaining factors. │
└────────────────────────────────────────────────────────────────────────────────┘
```

### 4.3 Geocoding Strategy
* **Phase 1 (Development):** Public Nominatim API (`nominatim.openstreetmap.org`) with structured query formatting for low-volume development testing.
* **Phase 2 (Production / Pre-Launch):**
  * **Self-Hosted Nominatim (PostGIS):** Ingests Illinois OSM extract for accurate, ToS-compliant address resolution and building polygon snapping.
  * **Photon (Elasticsearch/Lucene, Optional):** Lightweight search index (<2GB RAM) for low-latency (<20ms) typeahead address search in the UI input bar.

---

## 5. Spatial Database & Caching Architecture

### 5.1 Database Engine & Image
* **Engine:** **PostgreSQL 18 + PostGIS** extension.
* **Docker Context:** Runs as a dedicated container (`blockfactor-db`) isolated from any existing host database.
* **Volume Mount:** Persisted to the dedicated NVMe SSD mount point (`/media/.../blockfactor/pgdata`).

### 5.2 Spatial Coordinate Standard & Types
* **Standard Representation:** `geography(Point, 4326)` (WGS84 GPS latitude and longitude).
* **Rationale:** Direct GPS compatibility; native meter-based radial queries (`ST_DWithin`) without reprojection overhead.

### 5.3 Spatial Indexing Strategy
* **Index Type:** `GIST` indexes on all location geometry columns (`cta_stops`, `cpd_crimes`, `parks`, `bike_routes`).
* **Performance:** Executes radial queries across 300k+ spatial records in $<5\text{ ms}$.

### 5.4 Two-Tier Caching Architecture (100% Self-Contained)
To avoid dependencies on external host Redis instances, caching is partitioned across two self-contained tiers:

1. **In-Process Go Cache (Volatile Tier):**
   * Stored in-memory in Go (`sync.Map` / bounded LRU with TTL).
   * Used for high-frequency, ultra-short data: **Divvy GBFS dock & e-bike availability** (5-minute TTL).
2. **PostgreSQL Cache Tables (Persistent Tier):**
   * Stored in `UNLOGGED` tables with an indexed `expires_at` column.
   * **Google Places 400m Grid Cache:** 30-day TTL.
   * **FCC Broadband Address Cache:** Bi-annual refresh.
   * **OTP2 Commute Route Durations:** 7-day TTL.

---

## 6. Transit & Isochrone Engine (OpenTripPlanner 2.x)

### 6.1 Role & Capabilities
* **Engine:** OpenTripPlanner 2.x running in a dedicated Docker container (`blockfactor-otp`).
* **Inputs:**
  * CTA Static GTFS Schedule archive (`chicago-gtfs.zip`).
  * Cook County OpenStreetMap extract (`illinois.osm.pbf`).
* **Outputs:**
  * **Door-to-door transit commute durations** (walking + waiting + riding + transfers) to benchmark employment centers (The Loop, O'Hare, Midway, Fulton Market).
  * **Pedestrian & transit reach isochrone polygons** (e.g. 15-min and 30-min travel boundaries) streamed to MapLibre GL JS as GeoJSON.

### 6.2 Resource Controls & Persistence
* **JVM Heap Limit:** Strictly capped at `-Xmx5G` to prevent memory contention with co-located media and automation stacks.
* **Graph Persistence:** The graph is pre-compiled once (`otp --build data/`) and saved as `graph.obj` on the dedicated NVMe SSD. On restarts, OTP2 boots in $<15\text{ seconds}$ by loading the pre-compiled graph directly into RAM.

---

## 7. Frontend Client & Visualization Architecture

### 7.1 Framework & Core Tooling
* **Build Tool & Framework:** **React 19 + Vite** (Single Page Application).
* **Routing:** **TanStack Router**
  * Strict type-safe route definitions.
  * **Search Param Serialization:** Address, unit orientation (e.g. facing, floor), and custom factor weights ($0\text{–}100\%$) are continuously synchronized to the URL query string (`/report?address=...&w_transit=35&w_grocery=25&facing=S`).
  * Enables zero-friction link sharing and browser bookmarking that preserves exact report state.
* **Server State & Caching:** **TanStack Query**
  * Automatic request deduplication, background prefetching, and client-side caching.
* **OpenAPI Client:** Generated TypeScript API client (`openapi-typescript` + `openapi-fetch`) typed directly from backend OpenAPI specs.

### 7.2 Map Engine & WebGL Vector Rendering
* **Map Visualizer:** **MapLibre GL JS**
* **Vector Basemap Tiles:** CartoCDN / OpenFreeMap vector styles ($0 licensing fees, GPU accelerated).
* **Interactive Spatial Layers:**
  * **Isochrone Polygons:** Renders 15-min and 30-min pedestrian & transit reach boundaries as semi-transparent GeoJSON polygon fills.
  * **Protected Bike Lanes & Off-Street Trails:** Green vector linestrings with interactive hover tooltips.
  * **POI Density Clusters:** Coffee shops, gyms, and grocery markers with radial distance circles.

### 7.3 Link Sharing & Social Previews (Phasing)
* **Phase 1 (Home Server / Personal Use):** Client-side URL state via TanStack Router allows immediate link sharing with identical visual configuration.
* **Phase 2 (Public Launch):** A lightweight Go route handler (`GET /share/:report_id`) injects dynamic OpenGraph meta cards (title, summary score, static preview map snapshot) for iMessage/Slack/Twitter previews.

---

## 8. Database Migrations (`goose`)

### 8.1 Tooling & Workflow
* **Migration Engine:** **`goose`**
* **Migration Format:** Plain `.sql` files with standard `-- +goose Up` and `-- +goose Down` annotations located in `/db/migrations/`.
* **Execution Options:**
  * Standalone CLI for local schema management: `goose -dir db/migrations postgres "$DB_URL" up`.
  * Embedded migration runner: Can be embedded into the Go server binary via `goose.Up(db, "migrations")` to automatically migrate dev and CI databases on container startup.

---

## 9. Monorepo Orchestration & Developer Experience

### 9.1 Root `Makefile` Architecture
To ensure **100% parity between local development and GitHub Actions CI pipelines**, all high-level developer tasks and CI validation steps are driven via a root `Makefile`:

```makefile
.PHONY: dev build test lint migrate-up seed generate

# Start full local development stack (DB, OTP2, Go API with hot-reload, Vite)
dev:
	docker compose up -d db otp
	@echo "Starting Go API (Air) and Vite Frontend..."
	# runs concurrently via air and pnpm dev

# Run all unit and integration tests
test:
	go test -v ./services/api/...
	cd packages/web && pnpm test

# Run linters across Go and TypeScript
lint:
	golangci-lint run ./services/api/...
	cd packages/web && pnpm lint

# Run database migrations
migrate-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up

# Ingest and seed Chicago base datasets
seed:
	go run ./services/api/cmd/seed/main.go

# Regenerate OpenAPI contracts and client SDKs
generate:
	oapi-codegen -config api/openapi.cfg.yaml docs/openapi.yaml
	cd packages/web && pnpm generate-api
```

### 9.2 CI/CD Parity
* In GitHub Actions (self-hosted runner or GitHub-hosted), CI jobs simply invoke the Makefile targets (`make lint`, `make test`, `make generate-check`).
* Eliminates configuration drift between local developer machines and CI pipelines.

---

## 10. Observability & Monitoring Stack Integration

Because the home server already operates a production-grade monitoring stack (**Prometheus, Grafana, Loki, Promtail**), BlockFactor hooks directly into this infrastructure:

### 10.1 Structured Logging (`log/slog` $\rightarrow$ Promtail $\rightarrow$ Loki)
* Standard library **`log/slog`** with JSON formatting (`slog.NewJSONHandler(os.Stdout, ...)`).
* Structured fields: `trace_id`, `address`, `duration_ms`, `factor`, `cache_hit`, `status`.
* Promtail scrapes container stdout logs directly into Loki for fast querying and debugging in Grafana dashboards.

### 10.2 Metrics Collection (`/metrics` $\rightarrow$ Prometheus)
* Exposes a standard **`/metrics`** endpoint in the Go backend using `prometheus/client_golang`.
* **Key Custom Metrics:**
  * `blockfactor_score_request_duration_seconds` (Histogram by address hash / status)
  * `blockfactor_factor_duration_seconds` (Histogram partitioned by factor: `transit`, `grocery`, `safety`, `otp`, etc.)
  * `blockfactor_cache_hits_total` / `blockfactor_cache_misses_total` (Counter by cache domain: `places`, `otp`, `fcc`, `divvy`)
  * `blockfactor_external_api_calls_total` (Counter for Google Places and FCC API invocations to track quota consumption)

---

## 11. Architecture Decision Log

| # | Topic | Decision | Rationale | Status |
|---|---|---|---|---|
| 1 | Backend Language | **Go** | Concurrency primitives, low memory footprint, single binary deployment. | ✅ Resolved |
| 2 | Type Contracts | **OpenAPI Spec + Codegen** | Cross-stack type safety without gRPC gateway overhead. | ✅ Resolved |
| 3 | Concurrency Model | **`errgroup` Fan-Out + Semaphores** | Structured concurrency with 8s deadline & graceful factor degradation. | ✅ Resolved |
| 4 | Geocoding Engine | **Public Nominatim (Dev) $\rightarrow$ Self-Hosted Nominatim + Photon (Prod)** | ToS compliance, high accuracy, and fast typeahead. | ✅ Resolved |
| 5 | Database Engine | **PostgreSQL 18 + PostGIS** | Standard spatial engine, native GIS math, and GIST indexing. | ✅ Resolved |
| 6 | Spatial Coordinate Standard | **WGS84 (SRID 4326) / `geography`** | Direct GPS compatibility; native meter-based radial queries without reprojection bugs. | ✅ Resolved |
| 7 | Caching Architecture | **Self-Contained (PostgreSQL UNLOGGED + Go Memory Cache)** | Zero dependency on host Redis; $0 cost, no port conflicts. | ✅ Resolved |
| 8 | Transit Engine | **OpenTripPlanner 2.x (Docker, capped at 5GB JVM)** | Gold standard for transit scheduling, transfers, and native GeoJSON isochrones. | ✅ Resolved |
| 9 | Frontend Stack | **React + Vite + TanStack (Router/Query)** | Lightweight SPA, zero Node server runtime, native WebGL map support. | ✅ Resolved |
| 10 | Map Engine | **MapLibre GL JS** | Open-source WebGL vector map rendering with $0 tile costs. | ✅ Resolved |
| 11 | URL State & Link Sharing | **TanStack Router Search Params (Phase 1) + Go OG Previews (Phase 2)** | Deep-linkable custom weight presets now, social cards on public launch. | ✅ Resolved |
| 12 | Reverse Proxy | **Caddy (`/etc/caddy/Caddyfile`)** | Tailscale MagicDNS TLS auto-managed; path-based routing. | ✅ Resolved |
| 13 | Server Network Ingress | **Tailscale (Dev) $\rightarrow$ Cloudflare Tunnel (Prod)** | Zero-config private access now, secure public edge later. | ✅ Resolved |
| 14 | DB Migration Tooling | **`goose` (Plain SQL migrations)** | Single SQL format with Up/Down, embeddable in Go binary or CLI. | ✅ Resolved |
| 15 | Monorepo Orchestration | **Root `Makefile`** | Zero installation prerequisites; strict 1:1 parity between local dev and CI. | ✅ Resolved |
| 16 | HTTP Router | **Standard Library `net/http` (Go 1.22+)** | Zero third-party router dependencies; clean method-based routing patterns. | ✅ Resolved |
| 17 | Observability & Logging | **`log/slog` (JSON) + Prometheus `/metrics`** | Hooks directly into existing host Loki & Prometheus monitoring stack. | ✅ Resolved |
| 18 | Compose Topology | Split Compose (`docker-compose.yml` + `docker-compose.otp.yml`) vs. Single Stack | OTP2 graph build takes 15 min; separating keeps API rapid to restart. | 🔲 Open Question |
| 19 | Go Internal Package Layout | Granular factor packages (`internal/transit`, `internal/solar`) vs. Domain modules | Balance separation of concerns with idiomatic Go package design. | 🔲 Open Question |
| 20 | Data Ingestion Pipeline | CLI Seed tool (`cmd/seed`) vs. Background Cron Jobs | How monthly CTA/Crime dataset updates are ingested into PostGIS. | 🔲 Open Question |

