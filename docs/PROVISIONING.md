# BlockFactor: External Service Provisioning

**Status:** Active Reference
**See also:**
* [THIRD_PARTY_REQUIREMENTS.md](./THIRD_PARTY_REQUIREMENTS.md) — ToS compliance guardrails each integration must follow
* [ARCHITECTURE.md](./ARCHITECTURE.md) — §13 Pre-Flight Decisions (cloud/billing identity)
* [PRIVACY_AND_DATA_HANDLING.md](./PRIVACY_AND_DATA_HANDLING.md) — data handling guarantees these accounts must uphold

---

## Overview

THIRD_PARTY_REQUIREMENTS.md documents the ToS/compliance *rules* each external integration must follow. This document is the setup runbook: what was actually provisioned, how it's restricted, and how to reproduce or rotate it. Update it whenever an account, key, or budget configuration changes — it should always reflect what's actually live, not what was originally planned.

---

## 1. Google Cloud Platform (Places API)

**Account:** Developer's personal Google account. A dedicated GCP project was created for BlockFactor rather than reusing an existing project (Home Assistant, etc.) — see ARCHITECTURE §13.1 for the identity decision and its Phase 2 revisit trigger (dedicated business identity before public launch).

**API enabled:** Places API (New) — *not* the legacy Places API.

**API key:**
- Name: `blockfactor-places-dev`
- API restrictions: locked to Places API (New) only — confirmed applied.
- Application restrictions: **open item** — not yet applied. ISP is Xfinity (residential gateway); IP is assumed dynamic but unconfirmed in practice. Restrict to the home server's outbound IP before the Places client goes live in Phase 3 *only if* the check below shows it's stable.

**IP stability check (in progress):** run `curl -4 ifconfig.me` directly on the Beelink server (not through Tailscale) to get the real outbound IP, and log a reading periodically (including after a gateway reboot) before deciding whether to apply the IP restriction.

| Date | Public IP | Notes |
|---|---|---|
| 2026-08-22 | 98.34.58.45 | First reading |

**Budget / cost control:**
- Scope: this project only, Places API (New) only (not "All projects" / "All services")
- Type: **Alerts only** — spend cap enforcement wasn't available for Places API (New) as of setup; worth rechecking periodically since it's still in preview and may add coverage later.
- Amount: **$10/month**
- Thresholds: 50% ($5, actual), 90% ($9, actual), 100% ($10, actual), 100% ($10, forecasted)
- The AI Pro $10/month Cloud credit is intentionally **not** applied to this budget's savings, so the alert triggers on gross spend rather than net-of-credit.
- This amount is a placeholder pending real Phase 3 usage data — revisit once the Places client ships and a few weeks of dev-testing volume is observed.

**Environment variable:** `GOOGLE_PLACES_API_KEY` in `.env` (never committed — `.env` is gitignored; only `.env.example` is tracked, with the key left blank).

---

## 2. Chicago Data Portal (Socrata SODA API)

**Account:** Free account created at `data.cityofchicago.org`.

**App Token:** Generated via account → Developer Settings → Create New App Token.

**Why:** Lifts requests from Socrata's throttled unauthenticated tier to the authenticated tier (>10,000 req/hr) — see THIRD_PARTY_REQUIREMENTS §6. No billing involved.

**Environment variable:** `CHICAGO_APP_TOKEN` in `.env` (never committed).

---

## 3. Rotation & Incident Notes

- If the Places API key is ever exposed (committed to git, leaked in a client-side bundle, etc.), regenerate it immediately in GCP Console → Credentials, update `.env`, and restart affected services. The API + application restrictions limit blast radius but don't eliminate it.
- If the Chicago Data Portal token needs rotating, generate a new one in Developer Settings — the old one can be revoked independently without affecting other integrations.
- Neither credential grants access to any other project or service under the developer's personal Google account — see PRIVACY_AND_DATA_HANDLING.md and ARCHITECTURE §13.1 for the isolation rationale.

---

## 4. Open Items

- [ ] Confirm/apply IP-based application restriction on the Places API key.
- [ ] Revisit the $10/month budget once Phase 3's Places client is live and real usage data exists.
- [ ] Re-check whether spend cap enforcement has added Places API (New) coverage, and switch from alerts-only if so.
