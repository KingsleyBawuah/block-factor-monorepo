# BlockFactor: Data Source Reference

**Status:** Active Reference — grows as datasets are investigated
**See also:**
* [PRD.md](./PRD.md) — Scoring formulas that consume this data
* [ARCHITECTURE.md](./ARCHITECTURE.md) — §11 Data Ingestion Pipeline
* [IMPLEMENTATION_PHASES.md](./IMPLEMENTATION_PHASES.md) — Phase 2 deliverables

---

## Overview

This document records the *real, verified* shape of each external dataset BlockFactor ingests — actual field names, resource IDs, and category values pulled directly from the live source, not assumptions from the PRD/ARCHITECTURE prose. It exists because a first pass at designing the Phase 2 migration schemas from documentation alone produced several wrong assumptions, caught only by querying the real APIs. Update this doc whenever a new dataset is investigated, before writing the migration or ingestion parser for it.

**Caution:** search engines and AI-generated summaries can fabricate plausible-looking resource IDs. One showed up in this investigation (`3w5d-sru8` for bike routes) that does not exist in any actual source link — only in a search tool's synthesized prose. Resource IDs below were confirmed by actually querying `https://data.cityofchicago.org/resource/{id}.json`, not by trusting search summaries.

---

## Chicago Data Portal (SODA API) Datasets

### CTA 'L' Stops
- **Resource ID:** `8pix-ypme` ("CTA - System Information - List of 'L' Stops")
- **Real structure:** One row **per directional platform**, not per station. E.g. `"18th (54th/Cermak-bound)"` and `"18th (Loop-bound)"` are two separate rows (`stop_id` 30162 and 30161) for the same physical station.
- **Key fields:** `stop_id`, `station_name` (shared across a station's directional rows), `stop_name` (directional), `map_id` (groups directional rows into one station), `ada` (boolean), individual boolean columns per line: `red`, `blue`, `g` (green), `brn` (brown), `p` (purple), `y` (yellow), `pnk` (pink), `o` (orange). Coordinates via `location.latitude`/`location.longitude`.
- **Schema decision:** Ingest by grouping on `map_id`, computing a `lines TEXT[]` array from whichever boolean columns are true, storing **one row per station** — not the raw per-direction shape. Avoids double-counting in any future "count nearby stations" logic and avoids duplicate overlapping map markers in Phase 4.

### Crimes
- **Resource ID:** `ijzp-q8t2` ("Crimes - 2001 to Present")
- **Key fields:** `id` (Socrata's stable unique row ID — use this as `source_id`, not `case_number`), `date` (full ISO timestamp, not a bare date), `primary_type` (free text, e.g. `"BATTERY"`, `"CRIMINAL DAMAGE"` — not a violent/property flag), `iucr` and `fbi_code` (the actual classification codes with established violent/property mapping tables), `latitude`/`longitude`.
- **Schema decision:** Store the raw `primary_type` and `iucr` alongside our own derived `category` column — don't discard the source classification, since the mapping logic may need auditing or refinement later. Use `crime_date` as `TIMESTAMPTZ`, not `DATE`.
- **Data quality notes from live investigation:**
  - `CRIM SEXUAL ASSAULT` and `CRIMINAL SEXUAL ASSAULT` are the same real-world offense recorded under two different labels at different points in the dataset's 25-year history — both must map to the same category.
  - `OTHER OFFENSE` and `OFFENSE INVOLVING CHILDREN` are IUCR catch-all `primary_type` values that bundle dozens of unrelated `iucr` sub-codes — mapping them correctly requires grouping by `iucr`/`description`, not just `primary_type`. `RITUALISM` and `DOMESTIC VIOLENCE` are effectively dead legacy codes (24 and 1 record total since 2001, 0 in the trailing 12 months) but still need a safe bucket for stray historical rows.
- **Category coverage:** `cpd_crimes.category` is `NOT NULL CHECK (category IN ('violent', 'sexual', 'property', 'nuisance'))` — a fifth, unscored "excluded" set of real `primary_type`/`iucr` values is intentionally dropped by the ingestion CLI before insert rather than stored, since `S_safety` (PRD §5.9) only scores the four categories below. This is a deliberate ingestion-time filter, not an oversight. Full mapping, verified via live group-by queries against the SODA API (trailing-12-month volumes shown):

  | Category | Weight | `primary_type` values | Notable `iucr` sub-codes (under `OTHER OFFENSE` / `OFFENSE INVOLVING CHILDREN`) |
  |---|---|---|---|
  | **Violent** | 5× | Homicide, Assault, Battery, Robbery, Kidnapping, Human Trafficking, Domestic Violence, Weapons Violation, Concealed Carry License Violation, Stalking, Intimidation | Non-sexual child abuse/endangerment/abandonment/abduction (1750, 1710, 1755, 1790, 1780), Animal Abuse/Neglect (501A), Abuse/Neglect - Care Facility (500N), Animal Fighting (1682), Other Crime Against Person (5000), Telephone Threat/Harassment (2820, 2825, 2826, 2830), Order-of-Protection/No-Contact/GPS-monitoring violations (4386, 4387, 4388, 4389), Gun/Violent Offender/Arsonist registration failures (5110, 5111, 5112, 5130, 5131, 5132, 5121), Other Weapons Violation (5007), Board Plane with Weapon (4860), Unlawful Use of Body Armor (4740) |
  | **Sexual** | 5× | Criminal Sexual Assault (incl. legacy `CRIM SEXUAL ASSAULT` spelling), Sex Offense | Family-member child sexual abuse/assault (1751, 1752, 1753, 1754), Child pornography/distribution/possession (1582, 1519, 1726, 1537), Sex Offender registration failures (4650, 4651, 4652) |
  | **Property** | 1× | Theft, Burglary, Motor Vehicle Theft, Criminal Damage, Arson, Criminal Trespass | Stolen/altered temp plate, vehicle title/registration fraud, vehicle tampering (502P, 5002, 502R, 502T), Other Crime Involving Property (5001), Possession of Burglary Tools (4310), Other Arson/Explosive Incident (5003) |
  | **Nuisance** | 2× | Narcotics, Other Narcotic Violation, Prostitution, Public Peace Violation, Public Indecency, Obscenity, Interference with Public Officer | — |
  | *Excluded (unscored, dropped at ingestion)* | — | Gambling, Non-Criminal, Liquor Law Violation, Deceptive Practice, Ritualism | License Violation (5011), Eavesdropping (500E), Money Laundering (4800), Smoking Ban Violation (5009), Parole/Probation Violation (4625, 4510), Summary Closure Violation (5013), Hazardous Materials Violation (501H), Internet Offenses - Electronic Fencing (1147), Contribute to (Criminal) Delinquency of Child (1720, 1725), Harbor Runaway (1791) |

  This taxonomy and the 5×/5×/1×/2× weighting were worked out collaboratively (not a purely mechanical FBI UCR mapping) — see PRD §5.9 for the reasoning per category. Any `iucr` sub-code not listed above and not covered by a top-level `primary_type` row falls to the ingestion CLI's judgment; extend this table when a new one is encountered rather than guessing silently. `5121` (Arsonist registration) was found and added during the crimes parser build by re-running the same live group-by queries — it follows the established pattern that offender-registration-failure codes inherit the category of the underlying offense they register for, same as the Gun/Violent/Sex Offender codes already in this table.
- **Pagination:** trailing 12 months is ~225K rows — far past SODA's 1000-row default page cap. Socrata's own docs recommend filtering over deep `$offset` paging (performance degrades as the offset grows), and the internal `:id` field doesn't support `>` comparisons for this dataset. The real `id` column is declared `number` type in the dataset's own metadata (confirmed via `/api/views/ijzp-q8t2.json`), so the ingestion CLI pages via `id`-based keyset pagination (`ORDER BY id`, `WHERE id > <last seen>`) instead — O(1) per page regardless of depth, rather than degrading like `$offset` does.
- **Null coordinates:** ~0.03% of trailing-12-month rows (62 of 225,034 at time of writing) have no `latitude`/`longitude` at all — skewed toward `THEFT` and `DECEPTIVE PRACTICE` (fraud-type reports with no physical crime scene to geocode), spread across many districts with no single systemic cause. Since `cpd_crimes.geom` is `NOT NULL`, these rows are skipped at ingestion rather than inserted with a fabricated location.
- **Confirmed live/current:** sample pulled during this investigation included records from the current week, confirming the "updated daily" claim in ARCHITECTURE §11.2.

### Bike Routes
- **Resource ID:** `hvv9-38ut` (current). **Not** `3w5d-sru8` — that ID was a search-summary hallucination; it doesn't appear in any real result link. Deprecated prior versions exist at `2u3n-4ewv` (2014) and `8ec2-eaj2` (2021) — don't use those either.
- **Key fields:** `the_geom` (MultiLineString), `displayrou` (the actual category field), `street`, `st_name`, `f_street`/`t_street` (from/to cross streets), `mi_ctrline` (segment mileage).
- **Real `displayrou` categories** (verified via a full group-by count, not a sample): `Neighborhood Greenway`, `Buffered Bike Lane`, `Bike Lane`, `Protected Bike Lane`, `Marked Shared Lane`. **None of these is "trail"** — this dataset is street-based bike infrastructure only; it does not include the Lakefront Trail or 606.
- **Schema decision (pending):** originally assumed as map-only decoration (Phase 4) with a simplistic `protected_lane`/`trail` category split. Being reconsidered as a real scored factor (`S_bike`) given the real quality gradient across these 5 categories — see the in-progress PRD update.

### Parks
- **Resource ID:** `ejsh-fztr` ("CPD_Parks" — the actual populated dataset). **Not** `ej32-qgdr` ("Parks - Chicago Park District Park Boundaries (current)") or `xjk8-wnm8` ("parks") — both returned empty geometry/properties on every query despite HTTP 200 responses; likely stale/orphaned catalog entries despite their titles.
- **Key fields:** `the_geom` (Polygon/MultiPolygon), `park` (name), `park_no`, `park_class`, `acres`, `shape_area`, plus a large number of shapefile-truncated amenity boolean columns (`dog_friend`, `basketball`, `tennis_cou`, etc. — DBF 10-character field name limit artifacts).
- **Real `park_class` values:** `NEIGHBORHOOD PARK`, `COMMUNITY PARK`, `REGIONAL PARK`, `MINI-PARK`, `PASSIVE PARK`, `NATURE PRESERVE PARK`, `UNIMPROVED PARK`, `PARKWAY PARK`, `MAGNET PARK`, **`LINEAR PARK`** (22 rows), `CITYWIDE PARK`, `PARKWAYS PARK`, `PENDING` (unresolved classification — a real data-quality gap to handle gracefully, not error on).
- **Schema decision:** `LINEAR PARK` is almost certainly where The 606/Bloomingdale Trail and Lakefront Trail segments actually live in the data. PRD 5.5's "off-street trail access point" bonus should be computed from `parks WHERE park_class = 'LINEAR PARK'`, not from `bike_routes` — these are two separate datasets that had been conflated under one "trail" concept.
- **Data quality note:** 11 of 617 source polygons have self-intersecting rings (`ST_IsValid` fails) — a real defect in the source shapefile's largest, most complex boundaries (Washington Park, Grant Park, Lincoln Park, Humboldt Park, and others), confirmed by auditing the ingested data, not an ingestion bug. 3 of the 11 are `LINEAR PARK` rows (Major Taylor Bike Trail, Sauganash Trail, Midway Plaisance), so this isn't cosmetic — it would have affected `S_green`'s off-street trail bonus. The ingestion CLI repairs these with `ST_MakeValid()` before insert.

### City Boundary
- **Resource ID:** `qqq8-j68g` ("Boundaries - City")
- **Key fields:** `the_geom` (MultiPolygon GeoJSON), `shape_area`.
- **Confirmed matches original assumption** — no schema changes needed.

### 311 Service Requests (sewer/water backup)
- **Resource ID:** `v6vf-nfxy` ("311 Service Requests" — the unified post-2018 system; a `legacy_record` column flags older pre-2018 rows folded in).
- **Confirmed real `sr_type` value:** `"Water in Basement Complaint"` — exact string match needed for ingestion filtering. Adjacent but distinct types also exist: `"Sewer Cave-In Inspection Request"`, `"Sewer Cleaning Inspection Request"`, `"Alley Sewer Inspection Request"` (infrastructure maintenance requests, not necessarily actual backup incidents — worth a judgment call at ingestion time on whether to include these or filter to `"Water in Basement Complaint"` only).

---

## Not Yet Live-Verified (lower risk — fixed external standards, not Chicago-specific quirks)

- **FEMA NFHL flood zone codes** — official designations are `A`, `AE`, `AH`, `AO`, `AR`, `A99`, `V`, `VE`, `X`, `D`. This is a fixed, published national standard (confirmed via FEMA's own glossary), not a dataset-specific quirk, so the risk of a Chicago-specific surprise here is low. Still worth a live sample of the actual NFHL shapefile/GDB export before finalizing the `fema_flood_zones` migration, since field naming in the GIS export itself hasn't been checked.
- **OSM POI tags** (`shop=supermarket`, `shop=convenience`, `amenity=pharmacy`) — standardized global OSM tagging conventions, very low risk of surprise.
