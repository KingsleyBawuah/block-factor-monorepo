# R5 for isochrone polygons: output, hosting, memory, license, and the OTP-grid alternative

Ticket: [#89](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/89) (map: [#79](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/79); decisions: [#80](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/80); background: [#81](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/81), [findings](https://github.com/KingsleyBawuah/block-factor-monorepo/blob/research/otp-isochrones/docs/research/otp-isochrones.md)).
Researched 2026-10-24 against Conveyal R5 `v7.6` and OTP `2.9.0`. Nothing was downloaded or run; the heap numbers below are therefore **not measured** (see Q3).

## Short answer

| Question | Answer |
|---|---|
| 1. Polygons or grids? | **Grids.** R5's single-point endpoint returns a grid of travel times (a "travel-time surface"), as a Conveyal binary file or a GeoTIFF. There is no polygon output in the engine. Conveyal's own UI draws the polygon in the browser with a marching-squares script. We would contour it ourselves. |
| 2. Run as a service? | Maintained and current (v7.6, 2026-08-01), MIT, but **no official Docker image** that I could find and **no stable API**. The only server is the Conveyal Analysis backend, which needs **MongoDB** and is built to be driven by Conveyal's UI. |
| 3. Memory / coexistence | **Unverified.** No primary source gives a heap for a Cook County crop. R5's own examples use 2 GB (Docker, README) and 6 GB (Gradle dev run). Needs a measured spike. |
| 4. License | **Compatible.** R5 is MIT; MIT code can be used in an AGPL-3.0-only app. Running it as a separate service means no linking question at all. MongoDB (SSPL) is a separate dependency to note, not a blocker. |
| 5. Grid of OTP queries | Roughly **10,000 to 180,000 OTP queries per new address** for the 30-min transit polygon (estimate, math below), and almost every new address is a cache miss. Not viable as the primary design. |
| 6. Recommendation | **Use R5, behind a one-day spike that measures heap, latency, and setup effort.** Contour its grid in our own code. If the spike fails, descope the transit polygon for v1 rather than fan out through OTP. |

## Two terms, plainly

- **Travel-time surface (grid):** imagine a sheet of graph paper laid over Chicago. Each square is about 250 m wide. Write in each square how many minutes it takes to get there from your address. That sheet of numbers is the surface. It is a raster (a picture made of numbers), not a shape.
- **Contouring:** the same idea as the elevation lines on a hiking map. Draw a line that connects all squares at exactly 30 minutes and you get the outline of the "reachable in 30 min" area. That outline, closed up, is the isochrone polygon. The usual algorithm is called marching squares: it walks the grid and traces where values cross the threshold.

## Details

### Q1. Polygons or grids

- R5's single-point task is literally called `TravelTimeSurfaceTask`: "returning a surface of travel times to each destination. Several travel times to each destination are returned, representing selected percentiles of all travel times from the chosen origin" ([source](https://raw.githubusercontent.com/conveyal/r5/v7.6/src/main/java/com/conveyal/r5/analyst/cluster/TravelTimeSurfaceTask.java)).
- Its output formats are an enum of `GRID` (Conveyal flat binary) and `GEOTIFF` ("GeoTIFF file for download and use in GIS") (same file). The binary writer's own doc describes the layout: a header plus one integer per pixel, one or more percentiles per pixel ([TimeGridWriter.java](https://raw.githubusercontent.com/conveyal/r5/v7.6/src/main/java/com/conveyal/r5/analyst/cluster/TimeGridWriter.java)).
- I searched the file trees of `v7.0`, `v7.6` and the `dev` branch and found no isochrone, contour, or polygon-output class. An old `IsochroneFeature.java` (which the UI script credits) had commits up to 2023-10-19 but is gone from all three ([git tree](https://api.github.com/repos/conveyal/r5/git/trees/v7.6?recursive=1)).
- The Conveyal user manual says the isochrone is shown in the UI and offered as a GeoJSON download of the multipolygon "currently shown on the map", with GeoTIFF as "the underlying travel time surface" ([docs.conveyal.com/analysis](https://docs.conveyal.com/analysis/)).
- The UI's polygon code lives in the front end, not R5: `lib/utils/jsolines.ts` in `conveyal/analysis-ui` is a marching-squares implementation "ported from" the old R5 Java class ([source](https://github.com/conveyal/analysis-ui/blob/main/lib/utils/jsolines.ts)). That UI repo is MIT but has not had a commit since 2022-01-13 and says it requires R5 v6.4 and that "later versions of R5 no longer work with this repository" ([README](https://github.com/conveyal/analysis-ui/blob/main/README.md)). So the code is usable as a reference to port, not as a working piece.
- The wrappers confirm the "grid first, polygon later" pattern. `r5py` has an `Isochrones` class that builds a regular point grid (default 100 m), asks R5 for travel times to each point, then makes polygons with concave hulls in GeoPandas ([source](https://github.com/r5py/r5py/blob/main/src/r5py/r5/isochrones.py)). The R package `r5r` documents `isochrone()` as sampling road-network nodes ([docs](https://search.r-project.org/CRAN/refmans/r5r/html/isochrone.html)).

**How we would contour.** Fetch the GeoTIFF/grid for one origin, mark squares at or under the cutoff (15 min walk, 30 min transit), then either (a) run marching squares in Go, or (b) load the qualifying squares into PostGIS and merge them with `ST_Union` or shrink-wrap them with [`ST_ConcaveHull`](https://postgis.net/docs/ST_ConcaveHull.html). PostGIS has no raster-to-contour function that I could find in its reference (the `ST_Contour` doc page does not exist), so (a) or (b) is a small piece of code we own. Which is cleaner is a design question for the spike. *(Inferred.)*

**Windows and the median.** The request model carries `date`, `fromTime`, `toTime`, `percentiles`, `walkSpeed`, `maxTripDurationMinutes`, and a grid `zoom` that defaults to 9 ("~250 m by ~250 m") ([AnalysisRequest.java](https://raw.githubusercontent.com/conveyal/r5/v7.6/src/main/java/com/conveyal/analysis/models/AnalysisRequest.java)). Asking for percentile 50 over a 08:00 to 09:00 Tuesday window is the natural fit for decision #80. One caveat: R5's median is across all departure minutes and its random draws, while #80's commute figure is the median of seven OTP itineraries. The two numbers will not match exactly; the polygon and the commute pill are different statistics. R5 describes itself as planning "door-to-door trips at many different departure times in a time window" ([README](https://raw.githubusercontent.com/conveyal/r5/master/README.md)).

**Walk-only polygon.** The request has separate `accessModes`, `directModes`, `egressModes` and `transitModes` fields, and `r5py` supports a WALK transport mode, so a walk-only grid looks possible with the same service. I did not find a doc that states it outright. *(Inferred; verify in the spike.)*

### Q2. How R5 runs as a service today

- **Status:** latest release **v7.6, 2026-08-01**. Prior releases: v7.5 (2025-12-31), v7.4 (2025-04-13), v7.3 (2024-10-26), v7.2 (2024-04-18). Roughly one release every 4 to 8 months. Default branch `dev` last commit 2026-08-01; not archived; 252 open issues ([releases](https://github.com/conveyal/r5/releases)).
- **Artifacts:** each release attaches a self-contained fat JAR (`r5-v7.6-all.jar`). I found no Docker Hub repo named `conveyal/r5`; only third-party images exist ([Docker Hub search](https://hub.docker.com/v2/search/repositories/?query=conveyal)). The repo's `docker-compose.yml` has a commented-out `ghcr.io/conveyal/r5:latest`, which I could not confirm exists (anonymous registry access was denied). Treat "official image" as **unverified**, and plan to build our own.
- **The repo's `Dockerfile` looks stale:** it uses `FROM openjdk:11`, `JVM_HEAP_GB=2`, port 7070, while `build.gradle` sets a Java **21** toolchain and CI uses JDK 21 ([Dockerfile](https://github.com/conveyal/r5/blob/dev/Dockerfile), [build.gradle](https://raw.githubusercontent.com/conveyal/r5/v7.6/build.gradle), [CI](https://github.com/conveyal/r5/blob/dev/.github/workflows/gradle.yml)). A Java 21 runtime is the safe assumption.
- **What "the server" is:** `com.conveyal.analysis.BackendMain` (entry point for a "local (non-cloud) Conveyal Analysis server") ([BackendMain.java](https://raw.githubusercontent.com/conveyal/r5/v7.6/src/main/java/com/conveyal/analysis/BackendMain.java)). It connects to **MongoDB** on startup and, with `offline=true`, starts one in-process worker ([analysis.properties.template](https://raw.githubusercontent.com/conveyal/r5/v7.6/analysis.properties.template), [LocalWorkerLauncher.java](https://raw.githubusercontent.com/conveyal/r5/v7.6/src/main/java/com/conveyal/analysis/components/LocalWorkerLauncher.java)). I found no standalone `R5Main` entry point in the `dev` tree, and `docs/README.r5.md` still talks about Maven, so that file is out of date.
- **API surface:** the single-point call is `POST /api/analysis` on the backend, which proxies to a worker's `/single` endpoint ([BrokerController.java](https://raw.githubusercontent.com/conveyal/r5/v7.6/src/main/java/com/conveyal/analysis/controllers/BrokerController.java)). The request must reference a `regionId`, `projectId`, `bundleId` and `workerVersion`, all of which are database records created through other backend endpoints (for example `POST /api/bundle` for uploading OSM and GTFS) ([BundleController.java](https://raw.githubusercontent.com/conveyal/r5/v7.6/src/main/java/com/conveyal/analysis/controllers/BundleController.java)). Send `Accept: image/tiff` to get a GeoTIFF; otherwise you get the binary grid.
- **No stable contract:** "R5 does not currently expose a stable programming interface ('API' or 'SDK'). As we release new features, previous functions and data types may change." Conveyal also "does not provide technical support for third-party deployments" ([README](https://raw.githubusercontent.com/conveyal/r5/master/README.md)). We would be scripting an internal API, so pin the version and wrap it behind one small Go client.
- **Lighter route, worth testing in the spike:** `r5py` loads R5 directly from an OSM `.pbf` plus GTFS zips, with no MongoDB or backend, and returns polygons (see Q1). It downloads a fork JAR `v7.6-r5py` ([config](https://github.com/r5py/r5py/blob/main/src/r5py/util/classpath.py)), is current (v1.1.7, 2026-06-29), and is licensed "GPL-3.0-or-later or MIT" ([pyproject](https://github.com/r5py/r5py/blob/main/pyproject.toml)). Cost: a Python runtime plus a JVM in the same sidecar, a wrapper API we must write, and a polygon method (concave hull) that is cruder than marching squares. A 100 m point grid over the whole Cook County extent may also be heavy per request; unmeasured.

### Q3. Memory and coexistence with OTP

- **No primary source gives a heap for a Cook County crop plus CTA.** I did not find any R5 sizing table. Conveyal's input guidance says OSM files over 500 MB "may be rejected" on its hosted platform and that extracts should not "extend unnecessarily far" beyond the service area ([prepare-inputs](https://docs.conveyal.com/prepare-inputs)); it gives no memory figure.
- **Defaults in R5's own files:** Dockerfile `JVM_HEAP_GB=2`; README example `java -Xmx2g`; Gradle `runBackend` uses `-Xmx6G` ([README](https://raw.githubusercontent.com/conveyal/r5/master/README.md), [build.gradle](https://raw.githubusercontent.com/conveyal/r5/v7.6/build.gradle)). `r5py` defaults to 80% of system RAM and says R5 is memory hungry ([config doc](https://r5py.readthedocs.io/stable/user-guide/user-manual/configuration.html)). These are configuration defaults, not measurements for our data.
- **Our budget:** OTP is capped at `-Xmx5G` in `docker-compose.yml`. `free -h` on this machine at research time showed 30 GiB total and about 21 GiB available (a snapshot of the dev box, not necessarily the target home server).
- **Working assumption (unverified):** one JVM at 3 to 4 GB plus a small MongoDB should fit next to OTP's 5 GB, but I cannot back the 3 to 4 GB with a source. The graph build and the first request are the likely peaks (the backend builds the network on first use and replies 202 "Starting routing server" until ready, per `BrokerController`).
- **Spike to settle it:** build the Cook County crop + CTA network once, run a few single-point requests at `-Xmx2g`, `3g`, `4g`, and record peak RSS, build time, and request time. Cap the container with a memory limit so it cannot starve OTP.

### Q4. License compatibility with an AGPL-3.0-only public app

- R5 is **MIT** ([LICENSE](https://github.com/conveyal/r5/blob/dev/LICENSE); the GitHub API reports SPDX `MIT`). The FSF lists the Expat (MIT) license as "a lax, permissive non-copyleft free software license, compatible with the GNU GPL" ([FSF license list](https://www.gnu.org/licenses/license-list.html#Expat)). AGPLv3 is the GPLv3 variant that adds a network-use clause, so an MIT component can sit inside an AGPL project. *(The step from "GPL-compatible" to "AGPL-compatible" is my reading of the FSF's framing; the AGPL text itself is at [gnu.org/licenses/agpl-3.0](https://www.gnu.org/licenses/agpl-3.0.html). This is not legal advice.)*
- We would call R5 over HTTP as a separate process, so there is no linking or derivative-work question. If we redistribute its JAR or an image containing it, keep the MIT copyright and permission notice.
- **Related components:** `r5py` is "GPL-3.0-or-later or MIT" (pick MIT or GPL, both fine). R5 bundles GeoTools and other Java libraries; because we would not link them into our Go binary, they do not affect our license (not audited).
- **MongoDB flag:** the backend needs MongoDB. MongoDB Community Server is under the SSPL, whose copyleft applies "only when you are offering the functionality of MongoDB ... to third parties as a service", and "there is no copyleft condition for other SaaS applications that use MongoDB as a database" ([MongoDB SSPL FAQ](https://www.mongodb.com/legal/licensing/server-side-public-license/faq)). So it is fine for us, but it is a non-OSI dependency in a public AGPL project; worth a line in `THIRD_PARTY_REQUIREMENTS.md`. Whether a drop-in alternative to MongoDB works with R5's legacy driver is unverified.

### Q5. The alternative: many point-to-point OTP GraphQL queries, contoured in PostGIS

OTP 2.9.0's routing query is `planConnection`, one origin to one destination at a time, with `dateTime: { earliestDeparture }` and an optional `searchWindow`. There is no one-to-many query ([schema.graphqls](https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/application/src/main/resources/org/opentripplanner/apis/gtfs/schema.graphqls)). The R5 README makes the same point: R5 is "designed for one-to-many ... calculations ... offering substantially better performance than repeated calls to older tools that provide one-to-one routing results".

**Query count (my arithmetic, assumptions stated):**

- Assume the 30-minute transit reach is bounded by a ~20 km radius. This is an assumption, not a measured value. That gives a 40 km x 40 km box.
- Sample cell size sets the count: 1 km cells = 1,600 queries; 500 m = 6,400; 250 m (R5's default cell) = 25,600.
- Decision #80 uses departures every 10 min from 8:00 to 8:50 (seven departures, or seven plus the 9:00 edge). To match the median-of-window rule, each cell needs one query per departure: **about 11,000 (1 km) to 180,000 (250 m) queries per address.** With a single 8:30 departure instead, 1,600 to 25,600.
- Fewer queries are possible with tricks (coarse grid then refine near the boundary, skip cells far from any stop). Another option is querying only transit stops, then adding a walk radius from each reached stop. These are untested ideas. *(Inferred.)*

**Latency:** OTP publishes no per-query timing. If a query took 50 ms (assumption), 11,000 queries is about 9 minutes on one thread, or under a minute on 12 busy cores, and all of it competes with the commute and benchmark pills for the same 5 GB OTP. The OTP config key `server.apiProcessingTimeout` and `searchWindow` exist to bound each query ([router config](https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/doc/user/RouterConfiguration.md)), but they do not reduce the count. Must be measured to be trusted.

**Cacheability:** the privacy doc keys OTP results by "origin/dest snapped to 100m grid + mode + window" with a 7-day TTL ([PRIVACY_AND_DATA_HANDLING.md](../PRIVACY_AND_DATA_HANDLING.md)). A polygon cache keyed on the origin's 100 m cell would work, but a hit needs another user to score an address in the same 100 m cell in the same week. Practically every new address pays the full cost; only repeat lookups are free. Pre-computing for every 100 m cell in Cook County is far too many (county area is thousands of km2) without a pruning scheme.

**Contouring in PostGIS** is straightforward once cells are classified: build the grid with [`ST_SquareGrid`](https://postgis.net/docs/ST_SquareGrid.html), keep cells under the cutoff, then `ST_Union` or `ST_ConcaveHull`. The hard part is not PostGIS; it is producing the travel times.

**Consequence:** the OTP fan-out keeps one engine and the "GraphQL only" rule, but it uses the wrong tool at one to two orders of magnitude more work, and it gives no clean way to get the walk polygon.

### Q6. Recommendation and tradeoffs

**Recommend R5 as the single polygon engine** (15-min walk and 30-min transit from one service), with a **time-boxed spike first**. Reasons: it returns a whole reachability grid in one request, supports the 08:00 to 09:00 window natively, is MIT, and is actively released. OTP's own maintainers point to it for this job ([#81 findings](https://github.com/KingsleyBawuah/block-factor-monorepo/blob/research/otp-isochrones/docs/research/otp-isochrones.md)).

| Option | Upside | Downside |
|---|---|---|
| **A. R5 backend + MongoDB sidecar, we contour** | One request per address; walk and transit; window and median built in | New JVM + MongoDB; internal unstable API; ~250 m cells; heap unmeasured; we own the contouring code |
| **B. R5 via r5py sidecar** | No MongoDB; polygons out of the box | Python + JVM in one container; cruder polygon method; per-request cost unmeasured |
| **C. OTP grid fan-out** | One engine; GraphQL only | ~10k to 180k queries per address; load on OTP; poor cache hit rate |
| **D. Descope transit polygon (keep walk only)** | Zero new infrastructure | Breaks decision #80 item 3 for transit |

Spike pass criteria (suggested): peak heap with Cook County + CTA at or below ~4 GB beside OTP; a single-point request under the isochrone endpoint's latency budget (to be set in the next ticket); an end-to-end script that goes OSM + GTFS to a GeoTIFF to a contoured GeoJSON polygon. If A's MongoDB/backend plumbing is too heavy, try B before falling back to D.

## Open questions

1. Real heap and latency for Cook County + CTA (and Metra) in R5: needs the spike.
2. Is the backend path (A) or the `r5py` path (B) easier to operate headlessly? Can the backend be scripted without Conveyal's UI cleanly (region, project, bundle records)?
3. What latency budget does the isochrone endpoint get? #80 sets ~5 s for OTP commute answers but says only "map hides layers with a small notice" on isochrone errors.
4. Is ~250 m cell resolution acceptable for the walk polygon, or do we want a finer zoom (more memory)? Does a pgRouting-over-OSM walk polygon beat R5 for walk only?
5. Does R5's window-median grid conflict with the OTP-based commute pill number enough to confuse users?
6. Follow-ups outside this ticket: add "travel-time surface" and "contouring" to `docs/GIS_GLOSSARY.md`; add MongoDB (SSPL) and R5 (MIT) to `docs/THIRD_PARTY_REQUIREMENTS.md` if we proceed.

## Sources

- R5 repo and license: https://github.com/conveyal/r5 , https://github.com/conveyal/r5/blob/dev/LICENSE
- R5 releases: https://github.com/conveyal/r5/releases
- R5 README: https://raw.githubusercontent.com/conveyal/r5/master/README.md
- R5 source at v7.6: `TravelTimeSurfaceTask.java`, `TimeGridWriter.java`, `AnalysisRequest.java`, `BrokerController.java`, `BundleController.java`, `BackendMain.java`, `LocalWorkerLauncher.java`, `build.gradle`, `analysis.properties.template` under https://raw.githubusercontent.com/conveyal/r5/v7.6/
- R5 Dockerfile, docker-compose, CI: https://github.com/conveyal/r5/blob/dev/Dockerfile , https://github.com/conveyal/r5/blob/dev/docker-compose.yml , https://github.com/conveyal/r5/blob/dev/.github/workflows/gradle.yml
- R5 file tree (v6.0, v7.0, v7.6): https://api.github.com/repos/conveyal/r5/git/trees/v7.6?recursive=1
- Conveyal docs: https://docs.conveyal.com/analysis/ , https://docs.conveyal.com/analysis/methodology , https://docs.conveyal.com/prepare-inputs
- Conveyal UI isochrone code and README: https://github.com/conveyal/analysis-ui/blob/main/lib/utils/jsolines.ts , https://github.com/conveyal/analysis-ui/blob/main/README.md
- r5py: https://github.com/r5py/r5py , https://github.com/r5py/r5py/blob/main/src/r5py/r5/isochrones.py , https://r5py.readthedocs.io/stable/user-guide/user-manual/configuration.html
- r5r isochrone: https://search.r-project.org/CRAN/refmans/r5r/html/isochrone.html
- Docker Hub search: https://hub.docker.com/v2/search/repositories/?query=conveyal
- FSF license list: https://www.gnu.org/licenses/license-list.html ; AGPL text: https://www.gnu.org/licenses/agpl-3.0.html
- MongoDB SSPL FAQ: https://www.mongodb.com/legal/licensing/server-side-public-license/faq
- OTP 2.9.0 GraphQL schema, router config: https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/application/src/main/resources/org/opentripplanner/apis/gtfs/schema.graphqls , https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/doc/user/RouterConfiguration.md , https://raw.githubusercontent.com/opentripplanner/OpenTripPlanner/v2.9.0/doc/user/RouteRequest.md
- PostGIS: https://postgis.net/docs/ST_ConcaveHull.html , https://postgis.net/docs/ST_SquareGrid.html
- Repo files: `docker-compose.yml`, `docs/PRIVACY_AND_DATA_HANDLING.md`
