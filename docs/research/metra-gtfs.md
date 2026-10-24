# Metra GTFS: feed location, terms, and OTP behavior outside the street-map crop

Research for [#84](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/84) (map: [#79](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/79), decision: [#80](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/80)). Researched 2026-10-24. This is not legal advice; items marked "owner call" need your judgment.

## Short answer

1. **Feed:** `https://schedules.metrarail.com/gtfs/schedule.zip`, about 0.6 MB. A plain download works with no account or API key (checked with an HTTP request). Metra refreshes it at 3:00 AM for planned changes, and any time for sudden ones. A `published.txt` file next to it gives the publish timestamp. Only the realtime feeds need a key, and we are not using those.
2. **Terms:** nothing in Metra's data license mentions commercial use, advertising, affiliate links, or open-source licenses, and it does not prohibit them. It grants the right to "use, reproduce, and redistribute" the data. It adds conditions that matter to us:
   - Say the app is not affiliated with Metra.
   - Show when the data was last updated.
   - Serve the data from our own host.
   - Do not modify or delete the data.
   - Do not claim the data is accurate or timely.

   Metra can revoke the license at any time. Two points are ambiguous (see "Open questions"). Recommendation: treat Metra as allowed, record the terms, and email Metra once to confirm the monetization layer is fine.
3. **OTP and stops outside the crop (OTP 2.9.0, from source and docs, not tested by running a build):** the graph build does not fail. A stop with no street within 100 m is left unlinked, and OTP records a non-fatal "data import issue" for it. Unlinked stops cannot be walked to or from, so they are unusable as boarding or alighting points. Trains that pass through them are not affected (inferred). I do not recommend pre-filtering the feed. See section 3.

## 1. Where the feed is

Verified on Metra's developer pages (the live pages block scripted fetches, so the quotes below come from a browser-style request to the same URLs):

- Developer page: <https://metra.com/developers>
- Feed API page: <https://metra.com/metra-gtfs-api>

| Item | Detail | Source |
|---|---|---|
| Static schedule URL | `https://schedules.metrarail.com/gtfs/schedule.zip` | metra-gtfs-api page, "Raw GTFS Schedule Feed" |
| Publish timestamp | `https://schedules.metrarail.com/gtfs/published.txt` (it read `10/22/26 02:20:03 AM America/Chicago` when I fetched it) | metra-gtfs-api page |
| Planned updates | "For planned schedule updates, the new file will be published at 3:00:00 AM. However, there may be instances when sudden changes require an immediate update." | metra-gtfs-api page |
| Developer-page cadence | "The GTFS static schedule is updated regularly and may change within a 24-hour span. Developers should check for updates at least once every 24 hours for an updated schedule." | developers page |
| Key needed? | Static: no. A `HEAD` request to `schedule.zip` returned `200`, `Content-Type: application/zip`, with no credentials. Realtime: "All requests must include an API authentication key" at `https://gtfspublic.metrarr.com`. | metra-gtfs-api page, plus my request |
| Terms gate | The developers page says "To access the feeds, you must first agree to the License Agreement." That includes the static feed, even though the download itself is not technically gated. | developers page |

Facts about the feed I downloaded (636 KB zip, `Last-Modified` 2026-10-22):

- It contains 10 files and 241 stops across 11 routes, all under one agency (`METRA`).
- `calendar.txt` runs 2025-01-01 to 2026-12-31. That is relevant to the "warn when the feed expires within 14 days" rule in #80, and the current file expires in about two months.
- There is no `feed_info.txt` in the current feed.

**Upcoming change (owner heads-up).** Metra's developer page says a new static feed is coming "in the coming weeks". Per the [Developer Change Notice (10/1/2026 v2.0)](https://metra.com/document/metragtfsfeeddeveloperchangenoticepdf):

- Identifiers (`trip_id`, `service_id`, `shape_id`) change format.
- Five files are added: `feed_info.txt`, `transfers.txt`, `notes.txt`, `note_links.txt`, `trip_flexfields.txt`.
- Some of those files may contain only a header row.

OTP's GTFS loader should cope, but the build script must not hard-code ids or choke on extra files. I did not check how OTP 2.9.0 handles the new files. The Metra notice recommends allow-listing them.

## 2. Terms of use

The governing document is the "GTFS Realtime API Key Request License Agreement" at <https://metra.com/gtfs-realtime-api-key-request-license-agreement>. The developers page links it as "our license agreement" for all feeds. Its opening:

> "The Commuter Rail Division of the Northern Illinois Transit Authority (Metra) hereby grants you (Licensee) non-exclusive, limited, and revocable rights to use, reproduce, and redistribute Metra Data (Data) subject to the following Terms, as they may be updated or modified by Metra, at any time in its sole discretion."

Conditions that affect BlockFactor (quoted):

| Clause | What it means for us |
|---|---|
| "Licensee shall include in any representation of the Data that its product, application, or site (App) is not sponsored, affiliated, or operated by Metra." | Add a "not affiliated with Metra" line wherever Metra-derived numbers show. CTA has a similar rule in `THIRD_PARTY_REQUIREMENTS.md` section 3. |
| "Licensee shall include in any representation of the Data a statement including the date and time the Data was last updated." | Show the feed publish date (from `published.txt`, or the date of our graph build) next to Metra-derived commutes. |
| "Licensee may receive data from Metra directly, but its App must redistribute any data through Licensee's own host. Use of Licensee's App must not direct users to access data directly from Metra's servers." | Our API serves results. The browser never hits `schedules.metrarail.com`. |
| "Licensee must not state or imply that the data is accurate, complete, or timely. Licensee may however state that the data was obtained from Metra, and Licensee is redistributing through its own host." | Attribution is allowed but not required. Add a "schedule data, as published" disclaimer. |
| "Licensee must not modify or delete Data." | **Conflicts with pre-filtering the feed** (section 3). |
| "Metra trademarks and copyrighted materials, including any confusingly similar variants, may not be used in association with Data." | No Metra logo or line-color badges. We can name "Metra" as the data source (the same clause allows saying data came from Metra). |
| "Metra reserves the right to alter how it provides Data and/or no longer provide Data at any time without prior notice." and "Metra reserves the right to terminate this license" | Revocable at will. Keep Metra optional in the pipeline (consistent with #80, "CTA + Metra if terms allow"). |
| "DATA IS PROVIDED ON AN "AS IS" AND "AS AVAILABLE" BASIS ..." | Standard disclaimer, no liability. |
| Illinois governing law. | n/a |

**What the license does not say.** I found no clause about commercial use, advertising, fees, or open-source or copyleft redistribution. Silence is not permission in a legal sense, so treat this as "no stated prohibition", not "approved".

### Open questions (owner call)

1. **Does "redistribute through your own host" cover a public AGPL repo?** Our API serving derived commute minutes from our own host fits the license. Committing `schedule.zip` or a derived GTFS into the public repo does not clearly fit. Recommendation: never commit the feed. Download it at build time (the `make` target in #80) and keep it out of git, which is how a public repo can use it without redistributing the raw data.
2. **Website terms vs. data license.** The general site terms at <https://metra.com/terms-and-conditions> say "No portion of the Site may be used by our visitors for any commercial purposes." and bar robots and spiders. They govern the website (metra.com pages), while the data license covers the Data at `schedules.metrarail.com`. The feed host is a different host from the website, and the developer program exists so apps can consume it. I read these as separate, but this is a judgment call. Do not scrape metra.com pages.
3. **Commercial use and the affiliate layer.** The license is silent. A one-line email to Metra describing the use (free, open-source, hosting-cost affiliate links elsewhere on the page, not on Metra data) would settle it. Metra's request form asks for a "Purpose of Use", so you can state it there.
4. **Do I have to sign the form to download the static file?** Technically no, but the developers page says to agree first. Recommendation: submit the form (one-time, free) so there is a record that you agreed.

### Suggested bookkeeping (not done, per ticket scope)

- `ATTRIBUTION.md`: a Metra entry (provider, "Metra GTFS Static Schedule", "Used under the Metra GTFS License Agreement", and the non-affiliation and last-updated notes).
- `docs/THIRD_PARTY_REQUIREMENTS.md`: a Metra section parallel to CTA (section 3) with the guardrails above and the 24-hour update cadence.

## 3. What OTP does with stops outside the OSM crop

Version checked: `v2.9.0` (the tag `docker-compose.yml` pins via image `opentripplanner/opentripplanner:2.9.0`). Official docs say nothing explicit about this case, so I read the 2.9.0 source. Nothing below was confirmed by running a build.

### Context: the numbers for the Metra feed

Against the Cook County bounding box from the Census TIGERweb Counties layer (lon -88.2636 to -87.1112, lat 41.4697 to 42.1543), **47 of the 241 stops fall outside the box** and 194 fall inside. Examples outside: Kenosha, Antioch, Fox Lake, Harvard, Crystal Lake, Elgin, Aurora, Geneva, Elburn, Highland Park, Waukegan, Manhattan, University Park. Stops inside the box but outside the county (for example in DuPage) would still link, because the planned crop is a rectangle (`ARCHITECTURE.md` section 6.1, `osmium extract` to a bounding box). The true crop box is not defined in the repo yet, so treat 47 as illustrative.

### At graph build time

- **Stops are linked to streets in a dedicated step.** `StreetLinkerModule.linkTransitStops` loops over every transit stop and calls `VertexLinker.linkVertexPermanently`, asking for walkable street edges ([StreetLinkerModule.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/graph_builder/module/StreetLinkerModule.java)).
- **Linking radius for permanent links is 100 m.** In `VertexLinker.link`, the search starts at `INITIAL_SEARCH_RADIUS_DEGREES` (100 m) and only retries at `MAX_SEARCH_RADIUS_DEGREES` (1000 m) `if (streetVertices.isEmpty() && scope == Scope.REQUEST)`. Build-time links use `Scope.PERMANENT`, so a stop with no street edge within about 100 m stays **unlinked** ([VertexLinker.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/street/src/main/java/org/opentripplanner/street/linking/VertexLinker.java)).
- **No exception, no build failure.** `link` simply creates no edges when nothing is in range; I found no throw on that path. The `catch (Exception e)` there only cleans up temporary edges and rethrows genuine errors.
- **Warnings come as "data import issues", not errors.** After linking, `StopConnectivityModule` is always added to the build (`GraphBuilder.java`, `graphBuilder.addModule(factory.stopConnectivityModule())`). It walks 10 minutes from each stop and, if less than that is reachable, files an `IsolatedStop` issue: "Stop %s is isolated, only %s of walking possible" ([StopConnectivityModule.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/graph_builder/module/stopconnectivity/StopConnectivityModule.java)). `DirectTransferGenerator` also files `StopNotLinkedForTransfers` ("not near any other stops; no transfers are possible"). OTP prints a per-type summary at the end of the build. The docs describe this mechanism and say the full list can be written as an HTML report with `"dataImportReport": true` ([Troubleshooting-Routing.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/Troubleshooting-Routing.md)). Expect roughly one `IsolatedStop` per out-of-crop stop (inferred), so use that count as a check on the crop.
- **Edge case near the crop line.** A stop just outside the crop but within 100 m of a road that the crop kept would link to that dangling road (inferred from the radius logic; `--strategy complete_ways` keeps whole roads that cross the line, per the OTP docs on cropping). That is harmless.

### At routing time

- **Unlinked stops are not reachable on foot.** Their stop vertices have no street edges, so a walk from an address cannot reach them, and a rider cannot get off at one and walk (inferred from the graph model above; not tested).
- **The trips themselves still exist.** The GTFS patterns stay in the transit model, so a trip from an in-Cook station through out-of-crop stops to another in-Cook station still works (inferred; not tested).
- **Addresses outside the crop get an error, not a crash.** OTP reports `OUTSIDE_BOUNDS` ("Location was found, but it was located outside the street network") when a coordinate is outside the graph's convex hull and cannot connect, and `LOCATION_NOT_FOUND` otherwise ([LinkingContextFactory.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/routing/linking/LinkingContextFactory.java), [RoutingErrorCode.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/routing/api/response/RoutingErrorCode.java)).
- **Wrinkle for #80's "outside routing area = null" rule.** `Graph.calculateConvexHull()` builds the hull from `getVertices()` ([Graph.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/street/src/main/java/org/opentripplanner/street/graph/Graph.java)), which I believe includes transit stop vertices (inferred). If so, out-of-crop Metra stops stretch the hull, and an address between the crop edge and a distant stop could return `LOCATION_NOT_FOUND` instead of `OUTSIDE_BOUNDS`. The API client should therefore treat both codes (and `NO_STOPS_IN_RANGE`) as "outside routing area / unavailable", not rely on one code. This is worth a test in the build ticket.

### Should we pre-filter the GTFS feed to the crop?

**Recommendation: no.** Reasons:

1. Metra's license says "Licensee must not modify or delete Data." Cutting stops and trips out of the feed is arguably modification (owner call).
2. OTP's docs advise matching the two inputs by cropping the OSM, not the GTFS: "Download OSM PBF data for the same geographic region as your GTFS feed" ([Basic-Tutorial.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/Basic-Tutorial.md)), and cropping is for speed ("Excessively large OSM data can lead to significant increases in computation time", [Preparing-OSM.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/Preparing-OSM.md)). Neither page mentions filtering GTFS stops, and I found no OTP doc or issue recommending it.
3. Filtering safely means trimming `stops`, `stop_times`, `trips`, and `shapes` consistently, and leaves trips with truncated stop sequences. That is extra code to own for no clear benefit. Out-of-crop stops cost only a few dozen vertices.
4. The graph size is dominated by streets (the 5 GB heap question in #85), not by 47 stops.

If the owner later wants the crop to cover more Metra, widen the bounding box instead; that is also a cleaner fix for "Metra past Cook doesn't route".

### Suggested check for the build ticket (#85)

Build with the real feed and the real crop once, read the issue summary, and confirm: no build failure, `IsolatedStop` count roughly equals the out-of-crop stop count, and a Metra trip between two in-Cook stations still routes. This turns the "inferred" items above into verified facts.

## Sources

- Metra developer page: <https://metra.com/developers> (Wayback copy, 2026-07-09: <https://web.archive.org/web/20260709214946/https://metra.com/developers>)
- Metra GTFS API page (feed URLs, cadence, realtime auth): <https://metra.com/metra-gtfs-api> (Wayback copy, 2026-07-09: <https://web.archive.org/web/20260709214946/https://metra.com/metra-gtfs-api>)
- Metra license agreement: <https://metra.com/gtfs-realtime-api-key-request-license-agreement>
- Metra website Terms of Use: <https://metra.com/terms-and-conditions>
- Metra GTFS static developer change notice: <https://metra.com/document/metragtfsfeeddeveloperchangenoticepdf>
- Live feed and timestamp: <https://schedules.metrarail.com/gtfs/schedule.zip>, <https://schedules.metrarail.com/gtfs/published.txt>
- Cook County extent: <https://tigerweb.geo.census.gov/arcgis/rest/services/TIGERweb/State_County/MapServer/11>
- OTP 2.9.0 source and docs (tag `v2.9.0`): <https://github.com/opentripplanner/OpenTripPlanner/tree/v2.9.0>
- Repo context: `ARCHITECTURE.md` section 6.1, `docker-compose.yml` (`otp` service), `ATTRIBUTION.md`, `docs/THIRD_PARTY_REQUIREMENTS.md`
