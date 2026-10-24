# OTP version pin and memory (Phase 6 research, ticket #83)

Researched 2026-10-24 against primary sources (OTP docs and source at tag `v2.10.0`, GitHub releases, Docker Hub API). Anything I could not verify is marked **unverified** or **estimate**.

## Short answer

- **Pin `opentripplanner/opentripplanner:2.10.0`**, not `2.9.0`. 2.10.0 is the latest stable release (2026-09-09); 2.9.0 is the one before it. Nothing in the 2.10.0 changelog breaks a setup like ours (no custom config, GTFS GraphQL API only). Moving now is free because no graph has been built yet.
- **Memory (estimate, low confidence):** a CTA-only graph from a Cook County crop most likely builds in about 2-4 GB of heap and serves in about 1-3 GB, so the 5 GB `-Xmx` cap should fit. No one has published a Chicago measurement, so the first real build must measure it (see "How to confirm").
- **Yes, rebuild `graph.obj` on upgrade.** OTP refuses to load a graph whose serialization id differs from the running version. 2.9.0 uses id 249; 2.10.0 uses id 277.
- **Do not use `--build --serve` in one step.** Use `--build --save`, then `--load`. The one-step mode has a known bug and the maintainers have said they will remove it.

## 1. Which version

| Item | Finding | Source |
|---|---|---|
| Latest stable | 2.10.0, published 2026-09-09. Previous: 2.9.0 (2026-03-18), 2.8.1 (2025-09-22). No pre-releases flagged. | [Releases](https://github.com/opentripplanner/OpenTripPlanner/releases) |
| Docker tags | `2.9.0` and `2.10.0` both exist (amd64 + arm64). `latest` is a different digest from `2.10.0`: it is a moving dev snapshot (tags like `2.11.0_2026-10-22T22-58`). Never pin `latest`. | [Docker Hub tags API](https://hub.docker.com/v2/repositories/opentripplanner/opentripplanner/tags?page_size=100) |
| Java | 2.9.0 and 2.10.0 need Java 25 (2.9.0 changelog: "Update to Java 25", #7246). The image bundles its own JVM, so this only matters if you run the JAR directly. | [Changelog](https://docs.opentripplanner.org/en/v2.10.0/Changelog/), [Basic tutorial](https://docs.opentripplanner.org/en/v2.10.0/Basic-Tutorial/) |
| Image size | 2.9.0 about 273 MB (amd64); 2.10.0 about 294 MB. | Docker Hub tags API (above) |

### Changes since 2.9.0 (all are 2.10.0)

No changelog entry is labelled "breaking". Items that could matter in principle:

- **Config renames and moves**, none of which we use today: `includeOsmSubwayEntrances` renamed `includeOsmStationEntrances` (#7170); per-network GBFS settings moved into `otp-config.json` (#7886); "Cleanup regular transfer build parameters" (#7425, entry does not say which). OTP only warns about unknown config keys by default, so a stale key will not stop startup unless `--abortOnUnknownConfig` is used. ([Migrating configuration](https://docs.opentripplanner.org/en/v2.10.0/Migrating-Configuration/))
- **Removals** (none affect us): `GraphVisualizer`, vehicle routing optimization, relation route name application, safety normalizer, `UNDEFINED` alert severity, street notes (#7526, #7741, #7539, #7634, #7883, #7959).
- **Performance:** "Improvements in routing and graph build performance" is a headline item (14 PRs). Memory-specific PRs (#6724, #6752) landed earlier, in 2.8.0. ([Changelog](https://docs.opentripplanner.org/en/v2.10.0/Changelog/))
- **GTFS GraphQL API:** the 2.10.0 changelog adds queries (`alertConnection`, GTFS feed version, replacement-mode filter) and fixes; I found no removals. The same changelog (2.7.0 section) "un-deprecates `planConnection`, deprecates `plan`" in the GTFS API, so the Go client should use `planConnection`. This is outside the ticket's question, so check it against the schema when writing the client. ([Changelog](https://docs.opentripplanner.org/en/v2.10.0/Changelog/))

### Is it safe to be on the newest release?

2.10.0 is about six weeks old. I did not find a 2.10.x patch release. I did not review open 2.10.0 bug reports exhaustively (**unverified** that none affect Cook County + CTA). If a problem appears, 2.9.0 is the fallback and needs a graph rebuild to switch back (different serialization id).

## 2. Memory

### What the docs say

- Memory depends on area, transit density and whether OSM, elevation, and transit are included; the range is "less than one GB to more than 100 GB". Finland with all data is "a bit over 10 GB"; Germany 95 GB. ([System requirements](https://docs.opentripplanner.org/en/v2.10.0/System-Requirements/))
- The build loads all GTFS and OSM into memory, so it "can require significantly more memory than just running a server". The docs suggest building on a bigger machine and copying `graph.obj` to a smaller one. ([Basic tutorial](https://docs.opentripplanner.org/en/v2.10.0/Basic-Tutorial/))
- Portland TriMet: "at least 1GB". Official container quick-start (Berlin, VBB feed): `-Xmx8g` for both build and serve. ([Basic tutorial](https://docs.opentripplanner.org/en/v2.10.0/Basic-Tutorial/), [Container image](https://docs.opentripplanner.org/en/v2.10.0/Container-Image/))
- Give the JVM headroom: "Without sufficient free memory OTP can grind to a halt." ([Basic tutorial](https://docs.opentripplanner.org/en/v2.10.0/Basic-Tutorial/))
- Cropping OSM to the analysis area is explicitly recommended; oversized OSM raises build time and complexity. Keep only the crop in the data directory, or OTP loads both files. ([Preparing OSM](https://docs.opentripplanner.org/en/v2.10.0/Preparing-OSM/), [Basic tutorial](https://docs.opentripplanner.org/en/v2.10.0/Basic-Tutorial/))

### Published measurements I could find

None are for Chicago or for OTP 2.10. Treat them as order-of-magnitude only.

| Data point | Version | Source |
|---|---|---|
| Helsinki-area build: 9 GB down to 6 GB after the shape-points change; another feed with 60 million shape points: 21 GB down to 9 GB. Shipped in 2.8.0. | 2.8.0 | [PR #6752](https://github.com/opentripplanner/OpenTripPlanner/pull/6752) |
| Dropping the raw OSM database early during build cuts peak memory for large graphs (no numbers given). | 2.8.0 | [PR #6724](https://github.com/opentripplanner/OpenTripPlanner/pull/6724) |
| Berlin-Brandenburg GTFS + OSM build hit `OutOfMemoryError`; the reporter fixed it with `JAVA_OPTS=-Xmx6g` and a container limit of 8g. Heap of the failing run not stated. | 2.3.0 (old) | [Issue #5235](https://github.com/opentripplanner/OpenTripPlanner/issues/5235) |
| NYC (11 GTFS feeds + OSM): about 4 GB used under `-Xmx8G`, finished with `-Xmx3G` (slower). Portland: 1.4 GB used under `-Xmx2G`. Wiki says it is outdated. | OTP 0.10 era, 2014 | [Wiki: GraphBuilder](https://github.com/opentripplanner/OpenTripPlanner/wiki/graphbuilder) |

### Inputs for our case

- CTA GTFS zip (`google_transit.zip`): 68,738,293 bytes (about 66 MiB), last modified 2026-09-14 (HTTP HEAD, https://www.transitchicago.com/downloads/sch_data/google_transit.zip).
- Geofabrik Illinois `.osm.pbf`: 362,298,426 bytes (about 345 MB) on 2026-10-23 ([Geofabrik Illinois](https://download.geofabrik.de/north-america/us/illinois.html)). I did not download it, so the size of a Cook County crop is **unmeasured**.

### Estimate for BlockFactor (not measured)

| Phase | Estimate | Reasoning |
|---|---|---|
| Build (`--build --save`) | about 2-4 GB heap | Chicago is mid-sized next to the docs' reference points (Portland ">= 1 GB" at the low end; Helsinki 6 GB for a far larger multi-modal feed). The crop removes most of the 345 MB statewide file. 2.8.0+ build memory fixes help. |
| Serve (`--load`) | about 1-3 GB heap | Serving holds the graph and routing indexes but not the raw OSM/GTFS inputs. No realtime feeds, no elevation. |
| Cap | `-Xmx5G` should fit both | Confidence low. Adding Metra, or any elevation data, would push the build up. |

**Risks that would break the estimate:** adding Metra (Cook-only crop limits the OSM side, but its GTFS shapes still count), turning on elevation (DEM), or leaving the full Illinois `.pbf` in the data directory next to the crop.

### How to confirm (cheap, do on first build)

1. Build in a one-off container with a generous heap first (for example `-Xmx8G`), and note whether it completes. If it does, repeat at `-Xmx5G`; an `OutOfMemoryError: Java heap space` in the log means the cap is too small for the build.
2. Watch `docker stats` during build and serve for peak container memory. The `-Xmx` limit caps heap only; total process memory is higher (unverified size; measure it).
3. Record the `Graph loaded. |V|=... |E|=...` log line and the `graph.obj` size in the PR that adds the build script.

Because the build and serve run as separate processes, the one-off build can use a larger heap than the always-on server, if the host has the RAM (host RAM is not recorded in `docs/ARCHITECTURE.md`; **unverified**).

## 3. Does `graph.obj` need rebuilding after an upgrade?

**Yes, whenever the serialization version id changes.**

- Each `graph.obj` starts with a header holding the OTP serialization version id. On `--load`, OTP compares it to its own and aborts with "The graph file is incompatible with this version of OTP" if they differ. ([`SerializedGraphObject.java`](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.10.0/application/src/main/java/org/opentripplanner/routing/graph/SerializedGraphObject.java), [Configuration: serialization id](https://docs.opentripplanner.org/en/v2.10.0/Configuration/))
- The id is set in the root `pom.xml` per release: **2.8.0 and 2.8.1: 203; 2.9.0: 249; 2.10.0: 277** ([2.10.0 pom](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.10.0/pom.xml), [2.9.0 pom](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/pom.xml), [2.8.1 pom](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.8.1/pom.xml)). So patch releases can share a graph; 2.9.0 to 2.10.0 cannot.
- Check without running OTP: `head -c 29 graph.obj` prints the header; `--serializationVersionId` prints the running version's id. (The docs' example id of 7 is stale.)
- Corollary for the repo: record the OTP version next to `graph.obj` and rebuild on every version bump. Also, `graph.obj` bakes in `build-config.json`, so config changes need a rebuild too. ([Configuration](https://docs.opentripplanner.org/en/v2.10.0/Configuration/))

## 4. Commands and data directory (official Docker image)

### Directory layout

Mount a volume at **`/var/opentripplanner/`**. The image's entrypoint passes that path as the base directory, and OTP scans it for inputs and for `graph.obj`. ([Container image](https://docs.opentripplanner.org/en/v2.10.0/Container-Image/), [`docker-entrypoint.sh`](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.10.0/application/src/main/jib/docker-entrypoint.sh))

```
/var/opentripplanner/
  chicago-gtfs.zip       # name must end in .zip AND contain "gtfs"
  cook-county.osm.pbf    # must end in .pbf; keep only the crop here
  build-config.json      # optional, baked into the graph
  router-config.json     # optional, runtime routing options
  otp-config.json        # optional, feature flags
  graph.obj              # written by --save, read by --load
```

File-type detection is by extension plus name: GTFS must end `.zip` and contain `gtfs`; OSM must end `.pbf` ([Configuration: base directory](https://docs.opentripplanner.org/en/v2.10.0/Configuration/)). `chicago-gtfs.zip` already qualifies. All four JSON files are optional.

### Commands

The image's entrypoint is `java $JAVA_OPTS ... /var/opentripplanner/ $@`, so flags go straight after the image name, and heap is set with `JAVA_OPTS` (the one `docker-compose.yml` already uses). The official docs show `JAVA_TOOL_OPTIONS` instead; both are JVM-level settings and either works. (A third-party issue also shows `JAVA_OPTS` working on the image: [#5235](https://github.com/opentripplanner/OpenTripPlanner/issues/5235).)

```bash
# 1. Build once and save graph.obj into the volume
docker run --rm -e JAVA_OPTS='-Xmx5G' -v otp-data:/var/opentripplanner \
  opentripplanner/opentripplanner:2.10.0 --build --save

# 2. Serve (--load implies --serve); listens on 8080 inside the container
docker run -d -p 4003:8080 -e JAVA_OPTS='-Xmx5G' -v otp-data:/var/opentripplanner \
  opentripplanner/opentripplanner:2.10.0 --load
```

Flags, from `CommandLineParameters.java` ([source](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.10.0/application/src/main/java/org/opentripplanner/standalone/config/CommandLineParameters.java)):

| Flag | Meaning |
|---|---|
| `--build` | Build street + transit graph from the input files |
| `--save` | Write `graph.obj` (only takes effect when a transit build runs) |
| `--load` | Read `graph.obj` and serve it; `--serve` is implied |
| `--serve` | Run the API server (only useful after a build in the same process; see below) |
| `--buildStreet` / `--loadStreet` | Build and save only the street graph (`streetGraph.obj`), then layer transit on later. Useful if CTA GTFS refreshes quarterly but the street data does not. |
| `--port`, `--bindAddress`, `--maxThreads` | Server options; default port is 8080 |
| `--abortOnUnknownConfig` | Fail startup on unknown config keys (good for build, risky for serve) |
| `--serializationVersionId`, `--version` | Print and exit |

The GTFS GraphQL endpoint is `/otp/gtfs/v1` on that port ([GTFS GraphQL API](https://docs.opentripplanner.org/en/v2.10.0/apis/GTFS-GraphQL-API/)).

### Why not `--build --serve`

Open issue [#7948](https://github.com/opentripplanner/OpenTripPlanner/issues/7948): in the one-step mode, every transit search fails because the routing data is built before the graph is populated. Maintainers commented on 2026-10-15 and 2026-10-19 that the feature will be removed. Build, save, then load.

## 5. Things the repo should change (flagged, not changed here)

1. `docker-compose.yml` pins `2.9.0`; recommend `2.10.0`.
2. The `otp` service has **no `command:`**. The docs say at least one of `--load`, `--loadStreet`, `--build`, `--buildStreet` is required, so as written the container would not start usefully (exact failure mode **unverified**; I did not run it). It needs `command: ["--load"]`, plus a separate one-off build step.
3. `docs/ARCHITECTURE.md` §6.2 says `otp --build data/`; the real invocation is `--build --save` and then `--load`. Its "boots in < 15 seconds" claim is **unverified** (no source found).
4. `docs/IMPLEMENTATION_PHASES.md` mentions pulling `illinois.osm.pbf`. If the full file lands in the OTP data directory beside the crop, OTP loads both ([Basic tutorial](https://docs.opentripplanner.org/en/v2.10.0/Basic-Tutorial/)). Keep the full file out of that volume.

## Open questions

- Actual build and serve heap for CTA + Cook crop (needs one real build; see "How to confirm").
- Size of the Cook County crop in MB (not measured).
- RAM available on the home server for a one-off larger build heap.
- Whether Metra is added (decided in #80 as conditional on terms and memory); it changes the build estimate.
- Isochrone memory cost and API: out of scope here, covered by the isochrone ticket.

## Sources

- OTP 2.10.0 docs: [Changelog](https://docs.opentripplanner.org/en/v2.10.0/Changelog/), [Container image](https://docs.opentripplanner.org/en/v2.10.0/Container-Image/), [Basic tutorial](https://docs.opentripplanner.org/en/v2.10.0/Basic-Tutorial/), [System requirements](https://docs.opentripplanner.org/en/v2.10.0/System-Requirements/), [Configuration](https://docs.opentripplanner.org/en/v2.10.0/Configuration/), [Migrating configuration](https://docs.opentripplanner.org/en/v2.10.0/Migrating-Configuration/), [Preparing OSM](https://docs.opentripplanner.org/en/v2.10.0/Preparing-OSM/), [GTFS GraphQL API](https://docs.opentripplanner.org/en/v2.10.0/apis/GTFS-GraphQL-API/)
- OTP source at `v2.10.0`: [pom.xml](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.10.0/pom.xml), [`docker-entrypoint.sh`](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.10.0/application/src/main/jib/docker-entrypoint.sh), [`SerializedGraphObject.java`](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.10.0/application/src/main/java/org/opentripplanner/routing/graph/SerializedGraphObject.java), [`CommandLineParameters.java`](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.10.0/application/src/main/java/org/opentripplanner/standalone/config/CommandLineParameters.java)
- [GitHub releases](https://github.com/opentripplanner/OpenTripPlanner/releases), [Docker Hub tags](https://hub.docker.com/r/opentripplanner/opentripplanner/tags)
- PRs and issues: [#6724](https://github.com/opentripplanner/OpenTripPlanner/pull/6724), [#6752](https://github.com/opentripplanner/OpenTripPlanner/pull/6752), [#5235](https://github.com/opentripplanner/OpenTripPlanner/issues/5235), [#7948](https://github.com/opentripplanner/OpenTripPlanner/issues/7948)
- [OTP wiki: GraphBuilder](https://github.com/opentripplanner/OpenTripPlanner/wiki/graphbuilder) (outdated, 2014)
- Input sizes: [Geofabrik Illinois](https://download.geofabrik.de/north-america/us/illinois.html), CTA `google_transit.zip` (HTTP HEAD)
