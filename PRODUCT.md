# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Primary: Car-free and transit-oriented urban renters and buyers in Chicago, IL (sub-personas: car-free professionals, active commuters & cyclists, remote/hybrid workers) evaluating and comparing residential addresses before signing a lease or purchasing.

## Product Purpose

Provide an objective, analytical composite livability score (0–100) per Chicago residential address by evaluating hyper-local environmental, transit, logistical, and safety factors, enabling confident pre-lease and pre-purchase decisions without relying on generic real estate marketing or subjective gut feel.

## Positioning

An analytical, high-density location intelligence tool (in the lineage of Linear, Raycast, and Citymapper) that computes deterministic, hyper-local scores directly from official municipal datasets, GTFS schedules, OpenTripPlanner 2 routing, and city-wide block percentiles—never vague AI summaries or generic marketing fluff. Hard-scoped in v1 to the City of Chicago municipal boundary.

## Operating Context

Pre-lease / pre-purchase address discovery and comparison workflow. Users input a primary candidate address (geocoded via Nominatim with candidate disambiguation picker), optional workplace/campus destination for rush-hour transit routing (OpenTripPlanner 2), and optional unit orientation/floor metadata. Results display an address scorecard with 10 factor breakdowns, interactive MapLibre GL vector map overlays, and benchmark commute travel times to major employment hubs.

## Capabilities and Constraints

- **Scoring Scope:** Strictly scoped to the City of Chicago municipal boundary (`OUT_OF_COVERAGE` 422 error for coordinates outside boundary).
- **10 Core Factors:** Transit (`S_transit`), Commute Duration (`S_commute`), Bike Infrastructure (`S_bike`), Groceries (`S_grocery`), Third Places (`S_third_place`), Green Space & Parks (`S_green`), Noise Environment (`S_noise`), Public Safety (`S_safety`), Flood Risk (`S_flood`), Solar Exposure (`S_solar`).
- **Data Provenance:** Chicago Data Portal (CTA 'L' stops, bus headways, bike routes, parks, crimes, 311 flood incidents), OpenStreetMap (POIs, road hierarchy, track buffers), OpenTripPlanner 2 (GTFS-based routing), FEMA NFHL (flood zones), FCC (broadband).
- **Public Safety Normalization:** Crimes trailing 12 months radial penalty normalized against pre-computed ~500m Chicago block percentile distributions (relative rank within Chicago).
- **Privacy & Data Minimization:** Geocache by coordinate/grid cell, never persist raw address strings beyond request lifecycle, zero PII in logs or affiliate metrics.

## Brand Commitments

- **Design Philosophy:** Analytical, high-density location intelligence tool.
- **Anti-Vibe-Coding Bans:** Strictly no AI clichés (no sparkles/wands), no violet/purple neon radial glows, no emojis in headers/cards/tables.
- **Visual Identity:** Dark slate/zinc background palette (`zinc-950`), official CTA rail line color tokens (Red `#c60c30`, Blue `#00a1de`, Brown `#62361b`, Green `#009b3a`, Pink `#e27ea6`, Orange `#f9461c`, Purple `#522398`, Yellow `#f9e300`), sharp geometric vector icons (Lineicons), tight border radii (`rounded-sm`/`rounded-md`).
- **Typography:** Tabular monospace (`Geist Mono`) for all coordinates, distances, travel durations, and numeric scores; clean neutral sans (`Inter`/`Geist`) for body text.

## Evidence on Hand

- Full Product Requirements Document: [docs/PRD.md](docs/PRD.md)
- Technical Architecture Specification: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Implementation Roadmap: [docs/IMPLEMENTATION_PHASES.md](docs/IMPLEMENTATION_PHASES.md)
- Data Sources & Schemas: [docs/DATA_SOURCES.md](docs/DATA_SOURCES.md)
- OpenAPI Specification: [docs/openapi.yaml](docs/openapi.yaml)
- GIS Glossary & Mental Models: [docs/GIS_GLOSSARY.md](docs/GIS_GLOSSARY.md)
- Privacy & Data Handling Policy: [docs/PRIVACY_AND_DATA_HANDLING.md](docs/PRIVACY_AND_DATA_HANDLING.md)
- Web Implementation: [packages/web](packages/web) (React 19, TanStack Router, TanStack Query, Tailwind CSS v4, MapLibre GL JS)

## Product Principles

1. **Objective Data Over Subjective Sentiment:** Every score factor maps directly to verifiable municipal or spatial datasets with deterministic calculations.
2. **High-Density Analytical Precision:** Prioritize concise, actionable metrics, exact distances, and clear visual spatial anchors over verbose prose or marketing fluff.
3. **Local & Privacy-Conscious:** Prioritize self-hosted calculation and spatial caching; treat user search queries and locations with strict privacy safeguards.
4. **Authentic Municipal Detail:** Ground visual and spatial representations in authentic Chicago transit and spatial standards rather than generic abstractions.
