# OTP 2.x isochrones: how, and through which API?

Ticket: [#81](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/81) (map: [#79](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/79)).
Researched 2026-10-24 against the pinned image `opentripplanner/opentripplanner:2.9.0` (`docker-compose.yml`).

## Short answer

**OTP 2.9.0 cannot produce isochrones. The feature was removed.**

- OTP 2.x had one isochrone endpoint, `/otp/traveltime/isochrone`, in the "sandbox" (experimental, off by default; enabled with the `SandboxAPITravelTime` feature flag). It was REST, not GraphQL.
- It was **removed in OTP 2.6.0** (PR #5890, merged 2024-06-12). The 2.9.0 source tree has no traveltime code, no `SandboxAPITravelTime` flag, and no isochrone field in the GTFS GraphQL schema.
- The OTP project's stated reason: no capacity to maintain it, and OTP2 is deliberately a passenger-trip planner, not an analysis tool. They recommend R5 (Conveyal) or OTP 1.5 for isochrones.
- So `ARCHITECTURE.md` §6.1 ("Isochrones come from the travel-time API"), decision row 8 ("native GeoJSON isochrones"), and `IMPLEMENTATION_PHASES.md` Phase 6 line 199 are **wrong for the pinned version**. The question of a GraphQL-only exception is moot: there is no OTP isochrone API to except.
- Latency: no published figure exists for the removed endpoint (unverified), and nothing to measure on 2.9.0.

The decision this forces: where do the 15-min walk and 30-min transit polygons come from? Options are listed at the end.

## Details

### What it was (OTP 2.2 to 2.5, now gone)

Terms: an **isochrone** is a polygon outlining everywhere reachable within a time budget from a start point ("ripple on a pond", see `docs/GIS_GLOSSARY.md`). A **cutoff** is that time budget.

From the sandbox doc at tag v2.5.0 ([TravelTime.md](https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.5.0/docs/sandbox/TravelTime.md)):

| Aspect | Value |
|---|---|
| Status | Sandbox: experimental, no stability guarantees, off by default |
| Enable | `otp-config.json`: `"otpFeatures": { "SandboxAPITravelTime": true }` |
| Style | Plain REST `GET`, not GraphQL |
| Endpoints | `/otp/traveltime/isochrone` (vector boundaries); `/otp/traveltime/surface` (GeoTIFF raster of travel seconds) |
| Origin | `location` = `lat,lon` or a stop id |
| Departure | `time` = ISO-8601 instant, default now. **One instant only, no time window** |
| Budget | `cutoff` = ISO-8601 duration (e.g. `15M`); repeatable for several rings; default 1 hour |
| Mode | `modes`; the doc says "WALK is not implemented, use `WALK, TRANSIT` instead", so a walk-only polygon was not directly supported |
| Direction | `arriveBy` true/false |

The sandbox contract says authors "can change its API any time" and unmaintained features can be removed after notice ([SandboxExtension](https://docs.opentripplanner.org/en/v2.9.0/SandboxExtension/)). The doc did not state the response format in the page I read; the code class names (`IsochroneData`, `IsochroneRenderer`) suggest polygon output, but that is *inferred*, not verified.

### Removal timeline

- 2022-05-19: sandbox API merged, ported from OTP1 ([PR #4129](https://github.com/opentripplanner/OpenTripPlanner/pull/4129)).
- Dec 2023: issue [#4880](https://github.com/opentripplanner/OpenTripPlanner/issues/4880) proposes improvements (time window, arrive-by) but recommends a *standalone isochrone service based on R5*, because the project steering committee decided OTP is a trip engine.
- 2024-06-12: [PR #5890 "Remove TravelTime API"](https://github.com/opentripplanner/OpenTripPlanner/pull/5890): "There are no resources to maintain this and the current/first implementation of this has its limitations." Listed in the 2.6.0 [Changelog](https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.6.0/doc/user/Changelog.md).
- In #4880 maintainers confirm: the old implementation only showed reachability for a departure at exactly the given time (a faster trip at 8:10 would not count when asking for 8:00), and recommend R5 or OTP 1.5.
- 2.6.0 was published 2024-09-18; I verified by listing each tag's file tree that `ext/traveltime` exists in v2.2.0 and v2.5.0 and is absent in v2.6.0, v2.9.0, v2.10.0, and `dev-2.x`.

### What OTP 2.9.0 does say

- [Travel Time Analysis](https://docs.opentripplanner.org/en/v2.9.0/Analysis/): "we have decided not to port the OTP1 analysis features over to OTP2". Points to R5 and its R/Python wrappers.
- [OTP Sandbox list](https://docs.opentripplanner.org/en/v2.9.0/SandboxExtension/) and `OTPFeature.java` at v2.9.0 have no travel-time entry (only `SandboxAPIGeocoder`, `SandboxAPIMapboxVectorTilesApi`, `SandboxAPIParkAndRideApi` API flags): [source](https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/application/src/main/java/org/opentripplanner/framework/application/OTPFeature.java).
- The GTFS GraphQL schema at v2.9.0 mentions "polygon" only for stop geometry and flex zones, not reachability: [schema.graphqls](https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/application/src/main/resources/org/opentripplanner/apis/gtfs/schema.graphqls).
- Docs are inconsistent: the [Version Comparison](https://docs.opentripplanner.org/en/v2.9.0/Version-Comparison/) table still shows "Isochrones: yes" for OTP2, while the same page's Analysis section says such features "may be removed in the near future" and #5890 shows they were. Trust the code and changelog over that table.

### GraphQL-only rule (§6.1)

No conflict, because there is nothing to conflict with. The GTFS GraphQL API ([doc](https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/doc/user/apis/GTFS-GraphQL-API.md), endpoint `/otp/gtfs/v1`) remains the only OTP API the Go client needs. What does conflict is the sentence in §6.1 claiming OTP serves isochrones; that needs correcting.

### Latency

No primary source gives isochrone timings for OTP2 (unverified). Any estimate for a replacement must be measured locally.

## Options for the polygons (all *inferred*, none verified here)

- **A. R5 sidecar** (Conveyal, MIT, v7.6 released 2026-08-01 per [GitHub](https://github.com/conveyal/r5)): the engine OTP's maintainers recommend. Built for one-to-many travel times over a departure *time window*, which matches the "median of 8:00-9:00 Tuesday" decision in #80. Cost: a second JVM service and a second graph build. R5 states it has no stable API and no third-party support ([README](https://raw.githubusercontent.com/conveyal/r5/master/README.md)). Whether it returns polygons directly or travel-time grids we contour ourselves is not verified.
- **B. Fan-out through OTP GraphQL**: sample a grid of points around the address, ask OTP for travel time to each, and draw the polygon in PostGIS. Keeps one engine and the GraphQL-only rule, but many requests per address (cacheable by grid cell per the privacy doc).
- **C. Walk polygon from the street network** (PostGIS/pgRouting over OSM) for the 15-min walk; keeps OTP only for transit commutes.
- **D. Run OTP 2.5.0 beside 2.9.0**: has the endpoint but is unsupported, single-instant only, and a different graph format; not recommended.
- **E. Descope** the transit isochrone for v1.

## Open questions

- Which option (A to E) for walk vs. transit polygons? Single-instant 8:30 Tuesday (the ticket's requirement) versus the 8:00-9:00 window in #80 differ here; R5 handles windows natively, the others would be single-instant or repeated.
- If R5: does it return GeoJSON polygons or grids; memory/graph size for the Cook County crop; license/ops fit with the `-Xmx5G` budget.
- Docs to fix once decided: `ARCHITECTURE.md` §6.1, §12 row 8, §13.1 "OTP2 API" line; `IMPLEMENTATION_PHASES.md` Phase 6.

## Sources

- https://docs.opentripplanner.org/en/v2.9.0/Analysis/
- https://docs.opentripplanner.org/en/v2.9.0/SandboxExtension/
- https://docs.opentripplanner.org/en/v2.9.0/Version-Comparison/
- https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.5.0/docs/sandbox/TravelTime.md
- https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.6.0/doc/user/Changelog.md
- https://github.com/opentripplanner/OpenTripPlanner/pull/5890
- https://github.com/opentripplanner/OpenTripPlanner/pull/4129
- https://github.com/opentripplanner/OpenTripPlanner/issues/4880
- https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/application/src/main/java/org/opentripplanner/framework/application/OTPFeature.java
- https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/application/src/main/resources/org/opentripplanner/apis/gtfs/schema.graphqls
- https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/doc/user/apis/GTFS-GraphQL-API.md
- https://github.com/conveyal/r5 and https://raw.githubusercontent.com/conveyal/r5/master/README.md
- Git tree listings of tags v2.2.0, v2.5.0, v2.6.0, v2.9.0, v2.10.0 and `dev-2.x` via the GitHub API.
