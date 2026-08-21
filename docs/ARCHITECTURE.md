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

> ⚠️ Being actively designed. Content below represents current thinking, not finalized decisions.

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
│   - Concurrent Worker Pool (PostGIS, OTP2, Places, Sun)     │
│   - Normalization & Weighted Factor Scoring Engine          │
└──────────┬────────────┬────────────┬────────────┬───────────┘
           │            │            │            │
           ▼            ▼            ▼            ▼
┌──────────────────┐ ┌────────┐ ┌────────┐ ┌──────────────────┐
│ PostgreSQL 16    │ │ Redis  │ │  OTP2  │ │  External APIs   │
│ + PostGIS  :5433 │ │ :6379  │ │ :4003  │ │  - Google Places │
│ - CTA data       │ │ (host) │ │(Docker)│ │  - FCC Broadband │
│ - Bike/Trails    │ │ Divvy  │ │ - GTFS │ │  - Nominatim     │
│ - CPD Crimes     │ │ TTL    │ │ - OSM  │ │  - Divvy GBFS    │
│ - Parks/FEMA     │ │ cache  │ └────────┘ └──────────────────┘
│ - Places cache   │ └────────┘
│ - Percentiles    │
└──────────────────┘
```

---

## 4. Architecture Decision Log

| # | Question | Decision | Rationale | Status |
|---|---|---|---|---|
| 1 | Request pipeline timeout & graceful degradation | TBD | TBD | 🔲 Open |
| 2 | Report persistence model | TBD | TBD | 🔲 Open |
| 3 | In-memory cache layer | **Use existing host Redis** (`:6379`) | Already running, zero overhead, no sidecar needed | ✅ Resolved |
| 4 | Frontend rendering model | TBD | TBD | 🔲 Open |
| 5 | DB migration tooling | TBD | TBD | 🔲 Open |
| 6 | Reverse proxy | **Caddy** (`/etc/caddy/Caddyfile`) | Already the sole active proxy; Tailscale TLS auto-managed | ✅ Resolved |
| 7 | Home server network exposure | **Tailscale** for dev; **Cloudflare Tunnel** for Phase 2 public launch | Tailscale MagicDNS covers local/private access with no config; Cloudflare Tunnel adds zero-config public ingress when needed | ✅ Resolved |

