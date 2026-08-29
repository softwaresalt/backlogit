---
chunk_strategy: h1-h2-h3
description: "Multi-persona adversarial review evidence for shipment 131-S planning artifacts"
doc_type: closure
schema_version: "1.0"
source: docs/closure/2026-08-28-131s-adversarial-review-evidence.md
title: "Adversarial Review Evidence — Shipment 131-S"
---

# Adversarial Review Evidence — Shipment 131-S

**Shipment**: 131-S (Checkpoint create/write path security hardening)
**Feature**: 148-F
**Reviewed artifacts**: 060-DL, 148-F, 148.001-T through 148.005-T, 131-S manifest,
execution plan, grouping ledger
**Review date**: 2026-08-28
**Review type**: Stage planning/decision review (not code diff review)

## Review Configuration

| Parameter | Value |
|---|---|
| Agent | Adversarial Review (specialized) |
| Assembly model | claude-sonnet-5 |
| Reviewer count | 3 independent |
| Reviewer A | Tier 1: gemini-3.5-flash |
| Reviewer B | Tier 2: gpt-5.5 |
| Reviewer C | Tier 3: claude-opus-5 |
| Cross-model diversity | Yes (3 distinct model families) |

## Escalation Justification

The prior Stage session declared "Adversarial review: Not escalated (narrow scope,
no auth/crypto/PII)" and "Plan review: PASS (single-agent-declared-degradation)".
The operator determined this did not meet the dark-factory mandate requiring
multi-persona adversarial review for all code and decisions before submitting each PR.
Scope narrowness is not an exemption from the adversarial review gate.

## Review Dimensions Evaluated

1. Plan correctness (bug descriptions, proposed fixes)
2. Dependency graph validity (task ordering, wave parallelism)
3. Scope containment (non-goals, 146-F boundary)
4. Security and filesystem containment (U4 approach)
5. Constitution compliance (P-I through P-XI)
6. Task granularity (2-hour rule, width isolation)
7. Harness contracts (harness-ready labels, red-test contracts)
8. Observability and rollback (monitoring plan, triggers)
9. Deliberation quality (option selection, problem framing)
10. Shipment assembly (manifest completeness)
11. Residual risk (post-ship gaps)
12. Ledger integrity (28-ID arithmetic)

## Per-Reviewer Finding Matrix

### Reviewer A (Tier 1: gemini-3.5-flash)

| Dimension | Result | Notes |
|---|---|---|
| Plan correctness | PASS | Bug descriptions accurate, fixes appropriate |
| Dependency graph | PASS | No issues identified |
| Scope containment | PASS | Non-goals respected |
| Security/containment | PASS | No concerns raised |
| Constitution compliance | PASS | All principles satisfied |
| Task granularity | PASS | Within 2-hour bounds |
| Harness contracts | PASS | Contracts defined |
| Observability/rollback | PASS | Monitoring plan present |
| Deliberation quality | PASS | Option B justified |
| Shipment assembly | PASS | Manifest complete |
| Residual risk | PASS | No concerns |
| Ledger integrity | PASS | No concerns |
| **Unique findings** | None | Largely clean pass |

### Reviewer B (Tier 2: gpt-5.5)

| Dimension | Result | Findings |
|---|---|---|
| Plan correctness | PASS | No issues |
| Dependency graph | WARN | Raised M-1 (wave parallelism concern) — rejected after verification |
| Scope containment | WARN | Raised M-2 (scope too narrow for blast radius) — partially valid as P2 |
| Security/containment | WARN | Raised C-3 partially (O_NOFOLLOW portability) — valid as P2 |
| Constitution compliance | WARN | Raised M-3 (test-first evidence gap) — partially valid as P2 |
| Task granularity | WARN | Raised M-4 (U4 too large) — advisory |
| Harness contracts | WARN | Raised M-5 (harness-ready unsubstantiated) — advisory |
| Observability/rollback | WARN | Raised C-4 (observability incomplete) — valid as P2 |
| Deliberation quality | WARN | Raised M-6 (problem framing inaccurate) — advisory |
| Shipment assembly | WARN | Raised M-7 (manifest ordering) — advisory |
| Residual risk | WARN | Raised M-8 (under-tracked) — advisory |
| Ledger integrity | FAIL | Raised C-2 (arithmetic wrong) — rejected as factually incorrect |
| **Unique findings** | None | Multiple advisory findings |

### Reviewer C (Tier 3: claude-opus-5)

| Dimension | Result | Findings |
|---|---|---|
| Plan correctness | WARN | Raised U-1 (schema_version type confusion bypass) — valid P3 advisory |
| Dependency graph | WARN | Raised M-1 independently (wave parallelism) — rejected after verification |
| Scope containment | WARN | Raised M-2 independently (scope narrow) — partially valid |
| Security/containment | WARN | Raised C-3 (O_NOFOLLOW + package architecture) — valid P2 |
| Constitution compliance | WARN | Raised M-3 independently (evidence gap) — partially valid |
| Task granularity | WARN | Raised M-4 independently (U4 size) — advisory |
| Harness contracts | WARN | Raised M-5 independently (platform-fragile tests) — advisory |
| Observability/rollback | WARN | Raised C-4 independently (no slog logging) — valid P2 |
| Deliberation quality | WARN | Raised M-6 independently (framing inaccurate) — advisory |
| Shipment assembly | WARN | Raised M-7 independently (ordering) — advisory |
| Residual risk | PASS | Raised U-2 (U5 reachability) — valid P3 advisory |
| Ledger integrity | FAIL | Raised C-1 (provenance failure) — rejected as factually incorrect |
| **Unique findings** | U-1 (type confusion), U-2 (U5 reachability) | Deepest technical analysis |

### Consensus Derivation

| Finding | Reviewer A | Reviewer B | Reviewer C | Consensus |
|---|---|---|---|---|
| C-1 (provenance) | not raised | not raised | raised | LOW — rejected (factually incorrect) |
| C-2 (ledger arithmetic) | not raised | raised | not raised | LOW — rejected (factually incorrect) |
| C-3 (containment underscoped) | not raised | raised | raised | MEDIUM — partially valid as P2 |
| C-4 (observability gap) | not raised | raised | raised | MEDIUM — valid as P2 |
| M-1 (wave parallelism) | not raised | raised | raised | MEDIUM — rejected (factually incorrect) |
| U-1 (type confusion) | not raised | not raised | raised | LOW — valid P3 advisory |
| U-2 (U5 reachability) | not raised | not raised | raised | LOW — valid P3 advisory |

## Consensus Outcome

**Decision**: PASS — no verified P0/P1 findings after independent verification of
all reviewer claims against actual repository state and source code.

The reviewers produced HIGH-confidence CRITICAL findings (C-1, C-2) that were
independently verified as **factually incorrect** — the review agents failed to
correctly access repository state. All artifacts exist at origin/main, the merge
commit is verified, and the ledger arithmetic is correct. One additional finding
(M-1, wave parallelism claim) was also factually wrong after checking the plan's
explicit scoping language. Total rejected: 3 findings (C-1, C-2, M-1).

## Verified Findings

### P2 — Deferred (3 findings)

**AR-P2-1: U4 platform portability** (MEDIUM confidence)
The plan states "O_NOFOLLOW or equivalent real-root open" but does not specify
the Windows implementation strategy explicitly. The codebase actively supports
Windows (see `runtime.GOOS=="windows"` branches in fsutil.go).
**Affected files**: `internal/events/memory.go`, `internal/core/checkpoint_disposition.go`
**Action class**: `advisory` — plan language already accommodates platform equivalents
**Disposition**: Deferred to Ship. The Ship agent selects the concrete implementation.

**AR-P2-2: Create path MkdirAll not covered by U4** (MEDIUM confidence)
U4 scopes to disposition/rewrite paths. The create path's
`os.MkdirAll(checkpointDir, 0o755)` in `CreateCheckpoint` is not covered. The
create path receives `checkpointDir` from workspace configuration, not from
user-controlled input directly.
**Affected file**: `internal/events/memory.go` (line ~68, `CreateCheckpoint`)
**Action class**: `advisory` — trusted input path from workspace config
**Disposition**: Deferred. Can be tracked as a separate follow-up stash if the
threat model expands.

**AR-P2-3: Observability gap — no structured logging for rejections** (MEDIUM confidence)
The plan's monitoring section defines SLIs but does not specify `slog` calls for
security-relevant rejections in `CreateCheckpoint`. Currently zero slog calls in
the function.
**Affected file**: `internal/events/memory.go` (line ~68, `CreateCheckpoint`)
**Action class**: `gated_auto` — Ship agent can add slog calls during implementation
**Disposition**: Deferred to Ship. The monitoring plan provides the framework.

### P3 — Advisory (2 findings)

**AR-P3-1: U1 schema_version type confusion** (LOW confidence, single reviewer)
Valid JSON with `schema_version:"1"` (string) or `1.0` (float) passes
`json.Valid` but causes `json.Unmarshal` to return an `UnmarshalTypeError` on
the int-typed probe field. The V1 branch is skipped and the payload falls
through to the legacy write path.
**Affected file**: `internal/events/memory.go` (line ~78, V1 probe logic)
**Action class**: `advisory` — valid JSON with wrong schema_version type is not a
malformed input; legacy fallthrough is designed behavior
**Assessment**: This is a separate concern from U1's scope (malformed JSON
rejection). A payload with a string-typed `schema_version` is not a V1
checkpoint. Would require its own deliberation if deemed a vulnerability.

**AR-P3-2: U5 guards currently unreachable production code** (LOW confidence, single reviewer)
`CheckpointContext.Extra` is only populated via `UnmarshalJSON` in production.
No direct `CheckpointContext{Extra: ...}` construction exists outside the
unmarshal path (verified by grep against all non-test `.go` files).
**Affected file**: `internal/events/checkpoint_schema.go` (emit function)
**Action class**: `advisory` — defensive invariant is valid engineering practice
**Assessment**: Priority `low` is appropriate for a defensive guard.

## Rejected Findings (3 total)

### C-1: Provenance failure (REJECTED — factually incorrect)

**Reviewer source**: Reviewer C (Tier 3)
**Reviewer claim**: Cited merge commit 579d6b08 resolves to an unrelated PR #225
dependency-update merge. None of the artifacts exist in origin/main.

**Rejection evidence**:
- `gh pr view 381 --json number,title,state,mergeCommit` confirms PR #381 titled
  "chore(harness): stage checkpoint write-safety shipment 131-S", state MERGED,
  mergeCommit.oid = 579d6b088d5ab42c4aef0d91243abf578551097e
- `git cat-file -p 579d6b08` shows parents 6283c62d (main) and a82efede
  (chore/dark-stage-batch1), author softwaresalt, message matches PR title
- `git show origin/main:.backlogit/queue/131-S.md` returns the shipment manifest
- `git show origin/main:.backlogit/queue/148-F.md` returns the feature artifact
- All task files (148.001-T through 148.005-T), deliberation (060-DL), execution
  plan, and grouping ledger confirmed present via direct git show

**Root cause**: The review agent likely inspected working tree state in a
worktree behind origin/main, or failed to fetch remote refs before querying.

### C-2: Ledger arithmetic (REJECTED — factually incorrect)

**Reviewer source**: Reviewer B (Tier 2)
**Reviewer claim**: 28 − 8 − 1 − 1 = 18, not the stated 20 active entries.

**Rejection evidence**:
- 8 consumed by 130-S: 5A4DBE3C, C1808666, B212512E, E053034D, 4863B04B,
  48F28B8D, C0A382C7, B7CE5FF9
- 20 active: Group 1 (5) + Group 2 (3) + Group 3 (6) + Group 4 (6) = 20
- 8 + 20 = 28 ✓
- 40A985BB was a candidate discovered outside the original 28 bounded set
- 302EFF07 is "active but not in bounded set" — never part of the 28
- Neither should be subtracted from 28

### M-1: Wave 1 parallelism false (REJECTED — factually incorrect)

**Reviewer sources**: Reviewer B (Tier 2) and Reviewer C (Tier 3)
**Reviewer claim**: U1 and U4 both edit CreateCheckpoint in the same region of
memory.go, so Wave 1 parallelism is unsafe.

**Rejection evidence**: The plan explicitly states U4's file scope as
"internal/core/checkpoint_disposition.go, internal/events/memory.go
(disposition/rewrite paths only — U4 does not modify the create path in
memory.go)". U1 modifies CreateCheckpoint. U4 modifies disposition/rewrite paths.
No function overlap.

## Residual Risk Assessment

| Risk | Status | Tracking |
|---|---|---|
| U4 Windows portability | Tracked (AR-P2-1) | Ship agent responsibility |
| Create path MkdirAll containment | Tracked (AR-P2-2) | Potential follow-up stash |
| Schema_version type confusion | Tracked (AR-P3-1) | Separate deliberation if needed |
| U5 defensive-only scope | Noted (AR-P3-2) | Priority low is appropriate |

## Gate Status

| Gate | Status |
|---|---|
| Multi-persona adversarial review | ✅ COMPLETED (3 reviewers, 3 model families) |
| P0/P1 findings | ✅ NONE (after independent verification) |
| P2 findings recorded | ✅ 3 findings deferred with file/line and action class |
| P3 findings recorded | ✅ 2 findings noted as advisory with file/line |
| Rejected findings documented | ✅ 3 findings rejected with evidence (C-1, C-2, M-1) |
| 131-S admission readiness | ✅ READY for Ship claim |
| 28-ID grouping ledger | ✅ UNCHANGED — arithmetic verified correct |