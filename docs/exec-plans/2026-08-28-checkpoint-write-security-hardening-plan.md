---
chunk_strategy: h1-h2-h3
description: "Implementation plan for checkpoint create/write path security hardening"
doc_type: plan
schema_version: "1.0"
source: stage-dark-batch1
title: "Checkpoint create/write path security hardening"
---

# Checkpoint create/write path security hardening

**Source document**: `.backlogit/queue/060-DL.md` (deliberation 060-DL)
**Stash IDs**: 3A33E404, E429A031, EA1F5912, 35A27CD0, F89CADB7
**Covering feature**: Checkpoint create/write path security hardening
**Requires plan hardening**: no

## Objective

Harden the checkpoint create and write paths in `internal/events` and
`internal/atomicfile` to close five related security, correctness, and
data-integrity gaps that share the same call chain.

## Constitution Check

| Principle | Impact | Notes |
|---|---|---|
| I. Safety-First Go | ✅ Direct | Error wrapping, typed errors, no unsafe |
| II. Test-First | ✅ Direct | Harness-first per task; focused regression tests |
| III. Workspace Isolation | ✅ Strengthened | Symlink/TOCTOU hardening closes isolation gap |
| IV. CLI Containment | No change | |
| V. Observability | ✅ Improved | Classified write outcomes on both transports |
| VI. Single Responsibility | ✅ Compliant | No new dependencies |
| VII. Destructive Approval | No change | |
| VIII. Safety Modes | No change | |
| IX. Git-Friendly | No change | |
| X. Context Efficiency | No change | |
| XI. Merge Commits | No change | |

## Implementation Units

### U1 — Reject malformed JSON before checkpoint write (3A33E404)

**Stash**: 3A33E404 (bug, high)
**Package**: `internal/events`
**Files**: `memory.go`, `memory_test.go`
**Approach**:

1. Add a `json.Valid(data)` pre-check before the V1 probe in `CreateCheckpoint`.
2. When `json.Valid` returns false, return a typed `*CheckpointMalformedInputError`
   with a generic malformed-input message (no raw payload excerpt — checkpoint context may contain sensitive data per Constitution III).
3. Ensure no checkpoint file is written and no success-shaped result is returned.
4. Preserve legacy compatibility: valid JSON without `schema_version: 1` continues
   through the legacy path unchanged.

**Acceptance criteria**:
- Truncated V1-shaped payload returns typed error AND no checkpoint file exists
- Valid V1 payload behavior unchanged
- Valid legacy (non-V1) payload behavior unchanged
- Error surfaces consistently on CLI and MCP transports

**Harness contract**: `harness-ready` — source-shape harness asserting
`CheckpointMalformedInputError` type exists, then red test for the rejection behavior.

**Estimated effort**: ~2h (< 3 files, < 5 functions, < 4 test scenarios)

### U2 — Extend create-boundary context duplicate detection (E429A031)

**Stash**: E429A031 (task, medium)
**Package**: `internal/events`
**Files**: `checkpoint_strict.go`, `checkpoint_strict_test.go`, `memory.go`
**Approach**:

1. Extend `checkClosedSchemaNamespace` (or its token-stream walker) to detect
   exact-duplicate and case-fold-aliased context member names at the create boundary.
2. Reject with a new typed `*CheckpointDuplicateContextKeyError`
   that names the duplicate keys.
3. Ensure "fail before the write" ordering — rejection before `syncWriteFileAtomic`.

**Acceptance criteria**:
- Duplicate context keys in create payload are rejected with typed error
- No checkpoint file written for duplicate-key payloads
- Valid context payloads with unique keys are unchanged
- Case-fold variants detected (e.g. `shipment_id` vs `Shipment_Id`)

**Harness contract**: `harness-ready` — red test for duplicate context member rejection.

**Dependency**: After U1 (validation ordering must be established first)

**Estimated effort**: ~2h

### U3 — Classify syncWriteFileAtomic outcomes on checkpoint create (EA1F5912)

**Stash**: EA1F5912 (task, medium)
**Package**: `internal/events`, `internal/atomicfile`
**Files**: `memory.go`, `fsutil.go`, `atomicfile.go`, `mcp/checkpoint_tools.go`, `cli/checkpoint.go`
**Approach**:

1. Converge `internal/events` onto the existing `internal/atomicfile` outcome
   classification (`ErrWriteNotApplied`, `ErrWriteIndeterminate`).
2. Surface indeterminate checkpoint creates with path and context keys on both
   MCP and CLI transports via a new handler-level seam.
3. Add a classified write result to the create path.

**Acceptance criteria**:
- Indeterminate writes surface `ErrWriteIndeterminate` with path+context on both transports
- Successful writes unchanged
- NotApplied writes properly classified

**Harness contract**: `harness-ready` — red test for indeterminate write classification.

**Dependency**: After U1 (validation must precede write classification)

**Estimated effort**: ~2h

### U4 — Checkpoint filesystem containment hardening (35A27CD0)

**Stash**: 35A27CD0 (task, medium)
**Package**: `internal/events`, `internal/core`
**Files**: `checkpoint_disposition.go`, `checkpoint_read.go`, `memory.go` (read paths only — U4 does not modify the create path in memory.go)
**Approach**:

1. Add `O_NOFOLLOW` or equivalent real-root open to checkpoint read/write paths.
2. Add handle-or-content compare-and-swap precondition on rewrite operations.
3. Enforce no-clobber destinations for move operations.
4. Add adversarial tests for symlinked and concurrently-replaced checkpoint paths.

**Acceptance criteria**:
- Symlinked checkpoint targets are rejected with `ErrCheckpointTargetUnsafe`
- TOCTOU window between conformance read and atomic rewrite is closed
- Adversarial test: symlinked path rejected
- Adversarial test: concurrent replacement detected

**Harness contract**: `harness-ready` — red test for symlink rejection.

**Dependency**: Independent of U1-U3 (different code paths, can be parallelized)

**Estimated effort**: ~2h

### U5 — Validate CheckpointContext.Extra in emit() (F89CADB7)

**Stash**: F89CADB7 (bug, low)
**Package**: `internal/events`
**Files**: `checkpoint_schema.go`, `checkpoint_schema_test.go`
**Approach**:

1. Add `json.Valid(v)` check for each Extra value in `emit()`.
2. Return error naming the offending key if invalid.
3. Both `MarshalJSON()` and `Keys()` propagate the error.
4. Regression test: directly-constructed CheckpointContext with malformed Extra.

**Acceptance criteria**:
- MarshalJSON returns error for malformed Extra values
- Keys() returns error for malformed Extra values
- Error names the offending context key
- Valid Extra values unchanged

**Harness contract**: `harness-ready` — red test for malformed Extra rejection.

**Dependency**: After U1 (malformed input rejection should be established first)

**Estimated effort**: ~1.5h

## Dependency Graph

```text
U1 (malformed JSON gate)
 ├── U2 (context duplicates) — depends on U1
 ├── U3 (write classification) — depends on U1
 └── U5 (Extra validation) — depends on U1

U4 (filesystem containment) — independent, parallel-safe
```

## Wave Schedule

**Wave 1**: U1 (create path) + U4 (disposition/read paths — no memory.go overlap)
**Wave 2**: U2 + U3 + U5 (all depend on U1)

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Legacy checkpoint compatibility | Explicit test: valid non-V1 JSON passes through unchanged |
| Performance regression from json.Valid pre-check | json.Valid is O(n) single pass; checkpoint payloads are small |
| Symlink detection platform differences | Use O_NOFOLLOW on open (or platform equivalent) rather than Lstat-then-open which is itself TOCTOU |
| Concurrent write races in tests | Use t.TempDir() for isolation |

## Monitoring Plan (release-observability)

| SLI | Baseline | Alert Threshold |
|---|---|---|
| Checkpoint create error rate | ~0 malformed inputs | > 3 malformed rejections / session |
| Checkpoint create latency | < 50ms | > 200ms (json.Valid overhead) |
| Test suite pass rate | 100% | Any regression |

**Rollback trigger**: Any checkpoint create that previously succeeded now fails
for a non-malformed payload → revert immediately.

**Observation window**: 48h post-merge, owner: operator.

## Explicit Non-Goals

- No generic checkpoint refactor
- No payload size limit
- No changes to quarantine/abandon disposition (136-F)
- No changes to recovery policy or agent templates
- No reopening of shipped 146-F work (3C7AAC71, 90F2A9F8)

## Copilot Review Remediation (PR #381)

Findings addressed:
1. Removed premature harness-ready labels — applied by harness-architect, not Stage
2. U1: No raw byte prefix in error diagnostic — Constitution III / secrets safety
3. U2: Use distinct CheckpointDuplicateContextKeyError, not reuse UnknownFieldError
4. U3: Added MCP/CLI handler files to file list for transport parity
5. U4: Clarified file scope (disposition/read paths, not create-path memory.go)
6. Wave 1 parallelism: Confirmed safe — U1 and U4 target different code paths
7. U4: O_NOFOLLOW on open, not Lstat-then-open (avoids TOCTOU in detection itself)
8. Adversarial review: NOT escalated — standard plan review sufficient for this scope
   (5 tasks, narrow input-validation + containment changes, no auth/crypto/PII surface)

## Plan Review

<!-- plan-review-attempt: 0 -->
dispatch_mode: single-agent-declared-degradation
decision: PASS
rationale: Security-focused hardening with clear scope boundaries, test-first contracts, explicit non-goals, and wave-parallel dependency graph. All five units target the same call chain with no scope creep risk. Single-agent declared degradation is appropriate because this is a CLI-mode Stage session without multi-agent dispatch capability.
operator_authorization: approved (dark-mode pre-authorized, operator AFK)

