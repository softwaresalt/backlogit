---
chunk_strategy: h1-h2-h3
description: Source-verified and shipped semantics of the dorny/paths-filter predicate-quantifier every option; detect out-of-allowlist changes with an anchor pattern plus negations and fail-safe job-level gates.
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
**Discovered during**: Stage plan-review of the CI cost-gating design (stash `D760E508`), which FAILed three consecutive review rounds on this exact mental-model error.
**Shipped in**: `089-S` / feature `090-F` (PR #201, merge `fd5cc60c92bbcd478de62fac20fa8f2d1d636911`) using PR-only CI, job-level gating, a single Go 1.24 `test` context, and consolidated `CLI Reference Drift` inside `ci.yml`.
**Scope**: third-party GitHub Action `dorny/paths-filter` change-detection semantics and the backlogit guardrail that required checks must keep reporting.

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
- Default quantifier is `some` (`patterns.some(...)`), under which a positive allowlist is fail-**open** for any path not enumerated (unlisted dirs like `scripts/**`, `plugin/**`, `retired package tree`, `.mcp.json` skip the heavy jobs).

## Correct pattern

Detect the **existence of an out-of-allowlist (non-docs) changed file** using an anchor pattern plus negations under `every`:

```yaml
# CORRECT — code=='true' iff any changed file is outside the docs/backlog allowlist
filters: |
  code:
    - '**'
    - '!**/*.md'
    - '!docs/**'
    - '!.backlogit/**'
predicate-quantifier: 'every'
```

Under `every`, a file matches `code` iff it matches the anchor and satisfies every negation — i.e. it is NOT Markdown, NOT docs, and NOT backlog state. `code` is `'true'` iff at least one such file changed; `'false'` iff the PR is docs/backlog-only. The leading `**` mirrors the shipped `ci.yml` shape and makes the positive anchor explicit.

## Fail-safe rules that go with it

- Gate heavy steps with `if: needs.changes.outputs.code != 'false'` (use `!= 'false'`, **not** `== 'true'`), so an empty/absent output from an infra failure still runs the heavy work (fail-safe, not silently-skipped).
- Add `if: ${{ !cancelled() }}` at job level on gated jobs so a failed/absent `changes` job still runs verification rather than reporting a silently-passing skipped context to branch protection.
- Keep required contexts always-run at the job level (no workflow-level `paths:`/`paths-ignore:`); do step-level gating only, so a skipped step never removes a required job context.
- If the workflow is made PR-only to avoid push double-runs, keep branch protection requiring up-to-date branches or add `merge_group` before adopting a merge queue.
- For single-pattern intent (e.g. "cli-reference touched"), a lone pattern is quantifier-invariant: `cli_ref_touched: ['docs/cli-reference/**']` behaves identically under `some` or `every`.

## Why it matters

A gate that can **never fire** is as much a defect as one that fires incorrectly — and it fails silently (green checks, no build) which is easy to miss. Always add a **required-SKIP canary** (observe that a docs-only PR actually skips) AND a **must-RUN assertion** (a code PR actually runs), because either direction can be silently wrong.

## Related

- `docs/compound/github-actions/F013-workflow-sha-pinning.md` — workflow SHA-pinning / Go-version alignment.
- `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — CLI Reference Drift gate behavior.
- `docs/closure/2026-07-10-089-S-ci-cost-reduction-closure.md` — shipped PR-only CI cost reduction and required-context evidence.
- `tests/integration/ci_compliance_test.go` — characterization tests for trigger model, required contexts, fail-safe gates, and release provenance.
- `docs/exec-plans/2026-07-04-ci-cost-gating-plan.md` — original deferred design dossier that led to the shipped 089-S work.
