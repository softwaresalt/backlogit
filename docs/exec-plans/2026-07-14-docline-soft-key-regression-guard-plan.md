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
| D1 | Detect missing explicit soft keys persistently | Unit D1 adds a Git-tracked inventory and exact-value assertions. |
| D2 | Prove TDD RED then GREEN | D1 first fails on nine tracked documents; D2-D7 backfill them. |
| D3 | Preserve untracked scratch | Inventory uses `git ls-files`; no task includes the scratch path. |
| D4 | Keep tasks under two hours and fewer than three files | Backfill remains six one/two-file units. |
| D5 | Avoid schema/runtime expansion | No production parser or schema semantics change. |
| D6 | Share production scope truth | D1 filters tracked candidates through exported `internal/docline.Scope()`. |
| D7 | Contain every tracked read | D1 rejects symlink/junction/reparse or resolved-target escapes before reading. |

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

Run `git ls-files -z` from the repository root to obtain candidates, then filter every live and hermetic path through the exported `internal/docline.Scope()` descriptor. Do not copy include/exclude tables into the test. Require production scope changes to affect the guard automatically, with a fixture proving a descriptor exclusion is honored.

Before reading any selected path, `Lstat` each component and reject symlink, junction, or reparse entries; resolve the real target and require it remains under the repository/temporary-root boundary. Reject missing/non-regular targets. On platforms where creating a real link is unavailable, the real-link case may report an explicit platform skip only after an always-run synthetic reparse/symlink-classification negative proves rejection logic, so unsupported privileges cannot create a false pass.

Parse leading YAML with the existing dependency and require `chunk_strategy: h1-h2-h3` plus `schema_version` as YAML string `"1.0"`, not numeric `1.0`. Fail path-specifically on Git, containment, malformed YAML, missing key, scalar type, or wrong value.

Hermetic temporary-Git cases use the same inventory/scope/containment helper and cover:

1. compliant tracked Markdown;
2. each missing key;
3. numeric schema version, wrong chunk strategy, and malformed YAML;
4. invalid untracked Markdown excluded while an invalid tracked peer is found;
5. production Scope exclusions;
6. tracked link/reparse entry targeting outside root rejected before read.

**RED:** `go test ./tests/integration -run TestTrackedDoclineSoftKeys -count=1` names the nine live omissions while hermetic scope/containment negatives execute.
**GREEN:** after D2-D7, live corpus and all hermetic cases pass.

**Acceptance criteria:** production Scope is the sole filter; deterministic tracked inventory; fail-closed real-path containment; exact values/types; path-specific diagnostics; fewer than five functions; no scratch mutation.

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

1. Add D1 shared inventory/scope/containment guard and record RED on exactly nine live omissions.
2. Confirm hermetic missing-key, type/value, malformed YAML, untracked, production-Scope, synthetic reparse, and external-target link cases execute independently.
3. Apply D2-D7 while preserving body bytes.
4. Run the targeted test and require live plus all hermetic cases GREEN.
5. Run `go test ./...`, `go vet ./...`, `golangci-lint run`, and `gofmt -l .` with no output.
6. Run `go run ./cmd/backlogit docs lint` in a clean checkout and require zero violations.
7. Verify the protected scratch remains untracked, untouched, and unstaged.

## Decisions and Rationale

- **Integration test, not lint-rule change:** schema defaults remain legitimate for external ingestion.
- **Git inventory plus production Scope:** Git determines tracked candidates; `internal/docline.Scope()` is the only include/exclude authority.
- **Contain before read:** reject link/reparse components and outside real targets.
- **Hermetic fixtures:** missing-key, untracked, scope drift, and path escape negatives survive backfill.
- **Explicit platform fallback:** unsupported real-link creation never skips the always-run classifier negative.
- **Small backfill slices:** each metadata unit stays below the file-count heuristic.

## Risks and Caveats

- Git remains a repository prerequisite.
- Real symlink privilege varies; explicit skip plus always-run classification avoids false confidence.
- Production Scope changes intentionally alter the guard corpus and must update fixtures with the change.
- A deliberate schema version upgrade changes guard and corpus together.
- Local full-tree lint may inspect protected untracked scratch; never modify it to make local output green.

## Plan Hardening Signals

- **Public API, schema, or contract change:** absent — no production/schema behavior changes.
- **Security, auth, permission, or compliance:** present but test-local — path containment prevents tracked links from reading outside the workspace.
- **Migration, backfill, destructive action:** limited additive metadata backfill; reversible, no body edits.
- **External integration or dependency:** Git is already a repository prerequisite; no new dependency.
- **High runtime, rollout, or rollback risk:** absent; test-only guard plus metadata.

Requires plan hardening: no

## Runtime Verification and Closure

No shipped runtime surface changes. Verification is the targeted integration test, full Go gates, body-byte-preservation review, and CI. Closure should record the RED omission set, GREEN commit, and confirmation that the untracked scratch file was untouched. Rollback is commit revert.

## Constitution Check

- **I:** only a Go integration test is added; standard gates and idiomatic error assertions apply.
- **II (NON-NEGOTIABLE):** D1 is observed RED on known omissions before any backfill, then GREEN.
- **III/IV (NON-NEGOTIABLE containment):** Git candidates use production Scope, and link/reparse plus resolved-target checks keep every read inside the repository; scratch is excluded.
- **V:** failures name exact paths/fields and closure records RED/GREEN evidence.
- **VI:** no dependency is added; one test file owns inventory, production-Scope filtering, containment, and fixtures while metadata units remain isolated.
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
