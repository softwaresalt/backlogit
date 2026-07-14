---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Docline soft-key regression guard implementation plan'
source: docs/exec-plans/2026-07-14-docline-soft-key-regression-guard-plan.md
doc_type: plan
description: 'Test-first plan to enforce explicit docline chunk_strategy and schema_version values on tracked in-scope Markdown and backfill known omissions.'
docline:
    date: 2026-07-14T18:38:00Z
    origin: docs/decisions/2026-07-14-docline-soft-key-regression-decision.md
    linked_stash_ids:
        - A4BE2FAD
    review_state: blocked
---

# Docline Soft-key Regression Guard Implementation Plan

## Problem Frame

Docline defaults make missing `chunk_strategy` and `schema_version` keys invisible to current validation. The repository needs a persistent guard over committed authoring state. The guard must not touch or fail because of the intentional untracked scratch spike.

**Origin:** `docs/decisions/2026-07-14-docline-soft-key-regression-decision.md`.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| D1 | Detect missing explicit soft keys persistently | Unit D1 adds a git-tracked integration inventory and exact-value assertions. |
| D2 | Prove TDD RED then GREEN | D1 first fails on nine tracked documents; D2-D7 backfill them. |
| D3 | Preserve untracked scratch | Inventory uses `git ls-files`; no task includes the scratch path. |
| D4 | Keep tasks under 2 hours and fewer than 3 files | Backfill is split into six one/two-file units. |
| D5 | Avoid schema/runtime expansion | No production docline or schema file changes. |

## Scope Boundaries

### In Scope

One Go integration guard and the nine tracked Markdown files listed in the decision.

### Out of Scope

- `docs/decisions/2026-07-13-scratch-spike.md` (untracked; do not read for mutation, edit, delete, or stage).
- `internal/docline` production behavior.
- `schemas/docline/base-frontmatter-v1.schema.json`.
- docs/memory, docs/archive, and non-docline Markdown.

## Implementation Units

### Unit D1: Add live-corpus and hermetic soft-key guard

**Files:** `tests/integration/docline_soft_keys_test.go`
**Effort:** S, under 2 hours; 1 file; two tests plus at most two shared helpers (fewer than five functions total).
**Skill domain:** Go integration test.
**Execution posture:** test-first.
**Dependencies:** none.

Keep a live-corpus test that runs `git ls-files -z` from the repository root, filters to `docs/**` except `docs/memory/**` and `docs/archive/**` plus `README.md` and `AGENTS.md`, parses leading YAML with the existing dependency, and requires `chunk_strategy: h1-h2-h3` plus `schema_version` as YAML string `"1.0"` rather than numeric `1.0`. Fail path-specifically on Git errors, malformed YAML, missing keys, wrong scalar type, or wrong value.

Add a hermetic table test that creates temporary Git repositories, stages fixture paths, and calls the same guard helper. Required cases:

1. compliant tracked Markdown passes;
2. tracked Markdown missing `chunk_strategy` fails with path/field;
3. tracked Markdown missing `schema_version` fails with path/field;
4. numeric `schema_version: 1.0`, wrong `chunk_strategy`, and malformed YAML each fail distinctly;
5. invalid untracked Markdown is absent from findings while an invalid tracked peer is found;
6. tracked `docs/memory/**` and `docs/archive/**` fixtures are excluded.

The synthetic cases remain in CI after the live corpus is backfilled and do not depend on the protected local scratch file. Use one shared inventory/validation helper, one temporary-repository helper, and two tests so weakened parser or inventory logic cannot pass accidentally.

**RED:** `go test ./tests/integration -run TestTrackedDoclineSoftKeys -count=1` fails the live-corpus subtest and names exactly the nine tracked omissions while hermetic cases exercise missing-key, type, parse, and untracked/excluded behavior.
**GREEN:** after D2-D7, the live corpus passes and every hermetic positive/negative case remains active and passes.

**Acceptance criteria:** deterministic tracked-only live scope; hermetic negative coverage independent of repository state; exact values/types enforced; path-specific diagnostics; fewer than five functions; no scratch path staged or modified.
### Unit D2: Backfill 092-S closure metadata

**Files:** `docs/closure/2026-07-13-092-S-compound-refresh.md`, `docs/closure/2026-07-13-092-S-item-writer-utc-closure.md`
**Effort:** XS; 2 files.
**Skill domain:** closure Markdown metadata.
**Execution posture:** metadata-only green step.
**Dependencies:** D1 RED.

Add the two canonical keys without changing body bytes or other frontmatter semantics.

### Unit D3: Backfill parallel-test and lifecycle learnings

**Files:** `docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md`, `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`
**Effort:** XS; 2 files.
**Skill domain:** compound Markdown metadata.
**Execution posture:** metadata-only green step.
**Dependencies:** D1 RED.

### Unit D4: Backfill UTC timestamp learning

**Files:** `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md`
**Effort:** XS; 1 file.
**Skill domain:** compound Markdown metadata.
**Execution posture:** metadata-only green step.
**Dependencies:** D1 RED.

### Unit D5: Backfill deterministic-gates decision and plan

**Files:** `docs/decisions/2026-06-30-backlogit-deterministic-gates-slice-deliberation.md`, `docs/exec-plans/2026-06-30-backlogit-deterministic-gates-slice-plan.md`
**Effort:** XS; 2 files.
**Skill domain:** paired planning-document metadata.
**Execution posture:** metadata-only green step.
**Dependencies:** D1 RED.

### Unit D6: Backfill evals design metadata

**Files:** `docs/design-docs/autoharness-evals-gates-design.md`
**Effort:** XS; 1 file.
**Skill domain:** design Markdown metadata.
**Execution posture:** metadata-only green step.
**Dependencies:** D1 RED.

### Unit D7: Backfill covering-feature plan metadata

**Files:** `docs/exec-plans/2026-07-02-shipment-covering-feature-display-plan.md`
**Effort:** XS; 1 file.
**Skill domain:** plan Markdown metadata.
**Execution posture:** metadata-only green step.
**Dependencies:** D1 RED.

Units D3-D7 use the same acceptance criterion as D2: canonical keys added, body bytes and all unrelated frontmatter unchanged.

## Dependency Graph

`D1 RED → {D2, D3, D4, D5, D6, D7} → D1 GREEN`. Backfill units are independent.

## TDD and Quality-gate Sequence

1. Add D1 shared guard plus live and synthetic tests; run targeted RED and capture exactly nine live omissions.
2. Confirm hermetic cases independently cover missing keys, numeric/string distinction, malformed YAML, wrong value, untracked exclusion, and excluded tracked paths.
3. Apply D2-D7 while preserving body bytes.
4. Run the targeted test; require live-corpus GREEN and all synthetic cases GREEN.
5. Run `go test ./...`.
6. Run `go vet ./...`.
7. Run `golangci-lint run`.
8. Run `gofmt -l .` and require no output.
9. Run `go run ./cmd/backlogit docs lint` in a clean checkout and require zero violations.
10. Verify `git status --short` still lists the pre-existing scratch file only as untracked and never staged.
## Decisions and Rationale

- **Integration test rather than production lint rule:** the invariant concerns committed repository authoring, while schema defaults remain legitimate for ingestion.
- **Git-tracked inventory:** CI guards every committed document and leaves operator-owned scratch untouched.
- **Hermetic fixtures:** synthetic temporary repositories retain missing-key and untracked-exclusion negatives after the live corpus becomes compliant.
- **Exact values:** mere key presence would allow empty or drifted values.
- **Small backfill slices:** each unit stays below the file-count heuristic.

## Risks and Caveats

- Integration execution requires Git; this repository's CI checkout and development workflow already require it.
- Scope filtering duplicates the public docline scope at a high level; comments must identify that coupling.
- Synthetic tests must initialize and stage fixtures deterministically without reading global Git configuration.
- A future schema version intentionally updates test and corpus together.
- Local full-tree docs lint may inspect untracked scratch; do not modify the scratch to make a local command green. Use the targeted guard and clean-CI result honestly.

## Plan Hardening Signals

- **Public API, schema, or contract change:** absent — no production/schema behavior changes.
- **Security, auth, permission, or compliance:** absent.
- **Migration, backfill, destructive action:** limited additive metadata backfill; reversible, no body edits.
- **External integration or dependency:** Git is already a repository prerequisite; no new dependency.
- **High runtime, rollout, or rollback risk:** absent; test-only guard plus metadata.

Requires plan hardening: no

## Runtime Verification and Closure

No shipped runtime surface changes. Verification is the targeted integration test, full Go gates, body-byte-preservation review, and CI. Closure should record the RED omission set, GREEN commit, and confirmation that the untracked scratch file was untouched. Rollback is commit revert.

## Constitution Check

- **I:** only a Go integration test is added; standard gates and idiomatic error assertions apply.
- **II (NON-NEGOTIABLE):** D1 is observed RED on known omissions before any backfill, then GREEN.
- **III/IV (NON-NEGOTIABLE containment):** Git enumeration and writes remain inside the repository; scratch is excluded from mutation.
- **V:** failures name exact paths/fields and closure records RED/GREEN evidence.
- **VI:** no dependency is added; test and metadata units are isolated.
- **VII (NON-NEGOTIABLE):** no deletion/overwrite of the scratch file is authorized.
- **VIII:** blast radius is low; investigate-first research completed and hardening is not required.
- **IX:** the test guards committed human-readable state.
- **X:** deterministic inventory avoids repeated broad manual audits.
- **XI:** downstream delivery remains merge-commit-only.

No constitutional violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: BLOCKED

**Formal plan-review provenance:** NOT RUN. This invocation exposes no agent/task dispatch tool, so no independent reviewer persona was spawned and no formal gate result exists.

**Waiver authorization:** NONE. The operator's generic `stage next` command is workflow routing, not a waiver or approval signal.
**Refinement authorization:** the operator authorized one plan/backlog refinement cycle only; it is not formal review evidence and is not a waiver.
**Missing capability:** semantic reviewer subagent dispatch.
**Current disposition:** shipment `095-S`, feature `106-F`, and all member tasks are blocked. Preserved backlog artifacts are not harvest- or Ship-ready.
**Required unblock:** either append successful formal multi-persona review evidence for this exact plan, or obtain a new explicit plan-scoped operator waiver that names the plan, authorization, scope, risk, and expiry and is handled through the durable reservation/consumption contract in the governance plan.

### Informal Single-agent Assessment

The existing planning observations remain informal context only. They are not a formal gate verdict and cannot unblock harvest or Ship.
