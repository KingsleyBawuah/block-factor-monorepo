# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Phase 1 monorepo scaffolding:
  - Root `Makefile`, `docker-compose.yml` (PostgreSQL 18 + PostGIS, OTP2 2.9.0 with `profiles: ["transit"]`), and GitHub Actions CI workflow (`.github/workflows/ci.yml`).
  - Go API backend skeleton in `services/api/` with Go 1.22+ `net/http` routing, `log/slog` JSON logging, and `/metrics` Prometheus endpoint.
  - React 19 + Vite 8 frontend skeleton in `packages/web/` with Bun package manager, Biome linter, Tailwind CSS v4 design tokens, `shadcn/ui` primitives, and Lineicons.
  - Dual-stack OpenAPI codegen pipeline: `oapi-codegen` for Go and `openapi-typescript` for TypeScript, wired into `make generate` and `make generate-check`.
- OpenAPI 3.1 contract (`docs/openapi.yaml`) for `POST /api/v1/score`, `GET /api/v1/geocode`, `GET /api/v1/isochrones`, and `GET /api/v1/health`.
- `S_bike` scoring factor (PRD §5.11): quality-tiered proximity to bike infrastructure (protected lanes, buffered lanes, greenways, marked shared lanes) plus a directional network bonus for north-south/east-west coverage. Adds `bike` to `WeightOverrides` and `ScoreResponse.factors` in the OpenAPI contract, and rebalances the default weight presets (PRD §3.4) to include it.
- Phase 2 PostGIS schema (`db/migrations/`): 13 `goose` migrations covering `city_boundary`, `cta_stops`, `bus_corridors`, `bike_routes`, `parks`, `osm_pois`, `noise_sources`, `fema_flood_zones`, `sewer_311_incidents`, `cpd_crimes`, `safety_grid_percentiles`, `spatial_grid_cache`, and `cache_entries`, informed by live verification against the real Chicago Data Portal datasets (see `docs/DATA_SOURCES.md`).
