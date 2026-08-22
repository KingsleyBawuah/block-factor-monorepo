# BlockFactor: Privacy & PII Data Handling Guidelines

**Status:** Active Reference
**See also:**
* [PRD.md](./PRD.md) — Product Requirements & Input Fields (§3)
* [ARCHITECTURE.md](./ARCHITECTURE.md) — Caching Architecture (§5.4) & Observability (§10)
* [THIRD_PARTY_REQUIREMENTS.md](./THIRD_PARTY_REQUIREMENTS.md) — External API Compliance Guardrails
* [AGENTS.md](../AGENTS.md) — Engineering Standards & Working Agreements

---

## Overview

BlockFactor's core input — a person's home address, and optionally their workplace address — is inherently sensitive. An application built primarily through agentic engineering carries an extra burden of proof here: users have every reason to be skeptical that an "AI-built" app takes data handling seriously. This document exists to make that case concretely, not just assert it. Every rule below is enforceable and checkable in code review, not aspirational copy.

---

## 1. Principles

- **Data minimization first.** Collect only what a factor calculation actually needs. If a score can be computed from a coordinate, never persist the raw address string alongside it.
- **Never sell or share user data.** No user input, derived score, or behavioral signal is sold, rented, or shared with third parties for their own purposes. Third-party API calls (§4) are made *on the user's behalf* to compute a score — not to build a profile of the user for anyone else.
- **Prefer derived data over raw data.** A geocoded coordinate, a geohash, a grid cell, or a percentile rank is preferred wherever it satisfies the need — the raw address string should not travel further through the system than the geocoding step requires.
- **Every stored value has a stated lifetime.** Nothing address-linked is retained indefinitely without an explicit TTL and rationale (§3).

---

## 2. What We Collect — and What We Don't

### Collected (per PRD §3):

| Input | Sensitivity | Required? |
|---|---|---|
| Home street address | High — geocoded to an exact coordinate | Mandatory |
| Workplace/destination address | High — same as above | Optional |
| Unit metadata (window facing, floor, exposure) | Low — not individually identifying | Optional |
| Custom weight sliders | None | Optional |

### Explicitly not collected in v1:

- **No user accounts.** No email, password, or persistent user profile.
- **No device fingerprinting or cross-session tracking.** Prometheus metrics are never labeled by address or address hash (ARCHITECTURE §10.2) — this avoids both unbounded label cardinality *and* building any per-user history server-side.
- **No sale of behavioral data.** Affiliate click instrumentation (§4) records that a click happened and its context (partner, placement, intent level) — never the address that triggered it.

> **Why this matters for trust:** "We don't even ask for your email" is a concrete, verifiable claim — not a policy promise. As soon as accounts exist (Phase 2+), this section must be revisited (§7).

---

## 3. Persisted vs. Ephemeral

| Data | Storage | Lifetime | Keyed By |
|---|---|---|---|
| Google Places POI cache | `cache_entries` (LOGGED) | 30 days (Google ToS max) | 400m grid cell — **not** raw address |
| OTP2 commute route durations | `cache_entries` (LOGGED) | 7 days | Origin/dest snapped to 100m grid + mode + window — **not** raw address |
| FCC broadband records | `cache_entries` (LOGGED) | Refreshed bi-annually | Geocoded coordinate — **not** the address string (tightened from the PRD/ARCHITECTURE "cached per address" wording: the cache key is the coordinate the address resolved to, never the string itself) |
| Divvy GBFS station status | In-memory (`sync.Map`) | 5 minutes | Station ID (public infrastructure data, not user-linked) |
| Safety percentile grid | `safety_grid_percentiles` | Recomputed monthly | ~500m citywide grid (aggregate, not user-linked) |

**Rule:** every new cache table added in Phase 2+ must key on a coordinate, grid cell, or derived value — never the raw address string — unless there is a documented reason it can't (and if so, it needs its own TTL and a note added to this table).

**What is never persisted at all:** the raw address string itself, once geocoded. It exists in request memory for the duration of the geocode + score request and is not written to any table.

---

## 4. Third-Party Data Flows

| Service | What It Receives | Why | Self-Hostable? |
|---|---|---|---|
| Nominatim (geocoding) | Raw address text | Required to resolve to coordinates | Yes — Phase 2 self-hosts this, removing the third-party exposure entirely (THIRD_PARTY_REQUIREMENTS §1) |
| Google Places API | Coordinates only (never the address string) | POI search for `S_third_place` | No |
| FCC Broadband Map | Coordinates only | Broadband availability lookup | No |
| OpenTripPlanner 2 | Coordinates only | Commute routing | Yes — always self-hosted, never leaves our infrastructure |
| Affiliate partners (Lemonade, Dolly, etc.) | Click event + intent-level classification + UTM/subid | Conversion tracking (PRD §6.1) | N/A |

**Rule:** affiliate click payloads (PRD §6.1, `blockfactor_affiliate_clicks_total`) may include `partner`, `placement`, and `intent_level` — they must never include the raw address, coordinates, or any workplace destination string. If a future partner integration requires more context to function, that requires a decision recorded in this document, not a silent addition to a click handler.

---

## 5. Logging & Observability Guardrails

Extends the existing rule in ARCHITECTURE §10.1:

- **Logs:** structured `log/slog` fields use `geohash` (truncated location, not exact coordinates) — never raw addresses, never full-precision lat/long.
- **Metrics:** Prometheus metrics are never labeled by address, address hash, or user identifier of any kind (ARCHITECTURE §10.2) — both a cardinality control and a privacy control.
- **Error tracking:** if error tracking/APM is added in a later phase, request payloads (which contain addresses) must be scrubbed before capture — this is a blocking requirement for that integration, not a follow-up.

---

## 6. Retention & Deletion

- Every cache table has a stated TTL (§3) — none are unbounded.
- Derived tables (`safety_grid_percentiles`) are recomputed and swapped, not accumulated (ARCHITECTURE §11.3).
- There is currently no long-term store of anything address-linked: a score request is computed, returned, and its inputs are not retained beyond the caches in §3, which are keyed by coordinate rather than by user or address string.

---

## 7. Open Items for Phase 2+

This document must be revisited — not just extended — once any of the following ship, since each introduces genuinely new PII surface area:

- **Saved reports / share links** (ARCHITECTURE §7.5): once a report is persisted server-side rather than existing only in URL state, it needs its own retention policy and a deletion path.
- **User accounts** (if ever added): requires an explicit data export and account deletion flow before launch, not after.
- **Public-facing Privacy Policy:** this document is the internal engineering source of truth; a plain-language public Privacy Policy page should be derived from it before Phase 2 public launch, not written independently.
