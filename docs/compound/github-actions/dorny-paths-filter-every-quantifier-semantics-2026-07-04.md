---
chunk_strategy: h1-h2-h3
description: Source-verified semantics of the dorny/paths-filter predicate-quantifier every option; a positive multi-pattern allowlist under every is a constant-false no-op, so detect out-of-allowlist changes with all-negated patterns and a fail-safe gate direction.
doc_type: learning
docline:
    ms.date: 2026-07-04T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-05T01:50:00Z"
schema_version: "1.0"
source: docs/compound/github-actions/dorny-paths-filter-every-quantifier-semantics-2026-07-04.md
title: 'Compound: dorny/paths-filter predicate-quantifier every semantics (constant-false positive-allowlist no-op)'
---

## Compound: dorny/paths-filter predicate-quantifier every semantics

**Date**: 2026-07-04
**Discovered during**: Stage plan-review of the CI cost-gating design (stash `D760E508`), which FAILed three consecutive review rounds on this exact mental-model error. Captured here as a durable, tool-factual learning; the broader CI cost-gating workflow design remains **UNREVIEWED and deferred** (see `docs/exec-plans/2026-07-04-ci-cost-gating-plan.md` and `docs/memory/2026-07-04-stage-ci-gating-closure-compaction-session.md`).
**Scope**: third-party GitHub Action `dorny/paths-filter` change-detection semantics — independent of whether the deferred gating design ships.

## Problem

When gating expensive CI jobs so that docs-only PRs skip heavy build/test steps, the intuitive design is a **positive allowlist** of "unsafe" (code) paths evaluated under `predicate-quantifier: every`, e.g.:

```yaml
# WRONG — constant-false no-op
filters: |
  unsafe:
    - 'cmd/**'
    - 'internal/**'
    - 'schemas/**'
predicate-quantifier: 'every'
```

The author and two rounds of reviewers all held the wrong mental model (assuming `every` means "every changed file must be in the allowlist" or gitignore-style last-match-wins). It does not.

## Source-verified semantics (dorny/paths-filter `src/filter.ts`, README)

- A filter has a list of patterns; each pattern is an independent `picomatch` matcher.
- Under `predicate-quantifier: every`: `isMatch(file) = patterns.every(rule => rule.isMatch(file))` — a **single file** must match **ALL** patterns for that file to count.
- The filter output is `true` iff **some** changed file matches **all** patterns.
- Therefore, with multiple **disjoint positive** path patterns (e.g. `cmd/**`, `internal/**`, `schemas/**`), **no single file can match all of them** → the filter is **constant-false**. The "unsafe" flag never fires, so the gate silently skips heavy work on every PR, including pure code PRs (dangerous fail-open).
- Default quantifier is `some` (`patterns.some(...)`), under which a positive allowlist is fail-**open** for any path not enumerated (unlisted dirs like `scripts/**`, `plugin/**`, `npm/**`, `.mcp.json` skip the heavy jobs).

## Correct pattern

Detect the **existence of an out-of-allowlist (non-docs) changed file** using **all-negated patterns under `every`**:

```yaml
# CORRECT — unsafe=='true' iff any changed file is outside the docs allowlist
filters: |
  unsafe:
    - '!docs/**'
    - '!**/*.md'
    - '!.backlogit/**'
predicate-quantifier: 'every'
```

Under `every`, a file matches `unsafe` iff it satisfies every negation — i.e. it is NOT docs, NOT markdown, NOT backlog state. `unsafe` is `'true'` iff at least one such file changed; `'false'` iff the PR is docs-only.

## Fail-safe rules that go with it

- Gate heavy steps with `if: needs.changes.outputs.unsafe != 'false'` (use `!= 'false'`, **not** `== 'true'`), so an empty/absent output from an infra failure still runs the heavy work (fail-safe, not silently-skipped).
- Add `if: ${{ !cancelled() }}` at job level on gated jobs so a failed/absent `changes` job still runs verification rather than reporting a silently-passing skipped context to branch protection.
- Keep required contexts always-run at the job level (no workflow-level `paths:`/`paths-ignore:`); do step-level gating only, so a skipped required check never becomes a fail-open merge path.
- For single-pattern intent (e.g. "cli-reference touched"), a lone pattern is quantifier-invariant: `cli_ref_touched: ['docs/cli-reference/**']` behaves identically under `some` or `every`.

## Why it matters

A gate that can **never fire** is as much a defect as one that fires incorrectly — and it fails silently (green checks, no build) which is easy to miss. Always add a **required-SKIP canary** (observe that a docs-only PR actually skips) AND a **must-RUN assertion** (a code PR actually runs), because either direction can be silently wrong.

## Related

- `docs/compound/github-actions/F013-workflow-sha-pinning.md` — workflow SHA-pinning / Go-version alignment.
- `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — CLI Reference Drift gate behavior.
- `docs/exec-plans/2026-07-04-ci-cost-gating-plan.md` — deferred design (round-4 corrected, still unreviewed).
