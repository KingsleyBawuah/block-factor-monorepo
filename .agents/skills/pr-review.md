---
name: pr-review
description: Deep, critical review of a GitHub pull request before it merges. Use when asked to review a PR, check a PR before merge, or do a thorough/critical code review of a pull request in this repository.
---

# PR Review

Perform a maximally rigorous, critical review of the given pull request before it merges. This is not a rubber-stamp pass — assume there is a real defect to find, and don't stop until you've either found one or genuinely exhausted the angles worth checking. An "LGTM" with nothing wrong is a legitimate outcome, but it should be earned by actually looking, not defaulted to.

## Setup: isolated worktree, never the shared working directory

**Never run `git checkout` or `gh pr checkout` in the project's primary working directory (`block-factor-monorepo/`) or in another agent's worktree.** Doing so silently switches that directory's branch out from under whatever is running there — this has already happened once (during review of PR #39) and disrupted an in-progress task elsewhere. Always work from your own dedicated worktree instead — see `AGENTS.md` §6 "Git Worktree Isolation" for the full convention this follows.

1. From the repository root, check whether `../block-factor-monorepo.worktrees/agy/` already exists.
   - If it exists, `cd` into it.
   - If not, create it once (reuse the same directory for every future review): `git worktree add ../block-factor-monorepo.worktrees/agy main`
2. Inside your worktree: `git fetch origin && git checkout main && git pull` to sync the trunk to latest before reviewing anything against it.
3. Check out the PR's actual branch inside your worktree only: `gh pr checkout <PR-number>`. This is safe now — it only affects your own worktree, not the primary clone or Claude's worktree.
4. Copy `.env` into your worktree if it isn't already there, and run `bun install` under `packages/web/` if the PR touches frontend code — a fresh worktree only contains git-tracked files, so gitignored local config and installed dependencies don't carry over automatically.

## Review approach

- **Read the PR description and every commit message first** — understand what the author claims the change does and why, so you can check the code against its own stated intent, not just against your own assumptions.
- **Read the full diff with surrounding context**, not just the added/removed lines — open the changed files directly (or `git show`) so you can see how a change interacts with the rest of the function/file, not just the +/- hunks in isolation.
- **Verify claims against real evidence — don't just pattern-match.** Before flagging something, check it against the actual system: if a migration adds a sentinel value, check what `NUMERIC` precision it's really stored in before calling it an overflow risk; if a query assumes a table's shape, look at the live table; if a formula claims specific behavior at a boundary, trace through real numbers by hand or run the code. A demonstrated failure case (concrete inputs → wrong output, reproduced) is much stronger evidence than "this looks wrong" — produce one whenever it's cheap to do (a short script, a live `psql` query, a constructed test case, a `git log`/`grep` check of how similar code elsewhere actually behaves).
- **Be specific.** Cite exact file paths and line numbers, quote the actual code, and describe the concrete failure scenario (these inputs produce this wrong output) rather than an abstract category of bug.
- **Diagnose root cause, not just the symptom.** If you find a bug, check why the existing tests didn't catch it. That answer often points to a second, more durable fix worth calling out alongside the primary one — e.g. a test fixture that never modeled the real-world shape of the input, not just the one code path that mishandled it.
- **Push back on your own draft findings before posting them.** If a finding rests on an assumption (a specific numeric threshold, a specific data shape), verify the assumption first — a review that raises a concern which turns out to be factually wrong costs the author more time than it saves. It's fine to flag something as a genuine open question if you can't verify it cheaply, but say so explicitly rather than stating it as fact.

Cover, in roughly this order of priority:

1. **Correctness** — logic errors, off-by-one/boundary mistakes, incorrect formulas, wrong SQL, race conditions, error-handling gaps.
2. **Test coverage** — does the test suite actually exercise the new/changed behavior, including the specific edge cases and failure modes above? A test that would still pass if the bug were reintroduced is not real coverage — check this by mentally (or actually) reverting the fix against the test.
3. **Architecture & consistency** — does the change match existing patterns in the codebase (naming, error wrapping, migration style, query structure, staging-swap vs. plain-transaction choices)? Check `AGENTS.md`, `ARCHITECTURE.md` (especially the Decision Log), and `PRD.md` for the standards and prior decisions this change should be consistent with, and for whether a new decision here should be recorded there too.
4. **Security** — injection, unsafe deserialization, secrets in code/logs, missing auth checks, unvalidated external input, PII handling (see `docs/PRIVACY_AND_DATA_HANDLING.md`).
5. **Performance** — obviously wasteful queries/loops, missing indexes for a new query pattern, N+1s, unnecessary DB round-trips for data that can't affect the result.
6. **Documentation** — does `CHANGELOG.md` have an entry if the change is user-observable (see `AGENTS.md` §2 Rule 6 for the type-to-section mapping)? Do doc comments and `docs/*.md` still match the new behavior?

## Parallelize with subagents

Spawn subagents to cover the angles above in parallel rather than working through the list serially — one subagent per major concern area (correctness, test coverage, architecture/consistency, security/performance) is a reasonable split. Have each report back its findings with file:line references and a severity rating, then synthesize them into one coherent review yourself rather than posting each subagent's output separately or redundantly.

## Reporting findings

Structure the review like this:

1. **Executive summary & verdict** — approve, approve with minor recommendations, or changes requested — plus a one-paragraph summary of why.
2. **Findings, ranked most-severe first**, each with:
   - A severity marker (critical / warning / suggestion).
   - The exact file and line(s).
   - What's wrong, concretely — the failure scenario, not just a category.
   - A specific suggested fix.
3. **A short recommended action plan** — the concrete list of changes needed before merge, if any.

Post this as an actual PR comment or review (`gh pr comment`/`gh pr review` — see the `permissioned-github` skill for the exact permission-request flow) rather than only reporting back in-session, so the review leaves a record on the PR itself that other agents and the user can see and respond to.

## When you're done

Leave your worktree checked out to the PR branch you just reviewed rather than switching it back to `main` — if the review prompts a quick follow-up look, the branch is still there. It's fine for the worktree to sit on a stale branch between reviews; just repeat the fetch/checkout/`gh pr checkout` steps the next time you review something new.
