# OTP 2.x departure window and holiday service

Research for [#82](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/82), part of the Phase 6 map [#79](https://github.com/KingsleyBawuah/block-factor-monorepo/issues/79). Pinned version: `opentripplanner/opentripplanner:2.9.0` (`docker-compose.yml`). All OTP sources below are the `v2.9.0` git tag. Nothing here was run against a live OTP instance; see [Unverified](#unverified).

## Short answer

- **One query can cover a window, but it does not give you what the commute rule needs.** `planConnection` takes a `searchWindow` (a duration starting at `earliestDeparture`) and returns the best itineraries that depart inside it, with cursor paging. But it returns only the *best* options (a Pareto set), already trimmed by filters, not one itinerary per 10 minutes. A median over that list is not the median of "leave at 8:00, 8:10, ... 9:00".
- **Recommended: one `planConnection` per departure slot, batched into one HTTP request** with GraphQL aliases, each with `searchWindow: "PT0S"` (a single search at that exact time). Take `end - slot time` from the earliest-arriving itinerary, then the median of the slots.
- **Holiday detection: OTP does not expose `calendar_dates.txt` exceptions.** It exposes only the *resolved* result (which dates each trip runs). The reliable check is to read the GTFS zip we already feed OTP (`calendar.txt` + `calendar_dates.txt`) and compare the candidate Tuesday's active `service_id` set to the typical Tuesday. OTP's `Trip.activeDates` and `Pattern.tripsForDate` can cross-check it.
- **Feed validity window: `serviceTimeRange { start end }`** gives the loaded data's first and last service date across all feeds combined (Unix seconds). It is not per feed, and OTP does not expose `feed_info.txt` dates.

## 1. Window queries: what `planConnection` offers

Arguments (all from the `planConnection` field in [schema.graphqls](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/resources/org/opentripplanner/apis/gtfs/schema.graphqls)):

| Argument | Meaning | Default |
|---|---|---|
| `dateTime: { earliestDeparture }` | Depart-at time as an `OffsetDateTime`. `latestArrival` is the arrive-by alternative. Only one of the two (`@oneOf`). | "now" |
| `searchWindow: Duration` | Length of the window that starts at `earliestDeparture`. If null, OTP picks one. When paging, it is ignored and recomputed. | dynamic (see below) |
| `first: Int` | Max itineraries on a forward page. Maps to the request's `numItineraries`. | 50 (`numItineraries` default in [RouteRequest.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/RouteRequest.md)) |
| `after: String` | Cursor for the next page (use with `first`). | none |
| `before` / `last` | Backward paging. | none |
| `modes`, `preferences`, `itineraryFilter` | Modes (`transitOnly`, `directOnly`), preferences, filter tuning. | all transit + `WALK` |

Mapping of `first`/`last`/`after`/`before` onto `numItineraries` and the page cursor: [RouteRequestMapper.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/apis/gtfs/mapping/routerequest/RouteRequestMapper.java). The `Duration` scalar accepts a lenient ISO-8601 form such as `PT1H` or `1h` (schema.graphqls).

Response: `edges[].node` (the `Itinerary`), `edges[].cursor`, `pageInfo { hasNextPage endCursor searchWindowUsed }`, `routingErrors`, `searchDateTime`.

Search window behavior ([RouteRequest.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/RouteRequest.md), [RouterConfiguration.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/RouterConfiguration.md)):

- "All optimal travels that depart within the search window is guaranteed to be found."
- Zero means one search iteration; the API rejects values above 2 days; `transit.maxSearchWindow` caps it at `PT24H` by default.
- If unset, the window is computed dynamically: minimum `PT40M`, maximum `PT3H`, rounded to 10-minute steps (`dynamicSearchWindow` defaults). So the default window can be shorter than our 60 minutes. Set it explicitly if you use a window at all.

Why a window is the wrong tool for this rule:

- OTP uses Range Raptor with several criteria (arrival time, transfers, cost) and returns the Pareto-optimal set ([raptor package.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/raptor/src/main/java/org/opentripplanner/raptor/package.md)). A departure that arrives no earlier than a later departure is dropped, so slots can be missing.
- The itinerary filter chain then groups similar itineraries and keeps the best ones ([RouteRequest.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/RouteRequest.md), "Group by similarity filters"), and `first` crops the list.
- Raptor "time-shifts" the trip so legs are packed toward the start, and the initial wait after the access leg is removed ([raptor package.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/raptor/src/main/java/org/opentripplanner/raptor/package.md), "Understanding the search"). So `Itinerary.start` is often later than the requested time and `Itinerary.duration` can exclude the wait at the origin. A commute "leaving at 8:10" should be `end - 8:10`, not `duration`. *(Inferred from the docs; not run.)*

Paging, for completeness: `after: <endCursor>` returns itineraries departing after the last one returned (or at the same time), per the argument docs in schema.graphqls. Paging has documented corner cases that can duplicate or lose itineraries when results are cropped ([paging readme](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/model/plan/paging/cursor/readme.md)). Avoid it for this rule.

## 2. Recommended query strategy

1. Pick the target Tuesday (section 3). Build 7 slots: 8:00, 8:10, ..., 9:00 in `America/Chicago` (the 2026-11-03 example below is `-06:00`; confirm the offset per date with a tz library, not a constant).
2. Send **one HTTP request with 7 aliased `planConnection` fields**. The schema marks `planConnection` as `@async`, which runs aliased fields of one request in parallel ("useful when adding several queries in the same HTTP request"; schema.graphqls, top of file).
3. Per slot use `searchWindow: "PT0S"` (one search at exactly that departure time) and a small `first` (5). Take the itinerary with the earliest `end`; slot commute = `end - slot time`. Median across slots.
4. Slot with `routingErrors` and no edges: map codes to the #80 rules (see below). Slots with no route should be handled consistently (all-or-nothing is simplest).
5. Cache by (origin grid cell, destination, target service date), per the Phase 6 cache-key ticket. One request per cache miss, 7 searches inside.

Alternative if a single search per slot proves too slow: one `planConnection` with `searchWindow: "PT1H"` and `first: 50`, then derive each slot's commute client-side as `min(end of itineraries with start >= slot) - slot`. This is cheaper per request but depends on the Pareto and filter behavior above, so it is the fallback, not the default.

Example (values are synthetic; coordinates are placeholders):

```graphql
query CommuteSlots {
  s0800: planConnection(
    origin: { location: { coordinate: { latitude: 41.9, longitude: -87.7 } } }
    destination: { location: { coordinate: { latitude: 41.88, longitude: -87.63 } } }
    dateTime: { earliestDeparture: "2026-11-03T08:00:00-06:00" }
    searchWindow: "PT0S"
    first: 5
  ) {
    edges { node { start end duration waitingTime numberOfTransfers legs { mode } } }
    routingErrors { code description }
  }
  s0810: planConnection(
    origin: { location: { coordinate: { latitude: 41.9, longitude: -87.7 } } }
    destination: { location: { coordinate: { latitude: 41.88, longitude: -87.63 } } }
    dateTime: { earliestDeparture: "2026-11-03T08:10:00-06:00" }
    searchWindow: "PT0S"
    first: 5
  ) {
    edges { node { start end duration waitingTime numberOfTransfers legs { mode } } }
    routingErrors { code description }
  }
  # ... s0820 through s0900 the same way
}
```

Use GraphQL variables for origin/destination in the real client. Endpoint: `POST /otp/gtfs/v1` ([GTFS-GraphQL-API.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/apis/GTFS-GraphQL-API.md)); the compose file maps it to host port 4003.

`routingErrors[].code` values that line up with #80 ([schema.graphqls](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/resources/org/opentripplanner/apis/gtfs/schema.graphqls), `RoutingErrorCode`): `OUTSIDE_BOUNDS` (outside routing area), `NO_TRANSIT_CONNECTION` and `NO_STOPS_IN_RANGE` (no route), `OUTSIDE_SERVICE_PERIOD` (date outside loaded data), `WALKING_BETTER_THAN_TRANSIT` (transit removed because walking wins; relevant for short commutes).

## 3. Holiday service and the validity window

### What OTP exposes

- The GraphQL schema has no `calendar_dates`, exception, or `feed_info` fields (search of schema.graphqls). `Feed` has only `feedId`, `agencies`, `publisher`, `alerts`.
- OTP resolves `calendar.txt` plus `calendar_dates.txt` into per-`service_id` date lists: type `1` adds a date, type `2` removes it ([CalendarServiceDataFactoryImpl.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/model/calendar/impl/CalendarServiceDataFactoryImpl.java)). Those resolved dates are queryable:
  - `Trip.activeDates` (`YYYYMMDD` strings) and `Trip.serviceId`.
  - `Pattern.tripsForDate(serviceDate: "YYYYMMDD")`.
  - `Route.patterns(serviceDates: {start, end})` and `routes(serviceDates:)`; `end` is exclusive.
  - `Stop.stoptimesForServiceDate(date:)`.
- GTFS has no "holiday" flag. A holiday shows up only as a different set of active `service_id`s (or none) on that date ([calendar_dates.txt](https://gtfs.org/documentation/schedule/reference/#calendar_datestxt)). So "holiday Tuesday" must be inferred as "this Tuesday's service differs from a normal Tuesday".

### Recommended detection

1. At rebuild time (or on API start), read the GTFS zips we already hand to OTP. For each Tuesday in the validity window, compute the active `service_id` set: `calendar.txt` rows with `tuesday=1` and the date in range, plus `calendar_dates` type `1`, minus type `2`.
2. The normal Tuesday set is the most common set across Tuesdays. A Tuesday whose set differs is a holiday or special-service Tuesday: skip it.
3. Cross-check against OTP for the chosen date: `tripsForDate` or `stoptimesForServiceDate` on a few reference patterns should return a count close to other Tuesdays. This also confirms OTP loaded what we parsed.
4. OTP-only variant (no zip parsing): `trips { serviceId activeDates }` and group by date. Works, but returns every trip in the feed, so it is large. Use it only if parsing the zip in Go is unwanted.

### Validity window

- `serviceTimeRange { start end }` returns Unix seconds ([schema.graphqls](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/resources/org/opentripplanner/apis/gtfs/schema.graphqls)). It is computed from the first and last date that has any service in the loaded calendar data, with `end` being the start of the day *after* the last service date ([TimetableRepository.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/transit/service/TimetableRepository.java), [serviceTimeRangeImpl.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/apis/gtfs/datafetchers/serviceTimeRangeImpl.java)). Treat `end` as exclusive.
- It is the **union across all feeds**: with CTA and Metra loaded, the end is the later of the two, so it hides the earlier expiry. For the "expires within 14 days" warning (#80), compute each feed's expiry from its own zip, or via `trips(feeds: ["<feedId>"]) { activeDates }` (heavy). Use `serviceTimeRange` as a cross-check.
- The graph build itself trims service to `transitServiceStart` (default `-P1Y`) and `transitServiceEnd` (default `P3Y`) relative to the build date, so the range can be shorter than the feed ([BuildConfiguration.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/BuildConfiguration.md)).
- Asking for a date outside the range returns the `OUTSIDE_SERVICE_PERIOD` routing error.
- GTFS `feed_info.txt` has `feed_start_date` and `feed_end_date` ([spec](https://gtfs.org/documentation/schedule/reference/#feed_infotxt)); those are the publisher's stated reliability window, which can differ from actual calendar coverage. OTP's schema does not return them.

## Open questions raised

1. **Wait time:** should the commute include the wait at the origin (`end - slot time`, recommended) or exclude it (`duration`)? This changes every score.
2. **Slot count:** "every 10 min from 8:00 to 9:00" is 7 slots if 9:00 is included, 6 if not.
3. **"Normal" Tuesday across seasons:** the modal Tuesday set may be summer or school-year service depending on the feed. Should the pick be the next normal Tuesday that matches the *current* season's pattern?
4. **Walk-only results:** should `modes.transitOnly: true` be used so a short walkable trip is not returned as walking? Not verified how it interacts with `WALKING_BETTER_THAN_TRANSIT`.
5. **Zip parsing in Go vs OTP-only holiday detection:** the zip route adds a small GTFS parser; confirm that is acceptable (no new dependency needed for CSV).

## Unverified

- That `searchWindow: "PT0S"` returns the earliest-arrival itinerary for a single departure on this graph. The docs say zero means one search iteration ([RouteRequest.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/RouteRequest.md)); not run. Smoke-test on the first real graph.
- That `Itinerary.start` is time-shifted past the requested time in the 2.9.0 GraphQL output (inferred from the Raptor docs).
- That aliased `planConnection` fields run in parallel in a single request (schema `@async` description says so; not measured).
- How `transitOnly` interacts with the walking-better-than-transit filter.
- Whether OTP reads `feed_info.txt` dates at all (not found in the schema or docs searched).

## Sources

- [OTP v2.9.0 GTFS GraphQL schema](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/resources/org/opentripplanner/apis/gtfs/schema.graphqls)
- [RouteRequest.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/RouteRequest.md), [RouterConfiguration.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/RouterConfiguration.md), [BuildConfiguration.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/BuildConfiguration.md), [GTFS-GraphQL-API.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/doc/user/apis/GTFS-GraphQL-API.md)
- [Raptor package.md](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/raptor/src/main/java/org/opentripplanner/raptor/package.md)
- [Paging cursor readme](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/model/plan/paging/cursor/readme.md)
- Java: [RouteRequestMapper.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/apis/gtfs/mapping/routerequest/RouteRequestMapper.java), [serviceTimeRangeImpl.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/apis/gtfs/datafetchers/serviceTimeRangeImpl.java), [TimetableRepository.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/transit/service/TimetableRepository.java), [CalendarServiceData.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/model/calendar/CalendarServiceData.java), [CalendarServiceDataFactoryImpl.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/model/calendar/impl/CalendarServiceDataFactoryImpl.java), [TripImpl.java](https://github.com/opentripplanner/OpenTripPlanner/blob/v2.9.0/application/src/main/java/org/opentripplanner/apis/gtfs/datafetchers/TripImpl.java)
- GTFS spec: [calendar.txt, calendar_dates.txt, feed_info.txt](https://gtfs.org/documentation/schedule/reference/)
