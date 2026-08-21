# BlockFactor: Product Requirements Document

**Version:** 1.0 (Draft / In Review)
**Target Market:** Chicago, IL
**Repository Structure:** Monorepo (Go API + React/TanStack Frontend + PostGIS / Docker Services)
**License:** Proprietary / All Rights Reserved

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
- Target street address (geocoded via Nominatim).

### 3.2 Optional Unit Metadata
- **Primary Window Facing Direction:** N, NE, E, SE, S, SW, W, NW
- **Floor Level:** Integer (default: 1)
- **Unit Exposure:** Street-facing, Courtyard/Interior, Alley-facing

### 3.3 User Customization
- Sliders (0% to 100%) to individually weight the following factors, with **opinionated defaults** reflecting the primary persona's priorities:

| Factor | Default Weight | Rationale |
|---|---|---|
| Transit | 35% | Highest priority — core value proposition for car-free living |
| Groceries | 25% | Daily necessity; walking distance to a supermarket is a hard requirement for many |
| Third Places | 20% | Strong signal for neighborhood quality-of-life and remote work viability |
| Public Safety | 15% | Important but less differentiating across Chicago's transit-rich neighborhoods |
| Solar Exposure | 5% | Highly personal; apartment hunters rarely lead with this |

> Weights are normalized at runtime so they always sum to 100%, even if the user adjusts individual sliders.

---

## 4. Data Sources & Ingestion Strategy

| Domain | Attribute | Data Source | Ingestion & Cache Strategy |
|---|---|---|---|
| **Transit** | 'L' Rail Stops & Lines | Chicago Data Portal (CTA 'L' Stops) | PostGIS spatial point table; sync monthly. |
| **Transit** | High-Frequency Bus Lines | CTA Static GTFS Schedule | PostGIS; routes with ≤ 10 min headways. Sync quarterly. |
| **Micro-Mobility** | Divvy Docks & E-Bikes | Lyft / Divvy GBFS JSON Feed | Station coords static; live capacity cached in-memory (5m TTL). |
| **Cycling** | Protected Lanes & Trails | Chicago Data Portal (Bike Routes) | PostGIS linestring buffering; filter for protected lanes/trails. |
| **Travel Times** | Commute Durations | Self-Hosted OpenTripPlanner 2.x | Local HTTP API; computed on-demand via OSM + GTFS. |
| **Sustenance** | Supermarkets & Pharmacies | OpenStreetMap (Overpass API) | Ingested via Illinois OSM extract into PostGIS. |
| **Third Places** | Cafes, Gyms & Studios | Google Places API (New) | Nearby Search Pro; cached in PostGIS spatial grids (30-day TTL). |
| **Public Safety** | Reported Incident Density | Chicago Data Portal (Crimes) | Socrata API query (`$where=within_circle`) over trailing 12 mo. |
| **Solar Exposure** | Sunlight Angles & Hours | Astronomical Math (`suncalc`) | Local deterministic calculation based on coordinates/azimuth. |
| **Utilities** | Broadband & Fiber Access | FCC National Broadband Map API | Geocoded coordinate lookup; cached per address (refreshed bi-annually to align with FCC update cadence). Powers both the report's broadband availability display and the fiber affiliate CTAs. |

---

## 5. Scoring Formula & Logic

### 5.1 Composite Score Formula

```
BlockFactor = Σ(wᵢ · Sᵢ) / Σwᵢ
```

A weighted average of individual factor scores, where `wᵢ` are the user-configured weight sliders and `Sᵢ` are the normalized sub-scores (0–100).

---

### 5.2 Transit Score (`S_transit`)

- **100 pts** at ≤ 0.25 miles from a CTA 'L' stop. (~5 min walk)
- Linear decay to **0 pts** at 1.0 mile.
- **+10 pt bonus** for dual-line access OR a high-frequency bus stop (≤ 10 min headways) within 0.2 miles.

---

### 5.3 Grocery Score (`S_grocery`)

| Distance to Supermarket | Score |
|---|---|
| ≤ 0.3 miles | 100 |
| 0.3 – 0.75 miles | 75 |
| 0.75 – 1.0 miles | Linear decay from 75 → 0 |
| > 1.0 miles | 0 |

> **Note:** Corner stores cap at a maximum of 40 points.

> **TODO (open question):** Should pharmacy proximity contribute a secondary grocery sub-score? e.g., a CVS/Walgreens within 0.2 miles could add +10 pts for "daily essentials" coverage.

---

### 5.4 Third Places Score (`S_third_place`)

- Evaluates cached Google Places POIs within 0.5 miles.
- **Benchmark for 100 pts:** ≥ 3 independent cafes AND ≥ 1 fitness center within 0.4 miles.

---

### 5.5 Solar Score (`S_solar`)

**Base score by window direction:**

| Direction | Base Score |
|---|---|
| South (180°) | 100 |
| East (90°) | 85 |
| West (270°) | 75 |
| North (0°) | 40 |

**Exposure Modifier:**

| Exposure Type | Multiplier |
|---|---|
| Street-facing | 1.0× |
| Courtyard/Interior | 0.65× |
| Alley-facing | 0.50× |

**Floor Bonus:** +5 pts per floor above floor 2 (e.g., floor 3 = +5, floor 5 = +15).

`S_solar = BaseScore × ExposureModifier + FloorBonus`

---

### 5.6 Public Safety Score (`S_safety`)

- Performs a radial query against Chicago Data Portal crimes over the trailing 12 months.
- **Penalty Calculation:** `Penalty = (5 × Violent Count) + (1 × Property Count)`
  - Violent crimes (assault, robbery, homicide) are weighted 5× heavier than property crimes (theft, burglary, vandalism).
- Final score normalized against **Chicago block percentiles** — meaning the score is relative, not absolute.

#### How Percentile Normalization Works

1. At data ingestion time, we pre-compute the penalty score for a representative grid of ~500m blocks across Chicago and store the distribution.
2. When a user submits an address, we compute that address's raw penalty score and find where it falls in the city-wide distribution.
3. A block at the **0th percentile** (lowest crime) scores **100**. A block at the **99th percentile** (highest crime) scores **0**.
4. This means scores are always meaningful *relative to Chicago as a whole* — a score of 70 means "safer than 70% of Chicago blocks."

> **Open Question:** Should the percentile distribution be computed city-wide, or scoped to comparable neighborhoods (e.g., only transit-dense, dense urban blocks)? City-wide is simpler but may punish dense urban areas unfairly vs. low-density suburban-style blocks.

> **Open Question:** What query radius should we use for the radial crime lookup? 0.25 miles (1–2 block radius) is very hyper-local; 0.5 miles captures more signal but may bleed into adjacent neighborhoods.

---

## 6. Monetization Model

Contextual, non-intrusive **Native Utility Affiliate Cards** placed at the bottom of each generated report.

| Category | Partners | Payout |
|---|---|---|
| Home Broadband / Fiber | AT&T Fiber, Astound, Comcast/Xfinity | $50 – $120 per activation |
| Renters Insurance | Lemonade, Policygenius | $10 – $25 per policy bound |
| Moving Services | Truck rentals & movers | 5% – 15% booking revenue share |
