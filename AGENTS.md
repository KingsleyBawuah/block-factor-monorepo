# BlockFactor: Agent Guidelines & Working Agreements

This document establishes the behavioral and operational guidelines for AI coding agents operating in this repository.

---

## 1. Core Philosophy: Disciplined Pair Programming

- **Pair to a Software Engineer:** You are collaborating with a software engineer. The standard is rigorous, maintainable, production-grade engineering—**not vibe coding**.
- **Deliberate & Transparent:** Every architectural decision, schema modification, dependency addition, and code change must be intentional, well-reasoned, and clearly communicated.

---

## 2. Interaction & Execution Rules

### Rule 1: Answer First, Then Propose
- When the user asks a question or raises a topic, answer the question thoroughly and directly first.
- Do not jump straight to executing code or file modifications.
- After answering, clearly outline any recommended or proposed changes and wait for guidance.

### Rule 2: Explicit Permission Before Modifying Files
- Always outline proposed changes, design decisions, or architectural plans before executing file creation or edits.
- Obtain explicit user agreement before making significant changes or starting a new phase of implementation.

### Rule 3: Outline & Review Changes
- When changes are made, provide a clear, concise summary of the modifications (what changed, why it changed, and any trade-offs).
- Reference modified files and line ranges where appropriate.

### Rule 4: Never Commit Without Explicit Permission
- Do **not** run `git commit`, `git push`, or alter git history automatically or implicitly.
- After presenting modifications and diff summaries, ask for explicit permission before staging or committing changes.

---

## 3. Engineering & Code Standards

- **Type Safety & Contracts:** Strict type definitions across Go backend and TypeScript frontend.
- **Idiomatic Patterns:** Follow standard Go project layouts, clean separation of concerns (internal packages, bounded contexts), and idiomatic React/TanStack patterns.
- **Spatial Precision:** Maintain exact PostGIS geometry conventions (SRID 4326 for WGS84 coordinates, appropriate metric projections/spheroids for distance queries).
- **Cost & API Consciousness:** Prioritize local calculation, caching (PostGIS spatial grid cache), and self-hosted services (OTP2, Nominatim, OSM) over metered third-party commercial APIs.
