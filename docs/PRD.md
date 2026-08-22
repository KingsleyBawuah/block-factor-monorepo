# BlockFactor: Product Requirements Document

**Version:** 1.0 (Draft / In Review)  
**Target Market:** Chicago, IL  
**Repository Structure:** Monorepo (Go API + React/TanStack Frontend + PostGIS / Docker Services)  
**License:** Proprietary / All Rights Reserved  

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
- Target street address (geocoded via Nominatim).

### 3.2 Optional Unit Metadata
- **Primary Window Facing Direction:** N, NE, E, SE, S, SW, W, NW
- **Floor Level:** Integer (default: 1)
- **Unit Exposure:** Street-facing, Courtyard/Interior, Alley-facing

### 3.3 User Customization
- Sliders (0% to 100%) to individually weight the following factors, with **opinionated defaults** reflecting the primary persona's priorities:

| Factor | Default Weight | Rationale |
|---|---|---|
| Transit (`S_transit`) | 30% | Highest priority — core value proposition for car-free living |
| Groceries (`S_grocery`) | 20% | Daily necessity; walking distance to a full supermarket |
| Third Places (`S_third_place`) | 15% | Vital for neighborhood vibe, community, and remote work viability |
| Green Space & Parks (`S_green`) | 10% | Outdoor recreation, Lakefront/606 trail access, and mental well-being |
| Noise Environment (`S_noise`) | 10% | Acoustic comfort; balances transit proximity vs. 'L' track & arterial roar |
| Public Safety (`S_safety`) | 10% | Baseline security against violent & property incident density |
| Flood Risk (`S_flood`) | 3% | Guardrail against chronic Chicago basement/garden-unit flooding |
| Solar Exposure (`S_solar`) | 2% | Unit-specific natural light quality |

> Weights are dynamically normalized at runtime so they always sum to 100%, even when customized by the user.

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
| **Green Space** | Parks & Forest Preserves | Chicago Data Portal + OSM `leisure=park` | PostGIS spatial polygon table; sync quarterly. |
| **Noise Environment** | Track & Traffic Buffers | OSM Road Hierarchy + CTA Track Geometry | Deterministic spatial buffer evaluation in PostGIS. |
| **Flood Hazard** | FEMA Zones & Basement 311s | FEMA NFHL + Chicago Data Portal 311 | PostGIS polygon overlays + 311 sewer backup incident density. |
| **Public Safety** | Reported Incident Density | Chicago Data Portal (Crimes) | Socrata API query (`$where=within_circle`) over trailing 12 mo. |
| **Solar Exposure** | Sunlight Angles & Hours | Astronomical Math (`suncalc`) | Local deterministic calculation based on coordinates/azimuth. |
| **Utilities** | Broadband & Fiber Access | FCC National Broadband Map API | Geocoded coordinate lookup; cached per address (refreshed bi-annually). Powers broadband specs and affiliate CTAs. |

---

## 5. Scoring Formula & Logic

### 5.1 Composite Score Formula

```
BlockFactor = Σ(wᵢ · Sᵢ) / Σwᵢ
```

A weighted average of individual factor scores, where `wᵢ` are the user-configured weight sliders and `Sᵢ` are the normalized sub-scores (0–100).

---

### 5.2 Transit Score (`S_transit`)

- **100 pts** at ≤ 0.25 miles from a CTA 'L' stop (~5 min walk).
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

> **Note:** Corner stores / bodegas cap at a maximum of 40 points.
> **Pharmacy Bonus:** A pharmacy/drugstore (Walgreens, CVS) within ≤ 0.2 miles provides a +10 pt boost (capped at 100 total).

---

### 5.4 Third Places Score (`S_third_place`)

- Evaluates cached Google Places POIs within 0.5 miles.
- **Benchmark for 100 pts:** ≥ 3 independent cafes AND ≥ 1 fitness center/studio within 0.4 miles.

---

### 5.5 Green Space & Parks Score (`S_green`)

- **100 pts** at ≤ 0.1 miles from a recognized public park or community garden entrance.
- Decays to **50 pts** at 0.3 miles, and **0 pts** at 0.75 miles.
- **+15 pt bonus** for direct off-street trail corridor access (Lakefront Trail, 606/Bloomingdale Trail).

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

### 5.9 Public Safety Score (`S_safety`)

- Performs a radial query against Chicago Data Portal crimes over the trailing 12 months.
- **Penalty Calculation:** `Penalty = (5 × Violent Count) + (1 × Property Count)`
  - Violent crimes (assault, robbery, homicide) are weighted 5× heavier than property crimes (theft, burglary, vandalism).
- Final score normalized against **Chicago block percentiles** — meaning the score is relative, not absolute.

#### How Percentile Normalization Works

1. At data ingestion time, we pre-compute the penalty score for a representative grid of ~500m blocks across Chicago and store the distribution.
2. When a user submits an address, we compute that address's raw penalty score and find where it falls in the city-wide distribution.
3. A block at the **0th percentile** (lowest crime) scores **100**. A block at the **99th percentile** (highest crime) scores **0**.
4. This means scores are always meaningful *relative to Chicago as a whole* — a score of 70 means "safer than 70% of Chicago blocks."

---

## 6. Monetization & Native Affiliate Catalog

BlockFactor integrates non-intrusive, context-triggered **Native Affiliate Cards** into the address report.

### 6.1 Active v1 Affiliate Partners

| Category | Partner(s) | Trigger / Context | Compensation Model |
|---|---|---|---|
| **Broadband & Fiber** | AT&T Fiber, Astound Broadband, Comcast/Xfinity | FCC broadband card showing available ISP speeds & fiber lines | $50 – $120 CPA per activation |
| **Insurance (Multi-Product)** | **Lemonade** (Renters, Pet, Condo/Home) | Standard report summary, with dynamic pet bundle option | $10 – $25 CPA per policy bound |
| **On-Demand Moving Help** | **Dolly** | "Moving to this address?" CTA for local/micro apartment moves | ~$15 CPA per completed job |
| **Moving Quote Aggregator** | **Unpakt** | Full-service moving quote comparison card | $10 – $25 CPA per verified quote |
| **Container & Self-Move** | **PODS / U-Box** (U-Haul) | Relocation comparison modal for mid-to-long distance moves | 5% – 15% booking rev-share |
| **Off-Site Storage** | **CubeSmart / Extra Space Storage** | Unit size/lifestyle section (apartment storage optimization) | $20 – $50 CPA per unit rental |
| **Fitness & Third Places** | **ClassPass** | Displayed next to `S_third_place` fitness studio / gym count | $30 – $60 CPA per trial/membership |

### 6.2 Future Expansion Roadmap (Buyer Mode)

When buyer mode is activated, high-ticket mortgage and homeowner services will augment the affiliate drawer:
- **Digital Mortgage Origination:** Better.com ($100 – $200+ CPA per closed loan)
- **Mortgage Comparison:** Credible / LendingTree ($50 – $100 CPA per lead)
- **Homeowners/Condo Insurance:** Kin Insurance ($30 – $50 CPA per quote)
