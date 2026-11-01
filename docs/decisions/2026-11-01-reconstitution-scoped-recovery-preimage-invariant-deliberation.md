---
chunk_strategy: h1-h2-h3
description: 'P-021 C6 deliberation for stash entry 7E14915F (078-DL) separating the in-scope bounded preimage-invariant check and ctx lock propagation for 210-S / 209.002-T (PR #638) from the out-of-scope general cross-writer coordination and durable mutation attribution contract deferred in 7E14915F.'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - docs/decisions/2026-10-30-declarative-shipment-reconstitution-deliberation.md
    - docs/exec-plans/2026-10-30-declarative-shipment-reconstitution-plan.md
    - .backlogit/archive/078-DL.md
    - .backlogit/archive/080-DL.md
    - .backlogit/queue/209-F.md
    - .backlogit/queue/210-S.md
  promoted_to: backlog
  tags:
    - shipment
    - reconstitute
    - recovery
    - 7E14915F
    - 078-DL
    - D3C8D13B
    - 080-DL
    - 209-F
    - 210-S
  topic: 'Reconstitution scoped recovery preimage invariant vs deferred cross-writer attribution (7E14915F / 210-S)'
schema_version: "1.0"
source: docs/decisions/2026-11-01-reconstitution-scoped-recovery-preimage-invariant-deliberation.md
title: 'Deliberation: Reconstitution Scoped Recovery Preimage Invariant vs Deferred Cross-Writer Attribution (7E14915F / 078-DL)'
---

## Problem Frame

During shipment `210-S` (`209-F` / `209.002-T`, PR #638) remediation of declarative multi-shipment manifest reconstitution crash recovery (`internal/core/shipment_reconstitute_exec.go`), stash entry `7E14915F` was captured as a `DEFERRED SCOPE EXPANSION` for designing general durable mutation attribution or cross-writer coordination so crash recovery can distinguish reconstitution's own changes from later independent artifact updates. Under P-021 C6, Stage deliberation (`078-DL`) separates what belongs in-scope for `210-S` / `209.002-T` within the existing `reconstituteJournalState` contract surface from the broader cross-writer coordination architecture deferred in `7E14915F`.

## Options Evaluated

### Option A: Bounded Preimage-Invariant Check in `210-S` + Defer General Cross-Writer Attribution in `7E14915F` — Chosen

* **In-scope for `210-S` / `209.002-T` (PR #638) under P-021 C1/C3 (same contract surface, zero new schema)**:
  * In `rollbackReconstitute` (`internal/core/shipment_reconstitute_exec.go`), using ONLY the existing `reconstituteJournalState` fields (`Images`, `ScopedImages`, `Adoptions`, `EventIDs` — zero new journal fields or cross-writer attribution schema), enforce a fail-closed preimage-invariant check when `state.ScopedImages && image.Exists && !bytes.Equal(current, image.Content)`:
    1. Move the `knownLogs` / `discovered` / `pairs` (`oldID -> newID`) calculation before the `state.Images` validation loop.
    2. Parse `preArt` from `image.Content` and `curArt` from `current`. Reject rollback fail-closed (`reconstituteValidationError("snapshot %s changed outside reconstitution", image.Path)`) if any field that reconstitution never modifies differs between `preArt` and `curArt`: `ID`, `Title`, `ArtifactType`, `Description`, `Sprint`, `Priority`, `AssignedTo`, `Owner`, `Commit`, `!preArt.CreatedAt.Equal(curArt.CreatedAt)`, `!slices.Equal(preArt.References, curArt.References)`, any `CustomFields` key other than `"items"`, `"origin_feature"`, `"blocked_reason"`, `"claim_marker"` (checked in both directions across `preArt.CustomFields` and `curArt.CustomFields` — deterministically protecting independent `size`, `size_source`, `size_ruleset_version`, and `complexity` updates), or `curArt.Links` after rewinding `newID -> oldID` via `pairs`.
    3. When `owner` is not in `state.EventIDs` (a cross-reference-only snapshot owner), also reject rollback fail-closed if rewinding `newID -> oldID` across `ParentID`, `Dependencies`, `Links`, and shipment `CustomFields["items"]` (with `UpdatedAt = preArt.UpdatedAt`) does not match `preArt` via `recoveryArtifactMatchesAny`.
  * Complete the in-scope `applyReconstitution` `ctx` lock propagation for newly created features (`P2`).
  * Satisfies `TestRound3RecoveryPreservesIndependentSize` (`scoped=false/true`, `cross_reference=false/true`) and Copilot comment `4434730479` within the existing `210-S` contract surface.
* **Out-of-scope and deferred in `7E14915F` (`078-DL`) and `D3C8D13B` (`080-DL`) under P-021 C1/C2/C6**:
  * General cross-writer coordination or durable per-mutation/per-event attribution (for distinguishing post-crash independent edits to reconstitution-mutable fields such as `Status`, `Dependencies`, or `Labels` on direct targets in `state.EventIDs` when an OS process crashed before recovery) is a broader architectural contract outside `210-S`.
  * `7E14915F` remains an active stash entry linked to deliberation `078-DL` and `D3C8D13B` remains an active stash entry linked to deliberation `080-DL` for harvesting into separate future shipments after `210-S` closes; both are recorded as explicit residual risk on PR #638 (`Outcome: READY_WITH_FOLLOWUPS`, `Blocking findings: P0=0, P1=0` for `210-S` in-scope surface).
  * **Explicit P-021 C6 Disposition of Static-Trace Crash-Window Candidates (`internal/core/shipment_reconstitute_exec.go`)**:
    1. **Candidate 1 (`internal/core/shipment_reconstitute_exec.go:589,1008,1092` and `internal/core/stash.go:733` — post-crash stash archive snapshot overwrite)**: Live reconstitution holds `lockActiveStash(ws)` (`executeReconstitution` line 307) continuously across capture, execution, and in-process rollback. A race with `ArchiveStashEntry` (`internal/core/stash.go:733`) can only occur if an OS process crashes mid-reconstitution (releasing the OS stash lock) and an independent writer archives a stash entry before `recoverPendingShipmentOperations` runs. Confirmed part of the post-crash cross-writer coordination/attribution surface captured in `7E14915F` (`078-DL`), **out-of-scope for `210-S` under P-021 C1/C6**, and non-blocking (`Outcome: READY_WITH_FOLLOWUPS` citing `7E14915F` / `078-DL`), with **zero Go changes in `210-S`**.
    2. **Candidate 2 (`internal/core/shipment_reconstitute_exec.go:1017,1081` — post-crash independent update to an adopted `newID` artifact when `oldID` path is missing)**: Live reconstitution holds `lockArtifactMutation` (lock B) on both `oldID` and `newID` (`recordAdoption` line 453) continuously across execution and in-process rollback. An independent `SetArtifactSizeWithProvenance` update on `newID` while `oldID` is missing can only occur if an OS process crashes after the adoption rename (releasing lock B on `newID`) and an independent writer mutates `newID` before `recoverPendingShipmentOperations` runs. Confirmed part of the post-crash cross-writer attribution surface captured in `7E14915F` (`078-DL`), **out-of-scope for `210-S` under P-021 C1/C6**, and non-blocking (`Outcome: READY_WITH_FOLLOWUPS` citing `7E14915F` / `078-DL`), with **zero Go changes in `210-S`**.
    3. **Candidate 3 (`internal/core/shipment_reconstitute_exec.go:673,761` — crash window between `CreateArtifact`/`CreateShipment` and `recordCreated` journaling `CreatedIDs`)**: Exact same pre-journal creation window already captured in committed stash entry `D3C8D13B` (whose line numbers shifted from `~373-379, 454-460` at `c34de142` to `:673,761` in the current `internal/core/shipment_reconstitute_exec.go`) and deliberated in `080-DL` (`.backlogit/archive/080-DL.md`). Confirmed covered by `D3C8D13B` / `080-DL`, **out-of-scope for `210-S` under P-021 C1/C6**, and non-blocking (`Outcome: READY_WITH_FOLLOWUPS` citing `D3C8D13B` / `080-DL`), with **zero Go changes in `210-S`**.

### Option B: Expand `reconstituteJournalState` Schema with Post-Image / Per-Mutation Attribution inside `210-S` — Rejected

* Rejected for `210-S` because P-021 C1 forbids expanding the journal schema or introducing a new cross-writer attribution contract inside an active shipment remediation pass.

### Option C: Unconditional Byte-Snapshot Overwrite on `ScopedImages` Rollback — Rejected

* Rejected because overwriting artifacts without checking reconstitution-immutable fields clobbers independent updates to fields reconstitution never modifies (such as `size` and `complexity`) and fails `TestRound3RecoveryPreservesIndependentSize` and Copilot comment `4434730479`.

## P-021 C5/C6 Provenance and Triage Outcomes

* **Source References**: `stash_id=7E14915F` (`deliberation_id=078-DL`) and `stash_id=D3C8D13B` (`deliberation_id=080-DL`); `task_id=209.002-T`; `feature_id=209-F`; `shipment_id=210-S`; `PR=#638`; `review-thread ID=N/A` (threadless local review origin; correlated PR #638 Copilot review comment `4434730479`).
* **P-021 C6 Obligation (A) — Unconditional Duplicate Detection**: `CLEAN DUPLICATE SCAN`. Scanned active (`.backlogit/stash.jsonl`) and archived (`.backlogit/archive/`) stash entries; `7E14915F` (`078-DL`) is the sole entry covering reconstitution crash-recovery mutation attribution / cross-writer coordination (including Candidates 1 and 2 at `internal/core/shipment_reconstitute_exec.go:589,1008,1092` + `internal/core/stash.go:733` and `internal/core/shipment_reconstitute_exec.go:1017,1081`), and `D3C8D13B` (`080-DL`) is the sole entry covering the pre-journal `CreateArtifact`/`CreateShipment` crash window (Candidate 3 at `internal/core/shipment_reconstitute_exec.go:673,761`).
* **P-021 C6 Obligation (B) — Late-Identifier Reconciliation**: No late review-thread identifier found for the threadless local review origin (`review-thread ID: N/A` stands as the truthful terminal record for the local review path); `PR N/A` on `D3C8D13B` is reconciled to `PR #638`, and correlated PR #638 Copilot comment `4434730479` is recorded alongside the source refs above.
* **Gate Verdict for `210-S` (PR #638)**: `Outcome: READY_WITH_FOLLOWUPS`, `Blocking findings: P0=0, P1=0` for `210-S` in-scope surface (zero `.go` changes required).
