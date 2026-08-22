# BlockFactor: Third-Party Requirements & Compliance Guardrails

**Status:** Active Reference  
**See also:**
* [PRD.md](./PRD.md) — Product Requirements & Scoring Formulas
* [ARCHITECTURE.md](./ARCHITECTURE.md) — Technical Architecture & Caching Strategy
* [ATTRIBUTION.md](../ATTRIBUTION.md) — Legal Attribution Notices

---

## Overview

BlockFactor integrates multiple external APIs, spatial datasets, and transit feeds. Because BlockFactor is a closed-source platform with a planned commercial monetization layer in Phase 2, we must strictly adhere to the terms of service (ToS), licensing limitations, and technical guardrails of each integration.

---

## 1. OpenStreetMap & Nominatim (Geocoding)

### Guardrails & Policies:
* **Nominatim Usage Policy:**
  * The public endpoint (`nominatim.openstreetmap.org`) is strictly limited to **1 request per second** and is funded by donations for non-commercial open-source research.
  * Bulk scraping, automated background workers, or commercial traffic on the public endpoint violates the OSM Usage Policy.
* **Compliance Strategy:**
  * **Phase 1 (Development / Personal Use):** Permissible to use public Nominatim with a valid `User-Agent` header (`NOMINATIM_USER_AGENT`) and low request volume ($<1\text{ req/sec}$).
  * **Phase 2 (Pre-Launch & Commercial):** **Must transition to a self-hosted Nominatim container** on our own hardware or cloud server. Self-hosting eliminates all external rate limits, ensures 100% ToS compliance, and removes third-party dependencies.
* **ODbL License Attribution:**
  * Any map view or public report displaying OSM spatial data must display the attribution string: `© OpenStreetMap contributors` with a hyperlink to `https://www.openstreetmap.org/copyright`.

---

## 2. Google Places API (New)

### Guardrails & Policies:
* **Caching Policy (Section 3.2.3 of Google Maps Platform Terms):**
  * Google strictly prohibits pre-fetching, indexing, storing, or caching Google Maps Content except for temporary caching solely to improve the performance of your application.
  * Content may be cached for **up to 30 consecutive calendar days**, after which the cached data must be deleted or refreshed.
  * You may **permanently store Place IDs** (`place_id`), but place details (names, ratings, addresses) must adhere to the 30-day TTL.
* **Compliance Strategy:**
  * The PostgreSQL `spatial_grid_cache` table strictly enforces `expires_at = NOW() + INTERVAL '30 days'`.
  * Background cleanup scripts or Postgres TTL expiration queries automatically invalidate expired POI records.
* **Attribution:**
  * If individual Google POI business reviews or photos are rendered in the UI, the standard "Powered by Google" logo and author attributions must be displayed.

---

## 3. Chicago Transit Authority (CTA) GTFS & Data Portal

### Guardrails & Policies:
* **CTA Developer License Agreement:**
  * Data is provided "as is". CTA does not guarantee accuracy of real-time positions or schedule adherence.
  * We cannot state or imply that BlockFactor is an official CTA application or endorsed by the CTA.
* **Schedule Updates:**
  * CTA updates GTFS static schedules quarterly (effective with seasonal service changes in spring, summer, fall, winter).
  * The OTP2 graph build pipeline must be refreshed on quarterly release cadences to reflect current headways and route adjustments.

---

## 4. Divvy / Lyft GBFS Feeds

### Guardrails & Policies:
* **GBFS Open Data Standard:**
  * Real-time station capacity (available bikes, e-bikes, and open docks) is published via GBFS JSON feeds.
  * High-frequency polling directly to Lyft's CDN is discouraged.
* **Compliance Strategy:**
  * BlockFactor enforces an **in-memory Go cache with a 5-minute TTL** (`sync.Map`). All incoming user requests within that 5-minute window read from local memory, avoiding rate-limiting on the public GBFS CDN.

---

## 5. FCC National Broadband Map API

### Guardrails & Policies:
* **Public Data Usage:**
  * FCC Broadband serviceable location fabric data is public domain.
  * The dataset is officially updated bi-annually by the FCC (June and December filings).
* **Compliance Strategy:**
  * Address broadband records are permanently cached per geocoded coordinate point and refreshed semi-annually.

---

## 6. City of Chicago Data Portal (SODA API)

### Guardrails & Policies:
* **Socrata API Rate Limits:**
  * Unauthenticated SODA requests are throttled at lower hourly quotas.
  * Authenticated requests with an App Token (`X-App-Token`) enjoy generous rate limits ($>10,000\text{ req/hr}$).
* **Compliance Strategy:**
  * The `cmd/seed` ingestion tool uses parameterized `$where` and `$limit` clauses to pull only trailing 12-month records for crimes rather than fetching entire multi-year historical archives ($>7\text{ million}$ rows).
