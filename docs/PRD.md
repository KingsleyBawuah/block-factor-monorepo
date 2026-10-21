# BlockFactor: Product Requirements Document

**Version:** 1.0 (Draft / In Review)  
**Target Market:** Chicago, IL  
**Repository Structure:** Monorepo (Go API + React/TanStack Frontend + PostGIS / Docker Services)  
**License:** AGPL-3.0-only  

**See also:**
* [ARCHITECTURE.md](./ARCHITECTURE.md) — Technical Architecture Specification
* [IMPLEMENTATION_PHASES.md](./IMPLEMENTATION_PHASES.md) — Implementation Phases & Roadmap
* [AGENTS.md](../AGENTS.md) — Engineering Standards & Working Agreements

---

## 1. Executive Summary & Vision

BlockFactor is an analytical, location-intelligence platform engineered to evaluate, score, and rank residential rental properties for car-free and transit-oriented urban residents. It isolates hyper-local environmental and logistical variables to generate an objective, composite livability score (0 to 100) per address.

---

## 2. Target Personas

> **Primary Use Case:** Discovery & evaluation mode — users researching and comparing addresses *before* committing to a lease or purchase decision.

- **Car-Free Urban Professionals:** Renters and buyers relying on rapid rail (CTA 'L'), high-frequency bus corridors, Divvy micro-mobility, and walkable daily errands. BlockFactor quantifies what would otherwise be subjective gut-feel.
- **Active Commuters & Cyclists:** Residents seeking immediate access to protected bike lanes and off-street paved trails (e.g., Lakefront Trail, 606/Bloomingdale Trail).
- **Remote & Hybrid Workers:** Tenants and buyers prioritizing natural light exposure, quiet street buffers, third places (cafes, co-working spaces, libraries), and verified high-speed fiber internet availability.

> **Note:** While the primary persona is a renter, BlockFactor is equally applicable to prospective buyers evaluating a property or neighborhood prior to purchase.

---


## 3. Inputs & User Configuration

### 3.1 Mandatory Input
- **Home Street Address:** Candidate residential property address (geocoded via Nominatim).

> **Coverage Policy (v1):** Scoring is hard-scoped to the **City of Chicago boundary polygon** (ingested from the Chicago Data Portal at seed time). `POST /api/v1/score` rejects coordinates outside the boundary with HTTP 422 and a machine-readable `OUT_OF_COVERAGE` error code — no partial scoring.

> **Geocode Disambiguation:** The geocode step returns a ranked candidate list; when input is ambiguous (e.g. "100 Main St"), the UI presents a candidate picker for confirmation before scoring rather than silently selecting the top match.

### 3.2 Optional Commute Destination (Hybrid / In-Office Commuters)
- **Workplace / Destination Address:** Target office, coworking space (e.g. WeWork), university campus, or secondary frequent destination.
- **Commute Mode Preference:** Transit (v1 default and the only scored mode). Biking and Walking rubrics are deferred post-v1 — the `S_commute` rubric (5.10) is transit-specific (transfers, one-seat rides).
- **Departure Window:** Tuesday 8:30 AM (simulated rush-hour timetable).
- *If omitted (Fully Remote mode):* The commute factor is hidden and weights dynamically rebalance across the remaining 8 factors.

### 3.3 Optional Unit Metadata
- **Primary Window Facing Direction:** N, NE, E, SE, S, SW, W, NW
- **Floor Level:** Integer (default: 1)
- **Unit Exposure:** Street-facing, Courtyard/Interior, Alley-facing

### 3.4 User Customization & Persona Presets
Users can adjust individual weight sliders (0% to 100%). BlockFactor provides opinionated presets:

| Factor | Remote Worker Preset (Default) | Hybrid / In-Office Commuter Preset | Rationale |
|---|---|---|---|
| Transit (`S_transit`) | 25% | 17% | Static proximity to 'L' stations & high-frequency bus corridors |
| Commute Duration (`S_commute`) | **0% (Hidden)** | **17%** | Door-to-door transit time & transfer friction to user's workplace |
| Bike Infrastructure (`S_bike`) | 12% | 10% | Proximity and directional network coverage of protected/buffered bike lanes |
| Groceries (`S_grocery`) | 18% | 13% | Daily necessity; walking distance to a full supermarket |
| Third Places (`S_third_place`) | 13% | 9% | Vital for neighborhood vibe, cafes, and daytime remote work spots |
| Green Space & Parks (`S_green`) | 9% | 9% | Outdoor recreation, Lakefront/606 trail access, mental well-being |
| Noise Environment (`S_noise`) | 9% | 10% | Acoustic comfort; balances transit proximity vs. track/traffic noise |
| Public Safety (`S_safety`) | 9% | 10% | Baseline security against violent & property incident density |
| Flood Risk (`S_flood`) | 3% | 3% | Guardrail against chronic Chicago basement/garden-unit flooding |
| Solar Exposure (`S_solar`) | 2% | 2% | Unit-specific natural light quality and window orientation |

> **Dynamic Normalization:** Weights are dynamically normalized at runtime so active factors always sum to 100% ($\sum w_i = 100\%$).

> **Future consideration — dedicated Cyclist preset:** PRD §2 names "Active Commuters & Cyclists" as a target persona, but there is currently no preset chip weighting specifically for it — bike weight is folded into the two existing presets instead. Presets are purely a frontend default-values convenience (no representation in the OpenAPI contract or database schema), so adding a third preset later is a low-risk, purely additive change — deferred for now rather than designed as part of this pass.

---

## 4. Data Sources & Ingestion Strategy

| Domain | Attribute | Data Source | Ingestion & Cache Strategy |
|---|---|---|---|
| **Transit** | 'L' Rail Stops & Lines | Chicago Data Portal (CTA 'L' Stops) | PostGIS spatial point table; sync monthly. |
| **Transit** | High-Frequency Bus Lines | CTA Static GTFS Schedule | PostGIS; routes with ≤ 10 min headways. Sync quarterly. |
| **Micro-Mobility** | Divvy Docks & E-Bikes | Lyft / Divvy GBFS JSON Feed | Station coords static; live capacity cached in-memory (5m TTL). Powers the map dock-capacity layer only — not a scored factor. |
| **Cycling** | Protected Lanes & Trails | Chicago Data Portal (Bike Routes) | PostGIS linestring buffering; filter for protected lanes/trails. |
| **Travel Times** | Commute Durations | Self-Hosted OpenTripPlanner 2.x | Local HTTP API; computed on-demand via OSM + GTFS; commute durations cached in PostGIS (7-day TTL). |
| **Sustenance** | Supermarkets & Pharmacies | OpenStreetMap (Geofabrik Illinois extract) | Filtered POI import (`shop=supermarket`, `shop=convenience`, `amenity=pharmacy`) → PostGIS `osm_pois` table; sync quarterly. |
| **Third Places** | Cafes, Gyms & Studios | Google Places API (New) | Nearby Search Pro; cached in PostGIS spatial grids (30-day TTL). |
| **Green Space** | Parks & Forest Preserves | Chicago Data Portal (Parks dataset) | PostGIS spatial polygon table; sync quarterly. OSM `leisure=park` supplement deferred post-v1 (avoids dual-source dedup). |
| **Noise Environment** | Track & Traffic Buffers | OSM Road Hierarchy + CTA Track Geometry | Deterministic spatial buffer evaluation in PostGIS. |
| **Flood Hazard** | FEMA Zones & Basement 311s | FEMA NFHL + Chicago Data Portal 311 | PostGIS polygon overlays + 311 sewer backup incident density. |
| **Public Safety** | Reported Incident Density | Chicago Data Portal (Crimes) | SODA-ingested into PostGIS (`cpd_crimes`, trailing 12 mo); scored via local `ST_DWithin` radial queries — no live Socrata calls at request time. |
| **Solar Exposure** | Window Orientation & Floor | Deterministic lookup (no external data) | Orientation base score × exposure modifier + floor bonus (see 5.8); no astronomical library required. |
| **Utilities** | Broadband & Fiber Access | FCC National Broadband Map API | Geocoded coordinate lookup; cached per address (refreshed bi-annually). Powers broadband specs and affiliate CTAs. |

---

## 5. Scoring Formula & Logic

### 5.1 Composite Score Formula

```
BlockFactor = Σ(wᵢ · Sᵢ) / Σwᵢ
```

A weighted average of individual factor scores, where `wᵢ` are the user-configured weight sliders and `Sᵢ` are the normalized sub-scores (0–100).

> **Score Clamping:** All individual factor sub-scores `Sᵢ` are strictly bounded to the range [0, 100]:
> $$S_i = \min(100, \max(0, \text{CalculatedScore}))$$

> **Boundary Semantics:** All distance/duration band tables in this section use half-open intervals — a value landing exactly on a shared boundary falls into the *better* (higher-scoring) band.

> **Distance Measurement (v1):** Distances are geodesic straight-line (`ST_DWithin` on `geography` columns), not walking-network distances. Chicago's river branches and rail viaducts can make straight-line optimistic; thresholds are calibrated with this in mind. Network-based distances (OTP2 one-to-many) are a post-v1 refinement. Thresholds are documented in imperial units for readability — the implementation computes in **meters** internally, converts only at the presentation layer, and defines every threshold as a named constant in `internal/scoring`. Distance pills in the UI carry a "straight-line" qualifier to set expectations.

> **Barrier-Crossing Penalty (Post-v1 Refinement):** In Phase 2+, straight-line POI distances can be penalized when the line crosses a physical barrier (Chicago River, expressway trenches, rail yards via OSM `waterway` and `noise_sources` geometries). A barrier crossing applies a ~1.5× distance multiplier or disqualifies the POI. This refinement is deferred post-v1 to keep Phase 3 scoring unblocked; v1 scoring accepts the occasional "0.2 mi" grocery across the river as the cost of simplicity.

> **Degraded Factors:** A factor that times out or lacks data returns `null`; its weight is excluded and the remaining weights renormalize (the same mechanism used when the commute factor is hidden). The report visually flags degraded factors. If all active weights are 0, the API rejects the request with a validation error rather than dividing by zero.

---

### 5.2 Transit Score (`S_transit`)

**Base score — distance to nearest CTA 'L' stop:**

- **100 pts** at ≤ 0.25 miles from a CTA 'L' stop (~5 min walk).
- Linear decay to **0 pts** at 1.0 mile.

**Bonus — three independent components, all additive, then `clamp(BaseScore + Bonuses, 0, 100)`:**

1. **Dual-line access:** **+10 pts** if 2+ distinct CTA 'L' lines are reachable across stations within 1.0 mile. Encodes transfer/redundancy value, independent of physical direction — real 'L' lines curve too much (the Loop, the Brown Line's turn onto the Ravenswood branch, the Pink Line's jog south) for a geometric "reach" bonus to mean anything without dedicated per-line track geometry, which doesn't exist yet (see Future Consideration below).
2. **Bus directional coverage:** looks at high-frequency (≤ 10 min headway) bus corridors within 0.2 miles and how many distinct axes they cover:
   - No qualifying corridor: **+0 pts**.
   - Qualifying corridor(s) in one direction only (e.g. only east-west): **+10 pts**.
   - Qualifying corridors in both directions (east-west *and* north-south): **+15 pts**.

   Unlike rail, most Chicago bus routes run straight enough along a single arterial street that a geometric axis classification (`ST_Azimuth` on the corridor geometry) is meaningful — this mirrors `S_bike`'s (§5.11) directional network bonus.
3. **Rail service quality:** a continuous 0–20 pt score based on the best-scoring accessible CTA 'L' line's actual schedule, `AMPeakComponent + OvernightComponent`:
   - **AM peak component (0–10 pts):** 10 pts at ≤ 6 min mean headway (7:00–9:00 AM weekdays), linear decay to 0 pts at ≥ 20 min headway.
   - **Overnight component (0–10 pts):** 10 pts at ≤ 15 min mean overnight headway (1:00–4:00 AM, any day), linear decay to 0 pts at ≥ 30 min headway — chosen to match CTA's own published service standard of 30-min headways for non-Key routes even at peak, so anything sparser than the system's own peak baseline scores 0 here. A line with no scheduled overnight service at all also scores 0 pts on this component.

   Both components are derived from the same CTA static GTFS feed already used for the bus high-frequency derivation (`stop_times`), computed per line rather than hardcoded (CTA's 24-hour line roster changes — the Orange Line is slated for 24-hour Midway service in 2026 — so this must read from the live schedule at ingestion time, not a fixed list of line names). Continuous decay was chosen over discrete tiers to avoid misrepresenting real in-between cases like the Pink Line, which runs frequently by day (8–9 min peak headway) but sparsely overnight (~20 min, landing mid-curve on the overnight component rather than at either extreme) — a binary "24-hour: yes/no" flag would either overstate or understate that service level depending on which side of the cutoff it landed on.

> **Multi-Line Station Selection:** When multiple CTA 'L' lines are reachable within the same 1.0 mile radius used for the base distance score (whether at one station or several), the rail service quality component uses whichever accessible line scores highest — not an average, and not necessarily the line at the physically closest station. This mirrors how the dual-line bonus already considers all lines across all stations within that same radius, rather than only the closest one.

> **High-Frequency Definition (bus):** A bus stop qualifies as high-frequency if at least one route serving it has a mean scheduled headway of ≤ 10 minutes between 7:00–9:00 AM on weekdays, computed from GTFS `stop_times` during ingestion. (Unchanged from v1.)

> **Future Consideration — Rail Directional Reach (Post-v1):** The bus directional-coverage bonus has no rail equivalent — see the Dual-Line Access rationale above. Adding one would require ingesting real per-line track geometry (not just the raw, line-agnostic track segments `noise_sources` already stores for `S_noise`) so a local azimuth could be computed near each specific station rather than over an entire curving line. Not started; scoped separately from the rail service-quality work above.

> **Future Consideration — Metra Proximity (Post-v1):** `S_transit` currently only counts CTA 'L' stops and buses; Metra (Chicago's suburban commuter rail) isn't modeled as a standalone proximity amenity at all yet, even though a nearby Metra station is a real asset — particularly for commuting to the suburbs, which CTA doesn't reach. Unlike the §5.10 `S_commute` consideration below, this would need real new work: no Metra ingestion path exists yet (station locations, line data), and it'd mean either extending this factor's radius/bonus logic or designing a separate one. Not started.

> **Future Consideration — Line Schedule Display (Post-v1, Phase 5):** `cta_line_service` (this section's rail service-quality data) intentionally stores only the two headway numbers scoring needs (`am_peak_headway_minutes`, `overnight_headway_minutes`) — not full hours-of-operation, even though a "Runs 4:30 AM – 1:15 AM" / "24 Hours" badge on the Phase 5 factor breakdown card is a real, likely want. Deferred out of this migration as UI-display scope, not scoring scope, and because the right representation depends on Phase 5's actual query needs, which don't exist yet. Two implementation notes for whoever picks this up: (1) CTA schedules differ by day-type (`calendar.txt` defines distinct weekday/Saturday/Sunday service patterns for rail), so it's three spans per line, not one; (2) most lines run past midnight (e.g. ~4:30 AM to ~1 AM the *next* day), which breaks a naive same-day time range — Postgres has no `tzrange` type (confirmed against the live Postgres 18 instance: only `tsrange`/`tstzrange`/`daterange`/`numrange`/`int4range`/`int8range` and their multirange counterparts exist), and even `tstzrange` rejects a same-day wraparound (`lower bound must be <= upper bound`). The fix, if a real range type is worth the ceremony for future query needs (e.g. "is anything running right now"), is anchoring start/end to two different reference dates so the end date genuinely rolls over rather than wrapping — `tstzrange('2026-01-01 04:30', '2026-01-02 01:15')` — rather than modeling it as a single calendar-day span. If the only need turns out to be a static display string, plain nullable `TIME` columns are simpler than a range type and avoid the ceremony entirely.

---

### 5.3 Grocery Score (`S_grocery`)

| Distance to Supermarket | Score |
|---|---|
| ≤ 0.3 miles | 100 |
| 0.3 – 0.75 miles | 75 |
| 0.75 – 1.0 miles | Linear decay from 75 → 0 |
| > 1.0 miles | 0 |

> **Note:** Corner stores / bodegas cap at a maximum of 40 points.
> **Pharmacy Bonus:** A pharmacy/drugstore (Walgreens, CVS) within ≤ 0.2 miles provides a +10 pt boost (capped at 100 total).

---

### 5.4 Third Places Score (`S_third_place`)

Evaluates cached Google Places POIs within a 0.5 mile radius (~10-minute walk):

| Criterion within 0.5 mile POI Window | Score |
|---|---|
| ≥ 3 independent cafes **AND** ≥ 1 fitness center/studio within ≤ 0.4 miles | 100 |
| ≥ 2 independent cafes **OR** ≥ 1 fitness center/studio within ≤ 0.5 miles *(if not meeting the 100-pt benchmark)* | 75 |
| At least 1 independent cafe or gym within ≤ 0.5 miles | 40 |
| No qualifying third places within 0.5 miles | 0 |

> **Independence Heuristic:** Google Places data carries no "independent" flag. Cafes are classified as independent via a maintained chain-brand exclusion list (Starbucks, Dunkin', Peet's, etc.) matched against place names at cache-fill time.

---

### 5.5 Green Space & Parks Score (`S_green`)

- **100 pts** at ≤ 0.1 miles from the nearest boundary of a recognized public park or community garden polygon. (Distance is measured to the polygon edge — OSM park *entrance* nodes are too sparsely mapped to rely on.)
- Decays to **50 pts** at 0.3 miles, and **0 pts** at 0.75 miles.
- **+15 pt bonus** when an off-street trail access point (Lakefront Trail, 606/Bloomingdale Trail) lies within 0.25 miles (capped at 100 total).

---

### 5.6 Noise Environment Score (`S_noise`)

Evaluates acoustic livability by detecting proximity to major urban noise generators:

| Noise Source | Distance Threshold | Penalty Applied |
|---|---|---|
| Elevated CTA Tracks ('L') | < 150 ft | -35 pts |
| Elevated CTA Tracks ('L') | 150 – 300 ft | -20 pts |
| Expressways (I-90/94, I-290, etc.) | < 500 ft | -30 pts |
| Major 4-lane Arterial Corridors | < 100 ft | -15 pts |

- Base score starts at **100 pts** (quiet residential street) and applies cumulative proximity penalties (floored at 0).

> **Grade Separation:** Track penalties apply only to elevated and at-grade CTA segments. Subway segments (State St / Dearborn tunnels) are exempt — grade separation is derived from OSM `bridge` / `tunnel` tags on railway ways, with manual segment tagging as a fallback.

> **Scope (v1):** Metra rail, freight corridors, and O'Hare/Midway flight paths are not yet modeled as noise sources.

> **Future Factor — `S_stroad` / Street Safety (Post-v1):** Distinct from pure acoustic noise: a "stroad" (Strong Towns terminology — a street/road hybrid, designed for through-traffic speed but lined with frequent commercial driveways and turning conflicts) is dangerous and unpleasant to live near even when it isn't especially loud. To avoid a costly re-ingest later, `noise_sources` rows for `arterial`/`expressway` ways carry `lanes`, `maxspeed`, and a `has_turn_lane` flag from OSM at ingestion time (Phase 2) even though no scoring logic uses them yet. Road geometry (fast + wide + frequent turn lanes) is only a proxy for actual danger, though — a well-grounded factor should ideally combine it with real pedestrian/cyclist crash data (Chicago's Vision Zero dataset — raised as a candidate, entirely unresearched so far, see `docs/DATA_SOURCES.md` "Future / Under Consideration") rather than geometry alone, since a street with genuinely high crash rates nearby is worth surfacing directly regardless of its lane count. Scope, weighting, and whether this becomes its own factor or an `S_noise` enhancement are all still open.

---

### 5.7 Flood Risk Score (`S_flood`)

Addresses the critical Chicago risk of basement and garden-unit flooding:

- **Base Score:** 100 pts for FEMA Zone X (Minimal Flood Hazard); 40 pts for Zone A/AE (100-year flood zone).
- **Urban Sewer Backup Penalty:** -5 pts per reported 311 water/sewer backup incident within 0.25 miles over the past 24 months (floored at 0).

---

### 5.8 Solar Score (`S_solar`)

**Base score by window direction:**

| Direction | Base Score |
|---|---|
| South (180°) | 100 |
| Southeast (135°) | 92.5 |
| Southwest (225°) | 87.5 |
| East (90°) | 85 |
| West (270°) | 75 |
| Northeast (45°) | 62.5 |
| Northwest (315°) | 57.5 |
| North (0°) | 40 |

> **Intermediate Directions:** The four ordinal directions (SE, SW, NE, NW) score as the average of their two adjacent cardinal base scores (e.g. SE = avg(S, E) = 92.5). These are reachable both from explicit `unit.facing` input and from `InferStreetFacingOrientation`'s diagonal-avenue inference (addresses on Milwaukee, Archer, and similar diagonal avenues, per Chicago's 1909 Brennan numbering system — see `internal/scoring/orientation.go`).

**Exposure Modifier:**

| Exposure Type | Multiplier |
|---|---|
| Street-facing | 1.0× |
| Courtyard/Interior | 0.65× |
| Alley-facing | 0.50× |

**Floor Bonus:** +5 pts per floor above floor 2 (e.g., floor 3 = +5, floor 5 = +15).

`S_solar = clamp(BaseScore × ExposureModifier + FloorBonus, 0, 100)`

---

### 5.9 Public Safety Score (`S_safety`)

- Performs a radial query against Chicago Data Portal crimes over the trailing 12 months.
- **Penalty Calculation:** `Penalty = (5 × Violent Count) + (5 × Sexual Count) + (1 × Property Count) + (2 × Nuisance Count)`
  - Crimes are bucketed into four scored categories from the real `primary_type`/`iucr` classification (full mapping in `docs/DATA_SOURCES.md` "Category coverage") — a fifth, unscored "excluded" set (e.g. gambling, liquor licensing, parole/probation compliance) is dropped at ingestion rather than counted:
    - **Violent** (5×): assault, battery, robbery, homicide, kidnapping, weapons and concealed-carry-license violations, stalking/intimidation, non-sexual child abuse/endangerment, animal abuse.
    - **Sexual** (5×): sexual assault, sex offenses, family-member child sexual abuse, child pornography, sex-offender registration failures — weighted equal to Violent, both being crimes capable of extreme trauma or death.
    - **Property** (1×): theft, burglary, motor vehicle theft, criminal damage, arson, vehicle-fraud offenses.
    - **Nuisance** (2×): narcotics, prostitution, public peace violations, public indecency — real quality-of-life impact, but categorically less severe than a threat to physical safety.
- Final score normalized against **Chicago block percentiles** — meaning the score is relative, not absolute.

#### How Percentile Normalization Works

1. At data ingestion time, we pre-compute the penalty score for a representative grid of ~500m blocks across Chicago and store the distribution.
2. When a user submits an address, we compute that address's raw penalty score and find where it falls in the city-wide distribution.
3. A block at the **0th percentile** (lowest crime) scores **100**. A block at the **99th percentile** (highest crime) scores **0**.
4. This means scores are always meaningful *relative to Chicago as a whole* — a score of 70 means "safer than 70% of Chicago blocks."

---

### 5.10 Custom Commute Score (`S_commute` — Optional)

When a user provides an optional workplace or campus address, BlockFactor queries the local OpenTripPlanner 2 engine for a Tuesday 8:30 AM rush-hour door-to-door transit itinerary (walking + waiting + riding + transfers):

| Door-to-Door Commute Duration | Base Score | Commute Experience |
|---|---|---|
| ≤ 20 minutes | 100 | Gold-standard effortless commute |
| 20 – 35 minutes | 85 | Standard single-seat ride or quick transfer |
| 35 – 50 minutes | 55 | Manageable 2–3 days/week hybrid commute |
| 50 – 65 minutes | 25 | High-friction commute with multiple transfers |
| > 65 minutes | 0 | Severe transit friction |

- **One-Seat Ride Bonus:** +10 pts if the commute requires 0 transfers (direct train or bus route).
- **Transfer Friction Penalty:** -15 pts per transfer beyond 1 transfer (e.g. 2 transfers = -15 pts).
- **Formula:** `S_commute = clamp(BaseScore + OneSeatBonus - TransferPenalty, 0, 100)`

> **Departure Date Resolution:** "Tuesday 8:30 AM" resolves to the next Tuesday that falls inside the loaded GTFS feed's service validity window. If the feed's calendar has expired (e.g. a quarterly refresh slipped), the commute factor degrades to `null` rather than returning misleading itineraries.

> **Why penalize transfers when duration already includes them?** The explicit penalty intentionally double-weights transfers relative to raw duration: it captures friction beyond time cost — platform waits in weather, missed-connection risk, and cognitive load.

> **Benchmark Reference Commutes:** Regardless of whether a custom workplace is entered, the report UI displays reference commute duration pills to Chicago's 4 major economic hubs:
> * **The Loop (Clark/Lake):** e.g. `18 min (Pink Line direct)`
> * **Fulton Market / West Loop:** e.g. `14 min (Pink Line / Bike)`
> * **O'Hare International Airport:** e.g. `52 min (Pink → Blue at Clark/Lake)`
> * **Midway International Airport:** e.g. `35 min (Pink → Bus 62 / Orange)`
>
> Hub coordinates are defined as canonical constants in `internal/scoring`; the durations above are illustrative and always computed live, never hardcoded UI copy.

> **Future Consideration — Metra in OTP2 Routing (Post-v1):** Unlike §5.2's Metra proximity idea, this one is close to free: `S_commute` already plans to query OpenTripPlanner 2 (Phase 6) for real door-to-door itineraries. If Metra's GTFS feed gets loaded into that same OTP2 graph alongside CTA's, a commute to a suburban workplace would automatically route via Metra whenever it's genuinely the fastest option — no new scoring logic in this factor, just one more GTFS feed added to routing infrastructure already being built. Metra specifically matters here since it's Chicago's suburban commuter rail — exactly the kind of trip CTA (city-only) can't serve. Not started; contingent on Phase 6.

---

### 5.11 Bike Infrastructure Score (`S_bike`)

Evaluates proximity to genuinely useful cycling infrastructure, weighted by real safety/comfort quality rather than treating all "bike lanes" as equivalent:

| Nearest Qualifying Infrastructure (`bike_routes` category) | Base Score at ≤ 0.1 mi |
|---|---|
| Protected Bike Lane or Neighborhood Greenway | 100 |
| Buffered Bike Lane | 70 |
| Bike Lane (paint only) | 40 |
| Marked Shared Lane | 15 |
| None within 0.5 miles | 0 |

- Base score decays linearly to 0 at 0.5 miles.
- When multiple categories are present within range, the higher-scoring category wins — a Protected Bike Lane at 0.4 mi can outscore a Marked Shared Lane at 0.05 mi.
- **+15 pt network bonus** if qualifying segments (Buffered Bike Lane or better) exist in **both** a north-south-oriented direction and an east-west-oriented direction within 0.5 miles (capped at 100 total). Orientation is computed from each segment's bearing (PostGIS `ST_Azimuth`), bucketed as north-south (within ±45° of due north/south) or east-west (everything else).

> **Why the quality tiers:** Painted lanes and sharrows provide markings but no physical protection from vehicle traffic — a materially different safety experience from a protected lane or a calmed greenway street. Treating all "bike infrastructure" as equivalent would hide exactly the distinction that matters to a car-free cyclist choosing where to live.

> **Why the network bonus, not just a density count:** A single isolated protected-lane segment is less useful than a connected network — but more specifically, *direction* matters more than raw count. Two parallel lanes running the same axis don't help if your actual trips need to go the other way; genuinely useful access means being able to travel in more than one direction from home. Chicago's strict cardinal street grid makes a simple two-bucket (north-south / east-west) orientation check a reasonably accurate, low-complexity proxy for "can I actually get anywhere from here," without needing full compass-rose granularity.

> **Future Consideration — Divvy Dock Proximity Bonus:** Divvy is currently display-only (§4: station markers + live capacity badges on the Phase 4 map layer, not a scored factor). A small proximity bonus here is plausible — using only the GBFS feed's static `station_information` (dock location), never the live 5-minute-TTL `station_status` capacity data, since a scored factor needs to describe the location, not the moment. Deferred for now because Divvy stations are heavily sited near existing transit hubs and bike infrastructure, so a bonus here risks being highly correlated with what `S_transit`'s dual-line bonus and this factor's own network bonus already capture, rather than adding much genuinely new signal — worth a real look once real station density data is in hand from the Phase 4 work, not speculated about in advance.

---

## 6. Monetization & Native Affiliate Catalog

BlockFactor integrates non-intrusive, context-triggered **Native Affiliate Cards** into the address report.

### 6.1 Affiliate Intent Signaling & Conversion Mechanics
Native affiliate cards fire based on **intent signals** detected from the user's input profile, not purely on page-load visibility:

| Intent Signal | Triggered Cards | Rationale |
|---|---|---|
| Workplace/destination address entered | Commute-relevant services (moving, relocation) | User is actively lease-shopping with a known employer. |
| Unit metadata filled (floor/facing) | Insurance, storage optimization | User has toured or is seriously evaluating. |
| 2+ addresses pinned in comparison dock | All affiliate cards | Active decision-making stage — highest conversion intent. |
| Address only (no commute/metadata) | Broadband only (utility feature) | Casual research phase — avoid ad fatigue. |

**Click Instrumentation (Phase 8 Deliverable):** All affiliate card clicks carry a `click_event` metric (logged for funnel analysis) + UTM/subid passthrough to affiliate partners. This enables EPC (earnings per click) measurement and partner pruning in Phase 2.

### 6.2 Active v1 Affiliate Partners

| Category | Partner(s) | Trigger / Context | Compensation Model |
|---|---|---|---|
| **Broadband & Fiber** | AT&T Fiber, Astound Broadband, Comcast/Xfinity | FCC broadband card showing available ISP speeds & fiber lines (always-on, intent-agnostic) | $50 – $120 CPA per activation |
| **Insurance (Multi-Product)** | **Lemonade** (Renters, Pet, Condo/Home) | Standard report summary, with dynamic pet bundle option (intent-gated: metadata or 2+ compares) | $10 – $25 CPA per policy bound |
| **On-Demand Moving Help** | **Dolly** | "Moving to this address?" CTA for local/micro apartment moves | ~$15 CPA per completed job |
| **Moving Quote Aggregator** | **Unpakt** | Full-service moving quote comparison card (intent-gated: dest or metadata) | $10 – $25 CPA per verified quote |
| **Container & Self-Move** | **PODS / U-Box** (U-Haul) | Relocation comparison modal for mid-to-long distance moves (intent-gated: dest) | 5% – 15% booking rev-share |
| **Off-Site Storage** | **CubeSmart / Extra Space Storage** | Unit size/lifestyle section (apartment storage optimization) | $20 – $50 CPA per unit rental |
| **Fitness & Third Places** | **ClassPass** | Displayed next to `S_third_place` fitness studio / gym count | $30 – $60 CPA per trial/membership |

### 6.3 Future Expansion Roadmap (Buyer Mode)

When buyer mode is activated, high-ticket mortgage and homeowner services will augment the affiliate drawer:
- **Digital Mortgage Origination:** Better.com ($100 – $200+ CPA per closed loan)
- **Mortgage Comparison:** Credible / LendingTree ($50 – $100 CPA per lead)
- **Homeowners/Condo Insurance:** Kin Insurance ($30 – $50 CPA per quote)
