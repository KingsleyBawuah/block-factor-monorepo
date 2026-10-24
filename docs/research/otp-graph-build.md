# OTP graph build and serve: measured on this machine

Ticket: #85 (Wayfinder map #79). Measured 2026-10-24 with `opentripplanner/opentripplanner:2.10.0` (Java 25.0.4, graph serialization id 277) on a 12-core, 30 GB Linux box. Every number below was measured unless marked "unverified".

## Bottom line

- A 5 GB heap is enough to build and to serve, with or without Metra.
- Building needs far less than 5 GB. It still passes at 3 GB (with 4 full garbage collections) and fails at 2 GB (out of memory).
- Adding Metra costs almost nothing: +7 MB on `graph.obj`, same build time, no extra memory we could measure.
- Serving boots in about 9 to 10 seconds from `docker run` to the first HTTP 200, which meets the < 15 s claim.

## Results

Inputs: Illinois Geofabrik extract (346 MB) cropped to a Cook County bounding box (136 MB), CTA GTFS (66 MB, feed built 2026-09-14), Metra GTFS (0.6 MB, unmodified, files dated 2026-10-20).

### Build (`--build --save`)

| | CTA only | CTA + Metra |
|---|---|---|
| Heap cap 8 GB: wall-clock time | 120 s | 112 s |
| Heap cap 8 GB: peak heap in use (before a GC) | 4.6 GB | 4.6 GB |
| Heap cap 8 GB: peak live data (after a GC) | 3.9 GB | 3.4 GB |
| Heap cap 8 GB: container memory peak (docker stats) | 5.25 GiB | 5.20 GiB |
| Heap cap 5 GB: result | succeeds, 114 s, 0 full GCs | succeeds, 114 s, 0 full GCs |
| Heap cap 5 GB: peak heap in use | 4.6 GB | 4.7 GB |
| Heap cap 4 GB | not run | succeeds, 116 s, 0 full GCs |
| Heap cap 3 GB | not run | succeeds, 117 s, 4 full GCs |
| Heap cap 2 GB | not run | fails, OutOfMemoryError after 59 s |
| `graph.obj` size | 380 MB | 387 MB |
| Stops / patterns | 10,981 / 788 | 11,222 / 994 |
| Vertices / edges | 1,560,590 / 4,403,163 | 1,561,201 / 4,404,293 |
| `IsolatedStop` issues | 0 | 46 (non-fatal) |

The heap grows lazily toward its cap, so "peak heap in use" at 8 GB and 5 GB looks the same: the JVM only used what it needed and the 8 GB run simply never hit the wall. The real floor is the 3 GB / 2 GB result.

### Serve (`--load`, `-Xmx5G`)

| | CTA only | CTA + Metra |
|---|---|---|
| Boot: JVM start to "ready for routing" log line | 7.7 s | 7.7 s |
| Boot: `docker run` to first HTTP 200 on `/otp/` | 9.8 s | 9.9 s |
| Steady heap (live data right after a GC, idle) | about 1.0 to 1.1 GB | about 1.0 to 1.4 GB |
| Heap in use before a GC (peak seen) | 2.5 GB | 2.0 GB |
| Container memory (docker stats, after queries) | 2.85 GiB | 2.35 GiB |

The steady figures are noisy (G1 collects in cycles). Treat both graphs as "about 1 to 1.5 GB live".

### Queries (`planConnection`, Tuesday 2026-10-27, earliest departure 08:30 -05:00, `first: 1`)

| Query | CTA only | CTA + Metra |
|---|---|---|
| Lincoln Square (41.9667, -87.6885) to Clark/Lake (41.8858, -87.6310) | routes: walk, bus 49, walk, Blue Line, walk; 42 min. First call 1.32 s, then 0.18 to 0.32 s | same itinerary. First call 1.12 s, then 0.19 to 0.27 s |
| Hyde Park (41.7995, -87.5900) to Clark/Lake | routes (bus 28, Brown Line), 38 min; 0.13 to 0.18 s | routes via Metra Electric (`ME`), 32 min; 0.13 s |
| Ravenswood (41.9677, -87.6745) to Ogilvie (41.8827, -87.6411) | routes by CTA Brown Line only, 50 min; 0.18 s | routes on Metra `UP-N`, 24 min; 0.16 to 0.19 s |
| Wheaton (41.8661, -88.1070, DuPage County) to Ogilvie | `NO_STOPS_IN_RANGE`; 0.06 s | routes on Metra `UP-W`, 70 min; 0.09 s |
| Origin Milwaukee (43.0389, -87.9065) to Clark/Lake | `OUTSIDE_BOUNDS`; 0.015 s | `OUTSIDE_BOUNDS`; 0.014 s |
| Destination Milwaukee, origin Lincoln Square | `OUTSIDE_BOUNDS`; 0.009 s | not run |
| Destination Waukegan (42.3636, -87.8448, north of the crop) | not run | `LOCATION_NOT_FOUND`; 0.023 s |

Outside-area errors come back as HTTP 200 with an empty `edges` list and `routingErrors`, not a GraphQL `errors` entry. Verbatim:

```json
{"data":{"planConnection":{"edges":[],"routingErrors":[{"code":"OUTSIDE_BOUNDS","description":"Trip is not possible. You might be trying to plan a trip outside the map data boundary."}]}}}
```

`LOCATION_NOT_FOUND` text: "Destination is unknown.  Can you be a bit more descriptive?". `NO_STOPS_IN_RANGE` text: "The location was found, but no stops could be found within the search radius."

## Things worth knowing

- The Cook County bounding box is a rectangle, so it also includes parts of DuPage, Will, Lake, Kane and McHenry counties. Wheaton (DuPage) has street data, so Metra UP-W works there even though it is outside Cook County itself. The "Cook only" boundary is therefore fuzzy; a true county polygon would need `osmium extract --polygon` (not tried).
- Metra `IsolatedStop`: 46 stops fall outside the crop and are left unlinked. This matches the roughly 47 expected in #84.
- Other build issues are the same in both graphs and non-fatal. Notable counts (CTA + Metra): `InterliningTeleport` 32,263, `VertexWithoutEdges` 12,851, `GraphIslandCAR` 11,447, `GraphIslandWALK` 5,195, `PrunedStopIsland` 192, `StopNotLinkedForTransfers` 145 (21 CTA only), `TurnRestrictionBad` 1,048, `HopSpeedSlow` 386.
- OTP logs a warning that "Limiting transit service days to time period: [2025-10-24, 2029-10-25)". Unverified whether this matters for feeds that start later.
- Metra's calendar uses weekly service ids that end 2026-11-15 and 2026-12-31 (`A1B`); this feed only covers about 9 weeks from its publish date. CTA's runs to 2026-11-30. The expiry warning in #80 will fire often for Metra.
- The Lincoln Square result takes bus 49 plus the Blue Line instead of the Brown Line it sits next to. With `first: 1` and no tuning OTP returns whichever itinerary its default ranking likes; why it prefers this one is unverified.
- `/otp/actuators/health` returns 404 with default config. Readiness check used here: poll `/otp/` for HTTP 200. Whether a real health endpoint can be enabled is unverified.
- The OTP image has no `jcmd`, so heap numbers come from GC logging (`-Xlog:gc`), not from live heap dumps.
- Files written by the container are owned by root; clean up with Docker if you need to delete them.

## Commands

All data lives outside the repo in `/home/kingsleybawuah/orca/otp-spike-data/` (`$D` below).

```sh
D=/home/kingsleybawuah/orca/otp-spike-data
mkdir -p $D/dl $D/cta-only $D/cta-metra && cd $D/dl

# 1. downloads
curl -SL -O https://download.geofabrik.de/north-america/us/illinois-latest.osm.pbf
curl -SL -A "Mozilla/5.0" -o cta-gtfs.zip https://www.transitchicago.com/downloads/sch_data/google_transit.zip
curl -SL -o metra-gtfs.zip https://schedules.metrarail.com/gtfs/schedule.zip

# 2. Cook County extent (US Census TIGERweb, Counties layer 13, GEOID 17031)
curl -s "https://tigerweb.geo.census.gov/arcgis/rest/services/TIGERweb/State_County/MapServer/13/query?where=GEOID%3D%2717031%27&outFields=NAME,GEOID&returnExtentOnly=true&outSR=4326&f=json"
# -> xmin -88.263641, ymin 41.469705, xmax -87.111162, ymax 42.154292

# crop (osmium-tool 1.18.0, Debian stable-slim, run in Docker)
docker run --rm -v $D:/data debian:stable-slim sh -c "apt-get update -qq && apt-get install -y -qq osmium-tool && \
  osmium extract --bbox -88.263641,41.469705,-87.111162,42.154292 /data/dl/illinois-latest.osm.pbf -o /data/cook.osm.pbf --overwrite"

# build dirs: crop + GTFS only (the full Illinois pbf is NOT in them)
cp $D/cook.osm.pbf $D/dl/cta-gtfs.zip $D/cta-only/
cp $D/cook.osm.pbf $D/dl/cta-gtfs.zip $D/dl/metra-gtfs.zip $D/cta-metra/

# 3/4. build (repeat with -Xmx8G, 5G, 4G, 3G, 2G; GC log goes to the data dir)
docker run --name otp-spike-build -v $D/cta-only:/var/opentripplanner \
  -e JAVA_OPTS="-Xmx8G -Xlog:gc:file=/var/opentripplanner/gc.log" \
  opentripplanner/opentripplanner:2.10.0 --build --save
docker rm otp-spike-build   # same for $D/cta-metra
# container memory sampled once a second:  docker stats --no-stream --format '{{.MemUsage}}' otp-spike-build

# 5. serve
docker run -d --name otp-spike-serve -p 18080:8080 -v $D/cta-metra:/var/opentripplanner \
  -e JAVA_OPTS="-Xmx5G -Xlog:gc:file=/tmp/gc-serve.log" \
  opentripplanner/opentripplanner:2.10.0 --load
until [ "$(curl -s -o /dev/null -w '%{http_code}' http://localhost:18080/otp/)" = 200 ]; do sleep 0.1; done

# query (Ravenswood to Ogilvie, Tuesday 8:30)
curl -s -H 'Content-Type: application/json' http://localhost:18080/otp/gtfs/v1 -d '{"query":"{ planConnection(origin:{location:{coordinate:{latitude:41.9677,longitude:-87.6745}}}, destination:{location:{coordinate:{latitude:41.8827,longitude:-87.6411}}}, dateTime:{earliestDeparture:\"2026-10-27T08:30:00-05:00\"}, first:1) { edges { node { start end duration legs { mode transitLeg route { shortName longName agency { name } } from { name } to { name } } } } routingErrors { code description } } }"}'
docker rm -f otp-spike-serve
```

Peak heap figures come from the GC log: the largest "before" and "after" values on `Pause` lines (`grep -o '[0-9]*M->[0-9]*M([0-9]*M)'`).

## Sources

- Geofabrik Illinois extract: https://download.geofabrik.de/north-america/us/illinois-latest.osm.pbf (URL also in `docs/DATA_SOURCES.md`; file was 346 MB).
- CTA static GTFS: https://www.transitchicago.com/downloads/sch_data/google_transit.zip (same URL as `defaultCTAGTFSURL` in `services/api/cmd/seed/bus_corridors.go`; the zip bundles `developers_license_agreement.htm`).
- Metra static GTFS: https://schedules.metrarail.com/gtfs/schedule.zip (license and terms in #84 and `docs/research/metra-gtfs.md` on branch `research/metra-gtfs`).
- Cook County extent: US Census Bureau TIGERweb, State_County MapServer, layer 13 "Counties" (January 1, 2026 vintage): https://tigerweb.geo.census.gov/arcgis/rest/services/TIGERweb/State_County/MapServer/13
- OTP flags (`--build --save`, `--load`, data dir `/var/opentripplanner/`, GTFS names must contain "gtfs"): inherited from #83 and `docs/research/otp-version-memory.md` on branch `research/otp-version-memory`; the commands ran as written against the 2.10.0 image. I did not re-read docs.opentripplanner.org for this ticket, and the GraphQL `planConnection` fields were taken from the working queries above, not from the docs.
- osmium-tool 1.18.0, from Debian stable's package inside `debian:stable-slim`.
