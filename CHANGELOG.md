# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `.env.example`'s `DATABASE_URL` had the database name and password swapped relative to `docker-compose.yml`'s actual `POSTGRES_DB`/`POSTGRES_PASSWORD` values, which would have caused `make migrate-up` and the API's own DB connection to fail with an auth error.
- `docker-compose.yml`'s `db` service mounted the `pgdata` volume at `/var/lib/postgresql/data`, which PostgreSQL 18+ images no longer use — the `postgis/postgis:18-3.6-alpine` image expects the volume at `/var/lib/postgresql`, and the container would fail its healthcheck and error-loop indefinitely without this.

### Added

- Phase 1 monorepo scaffolding:
  - Root `Makefile`, `docker-compose.yml` (PostgreSQL 18 + PostGIS, OTP2 2.9.0 with `profiles: ["transit"]`), and GitHub Actions CI workflow (`.github/workflows/ci.yml`).
  - Go API backend skeleton in `services/api/` with Go 1.22+ `net/http` routing, `log/slog` JSON logging, and `/metrics` Prometheus endpoint.
  - React 19 + Vite 8 frontend skeleton in `packages/web/` with Bun package manager, Biome linter, Tailwind CSS v4 design tokens, `shadcn/ui` primitives, and Lineicons.
  - Dual-stack OpenAPI codegen pipeline: `oapi-codegen` for Go and `openapi-typescript` for TypeScript, wired into `make generate` and `make generate-check`.
- OpenAPI 3.1 contract (`docs/openapi.yaml`) for `POST /api/v1/score`, `GET /api/v1/geocode`, `GET /api/v1/isochrones`, and `GET /api/v1/health`.
- `S_bike` scoring factor (PRD §5.11): quality-tiered proximity to bike infrastructure (protected lanes, buffered lanes, greenways, marked shared lanes) plus a directional network bonus for north-south/east-west coverage. Adds `bike` to `WeightOverrides` and `ScoreResponse.factors` in the OpenAPI contract, and rebalances the default weight presets (PRD §3.4) to include it.
- Phase 2 PostGIS schema (`db/migrations/`): 13 `goose` migrations covering `city_boundary`, `cta_stops`, `bus_corridors`, `bike_routes`, `parks`, `osm_pois`, `noise_sources`, `fema_flood_zones`, `sewer_311_incidents`, `cpd_crimes`, `safety_grid_percentiles`, `spatial_grid_cache`, and `cache_entries`, informed by live verification against the real Chicago Data Portal datasets (see `docs/DATA_SOURCES.md`).
- `S_safety` crime taxonomy (PRD §5.9): `cpd_crimes.category` expanded from a `violent`/`property` split to four scored categories (`violent`, `sexual`, `property`, `nuisance`), with a fifth unscored "excluded" set dropped at ingestion. `safety_grid_percentiles` now stores raw per-category counts alongside the combined `penalty_score`. Full `primary_type`/`iucr` mapping verified against the live Chicago Data Portal API (see `docs/DATA_SOURCES.md` "Category coverage").
- Data ingestion CLI (`services/api/cmd/seed`), first working dataset: `--dataset=boundary` fetches Chicago's city limits from the Chicago Data Portal and loads it into `city_boundary`, replacing the table's single row on each run. Adds the generic building blocks every later dataset PR reuses: a Socrata (SODA) API client (`internal/clients`, `X-App-Token` auth) and a PostGIS connection pool (`internal/spatial`, via `pgx/v5`).
- Three more seed CLI datasets: `--dataset=cta` groups the Chicago Data Portal's per-directional-platform rows into one `cta_stops` row per physical station (unioning line flags); `--dataset=bike_routes` and `--dataset=parks` load their respective datasets directly via `ST_GeomFromGeoJSON`. `--all` now runs all four implemented datasets in one pass. Parks are also repaired with `ST_MakeValid()` on ingest — 11 of 617 source park boundaries (including Washington Park, Grant Park, and Lincoln Park) have self-intersecting rings, a real data-quality issue in the source shapefile, not an ingestion bug.
