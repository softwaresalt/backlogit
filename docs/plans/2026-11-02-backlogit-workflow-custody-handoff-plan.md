---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for Feature 210-F (Shipment 211-S, Stash 6DA6A627, Deliberation 081-DL, Blocker 1D48B514): implement native Backlogit Workflow Custody, Pre-Existing Change Adoption, and Stage-to-Ship Handoff domain extension (workflow-custody/v1 envelope, Profile A operator_verified CustodyApprovalToken + Profile B agent_workflow_handoff, atomic rename/case-only rename custody, canonical-direct CLI/MCP custody preview/offer/accept/verify, and derived SQLite custody_receipts projection).'
doc_type: plan
docline:
  backlogit_observed_head: f546e0f13a5e71f81f037c5e24e46146511f119e
  deliberation_id: 081-DL
  feature_id: 210-F
  gsd_head: 46bf4157b1001090efa6cd9b4f09e6d2eff35fb4
  observed_at: "2026-11-02T00:57:59Z"
  plan_hardening_status: completed
  plan_id: backlogit-workflow-custody-handoff
  requires_plan_hardening: "yes"
  review_status: passed_plan_review
  revision: 3
  shipment_id: 211-S
  source_decision: docs/decisions/2026-11-02-backlogit-workflow-custody-handoff-deliberation.md
  stash_id: 6DA6A627
  status: approved
  updated_at: "2026-11-02T07:05:00Z"
schema_version: "1.0"
source: docs/plans/2026-11-02-backlogit-workflow-custody-handoff-plan.md
title: 'Implementation Plan — Backlogit Workflow Custody, Pre-Existing Change Adoption, and Stage-to-Ship Handoff (210-F / 211-S)'
---

> **Document Status**: `approved` (`revision: 3`, `feature_id: 210-F`, `shipment_id: 211-S`, `stash_id: 6DA6A627`, `deliberation_id: 081-DL`) — Execution-ready `backlogit` implementation plan derived from `docs/decisions/2026-11-02-backlogit-workflow-custody-handoff-deliberation.md` and hardened under P-006.
> **Blocker Context**: Authorized Stage planning and decomposition for GSD blocker `1D48B514`. Existing holds on external GSD shipments `233-S` (`6B23AD1F` direct `kbdb` ruling + `FOREIGN_FILE_OVERLAP` on 7 revised Stage docs) and `234-S` remain strictly in force and are not bypassed or released by this plan.

## Objective

Deliver the native **Backlogit Workflow Custody, Pre-Existing Change Adoption, and Stage-to-Ship Handoff** domain extension (`workflow-custody/v1`) under covering feature `210-F` and shipment `211-S` (resolving `OD-1`, `OD-2`, and `OD-3` per `081-DL`):

* Promote optional `workflow_id` (alongside `branch`) on `shipment` in `defaultHeaderDef()` and `upgradeLegacyGeneratedHeaderDef` while storing the full `workflow-custody/v1` envelope under `custom_fields.workflow_custody` (`OD-1`, `U1` / `210.001-T`).
* Declare `workflow-custody/v1` types, event constants, sentinel errors, and `ErrNotImplemented` stubs (`U2` / `210.002-T`), followed by Lstat-confined path/rename validation and `Profile A` (`operator_verified`) `CustodyApprovalToken` minting and verification via `TrustAnchors` (`OD-2`, `RD1`, `RD2`, `U3` / `210.003-T`).
* Implement canonical Markdown/JSONL custody `preview`, `offer`, `accept`, and `verify` operations with `NonceLedger` replay protection, `Profile B` (`agent_workflow_handoff`) verification, `shipment-operation/v1` crash recovery journals, and `ErrWriteNotApplied` vs `ErrWriteIndeterminate` durability semantics (`U4` / `210.004-T`).
* Add the derived SQLite `custody_receipts` projection table in `EnsureSchema` and `Rehydrate` rebuild on `backlogit sync` (`OD-3`, `RD4`, `U5` / `210.005-T`) while keeping all CLI/MCP custody operations strictly canonical-direct (`U6` / `210.006-T` and `U7` / `210.007-T`).

## 1. Executive Conclusion & Observed Baseline

### 1.1 Verdict: **YES — Partial / Narrow Native Domain Extension Required in `backlogit` (Paired with Targeted GSD Gate & Adapter Fixes)**

`backlogit` **cannot** satisfy GSD requirements **R1–R8** through existing CLI/MCP API composition or `header-def.yaml` / WIT configuration alone, **nor** does it require a greenfield subsystem. Instead:

1. **What `backlogit` Already Provides (Substantial Low-Level Storage, Locking, Hashing, Journaling, and Approval Primitives)**:
   - `backlogit` already implements the core storage, locking, canonical hashing, transactional journaling, compare-and-swap (CAS), and approval primitives needed to host workflow custody:
     - **Canonical JSON & raw-byte SHA-256 hashing with UTF-8 / CRLF->LF normalization**: `internal/canonical/canonical.go:62-95` (`Canonicalize`, `Hash`, `HashBytes`).
     - **Power-loss-durable atomic file writes & two-class error contract (`ErrWriteNotApplied` vs `ErrWriteIndeterminate`)**: `internal/atomicfile/atomicfile.go`, `internal/core/durable_fs.go`, `internal/core/mutation_envelope.go:25-120` (`MutationEnvelope`), and `internal/events/stream.go:20-210` (`EventWriter`).
     - **Stable-identity cross-process advisory locks**: `shipment-lifecycle-global` (`internal/core/shipment.go:278-315`), per-shipment membership lock `A` (`lockShipmentMembership`), per-item log lock `C` (`events.LockItemLogCrossProcess` / `lockShipmentReconcileItemLog`, `internal/events/stream.go:180-215`, `internal/core/shipment_reconcile.go:130-137`), and sorted artifact-mutation lock batch `B` (`lockArtifactMutations` / `lockReconstituteOwners`, `internal/core/shipment_reconstitute_exec.go:175-196`).
     - **Two-phase durable operation journals (`shipment-operation/v1` under `.backlogit/ops/`) and startup crash-recovery barrier (`recoverPendingShipmentOperations`)**: `internal/core/shipment_ops.go:23-240`, `internal/core/shipment_recovery.go:140-260`.
     - **Zero-write `--dry-run` preview workspaces (`NewReadOnlyWorkspace`, `NewReadOnlyIndexWorkspace`)**: `internal/core/workspace_readonly.go`, `internal/core/workspace_readonly_index.go:18-42`, `internal/mcp/shipment_manifest_handlers.go:26-73`.
     - **Request-identity digest binding, idempotent outcome classification (`reconciled`/`no_op`/`conflict`/`indeterminate`), and optional Ed25519/HMAC authorization tokens with atomic nonce consumption**: `internal/core/shipment_reconcile.go:26-105`, `internal/core/shipment_reconcile_authorization.go:65-210`, `internal/core/authorization_token.go:10-48`, `internal/core/nonce_ledger.go:37-190`.

2. **Why Existing Public `backlogit` Surfaces Are Insufficient Without a Narrow Extension**:
   - **Public `create_item` / `update_item` cannot persist structured custom fields or update `references`/`branch`**: While `models.Artifact` (`internal/models/artifact.go:42-69`) and `updateArtifactUngated` (`internal/core/artifacts.go:634-662`) internally support `CustomFields`, `References`, and `Links`, the public MCP tool `backlogit_update_item` (`internal/mcp/tools.go:60-81`) and CLI `backlogit update` (`internal/cli/update.go:21-220`) expose **neither** `custom_fields`, `references`, nor `branch` (only `harness_status`, `size*`, and `complexity*` are wired into `CustomFields`).
   - **Existing governed operations are hard-coded to narrow, unrelated transitions**:
     - `ReconstituteShipments` (`internal/core/shipment_reconstitute.go:88-107`, committed on branch `210-S`) redistributes backlog items across shipments, and `validateReconstituteImagePath` (`internal/core/shipment_reconstitute_validate.go:74-96`) explicitly rejects any path outside `.backlogit/` Markdown artifacts and `.backlogit/archive/{stash,provenance_corrections}.jsonl`. It cannot record custody or adoption of repository paths (`docs/**`, `.gsd/config.yaml`, `internal/**`).
     - `ReconcileShipmentToShipped` (`internal/core/shipment_reconcile_transaction.go:53-250`) only repairs archived shipments to `archived_status: shipped` when a merge commit is reachable from `reconcile.trusted_refs`.
     - `NormalizeBlockedShipment` (`internal/core/shipment_recovery.go:23-138`) only accepts `shipment-bootstrap-snapshot/v1` for `target: blocked`.
     - `DisposeQueuedShipment` (`internal/core/shipment_disposition.go:34-215`) only transitions `queued -> abandoned`.
     - `CheckpointV1` (`internal/events/checkpoint_schema.go:20-85`) enforces a closed top-level namespace for ephemeral session crash recovery (`agent` limited to `ship|stage`, 64 KiB cap) and is resolved/archived at session end.
     - `backlogit_append_comment` (`internal/mcp/tools.go:128-137`) appends untyped prose to `logs/<id>.jsonl` with no schema validation, no content digests, no role/authority checks, no CAS, and no supersession semantics (R2, R4, R5, R7).
   - **Missing Domain Semantics**: `backlogit` lacks a governed **Workflow Custody & Change-Set Handoff/Adoption** contract that binds:
     1. A stable `workflow_id` + `scope_branch` + `base_commit_sha` + shipment set (`R1`, `R3`);
     2. Exact path/patch change-set entries with `(base_blob_sha256, index_sha256, worktree_sha256, patch_digest_sha256, staged_state, contributor, source_kind)` (`R2`, `R4`);
     3. Producer-to-recipient handoff offers and explicit operator/role acceptance receipts (`offer -> accept | reject | supersede`) that automatically invalidate when any covered path's content digest drifts (`R5`, `R6`, `R7`);
     4. Storage inside `.backlogit/` (Markdown frontmatter projection + append-only `.backlogit/logs/<shipment_id>.jsonl` + `.backlogit/ops/` journal) that avoids the pre-commit bootstrap deadlock (`R6`, `R8`).

---

### 1.2 Immutable Observed Workspace Baseline (`2026-11-02T00:57:59Z`)

| Surface | Observed State | Evidence / Pin |
|---|---|---|
| **GSD Workspace** (`C:\Source\GitHub\GSD`) | Branch `plan/backlogit-workflow-custody-handoff` at commit `46bf4157b1001090efa6cd9b4f09e6d2eff35fb4` | Pre-existing working/index/untracked state and `.gsd/worktree-baseline.json` preserved untouched. Engram daemon (`PID 60844`) bound (`ENGRAM_OK` for GSD). |
| **Backlogit Released Binary** | `backlogit version 1.12.0` (`v1.12.0` tag -> commit `718e710bb7ff8b1d4b4e004559c613fc5c5043ad`) | Includes `ReconcileShipmentToShipped` (`167-F`), `AuthorizationToken` (`170-F`), `CheckpointV1` disposition (`136-F`/`146-F`), `EvidenceDAG` (`162-F`), `PolicyEngine` (`163-F`). Does **not** include `207-S` (`remove`/`move`/`reorder`), `208-S` (`complexity` provenance), `209-S` (`adopt --keep-id`), or `210-S` (`reconstitute`). |
| **Backlogit `main` Branch** | Commit `45838e8af3312e64be33772c46f3f376a1ab6680` (PR #637 merge) | Adds `204-S` (`scheduler_baseline_claim`), `205-S` (single MCP daemon lock), `206-S` (gate error parity), `207-S` (`shipment remove/move/reorder`), `208-S` (`complexity_source`), `209-S` (`adopt --keep-id`, doctor manifest checks), and `210-S` stage artifacts. |
| **Backlogit Active Branch `HEAD`** | Branch `feat/210-s-declarative-multi-shipment-manifest-reconstitution-209-f` at commit `f546e0f13a5e71f81f037c5e24e46146511f119e` | Adds `ReconstituteShipments` (`internal/core/shipment_reconstitute*.go`) + CLI `backlogit shipment reconstitute` + MCP `backlogit_reconstitute_shipment` (`30c76e08`) + recovery/snapshot hardening (`fcd5e4ef`, `f546e0f1`). **Not yet merged to `main` or released.** |
| **Backlogit Uncommitted Working Tree (In-Flight / Concurrent)** | 4 dirty tracked files at initial probe (`2026-11-02T00:57:59Z`), 1 subsequent concurrent tracked edit observed during probe (`.autoharness/config.yaml`), and 2 untracked continuity files | Treated strictly as concurrent in-flight edits in an active external workspace (read-only fallback declared: `ENGRAM_DEGRADED` for cross-workspace backlogit queries to guarantee zero IPC/telemetry writes in `C:\Source\GitHub\backlogit`):<br>1. `internal/core/shipment_reconstitute_exec.go` (`HEAD` blob `5edbf63ed0e713ec6459247d114337b3776f08b2864` -> working SHA-256 `786b6c2d68f83604776def04561da674045a455408709114337b3776f08b2864`): adds `schedulerBaselineClaimKey` to `mutableField` (`line 113`) and extracts `reconstituteDirectOwners` (`lines 255-287`, `560`) so `EventIDs` only includes direct mutation owners.<br>2. `internal/core/reconstitution_round3_review_test.go` (`HEAD` blob `966ec275dfec4bd79ee06d06c85cd9966ef67bd5` -> working SHA-256 `762d8a5b9c39ca78ee55616ca909a2ff2dfad8c23ba067398ab13c48bbd3b6a5`): adds `TestRound3ReviewClaimMarkerRollback`, `TestRound3ReviewRelatedOwnerIndependentChanges`, `TestRound3ReviewEventIDsDirectOwnersOnly`.<br>3. `.backlogit/archive/078-DL.md` (`HEAD` blob `61e474668ddefd06a999bf3ddb70b2ea315608cd` -> working SHA-256 `9183e8a9816ebab2a4cad0bba674c037783004a3b86ee4c3fbbf4a0aebb284b1`): P-021 C6 notes for Candidates 4 & 5.<br>4. `docs/decisions/2026-11-01-reconstitution-scoped-recovery-preimage-invariant-deliberation.md` (`HEAD` blob `ac82b4eca11a0aa3923e9ab1b50dc5b1ae8e8500` -> working SHA-256 `88907aa36b981a9f20ebf5464e53b0f5fd0ec57c18a39da16f752c56801727eb`): matching deliberation update.<br>**Bounded Concurrent-State Disclaimer**: While `HEAD` (`f546e0f13a5e71f81f037c5e24e46146511f119e`) and branch (`feat/210-s-...`) remained unchanged during initial drafting, an external concurrent edit to `.autoharness/config.yaml` appeared in `backlogit` after the initial 4-file probe (shifting the aggregate working-tree `git diff HEAD` digest from `20e3b258...` to `573f139b...`). Our read-only session never wrote to or read secret values from `backlogit`, and the independent rubber-duck review verified primary source files directly rather than relying on parent-only diff summaries. |

> **Completion Freshness Footnote (`2026-11-02T01:18:20Z`)**: During final completion check, the concurrent external `backlogit` branch (`feat/210-s-declarative-multi-shipment-manifest-reconstitution-209-f`) advanced by **one commit** from observed baseline `f546e0f13a5e71f81f037c5e24e46146511f119e` to `b8c2a3be6ca5eda14f346041d3d1fc240da01973` (`fix(core): allow scheduler_baseline_claim in reconstitute rollback and scope direct event IDs`), committing the exact 4 in-flight reconstitution paths inventoried above plus 3 closure/memory docs (`zero` new CLI, MCP, or public custody APIs). This represents branch-commit promotion of the already-assessed in-flight `210-S` reconstitution fixes — **not** a released `backlogit` capability and **not** a completed custody implementation. The rubber-duck review (`4f911545`) and `revision: 2` readiness verdict apply to the `revision: 2` design and the original pinned baseline (`f546e0f1...`); no claim is made that reviewer `4f911545` independently inspected commit `b8c2a3be...`.

---

## 2. Root-Cause Analysis of the GSD Incident (`1D48B514` / `233-S` / `234-S`)

In GSD (`46bf4157b1001090efa6cd9b4f09e6d2eff35fb4`), three interacting mechanics caused legitimate Stage-authored and operator-edited files to collide with the worktree cleanliness gate:

1. **Identity-Blind Foreign Baseline Capture (`internal/gates/worktree.go:53-66, 719-770`)**:
   - `WorktreeBaseline` (`internal/gates/worktree.go:59-65`) stores only `SchemaVersion`, `ShipmentID`, `HeadSHA`, `CapturedAtUTC`, and `ForeignPaths map[string]WorktreeBaselineEntry` (`Path`, `IndexSHA256`, `WorktreeSHA256`). It records **no producer role (`stage` vs `operator` vs `ship`), no workflow ID, no scope branch, and no adoption/handoff status**.
   - `CaptureWorktreeBaselineForShipment` (`internal/gates/worktree.go:725-770`) classifies any dirty path not in Part A (`.backlogit/` plus `.gsd/carry-forward.yaml` committed at `HEAD`) and not named by an **active** stash entry (`sr.StashReferencedPaths(cands)`, `line 749`) into `ForeignPaths`.
   - Once Stage Step 5.6 archives the consumed stash entries, or when Stage authors deliberation/plan/review docs (`docs/decisions/*`, `docs/plans/*`, `docs/reviews/*`) for the workflow, any subsequent baseline capture classifies those uncommitted Stage docs as `ForeignPaths`.
2. **Implicit Baseline Mutation on Read-Only / Advisory `pre_claim` Probes (`internal/cli/cli.go:1040-1059`)**:
   - In `internal/cli/cli.go:1040-1059`, running `gsd gate pipeline-topology --phase pre_claim` sets `capture = true` whenever `len(blocking) == 0 && !claimNotObserved` (`lines 1044-1046`), calling `gates.CaptureWorktreeBaselineForShipment` (`line 1053`) and overwriting `.gsd/worktree-baseline.json` **even when invoked as a read-only/advisory check**.
   - When Stage subsequently revised 7 of the 9 staged Track A/B docs during review remediation, `ClassifyWorktreeDeltas` (`internal/gates/worktree.go:833-877`) saw those 7 paths in `baseline.ForeignPaths` with mutated `WorktreeSHA256` (`foreignDigestMutated(workspace, p, be)` at `line 855`) and halted with `FOREIGN_FILE_OVERLAP`.
3. **Why `.gsd/carry-forward.yaml` Cannot Solve This (`schemas/carry-forward.schema.json:1-47`)**:
   - `.gsd/carry-forward.yaml` is restricted by regex `^\.gsd/([A-Za-z0-9._-]+/)*[A-Za-z0-9._-]+$` (`schemas/carry-forward.schema.json:27`) to paths under `.gsd/`, is read only from committed `HEAD`, and only expresses *"can remain dirty in the worktree without blocking"* — **not** *"this specific diff/blob revision is adopted into workflow/shipment custody and authorized for branch commit"* (`R2`, `R4`, `R6`).

---

## 3. Requirements & Capability Matrix (`R1`–`R8`)

| Req | Requirement Summary | Released `v1.12.0` (`718e710b`) | Committed Branch `HEAD` (`f546e0f1`) & In-Flight Diff | Reusable `backlogit` Primitives (Exact Source Citations) | Remaining Semantic Gap | Owning Product |
|---|---|---|---|---|---|---|
| **R1** | Durable system of record for workflow iteration, accepted contributions, custody handoffs, approval refs, task/shipment DAG, and branch/commit/PR links; Git owns content identities; no parallel `.gsd/workflows` DB. | **Partial**: Task/shipment DAG (`dependencies.go`), commit links (`commits.go`), artifact links (`artifacts.go:1150`), and shipment manifests exist. No workflow/custody/handoff record exists. | Same as `v1.12.0` plus `ReconstituteShipments` (`shipment_reconstitute.go:88`) for manifest splits/regrouping. | - `models.Artifact` (`internal/models/artifact.go:42-69`)<br>- `AssociateCommit` (`internal/core/commits.go:53-195`)<br>- `AddDependency` (`internal/core/dependencies.go`)<br>- `AddArtifactLink` (`internal/core/artifacts.go:1150-1192`)<br>- `EventWriter` (`internal/events/stream.go:40-120`) | No structured representation of a workflow custody envelope or change-set handoff/adoption receipt on shipments or in `.backlogit/logs/<id>.jsonl`. | **Backlogit** (persistence & query) + **GSD** (orchestration & Git content verification) |
| **R2** | Agent-assisted single/multi-choice adoption of pre-existing changes (stash entries, `.gsd/config.yaml`, docs, code, staged+unstaged mixtures). Operator selects via diff previews/IDs; `backlogit` stores structured decision/approval refs + accepted immutable revision. Freeform comments are not custody. | **Absent**: `append_comment` (`internal/mcp/tools.go:128-137`) is freeform text. `CorrectStashProvenance` (`internal/core/stash_provenance.go:68-175`) only repoints `harvested_artifact_id` on archived stash entries. | `ReconstituteShipments` (`internal/core/shipment_reconstitute_plan.go:180-235`) supports declarative YAML plans + `--dry-run`, but only for backlog item redistribution. | - `canonical.Hash` / `canonical.HashBytes` (`internal/canonical/canonical.go:73-95`)<br>- `NewReadOnlyWorkspace` (`internal/core/workspace_readonly.go`)<br>- `CorrectStashProvenance` (`internal/core/stash_provenance.go:68`) for stash-only provenance | Needs a governed operation to preview (`--dry-run`) and record structured adoption receipts binding chosen candidate IDs/paths, pre/post index & worktree SHA-256 digests, and operator approval metadata. | **GSD** (interactive diff discovery & choice UI) + **Backlogit** (durable adoption receipt & `--dry-run` preview) |
| **R3** | Stable `workflow_id` scoping canonical workspace root, `scope_branch`, `base_commit_sha`, and approved shipment set across session/model/branch transitions. Branch-first (never direct edits/commits on `main`). | **Partial**: `shipment` has optional `branch` field in `defaultHeaderDef()` (`internal/config/defaults.go:150-153`), and `CheckpointContext` has `Branch` (`internal/events/checkpoint_schema.go:68`). Neither `update` CLI nor MCP `update_item` can set `shipment.branch` (`internal/cli/update.go:175-215`, `internal/mcp/tools.go:60-81`)! | In-flight `210-S` preserves `scheduler_baseline_claim` (`shipment_reconstitute_exec.go:113`), still no `workflow_id`, `scope_branch`, or `base_commit_sha` binding on shipments. | - `defaultHeaderDef()` shipment `branch` (`internal/config/defaults.go:150-153`)<br>- `isGitObjectName` / `isAncestor` / `inGitWorktreeBounded` (`internal/core/shipment_gate.go:34-210`)<br>- `ResolveDefaultBranch` (`internal/config/reconcile_defaults.go:51-95`) | `shipment` needs governed binding of `workflow_id`, `scope_branch` (rejecting default branch `main`/`master` for mutable work), and `base_commit_sha`, reachable via CLI & MCP. | **Backlogit** (schema + branch/base binding op) + **GSD** (branch-before-mutation enforcement) |
| **R4** | Change-set custody (not blanket directory allowlist): exact paths/patches or approved combined candidate, pre/post hashes, staged vs worktree distinction, contributor/source, accepting role, allowed action, approval evidence, revision & expiry/supersession, path containment. | **Absent**: `validateReconstituteImagePath` (`internal/core/shipment_reconstitute_validate.go:74-96`) is limited to `.backlogit/` `.md`/`.jsonl`. No repo-path change-set custody model exists. | Same gap at `f546e0f1` and in-flight working tree. | - `readReconstitutePlanFile` Lstat component walk + `IsSymlinkOrReparsePoint` (`internal/core/shipment_reconstitute_plan.go:109-175`)<br>- `checkRecoveryPath` (`internal/core/shipment_reconstitute_validate.go:101-130`)<br>- `rejectDuplicateJSONMembers` (`internal/core/json_member_scan.go`) | Needs typed `CustodyEntry` validation (repo-relative POSIX path, no `..`/drive/symlink/reparse escape, `base_blob_sha256`, `index_sha256`, `worktree_sha256`, `patch_digest_sha256`, `staged_state`, `contributor`, `source_kind`, `allowed_action`, `revision`). | **Backlogit** (record schema, path/hash/containment validator) + **GSD** (computing live Git index/worktree/patch digests) |
| **R5** | Producer-to-recipient handoff (`offer -> accept`), exact artifact/content binding, no self-authorized ownership, operator contributions use same acceptance path, content edits invalidate prior approvals/reviews, crash-safe idempotent persistence, no background daemon required. | **Partial**: `ReconcileShipmentToShipped` (`internal/core/shipment_reconcile_transaction.go:53-250`) has A/B/C/D locking, idempotency keys, request-identity digests, and crash-safe resume; `DisposeQueuedShipment` (`internal/core/shipment_disposition.go:34-215`) validates deliberation `authorizes_disposition`. | `executeReconstitution` (`internal/core/shipment_reconstitute_exec.go:300-495`) adds multi-owner `shipment-operation/v1` preimage journaling and rollback. | - `shipmentLifecycleJournal` (`internal/core/shipment.go:63-77`)<br>- `MutationEnvelope` (`internal/core/mutation_envelope.go:42-120`)<br>- `lockShipmentReconcileCThenB` (`internal/core/shipment_reconcile_transaction.go:130-145`)<br>- `guardArchivedStatusUnchangedSince` (`internal/core/shipment_reconcile_cas.go:48-70`) | Needs governed `offer` / `accept` / `supersede` state machine where: (a) `producer_role != accepting_role` (or `operator` acceptance), (b) revision increments on content change, and (c) digest mismatch automatically invalidates stale custody/review state. | **Backlogit** (transactional custody state machine & journal) + **GSD** (review freshness invalidation) |
| **R6** | Read-only preview/advisory gate MUST NOT create/re-capture foreign baseline. Formal adoption receipt overlays baseline without deleting history. Avoid pre-commit bootstrap deadlock. Explain cooperative local trust model. | **Partial**: `backlogit` dry-runs (`runShipmentManifestOpWith`, `internal/cli/shipment_manifest.go:45-68`) are strictly zero-write via `NewReadOnlyWorkspace`. GSD's `internal/cli/cli.go:1040-1059` violates R6 by writing `.gsd/worktree-baseline.json` during `pre_claim`. | Same in `f546e0f1`: `backlogit` dry-run tests (`shipment_reconstitute_dryrun_test.go`, `shipment_reconstitute_coldserver_test.go`) enforce zero writes. | - `NewReadOnlyWorkspace` / `NewReadOnlyIndexWorkspace` (`internal/core/workspace_readonly_index.go:18-42`)<br>- `manifestWorkspaceWith` (`internal/mcp/shipment_manifest_handlers.go:33-73`) | 1. **GSD**: Stop implicit `CaptureWorktreeBaselineForShipment` in advisory `pre_claim` (`internal/cli/cli.go:1042-1054`); require explicit baseline capture at workflow start.<br>2. **Backlogit + GSD**: Persist custody receipts in `.backlogit/` (Part A allowed) so pre-commit gates verify the live receipt + live file hashes without requiring the receipt to be committed first. | **GSD** (read-only gate fix + overlay evaluation) + **Backlogit** (receipt storage in `.backlogit/` + zero-write verify op) |
| **R7** | Not authority expansion: accepted contribution != scope expansion, destructive approval, hold release, merge approval, or role override. Typed authority refs/role validation; unknown/superseded/cross-workflow/cross-branch receipts fail closed. | **Partial**: `AuthorizationToken` (`internal/core/authorization_token.go:10-48`), `NonceLedger` (`internal/core/nonce_ledger.go:37-190`), and `policyengine` (`internal/policyengine/types.go:30-75`) enforce narrow scope binding and fail closed on wrong shipment or replay. | Same in `f546e0f1`. | - `VerifyAuthorizationToken` & `ApprovalRejectionError` (`internal/core/shipment_reconcile_authorization.go:118-215`)<br>- `NonceLedger.RecordBound` (`internal/core/nonce_ledger.go:75-115`) | Custody verification must enforce a closed `allowed_action` enum (`stage_artifact_commit`, `preexisting_adoption_commit`, `ship_task_commit`) and explicitly forbid treating a custody receipt as hold release (`6B23AD1F`), shipment claim, P-004 surface waiver, or PR merge approval. | **Backlogit** (strict enum + binding validation) + **GSD** (separate hold/review/merge gates) |
| **R8** | Git-friendly, human-readable Markdown/YAML persistence in `backlogit`, disposable SQLite index, CLI + MCP parity, capability advertisement in `MetadataCatalog` & `.gsd/backlog-registry.yaml`, version negotiation, adapter independence. | **Partial**: `BuildMetadataCatalog` (`internal/core/metadata_catalog.go:88-165`), `UpsertItem` custom-field projection (`internal/db/upsert_projection.go:25-135`), and `backlogit_get_version` (`internal/mcp/version_tool.go`) exist. | `210-S` adds `reconstitute_shipment` to `.autoharness/backlog-registry.yaml:411-419`. | - `BuildMetadataCatalog` (`internal/core/metadata_catalog.go:88-165`)<br>- `ToFrontmatterMap` (`internal/models/artifact.go:86-151`)<br>- `schemas/backlog-tool-registry.schema.json` (GSD) | Needs CLI (`backlogit custody ...`) + MCP (`backlogit_custody_*`) parity, SQLite rehydration projection from Markdown + JSONL, and `features.workflow_custody: true` capability negotiation in GSD's backlog registry. | **Backlogit** (Markdown/JSONL codec, SQLite rehydration, CLI/MCP parity) + **GSD** (registry schema & adapter negotiation) |

---

## 4. Architecture Evaluation & Alternatives Considered

### 4.1 Recommended Architecture: Narrow Native Domain Extension on `shipment` Artifacts + Append-Only Custody Log

Rather than inventing a parallel database or overloading unrelated reconstitution/reconcile commands, `backlogit` should add a **narrow, purpose-built Workflow Custody surface** (`internal/core/custody*.go`, `internal/cli/custody.go`, `internal/mcp/custody_handlers.go`) that composes existing `backlogit` primitives:

1. **Source-of-Truth Storage in `.backlogit/` (Markdown + JSONL)**:
   - **Shipment Frontmatter Projection (`.backlogit/queue/<shipment_id>.md`)**: Store the active workflow custody summary under `custom_fields.workflow_custody` (or top-level `header-def.yaml` shipment fields `workflow_id`, `scope_branch`, `base_commit_sha`, `custody_revision`, `custody_head_digest`) so it is human-readable in Git diffs and automatically survives `models.Artifact.ToFrontmatterMap()` (`internal/models/artifact.go:147-149`).
   - **Append-Only Custody Receipt Log (`.backlogit/logs/<shipment_id>.jsonl`)**: Every custody event (`workflow_custody_bound`, `custody_handoff_offered`, `custody_handoff_accepted`, `custody_adoption_recorded`, `custody_receipt_superseded`, `custody_receipt_invalidated`) is durably appended via `appendShipmentReconcileEvent` / `EventWriter` under the canonical `shipment-lifecycle-global -> lockShipmentMembership(A) -> LockItemLogCrossProcess(C) -> lockArtifactMutations(B)` lock order (`internal/core/shipment_reconcile_transaction.go:85-145`).
   - **Crash Recovery Journal (`.backlogit/ops/shipment-operation-<correlation_id>.json`)**: Multi-file writes (shipment Markdown + item log + optional SQLite index projection) use the existing `shipmentLifecycleJournal` (`schema_version: "shipment-operation/v1"`, `internal/core/shipment.go:63-77`) and `recoverPendingShipmentOperations` barrier.
   - **Canonical Read Independence & Optional Disposable SQLite Projection (`.backlogit/backlogit.db`, `RD4`)**: All core custody operations (`preview`, `offer`, `accept`, `verify`) read and validate the canonical shipment Markdown (`.backlogit/queue/<shipment_id>.md`), item log (`.backlogit/logs/<shipment_id>.jsonl`), and `.backlogit/ops/` nonce ledger directly via `readShipmentFromDiskReadOnly` / `NewReadOnlyWorkspace` (matching `ReorderShipment` in `internal/core/shipment_manifest.go:349-358` and `ReconcileShipmentToShipped` in `internal/core/shipment_reconcile_transaction.go:154-166`). The SQLite `custody_receipts` projection is an **optional derived optimization** for `backlogit query` SQL reporting rehydrated on `backlogit sync` — never a prerequisite for CLI/MCP custody operations and never a parallel state authority.

2. **Why This Eliminates the Pre-Commit Bootstrap Deadlock (`R6`)**:
   - In GSD's Two-Part Clean-Tree Contract (`P-011`), `.backlogit/` is a built-in Part A allowed prefix (`internal/gates/worktree.go:753`).
   - When an operator approves adopting pre-existing dirty files (or accepting a Stage->Ship handoff), `backlogit` writes the custody receipt into `.backlogit/queue/<shipment_id>.md` and `.backlogit/logs/<shipment_id>.jsonl` **on the active non-`main` work branch** (`plan/*`, `chore/stage-*`, or `feat/*`).
   - Because `.backlogit/` is Part A allowed, writing the receipt into `.backlogit/` does **not** create a new foreign-file violation.
   - GSD's `ClassifyWorktreeDeltas` (`internal/gates/worktree.go:833-877`) queries `backlogit` (read-only) for active, non-superseded custody receipts bound to `(workflow_id, shipment_id, scope_branch, base_commit_sha)` and verifies that each covered path's live `(index_sha256, worktree_sha256)` matches the receipt's accepted digests. If they match, the path is classified as **`Adopted` / `Owned`** (eligible to be staged and committed in the same commit as the `.backlogit/` receipt), completely avoiding the *"must commit receipt before allowed to commit adopted file"* deadlock!

### 4.2 Alternatives Rejected

| Alternative | Why Rejected |
|---|---|
| **Option A: Pure Configuration / Existing API Composition (No `backlogit` Go changes)** | **Impossible on current source**: `backlogit_update_item` (`internal/mcp/tools.go:60-81`) and `backlogit update` (`internal/cli/update.go:21-220`) cannot write `custom_fields`, `references`, or `branch`. Using `backlogit_append_comment` (`logs/<id>.jsonl`) provides zero schema validation, zero CAS/digest verification, zero supersession semantics, and violates `R2`/`R4`/`R7`. |
| **Option B: Repurpose `ReconstituteShipments` (`210-S`) or `ReconcileShipmentToShipped` (`167-F`)** | **Violates domain separation (`R7`)**: `ReconstituteShipments` redistributes backlog items across shipments and `validateReconstituteImagePath` (`internal/core/shipment_reconstitute_validate.go:74-96`) forbids non-`.backlogit/` paths. `ReconcileShipmentToShipped` is a post-merge archive repair requiring a commit already on `trusted_refs`. Overloading either would conflate destructive manifest/archive repair with pre-commit change-set custody. |
| **Option C: Store Custody & Adoption State Only in `.gsd/workflows/` or `.gsd/worktree-baseline.json`** | **Violates `R1` and `R8`**: Creates a parallel workflow/approval database in `.gsd/`, splits the system of record between GSD and `backlogit`, and leaves `.gsd/worktree-baseline.json` gitignored (invisible across clones/PRs). |

---

## 5. Proposed Data & Operation Contract (PROPOSED — Not Yet Existing CLI/MCP Surfaces)

> **Important Naming Notice**: All CLI commands (`backlogit custody ...`) and MCP tools (`backlogit_custody_*`) below are **PROPOSED** additions for the `backlogit` owner to implement. They do **not** exist in `backlogit v1.12.0` or at branch `HEAD` `f546e0f13a5e71f81f037c5e24e46146511f119e`.

### 5.1 Proposed Schema: Workflow Custody Envelope & Receipt (`workflow-custody/v1`)

Illustrative sample structured record (stored in `custom_fields.workflow_custody` on `shipment` `.md` and as typed event deltas in `.backlogit/logs/<shipment_id>.jsonl`; **all hashes/timestamps below are illustrative placeholders, not live approvals**):

```yaml
# Illustrative sample inside .backlogit/queue/233-S.md frontmatter (PROPOSED)
custom_fields:
  branch: "chore/stage-233-S"
  workflow_custody:
    schema_version: "workflow-custody/v1"
    workflow_id: "wf-20261102-233s-stage"
    shipment_id: "233-S"
    scope_branch: "chore/stage-233-S"
    base_commit_sha: "0000000000000000000000000000000000000000"
    custody_revision: 2
    active_receipt_id: "rcpt-0002"
    active_receipt_digest: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    receipts:
      - receipt_id: "rcpt-0002"
        revision: 2
        supersedes_receipt_id: "rcpt-0001"
        kind: "preexisting_adoption" # enum: preexisting_adoption | stage_to_ship_handoff | ship_task_custody
        authority_profile: "operator_verified" # enum: operator_verified | agent_workflow_handoff
        status: "accepted"           # enum: offered | accepted | rejected | superseded | invalidated
        producer_role: "operator"    # enum: operator | stage | ship | orchestrator
        accepting_role: "operator"   # enum: operator | stage | ship
        allowed_action: "preexisting_adoption_commit" # enum: stage_artifact_commit | preexisting_adoption_commit | ship_task_commit
        decision_ref: "operator-choice:sess-01/choice-02" # context link only (NOT self-sufficient authority)
        candidate_set_digest: "4444444444444444444444444444444444444444444444444444444444444444"
        approval_proof:
          channel: "operator_choice_token_v1" # issued by designated operator approval/choice channel, never agent-minted
          key_id: "operator-anchor-01"
          alg: "ed25519"
          principal: "operator:local"
          authorized_role: "operator"
          nonce: "019a0000-0000-7000-8000-000000000001"
          request_identity_digest: "5555555555555555555555555555555555555555555555555555555555555555"
          not_before_utc: "2026-11-02T01:00:00Z"
          not_after_utc: "2026-11-02T01:15:00Z"
          signature: "BASE64URL_SIGNATURE_PLACEHOLDER"
        idempotency_key: "adopt-233s-rev2"
        offered_at_utc: "2026-11-02T01:00:00Z"
        accepted_at_utc: "2026-11-02T01:01:00Z"
        expires_at_utc: "2026-11-09T01:01:00Z"
        entries:
          - path: "docs/plans/2026-11-01-track-a-stateless-runtime-and-process-lifecycle-plan.md"
            change_kind: "modified"             # enum: added | modified | deleted | renamed | staged_and_unstaged
            staged_state: "unstaged_only"       # enum: staged_only | unstaged_only | staged_matches_worktree | staged_diverges_accepted
            base_blob_sha256: "1111111111111111111111111111111111111111111111111111111111111111"
            index_sha256: "1111111111111111111111111111111111111111111111111111111111111111"
            worktree_sha256: "2222222222222222222222222222222222222222222222222222222222222222"
            patch_digest_sha256: "3333333333333333333333333333333333333333333333333333333333333333"
            contributor: "stage"                # enum: operator | stage | ship | mixed_reconciled
            source_kind: "stage_artifact"       # enum: stage_artifact | operator_config | operator_code_or_doc | stash_companion
          - path: "docs/decisions/2026-11-01-track-a-deliberation.md"
            old_path: "docs/decisions/2026-11-01-Track-A-Deliberation.md" # required when change_kind == "renamed"
            change_kind: "renamed"
            case_only_rename: true              # true when strings.EqualFold(old_path, path) && old_path != path
            staged_state: "staged_matches_worktree"
            old_base_blob_sha256: "6666666666666666666666666666666666666666666666666666666666666666"
            base_blob_sha256: ""                # empty when destination path did not exist at base_commit_sha
            index_sha256: "6666666666666666666666666666666666666666666666666666666666666666"
            worktree_sha256: "6666666666666666666666666666666666666666666666666666666666666666"
            patch_digest_sha256: "7777777777777777777777777777777777777777777777777777777777777777"
            contributor: "operator"
            source_kind: "operator_code_or_doc"
```

### 5.2 Key Invariants of the Contract

1. **Branch Scope Is Primary (`R3`)**:
   - `scope_branch` is mandatory and validated on every custody operation.
   - `scope_branch` **MUST NOT** equal the repository's default branch (`ResolveDefaultBranch`, e.g., `main` or `master`) for any mutable custody action (`stage_artifact_commit`, `preexisting_adoption_commit`, `ship_task_commit`).
   - Explicit branch transitions (e.g., `plan/<slug>` -> `chore/stage-<shipment_id>` -> `feat/<shipment_id>-<slug>`) require a governed `custody_handoff_offered` / `custody_handoff_accepted` transition that binds the new `scope_branch` and `base_commit_sha` while preserving `workflow_id`.
2. **Exact Change-Set, Mixed-Edit, and Rename Reconciliation (`R2`, `R4`, `RD2`)**:
   - Every `CustodyEntry` binds `base_blob_sha256` (at `base_commit_sha`), `index_sha256` (Git index stage 0), `worktree_sha256` (LF-normalized canonical digest via `canonical.HashBytes`), and `patch_digest_sha256`.
   - **Mixed Staged/Unstaged or Mixed Human/Agent Edits**: When `index_sha256 != base_blob_sha256` **and** `worktree_sha256 != index_sha256`, the entry is rejected unless `staged_state == "staged_diverges_accepted"` **and** `contributor == "mixed_reconciled"` backed by a Profile A `operator_verified` approval token (`RD1`).
   - **Rename & Case-Only Rename Semantics (`RD2`)**:
     - `change_kind` supports `added | modified | deleted | renamed | staged_and_unstaged`.
     - A rename is represented either as a single atomic `change_kind: "renamed"` entry carrying both `old_path` and `path` (with `old_base_blob_sha256` pinning the pre-image at `base_commit_sha` and `index_sha256`/`worktree_sha256`/`patch_digest_sha256` pinning the destination content, including combined rename+edit), or as paired `deleted` + `added` entries linked by a mandatory `rename_group_id`. Both sides of a rename MUST be adopted and reconciled together in the same receipt; selecting only one side of a staged rename or linked pair fails closed with `ErrPartialRenameAdoptionRejected`.
     - **No Similarity Heuristics as Authority**: Git rename similarity scores (`-M`) are never treated as custody authority; both `old_path` and `path` and their exact pre/post SHA-256 digests must be explicitly bound in the receipt.
     - **Windows Case-Only Renames (`Foo.md` -> `foo.md`)**: On case-insensitive filesystems, `custody verify` inspects the directory entry's exact on-disk case (`os.ReadDir` / handle final path per `2026-10-27-pin-the-handle-not-the-path.md`) and reconciles both `old_path` (from `WorktreeBaseline.ForeignPaths`) and `path` together so `old_path` is never left orphaned as a phantom `Overlap` in `ClassifyWorktreeDeltas` (`internal/gates/worktree.go:863-870`).
     - **Unsupported Change Kinds Fail Closed**: Unmodeled git status codes (such as unmerged conflict entries `U` or unlinked copy entries) fail closed with `ErrUnsupportedCustodyChangeKind` — never silently dropping custody.
   - Every `path` and `old_path` is validated through component-by-component `Lstat` + `IsSymlinkOrReparsePoint` (`internal/core/shipment_reconstitute_plan.go:142-163`), rejecting `..`, empty segments, backslashes, drive letters/colons, `.git/**`, and symlink/junction escapes, and every subsequent worktree digest read opens the target via a handle-pinned, no-follow open (`O_NOFOLLOW` / Windows `FILE_FLAG_OPEN_REPARSE_POINT`) and verifies the opened file's final path and file identity against the `Lstat` pre-check (`2026-10-27-pin-the-handle-not-the-path.md`) so a post-`Lstat` symlink or junction swap cannot redirect the read.
3. **Automatic Content-Drift Invalidation (`R5`, `R7`)**:
   - A receipt is valid for `path` **if and only if** the current workspace `index_sha256` and `worktree_sha256` match the receipt's recorded digests (or `worktree_sha256` matches both after `git add` stages the exact approved worktree content, and for `renamed` entries `old_path` is absent from the index/worktree except under a verified `case_only_rename` alias).
   - If any subsequent edit changes `worktree_sha256` or `index_sha256`, the prior receipt is evaluated as **`STALE_DIGEST_INVALIDATED`** (fail closed) until a new `revision` superseding the prior `receipt_id` is offered and accepted. When a reviewed plan or Stage artifact changes digest, GSD's review gate also marks the prior review attempt stale.
4. **Two Distinct Authority Profiles & Mandatory Operator Approval Floor (`R5`, `R7`, `RD1`)**:
   - **Deterministic Rule — Confirmation-Only Is Never Operator Custody Authority (`RD1`)**: A bare `--confirm: true` boolean, an asserted `by: "operator"` string, a timestamp, or a committed `decision_ref` document can all be self-authored by an agent and **MUST NEVER** be accepted as proof of independent human approval.
   - **Profile A — `operator_verified` (Mandatory for `kind: preexisting_adoption`, `contributor: operator | mixed_reconciled`, or any path in `WorktreeBaseline.ForeignPaths` without prior workflow custody)**:
     - Requires a cryptographically verified `approval_proof` (`CustodyApprovalToken`) issued through a designated operator approval/choice channel (e.g., host-minted interactive choice receipt or out-of-workspace operator trust-anchor key) that is **not callable as `operator` by ordinary agent MCP/CLI acceptance tools**.
     - Generalizes `backlogit`'s existing `AuthorizationToken` (`internal/core/authorization_token.go:10-48`), `VerifyAuthorizationToken` / `AuthorizeApproval` (`internal/core/shipment_reconcile_authorization.go:65-210`), `TrustAnchors` (`internal/config/trust_anchor.go`), and `NonceLedger.RecordBound` (`internal/core/nonce_ledger.go:75-115`) under a domain-separated digest (`backlogit/workflow-custody/request-identity/v1`) binding: `(issuer/key_id, principal, authorized_role == "operator", workflow_id, shipment_id, scope_branch, base_commit_sha, candidate_set_digest, allowed_action, target_revision, not_before_utc, not_after_utc, nonce)`.
     - Under locks `global -> A -> C -> B`, live `accept` verifies the signature against the resolved operator `TrustAnchor`, checks `not_before_utc <= now <= not_after_utc`, verifies live file digests equal `candidate_set_digest` (CAS), and atomically consumes `nonce` bound to `request_identity_digest` in `NonceLedger` (`--dry-run` verifies without consuming).
     - **Fail-Closed When Channel Unavailable**: Agent single/multi-choice UI remains the presentation/input layer, while the host/operator channel mints the `approval_proof`. If the host environment has no configured operator approval channel or trust anchor, `backlogit` and GSD MUST return explicit `APPROVAL_CHANNEL_UNAVAILABLE` (or `CUSTODY_CAPABILITY_UNSUPPORTED`) and halt — **never** downgrading to confirmation-only (`--confirm`).
   - **Profile B — `agent_workflow_handoff` (Permitted for `kind: stage_to_ship_handoff` and `ship_task_custody` Without Repeated Human Consent)**:
     - Routine agent-to-agent handoffs do **not** require a human `approval_proof` token **if and only if** all of the following hold under lock:
       1. `producer_role != accepting_role` (e.g., `stage -> ship` or `orchestrator -> ship`);
       2. Every path in `entries[]` was either (a) created/modified within the current workflow after `base_commit_sha` (not present in `WorktreeBaseline.ForeignPaths` as un-adopted pre-existing dirt) or (b) already adopted at that exact baseline digest by an earlier valid **Profile A (`operator_verified`)** receipt in the same `workflow_id`;
       3. Every path falls within the producer role's permitted path/file-budget scope (e.g., Stage planning/review/backlog paths for `stage_to_ship_handoff`; task file-budget paths for `ship_task_custody`);
       4. Upstream workflow gates for the handoff are satisfied (e.g., current-HEAD review manifest verdict `PASS`).
     - If any entry in a handoff touches an un-adopted `ForeignPaths` file or mixed human+agent edit, `backlogit custody accept` rejects `Profile B` with `ErrOperatorApprovalRequired` and requires `Profile A`.
5. **Cooperative Local Trust Model & Explicit Key-Separation Assumptions (`R6`, `R7`, `RD1`)**:
   - **What Profile A + Profile B protect against**: Agent tool calls self-supplying `--confirm: true` or fabricating a `decision_ref` to adopt unapproved dirty files (`RD1`), accidental cross-session/cross-role contamination, stale baseline overlap false-positives (`1D48B514`), TOCTOU races between concurrent CLI/MCP calls, and torn multi-file writes.
   - **Explicit Trust-Boundary Assumptions**: Profile A's `CustodyApprovalToken` guarantees independent operator origin **only when the signing key or host choice-channel issuer is isolated from the agent process** (e.g., held by the host IDE/CLI parent supervisor or an external operator key pin outside the agent's writable/readable sandbox). A shared local secret readable by the agent process or an unverified local log does **not** defeat a hostile same-user OS process. Furthermore, a custody receipt is **custody approval only** — never merge approval, hold release (`6B23AD1F`), scope expansion, or destructive action approval.

### 5.3 Proposed CLI & MCP Operations (Parity Contract)

| Proposed Abstract Op | Proposed CLI Command | Proposed MCP Tool | Mode / Mutability | Purpose |
|---|---|---|---|---|
| `preview_custody` | `backlogit custody preview {{shipment_id}} --candidates-file {{candidates_file}}` | `backlogit_custody_preview` | **Read-Only (`--dry-run`)** (`NewReadOnlyWorkspace`) | Validates candidate paths (including `renamed` pairs), computes `candidate_set_digest` and required `authority_profile` (`operator_verified` vs `agent_workflow_handoff`), and previews adoption classification with zero writes. |
| `offer_custody` | `backlogit custody offer {{shipment_id}} --spec {{spec}}` | `backlogit_custody_offer` | Governed Write (or `--dry-run` zero-write) | Records a `status: "offered"` custody receipt binding `workflow_id`, `scope_branch`, `base_commit_sha`, `producer_role`, `entries[]`, and `candidate_set_digest`. |
| `accept_custody` | `backlogit custody accept {{shipment_id}} --receipt {{receipt_id}} --by {{by}} --idempotency-key {{idempotency_key}}` | `backlogit_custody_accept` (Accepts `Profile B` agent handoffs directly; `Profile A` `operator_verified` requires valid `auth_token` from the operator approval channel and rejects bare `confirm: true`) | Governed Write (or `--dry-run` zero-write) | Under locks `global -> A -> C -> B`, verifies `authority_profile` (`CustodyApprovalToken` + `NonceLedger` consume for `Profile A`, or role/scope/gate invariants for `Profile B`), verifies live digests == offered digests (CAS), transitions receipt to `accepted`, increments `custody_revision`, and supersedes prior active receipt. |
| `verify_custody` | `backlogit custody verify {{shipment_id}} --branch {{branch}}` | `backlogit_custody_verify` | **Read-Only** (`NewReadOnlyWorkspace`) | Pure, zero-write verifier reading canonical Markdown + JSONL + trust anchors. Returns per-path verdicts (`ADOPTED_OPERATOR_VERIFIED`, `HANDOFF_AGENT_VERIFIED`, `UNVERIFIED_OPERATOR_PROVENANCE`, `STALE_DIGEST`, `PARTIAL_RENAME`, `UNCOVERED`, `WRONG_BRANCH`, `SUPERSEDED`). |

---

## 6. Implementation Units (Feature `210-F` / Shipment `211-S`)

To respect the 2-hour rule (`<= 3 files`, `<= 5 functions`, `<= 4 test scenarios`, single skill domain per task) and `backlogit`'s P-002.1 cycle-31 + `harness-architect` rule 6 test-first conventions (where every task's RED phase touches **ONLY** its `*_test.go` file with zero production files modified, compiles cleanly via `go test -timeout=30m -run=^$ -count=1 ./...`, and is observed failing before the GREEN phase runs), the 4 logical milestones (`Unit B1`–`Unit B4`) decompose into **7 atomic tasks (`210.001-T` through `210.007-T`)**:

| Unit / Task ID | Milestone Mapping | Target Files (`<= 3` per task) | Functions (`<= 5`) / Test Scenarios (`<= 4`) | Size / Complexity | Dependencies |
|---|---|---|---|---|---|
| **`U1` (`210.001-T`)** | `Unit B1a` (`OD-1`): Shipment `workflow_id` Header Schema & Legacy Upgrade | `internal/core/artifacts_headerdef_test.go`<br>`internal/config/defaults.go`<br>`internal/config/headerdef.go` | 2 functions, 3 test scenarios | Size: `XS`<br>Complexity: `low` | None (Root) |
| **`U2` (`210.002-T`)** | `Unit B1b`: `workflow-custody/v1` Types, Event Constants, Sentinel Errors & `ErrNotImplemented` Stubs | `internal/core/custody_shape_test.go`<br>`internal/core/custody_types.go`<br>`internal/core/custody_exec.go` | Stub declarations only (zero unverified behavior), 3 AST test scenarios | Size: `S`<br>Complexity: `low` | `210.001-T` |
| **`U3` (`210.003-T`)** | `Unit B1c` (`OD-2`, `RD1`, `RD2`): Custody Validator, Lstat + Handle-Pinned Rename/Path Containment & `Profile A` Token Mint/Verify | `internal/core/custody_validate_test.go`<br>`internal/core/custody_validate.go`<br>`internal/core/custody_types.go` | 5 functions, 4 test scenarios | Size: `S`<br>Complexity: `medium` | `210.002-T` |
| **`U4` (`210.004-T`)** | `Unit B2`: Canonical Markdown/JSONL Custody Preview, Offer, Accept, Verify & `.backlogit/ops/` Recovery Journal | `internal/core/custody_exec_test.go`<br>`internal/core/custody_exec.go`<br>`internal/core/shipment_ops.go`<br>`internal/core/shipment_recovery.go`<br>`internal/core/artifacts.go` | 5 functions, 4 test scenarios | Size: `M`<br>Complexity: `medium` | `210.003-T` |
| **`U5` (`210.005-T`)** | `Unit B3` (`OD-3`, `RD4`): Derived SQLite `custody_receipts` Schema Table, Live `UpsertItem` & `backlogit sync` Rehydration | `internal/db/custody_test.go`<br>`internal/db/schema.go`<br>`internal/db/rehydration.go`<br>`internal/db/queries.go`<br>`internal/db/upsert_tx.go` | 4 functions, 3 test scenarios | Size: `S`<br>Complexity: `low` | `210.004-T` |
| **`U6` (`210.006-T`)** | `Unit B4a` (`RD4`): MCP `backlogit_custody_*` Tools & Canonical-Direct Handlers | `internal/mcp/custody_handlers_test.go`<br>`internal/mcp/custody_handlers.go`<br>`internal/mcp/tools.go`<br>`.autoharness/backlog-registry.yaml` | 4 functions, 3 test scenarios | Size: `S`<br>Complexity: `low` | `210.004-T` *(independent of `210.005-T` per `RD4`)* |
| **`U7` (`210.007-T`)** | `Unit B4b`: CLI `backlogit custody`, `.autoharness/backlog-registry.yaml` CLI Parity & CLI Docs | `internal/cli/custody_parity_test.go`<br>`internal/cli/custody.go`<br>`internal/cli/root.go`<br>`.autoharness/backlog-registry.yaml`<br>*(+ generated `docs/cli-reference/`)* | 4 functions, 3 test scenarios | Size: `S`<br>Complexity: `low` | `210.005-T`, `210.006-T` |

### U1: Promote `shipment.workflow_id` in `defaultHeaderDef` and `upgradeLegacyGeneratedHeaderDef` (`code`) — `210.001-T`

* **Domain**: `code`
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `internal/core/artifacts_headerdef_test.go` ONLY — ZERO production files touched)**:
    1. In `internal/core/artifacts_headerdef_test.go`, add `TestU1_ShipmentWorkflowIDHeaderDefAndLegacyUpgrade` calling only already-existing `config.WriteDefaults`, `config.LoadHeaderDef`, and `db.GenerateSchemaExtensions` / `db.ApplySchemaExtensions` across 3 subtests:
       * `default_header_def_includes_shipment_workflow_id_and_branch`: verifies a freshly initialized workspace's `header-def.yaml` defines both `workflow_id` (`type: "string"`, `optional: true`) and `branch` (`type: "string"`, `optional: true`) on `shipment`.
       * `legacy_generated_header_def_upgrades_all_bitmask_variants`: verifies `config.LoadHeaderDef` transparently upgrades every prior generated `header-def.yaml` variant (missing `task.complexity`, `task.complexity_source`/`complexity_ruleset_version`, `shipment.status=blocked`, and/or `shipment.workflow_id`) to include `shipment.workflow_id`, while leaving an operator-customized `header-def.yaml` untouched.
       * `sqlite_schema_extensions_project_workflow_id`: verifies `db.GenerateSchemaExtensions` and `db.ApplySchemaExtensions` add `workflow_id TEXT` to the `items` table idempotently and `db.UpsertItem` projects `custom_fields.workflow_id` into `items.workflow_id`.
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 (zero production files modified) and `go test -timeout=30m -count=1 -run '^TestU1_ShipmentWorkflowIDHeaderDefAndLegacyUpgrade$' ./internal/core` fails because `shipment.workflow_id` is not yet in `defaultHeaderDef()` or `upgradeLegacyGeneratedHeaderDef`.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/config/defaults.go`, add `"workflow_id": {Type: "string", Optional: true}` to `Types["shipment"].Fields` in `defaultHeaderDef()`.
    2. In `internal/config/headerdef.go`, extend `upgradeLegacyGeneratedHeaderDef` from `mask < 8` to `mask < 16` (`dropWorkflowID := mask&8 != 0`, deleting `prior.Types["shipment"].Fields["workflow_id"]` when `dropWorkflowID` is true, and restoring `cfg.Types["shipment"].Fields["workflow_id"] = current.Types["shipment"].Fields["workflow_id"]` on match).
* **Affected files**:
  * `internal/core/artifacts_headerdef_test.go`
  * `internal/config/defaults.go`
  * `internal/config/headerdef.go`
* **Acceptance criteria**:
  * RED phase touches ONLY `internal/core/artifacts_headerdef_test.go` (zero production files modified), compiles cleanly (`go test -timeout=30m -run=^$ -count=1 ./...` exits 0), and fails on `TestU1_ShipmentWorkflowIDHeaderDefAndLegacyUpgrade`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/config/... ./internal/core/...`.
* **Execution posture**: test-first.

### U2: Declare `workflow-custody/v1` types, event constants, sentinel errors, and `ErrNotImplemented` stubs (`code`) — `210.002-T`

* **Domain**: `code`
* **Depends on**: U1 (`210.001-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `internal/core/custody_shape_test.go` ONLY — ZERO production files touched)**:
    1. In `internal/core/custody_shape_test.go`, add `TestU2_WorkflowCustodyDeclarationsSourceShape` using `go/parser` and `go/ast` only (reading files from disk with **ZERO direct Go symbol references** to not-yet-declared types or functions so `go test -timeout=30m -run=^$ -count=1 ./...` compiles cleanly before `custody_types.go` and `custody_exec.go` exist) across 3 subtests:
       * `custody_types_and_constants_declared`: asserts `internal/core/custody_types.go` declares constants `WorkflowCustodySchemaVersion = "workflow-custody/v1"`, `CustodyRequestIdentityDomain = "backlogit/workflow-custody/request-identity/v1"`, `DefaultCustodyReceiptExpiry`, `MaxCustodyApprovalTokenWindow`, event constants (`EventWorkflowCustodyBound`, `EventCustodyHandoffOffered`, `EventCustodyHandoffAccepted`, `EventCustodyAdoptionRecorded`, `EventCustodyReceiptSuperseded`, `EventCustodyReceiptInvalidated`), sentinel errors (`ErrCustodyDefaultBranchForbidden`, `ErrPartialRenameAdoptionRejected`, `ErrUnsupportedCustodyChangeKind`, `ErrOperatorApprovalRequired`, `ErrApprovalChannelUnavailable`, `ErrMixedIndexWorktreeDivergence`, `ErrCustodyStaleDigest`, `ErrCustodyWrongBranch`), and structs (`WorkflowCustodyEnvelope`, `CustodyReceipt`, `CustodyEntry`, `CustodyApprovalProof`, `CustodyCandidateSpec`, `CustodyPreviewOptions`, `CustodyPreviewResult`, `CustodyOfferOptions`, `CustodyAcceptOptions`, `CustodyMutationResult`, `CustodyVerifyOptions`, `CustodyPathVerdict`, `CustodyVerifyResult`).
       * `custody_validator_and_token_signatures_declared`: asserts `ValidateCustodyEnvelope`, `ValidateCustodyEntryPath`, `CustodyRequestIdentityDigest`, `MintCustodyApprovalToken`, and `VerifyCustodyApprovalToken` are declared with their exact exported parameter and return types (without freezing function bodies so U3 can replace stubs with real implementations).
       * `custody_execution_and_verify_signatures_declared`: asserts `internal/core/custody_exec.go` declares `PreviewCustodyCandidates`, `OfferCustodyHandoff`, `AcceptCustodyHandoff`, and `VerifyCustodyState` with their exact exported parameter and return types (without freezing function bodies so U4 can replace stubs with real implementations).
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 (zero production files modified) and `go test -timeout=30m -count=1 -run '^TestU2_WorkflowCustodyDeclarationsSourceShape$' ./internal/core` fails because `custody_types.go` and `custody_exec.go` do not exist yet.
  * **Second (GREEN phase, implemented by `build-feature` — declarations and `ErrNotImplemented` stubs ONLY, zero unverified behavior)**:
    1. Create `internal/core/custody_types.go` declaring the exported constants, sentinel errors, structs, and `blerrors.ErrNotImplemented` stubs for `ValidateCustodyEnvelope`, `ValidateCustodyEntryPath`, `CustodyRequestIdentityDigest`, `MintCustodyApprovalToken`, and `VerifyCustodyApprovalToken`.
    2. Create `internal/core/custody_exec.go` declaring `blerrors.ErrNotImplemented` stubs for `PreviewCustodyCandidates`, `OfferCustodyHandoff`, `AcceptCustodyHandoff`, and `VerifyCustodyState`.
* **Affected files**:
  * `internal/core/custody_shape_test.go`
  * `internal/core/custody_types.go`
  * `internal/core/custody_exec.go`
* **Acceptance criteria**:
  * RED phase touches ONLY `internal/core/custody_shape_test.go` (zero production files modified), compiles cleanly (`go test -timeout=30m -run=^$ -count=1 ./...` exits 0), and fails on `TestU2_WorkflowCustodyDeclarationsSourceShape`.
  * GREEN phase implements ONLY declarations and `ErrNotImplemented` stubs (zero unverified behavior) and passes `go test -timeout=30m -count=1 -race ./internal/core/...`.
* **Execution posture**: test-first.

### U3: Implement `workflow-custody/v1` validator, Lstat rename/path containment, and `Profile A` `CustodyApprovalToken` mint/verify (`code`) — `210.003-T`

* **Domain**: `code`
* **Depends on**: U2 (`210.002-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `internal/core/custody_validate_test.go` ONLY — ZERO production files touched)**:
    1. In `internal/core/custody_validate_test.go`, add `TestU3_CustodyValidationAndApprovalTokenVerification` calling the already-declared U2 symbols (`ValidateCustodyEnvelope`, `ValidateCustodyEntryPath`, `CustodyRequestIdentityDigest`, `MintCustodyApprovalToken`, `VerifyCustodyApprovalToken`) across 4 subtests:
       * `path_containment_and_symlink_reparse_rejection`: verifies `ValidateCustodyEntryPath` normalizes `ws.RootPath` via `filepath.Abs` + `filepath.EvalSymlinks` (including a relative `ws.RootPath` like `"."`), walks every path component with `os.Lstat` + `IsSymlinkOrReparsePoint`, rejects `..`, empty segments, backslashes, drive letters/UNC prefixes, `.git/**`, and leaf or intermediate symlinks/junctions (`mklink /J`), and verifies handle-pinned, no-follow file reads (`O_NOFOLLOW` / Windows `FILE_FLAG_OPEN_REPARSE_POINT` with opened-path/file-ID identity verification against the `Lstat` pre-check) deterministically reject a post-`Lstat` symlink or directory junction swap while accepting valid repo-relative POSIX paths (`docs/plans/...`, `internal/...`).
       * `atomic_rename_case_only_rename_and_unsupported_kind_guards` (`AC-11`, `RD2`): verifies `change_kind: "renamed"` requires non-empty `old_path`, `path`, and `old_base_blob_sha256` (or paired `deleted`+`added` entries sharing `rename_group_id`), detects `case_only_rename` when `strings.EqualFold(old_path, path) && old_path != path`, rejects partial rename selection with `ErrPartialRenameAdoptionRejected`, and rejects unmodeled status kinds (e.g., `unmerged`, `copied`) with `ErrUnsupportedCustodyChangeKind`.
       * `mixed_index_worktree_divergence_and_duplicate_key_guards` (`AC-03`): verifies an entry with `index_sha256 != base_blob_sha256` and `worktree_sha256 != index_sha256` fails with `ErrMixedIndexWorktreeDivergence` unless `staged_state == "staged_diverges_accepted"` and `contributor == "mixed_reconciled"` under `authority_profile == "operator_verified"`; also verifies `unicode.SimpleFold` duplicate JSON member rejection and YAML duplicate key rejection.
       * `profile_a_custody_approval_token_mint_verify_and_expiry_bounds` (`AC-02`, `AC-06`, `OD-2`, `OD-3`): verifies `CustodyRequestIdentityDigest` (domain-separated by `backlogit/workflow-custody/request-identity/v1`), `MintCustodyApprovalToken` (CLI/host-supervisor Ed25519 signer), and `VerifyCustodyApprovalToken` against `TrustAnchors`, enforcing `authorized_role == "operator"`, `not_after_utc - not_before_utc <= 15m` (`MaxCustodyApprovalTokenWindow`), default receipt `expires_at_utc = accepted_at_utc + 168h` (`DefaultCustodyReceiptExpiry`), and failing closed on tampered signature, wrong `shipment_id`/`workflow_id`/`scope_branch`/`candidate_set_digest`, or expired window.
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 (zero production files modified) and `go test -timeout=30m -count=1 -run '^TestU3_CustodyValidationAndApprovalTokenVerification$' ./internal/core` fails with `ErrNotImplemented`.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. Populate the full struct fields on `WorkflowCustodyEnvelope`, `CustodyReceipt`, `CustodyEntry`, `CustodyApprovalProof`, `CustodyCandidateSpec`, `CustodyPreviewOptions`, `CustodyPreviewResult`, `CustodyOfferOptions`, `CustodyAcceptOptions`, `CustodyMutationResult`, `CustodyVerifyOptions`, `CustodyPathVerdict`, and `CustodyVerifyResult` in `internal/core/custody_types.go`.
    2. Create `internal/core/custody_validate.go` implementing `ValidateCustodyEnvelope`, `ValidateCustodyEntryPath` (including handle-pinned no-follow file read helper with opened-path/file-ID verification against `Lstat` pre-check), `CustodyRequestIdentityDigest`, `MintCustodyApprovalToken`, and `VerifyCustodyApprovalToken` (taking variadic `anchors ...config.TrustAnchor` so 3-arg and explicit-anchor calls both work), removing their `ErrNotImplemented` stubs from `internal/core/custody_types.go`.
    3. Update `assertCustodyFuncs` in `internal/core/custody_shape_test.go` (Option 2 contract alignment) to accept functions declared in either `custody_types.go` or `custody_validate.go`, and strengthen `internal/core/custody_validate_test.go` with behavioral test assertions alongside the AST checks.
* **Affected files**:
  * `internal/core/custody_validate_test.go`
  * `internal/core/custody_shape_test.go`
  * `internal/core/custody_validate.go`
  * `internal/core/custody_types.go`
* **Acceptance criteria**:
  * RED phase touches ONLY `internal/core/custody_validate_test.go` (zero production files modified), compiles cleanly (`go test -timeout=30m -run=^$ -count=1 ./...` exits 0), and fails on `TestU3_CustodyValidationAndApprovalTokenVerification`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/core/...`.
* **Execution posture**: test-first.

### U4: Implement canonical Markdown/JSONL custody preview, offer, accept, verify, and crash-safe journal execution (`code`) — `210.004-T`

* **Domain**: `code`
* **Depends on**: U3 (`210.003-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `internal/core/custody_exec_test.go` ONLY — ZERO production files touched)**:
    1. In `internal/core/custody_exec_test.go`, add `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery` calling the already-declared U2 symbols (`PreviewCustodyCandidates`, `OfferCustodyHandoff`, `AcceptCustodyHandoff`, `VerifyCustodyState`) across 4 subtests:
       * `preview_and_verify_zero_write_and_default_branch_refusal` (`AC-05`, `AC-08`, `AC-10`): verifies `PreviewCustodyCandidates` and `VerifyCustodyState` run over `NewReadOnlyWorkspace` with zero file/SQLite/journal/lock writes, use `--no-replace-objects` for git status/blob inspection, compute worktree digests through the handle-pinned, no-follow reader with opened-path/file-ID identity verification (rejecting post-validation symlink/junction swaps), refuse `scope_branch == defaultBranch` (`main`/`master`) with `ErrCustodyDefaultBranchForbidden`, and report `WRONG_BRANCH` on cross-branch verification.
       * `profile_b_stage_to_ship_handoff_and_stale_digest_invalidation` (`AC-01`, `AC-04`, `AC-07`, `AC-11`): verifies `OfferCustodyHandoff` + `AcceptCustodyHandoff` under `Profile B` (`agent_workflow_handoff`, `producer_role: "stage"`, `accepting_role: "ship"`) transitions `offered -> accepted`, writes `custom_fields.workflow_custody`, `custom_fields.workflow_id`, and `custom_fields.branch` on `.backlogit/queue/<shipment_id>.md`, appends `custody_handoff_offered` and `custody_handoff_accepted` events to `.backlogit/logs/<shipment_id>.jsonl`, returns `HANDOFF_AGENT_VERIFIED` on `VerifyCustodyState` (including Windows `case_only_rename` directory-entry case verification), and returns `STALE_DIGEST` immediately when a covered file changes by 1 byte.
       * `profile_a_operator_adoption_nonce_replay_and_unverified_rejection` (`AC-02`, `AC-06`): verifies `Profile A` (`operator_verified`) requires a valid `CustodyApprovalToken`, atomically consumes `nonce` via `NonceConsumer.ConsumeBound(ctx, nonce, notAfter, requestIdentityDigest)` (`internal/core/nonce_consume.go:70-84`, backed by its external rollback-resistant `NonceConsumeProvider` and `NonceLedger` projection) on live `AcceptCustodyHandoff` (while `--dry-run` verifies without consuming), fails closed with `ErrNonceProviderUnavailable` when the external provider is nil/unavailable, rejects bare confirmation-only or missing trust-anchor configs with `ErrAuthorizationRequired` / `ErrApprovalChannelUnavailable`, rejects replayed nonces with `ErrApprovalReplay` / `ErrNonceReplayed` even after the workspace `NonceLedger` projection is deleted or rolled back, rejects `Profile B` on `ForeignPaths` with `ErrOperatorApprovalRequired`, and verifies `VerifyCustodyState` flags any unverified operator receipt in frontmatter as `UNVERIFIED_OPERATOR_PROVENANCE`.
       * `concurrent_locks_crash_journal_rollback_and_indeterminate_write_contract` (`AC-09`): verifies `OfferCustodyHandoff` and `AcceptCustodyHandoff` acquire `lockShipmentLifecycleGlobal -> lockShipmentMembership(A) -> events.LockItemLogCrossProcess(C) -> lockArtifactMutations(B)` (`C` before `B`, carrying held-lock context `withHeldItemLogLock` and `withHeldArtifactLocks` through persistence and append), journal preimages via `shipment-operation/v1` (`Operation: "custody_offer" | "custody_accept"`), roll back or reconcile cleanly on pre-commit failure (`ErrWriteNotApplied`) or when reopening a workspace with a pending `intent` journal via `recoverPendingShipmentOperations` / `reconcileShipmentLifecycleIntent` for both `custody_offer` and `custody_accept`, never roll back after the commit point (`ErrWriteIndeterminate`), and refuse ungoverned `UpdateArtifact` / `writeArtifactFileGoverned` edits to `custom_fields.workflow_custody`.
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 (zero production files modified) and `go test -timeout=30m -count=1 -run '^TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery$' ./internal/core` fails with `ErrNotImplemented`.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/core/custody_exec.go`, implement `PreviewCustodyCandidates`, `OfferCustodyHandoff`, `AcceptCustodyHandoff`, `VerifyCustodyState`, and `reconcileCustodyIntent` (acquiring `global -> A -> C -> B` with `C` before `B` and carrying held-lock context, operating directly on canonical Markdown + JSONL + `.backlogit/ops/` without requiring SQLite, and updating SQLite via `bldb.UpsertItem` when `ws.DB != nil`).
    2. In `internal/core/shipment_ops.go` and `internal/core/shipment_recovery.go`, register `shipmentOperationCustodyOffer = "custody_offer"` and `shipmentOperationCustodyAccept = "custody_accept"` in `validateShipmentLifecycleJournalRecord` (`shipment_ops.go`) and dispatch them in `reconcileShipmentLifecycleIntent` (`shipment_recovery.go:126-155`) to `reconcileCustodyIntent`.
    3. In `internal/core/artifacts.go`, add the ungoverned `custom_fields.workflow_custody` mutation refusal guard in `UpdateArtifactWithGate` / `writeArtifactFileGoverned` so generic artifact writes cannot tamper with `custom_fields.workflow_custody`.
* **Affected files**:
  * `internal/core/custody_exec_test.go`
  * `internal/core/custody_exec.go`
  * `internal/core/shipment_ops.go`
  * `internal/core/shipment_recovery.go`
  * `internal/core/artifacts.go`
* **Acceptance criteria**:
  * RED phase touches ONLY `internal/core/custody_exec_test.go` (zero production files modified), compiles cleanly (`go test -timeout=30m -run=^$ -count=1 ./...` exits 0), and fails on `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/core/...`.
* **Execution posture**: test-first.

### U5: Add derived SQLite `custody_receipts` schema table and `backlogit sync` rehydration projection (`code`) — `210.005-T`

* **Domain**: `code`
* **Depends on**: U4 (`210.004-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `internal/db/custody_test.go` ONLY — ZERO production files touched)**:
    1. In `internal/db/custody_test.go`, add `TestU5_CustodyReceiptsSchemaAndRehydration` calling only already-existing `db.Open`, `db.EnsureSchema`, `db.Rehydrate`, and `database.Query` across 3 subtests:
       * `ensure_schema_creates_custody_receipts_idempotently`: verifies `db.EnsureSchema` creates table `custody_receipts(shipment_id, receipt_id, workflow_id, scope_branch, base_commit_sha, revision, kind, authority_profile, status, producer_role, accepting_role, allowed_action, candidate_set_digest, offered_at_utc, accepted_at_utc, expires_at_utc, entries_json, PRIMARY KEY (shipment_id, receipt_id))` plus indexes `idx_custody_receipts_shipment` and `idx_custody_receipts_workflow`, and is idempotent across repeated `EnsureSchema` calls.
       * `rehydrate_and_live_upsert_item_rebuild_custody_receipts_and_clear_ghosts`: verifies that calling public `db.UpsertItem` on a shipment `.md` carrying `custom_fields.workflow_custody` immediately populates and updates `custody_receipts` in the same SQLite transaction **before** `db.Rehydrate` (`backlogit sync`) runs, and that `db.Rehydrate` rebuilds `custody_receipts` from shipment Markdown and clears stale rows when a shipment is removed.
       * `introspect_schema_includes_custody_receipts`: verifies `db.IntrospectSchema` reports `custody_receipts` and its indexes deterministically for `MetadataCatalog`.
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 (zero production files modified) and `go test -timeout=30m -count=1 -run '^TestU5_CustodyReceiptsSchemaAndRehydration$' ./internal/db` fails because `custody_receipts` is not created by `EnsureSchema` or populated by `UpsertItem`/`Rehydrate`.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/db/schema.go`, add `CREATE TABLE IF NOT EXISTS custody_receipts (...)` and `CREATE INDEX IF NOT EXISTS idx_custody_receipts_shipment` / `idx_custody_receipts_workflow` to `EnsureSchema`.
    2. In `internal/db/rehydration.go`, `internal/db/upsert_tx.go`, and `internal/db/queries.go`, clear `DELETE FROM custody_receipts` during `Rehydrate` Phase 2 (`Clear`), call `upsertCustodyReceiptsFromArtifactTx(ctx, tx, artifact)` during `Rehydrate` Phase 3 (`Batch-insert`) and in `upsertItemTx` (`internal/db/upsert_tx.go:25`), and update public `UpsertItem` (`internal/db/queries.go:132-178`) to execute the `items` upsert and `upsertCustodyReceiptsFromArtifactTx` inside a single SQLite transaction (`db.BeginTx`) so live `backlogit custody offer`/`accept` mutations immediately update `custody_receipts` prior to `backlogit sync`.
* **Affected files**:
  * `internal/db/custody_test.go`
  * `internal/db/schema.go`
  * `internal/db/rehydration.go`
  * `internal/db/queries.go`
  * `internal/db/upsert_tx.go`
* **Acceptance criteria**:
  * RED phase touches ONLY `internal/db/custody_test.go` (zero production files modified), compiles cleanly (`go test -timeout=30m -run=^$ -count=1 ./...` exits 0), and fails on `TestU5_CustodyReceiptsSchemaAndRehydration`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/db/...`.
* **Execution posture**: test-first.

### U6: Register and implement `backlogit_custody_*` MCP tools and canonical-direct handlers (`code`) — `210.006-T`

* **Domain**: `code`
* **Depends on**: U4 (`210.004-T`) *(intentionally independent of U5 / `210.005-T` per `RD4` so MCP custody operations execute directly against canonical `.md`/`.jsonl` even without SQLite)*
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `internal/mcp/custody_handlers_test.go` ONLY — ZERO production files touched)**:
    1. In `internal/mcp/custody_handlers_test.go`, add `TestU6_MCPCustodyToolsCanonicalDirectAndColdServer` calling only already-existing `NewServer`, `NewServerForRoot`, `ListTools`, and `InvokeTool` across 3 subtests:
       * `mcp_custody_preview_and_verify_cold_server_zero_write` (`AC-08`, `AC-10`, `RD4`): invokes `backlogit_custody_preview` and `backlogit_custody_verify` on a cold MCP server (`NewServerForRoot(root)` where `s.Workspace == nil`, using `s.manifestWorkspace(ctx, true)` / `core.NewReadOnlyWorkspace`) with `.backlogit/backlogit.db` absent, confirming zero files/DB created and valid JSON verdicts returned.
       * `mcp_custody_offer_and_accept_profile_a_and_profile_b` (`AC-01`, `AC-02`, `AC-06`, `AC-10`, `RD4`): invokes `backlogit_custody_offer` and `backlogit_custody_accept` for both `Profile B` (`stage_to_ship_handoff`) and `Profile A` (`preexisting_adoption` with signed `auth_token`), verifies that live `offer`/`accept` also succeed on a cold MCP server when the disposable SQLite cache (`.backlogit/backlogit.db`) is unusable/corrupted by falling back to a DB-optional writable canonical workspace (`ws.DB == nil` with `HeaderDef` loaded and pending `.backlogit/ops/` recovery run under `lockShipmentLifecycleGlobal`), and verifies that `Profile A` without `auth_token` (or with bare `confirm: true`) fails closed with structured authorization error.
       * `mcp_custody_error_mapping_and_non_nil_slices`: verifies all collection fields (`entries`, `verdicts`, `receipts`) serialize as `[]` (never `null`) and responses remain `< 10 KiB`.
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 (zero production files modified) and `go test -timeout=30m -count=1 -run '^TestU6_MCPCustodyToolsCanonicalDirectAndColdServer$' ./internal/mcp` fails because `backlogit_custody_*` tools are not registered yet.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. Create `internal/mcp/custody_handlers.go` implementing `custodyWorkspace(ctx, dryRun)` (using `core.NewReadOnlyWorkspace` for `preview`/`verify`/dry-run, and for live `offer`/`accept` using `s.requireWorkspace(ctx)` with a DB-optional writable canonical workspace fallback when the disposable SQLite cache is unusable so canonical Markdown/JSONL writes and `.backlogit/ops/` recovery still succeed with `ws.DB == nil`), `handleCustodyPreview`, `handleCustodyOffer`, `handleCustodyAccept`, and `handleCustodyVerify`, and register `backlogit_custody_preview`, `backlogit_custody_offer`, `backlogit_custody_accept`, and `backlogit_custody_verify` in `internal/mcp/tools.go` (`RegisterTools`).
    2. In `.autoharness/backlog-registry.yaml`, add `preview_custody`, `offer_custody`, `accept_custody`, and `verify_custody` with `mcp_tool` and interim `mcp_only: true` (plus `features.workflow_custody: true`) so the Wave 5 full-suite convergence gate (`TestRegistryParity_EveryMCPToolMappedOrDeferred`) passes cleanly before `U7` (`210.007-T`) wires the CLI commands and replaces `mcp_only: true` with `cli_command`.
* **Affected files**:
  * `internal/mcp/custody_handlers_test.go`
  * `internal/mcp/custody_handlers.go`
  * `internal/mcp/tools.go`
  * `.autoharness/backlog-registry.yaml`
* **Acceptance criteria**:
  * RED phase touches ONLY `internal/mcp/custody_handlers_test.go` (zero production files modified), compiles cleanly (`go test -timeout=30m -run=^$ -count=1 ./...` exits 0), and fails on `TestU6_MCPCustodyToolsCanonicalDirectAndColdServer`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/mcp/... ./internal/cli/...`.
* **Execution posture**: test-first.

### U7: Wire `backlogit custody` CLI subcommands, promote `.autoharness/backlog-registry.yaml` CLI commands, and verify CLI/MCP registry parity (`code`) — `210.007-T`

* **Domain**: `code`
* **Depends on**: U5 (`210.005-T`), U6 (`210.006-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `internal/cli/custody_parity_test.go` ONLY — ZERO production files touched)**:
    1. In `internal/cli/custody_parity_test.go`, add `TestU7_CustodyCLIMCPParityAndRegistryContract` calling only already-existing `NewRootCommand()`, `Execute()`, and `mcpinternal.NewServer` across 3 subtests:
       * `cli_custody_preview_offer_accept_verify_and_jsonrpc_parity` (`AC-01`–`AC-11`): executes `backlogit custody preview`, `backlogit custody offer`, `backlogit custody accept`, and `backlogit custody verify` (both standard `--json` and `--jsonrpc` modes) and compares normalized JSON envelopes and error codes 1:1 against the MCP tools in separate `t.TempDir()` workspaces.
       * `registry_parity_and_authoritative_dispatch`: looks up `preview_custody`, `offer_custody`, `accept_custody`, and `verify_custody` in `.autoharness/backlog-registry.yaml` (asserting non-empty `cli_command` and `mcp_only: false`) and verifies `TestRegistryParity_*` passes with zero Class-B/C/E drift.
       * `sql_query_projection_after_custody_accept`: verifies that immediately after live `backlogit custody accept` (before `backlogit sync`) and after `backlogit sync`, `SELECT shipment_id, receipt_id, authority_profile, status FROM custody_receipts` returns the accepted receipt row.
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 (zero production files modified) and `go test -timeout=30m -count=1 -run '^TestU7_CustodyCLIMCPParityAndRegistryContract$' ./internal/cli` fails because `NewCustodyCmd` and the `cli_command` registry bindings are not wired yet.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. Create `internal/cli/custody.go` implementing `NewCustodyCmd(cwd *string)` with subcommands `preview`, `offer`, `accept`, and `verify` (routing `preview`, `verify`, and `--dry-run` through `core.NewReadOnlyWorkspace` so dry-runs are strictly zero-write, and falling back to a DB-optional writable canonical workspace when SQLite initialization fails during live `offer`/`accept`), and register `root.AddCommand(NewCustodyCmd(&cwd))` in `internal/cli/root.go`.
    2. In `.autoharness/backlog-registry.yaml`, promote `preview_custody`, `offer_custody`, `accept_custody`, and `verify_custody` from interim `mcp_only: true` to full `cli_command` (`{{...}}` placeholders matching `cobra.ExactArgs` and required flags) and `params` mappings (omitting `governed: true`, matching `remove_from_shipment`, `move_shipment_item`, `reorder_shipment`, and `reconstitute_shipment`).
    3. Run `go run ./cmd/gen-docs docs/cli-reference` to generate `docs/cli-reference/backlogit_custody*.md` and verify `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Affected files**:
  * `internal/cli/custody_parity_test.go`
  * `internal/cli/custody.go`
  * `internal/cli/root.go`
  * `.autoharness/backlog-registry.yaml`
  * `docs/cli-reference/backlogit_custody.md` *(generated via `cmd/gen-docs`)*
* **Acceptance criteria**:
  * RED phase touches ONLY `internal/cli/custody_parity_test.go` (zero production files modified), compiles cleanly (`go test -timeout=30m -run=^$ -count=1 ./...` exits 0), and fails on `TestU7_CustodyCLIMCPParityAndRegistryContract`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/cli/... ./internal/mcp/... ./internal/core/... ./internal/db/...`.
  * `go run ./cmd/gen-docs docs/cli-reference` followed by `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Execution posture**: test-first.

## Dependency Graph

* **Feature**: `210-F` (`Backlogit Workflow Custody, Pre-Existing Change Adoption, and Stage-to-Ship Handoff (1D48B514)`)
* **Shipment**: `211-S` (queued, containing `210-F` + `210.001-T` through `210.007-T`)
* **Task Execution DAG**:
  * **Wave 1**: `210.001-T` (`U1` — `shipment.workflow_id` in `defaultHeaderDef` and `upgradeLegacyGeneratedHeaderDef`)
  * **Wave 2**: `210.002-T` (`U2` — blocked by `210.001-T`; `workflow-custody/v1` types, constants, sentinel errors, and `ErrNotImplemented` stubs)
  * **Wave 3**: `210.003-T` (`U3` — blocked by `210.002-T`; custody validator, Lstat rename/path containment, and `Profile A` `CustodyApprovalToken` mint/verify)
  * **Wave 4**: `210.004-T` (`U4` — blocked by `210.003-T`; canonical Markdown/JSONL custody `preview`, `offer`, `accept`, `verify`, and `.backlogit/ops/` recovery journal)
  * **Wave 5 (Parallelizable Canonical-Direct MCP + Optional Derived SQLite Projection)**:
    * `210.005-T` (`U5` — blocked by `210.004-T`; derived SQLite `custody_receipts` table and `Rehydrate` rebuild)
    * `210.006-T` (`U6` — blocked by `210.004-T`; MCP `backlogit_custody_*` tools and handlers operating directly on canonical `.md`/`.jsonl`)
  * **Wave 6**: `210.007-T` (`U7` — blocked by `210.005-T` and `210.006-T`; CLI `backlogit custody`, `.autoharness/backlog-registry.yaml`, `TestRegistryParity_*`, and `cmd/gen-docs`)

## Decisions and Rationale (`081-DL` — Resolutions of `OD-1`, `OD-2`, `OD-3`)

1. **`OD-1` Resolved — Store Envelope in `custom_fields.workflow_custody` and Promote `shipment.workflow_id` (`U1` / `210.001-T`)**:
   * *Decision*: Store the nested `workflow-custody/v1` receipt envelope under `custom_fields.workflow_custody` on `shipment` Markdown files while promoting `workflow_id` (alongside existing `branch`) as a top-level optional string field on `shipment` in `defaultHeaderDef()` (`internal/config/defaults.go`) with an automatic bitmask upgrade in `upgradeLegacyGeneratedHeaderDef` (`internal/config/headerdef.go`).
   * *Rationale*: Keeps top-level `header-def.yaml` scalar fields flat and SQLite-column-projectable (`items.workflow_id`, `items.branch`) while preserving the full structured receipt history in `custom_fields.workflow_custody`.
2. **`OD-2` Resolved — Enforce `Profile A` `CustodyApprovalToken` via `TrustAnchors` + `NonceConsumer.ConsumeBound` with CLI/Host-Supervisor Signer Support (`U3` / `210.003-T`, `U4` / `210.004-T`)**:
   * *Decision*: `Profile A` (`operator_verified`, mandatory for `preexisting_adoption`, `ForeignPaths`, and `mixed_reconciled` edits) ALWAYS requires an Ed25519/HMAC `CustodyApprovalToken` verified against `TrustAnchors` and consumed via `NonceConsumer.ConsumeBound` (`internal/core/nonce_consume.go:70-84`, backed by an external, rollback-resistant `NonceConsumeProvider` and `NonceLedger` projection) under locks `shipment-lifecycle-global -> A -> C -> B`, with CLI/host-supervisor signer support (`MintCustodyApprovalToken`). Confirmation-only (`--confirm: true` or `decision_ref`) is never accepted; missing trust-anchor, approval channel, or external `NonceConsumeProvider` support fails closed with `ErrApprovalChannelUnavailable` / `ErrAuthorizationRequired` / `ErrNonceProviderUnavailable`.
   * *Rationale*: Satisfies `RD1` and `R5`/`R7` by ensuring an agent cannot self-authorize adoption of operator or foreign working-tree files.
3. **`OD-3` Resolved — `168h` Default Receipt Expiry, `15m` Token Window, and Full 4-Milestone Delivery (`B1`–`B4`) in Shipment `211-S` (`U3`–`U7`)**:
   * *Decision*: Default receipt `expires_at_utc` to `168h` (7 days) from `accepted_at_utc` and cap `CustodyApprovalToken` validity (`not_after_utc - not_before_utc`) at `15m`. Include both the canonical-direct MVP (`B1 -> B2 -> B4`) and the derived SQLite `custody_receipts` projection (`B3`, `210.005-T`) in Shipment `211-S` while keeping `B4a` (`210.006-T`) independent of `B3` so CLI/MCP custody operations never depend on SQLite.
   * *Rationale*: Delivers immediate `backlogit query` SQL observability while preserving `RD4` canonical Markdown/JSONL independence.

---

## 7. GSD Integration Responsibilities & Remediation Sequencing

Once `backlogit` lands Shipment `211-S` (`210-F`), GSD owns the following changes (to be planned and executed in GSD's own pipeline, **not** inside `backlogit`):

1. **Fix Advisory/Read-Only `pre_claim` Baseline Mutation (`internal/cli/cli.go:1040-1059`) — Immediate GSD Remedy for `1D48B514`**:
   * Decouple read-only topology inspection from baseline mutation: `gsd gate pipeline-topology --phase pre_claim` MUST NOT call `gates.CaptureWorktreeBaselineForShipment` unless an explicit `--capture-baseline` flag (or the governed claim operation `gsd op` / `Step 0.5` post-branch-creation step) requests it.
   * Preserve the initial baseline captured at workflow start rather than overwriting `.gsd/worktree-baseline.json` on intermediate advisory probes.
2. **Overlay Verified `backlogit` Custody Receipts in `ClassifyWorktreeDeltas` (`internal/gates/worktree.go:833-877`)**:
   * Extend `WorktreeBaselineReader` with an optional `CustodyReceiptReader` interface that calls `backlogit custody verify <shipment-id> --branch <current-branch> --json` (read-only).
   * For any path in `baseline.ForeignPaths`, GSD requires a verdict of **`ADOPTED_OPERATOR_VERIFIED` (`Profile A`)** (or `HANDOFF_AGENT_VERIFIED` where the path's baseline digest was previously adopted under `Profile A` in the same `workflow_id`). A confirmation-only or unverified receipt for a `ForeignPaths` entry returns `UNVERIFIED_OPERATOR_PROVENANCE` and is **rejected fail-closed** (`FOREIGN_FILE_OVERLAP`).
   * For `change_kind: "renamed"`, reconcile both `old_path` and `path` together so `old_path` in `baseline.ForeignPaths` is marked covered when `path` (and `old_path` removal / Windows case-only rename) verifies as `ADOPTED_OPERATOR_VERIFIED`.
   * If the receipt is missing, `UNVERIFIED_OPERATOR_PROVENANCE`, `STALE_DIGEST`, `PARTIAL_RENAME`, `WRONG_BRANCH`, or `SUPERSEDED`, fail closed with `FOREIGN_FILE_OVERLAP` / `WORKTREE_DIRTY`.
3. **Interactive Choice-Assisted Adoption UX & Operator Approval Channel in Stage / Orchestrator (`R2`, `RD1`)**:
   * When pre-existing dirty files exist at intake or before staging/claim, GSD enumerates the candidate diffs + SHA-256 digests, presents a numbered single/multi-choice prompt to the operator (with diff preview), and routes the operator's selection through the host/operator approval channel to mint the bound `CustodyApprovalToken` (`Profile A`) before calling `backlogit custody offer` + `backlogit custody accept`.
   * If the host environment does not provide a verifiable operator approval/choice channel or configured operator trust anchor, GSD halts with explicit `APPROVAL_CHANNEL_UNAVAILABLE` and never self-signs or substitutes `--confirm: true`.
4. **Adapter Independence & Version Negotiation (`.gsd/backlog-registry.yaml`, `schemas/backlog-tool-registry.schema.json`) (`R8`)**:
   * Add optional `features.workflow_custody: boolean` (`default: false`) and optional operations `preview_custody`, `offer_custody`, `accept_custody`, `verify_custody` to `schemas/backlog-tool-registry.schema.json` (via a new versioned schema `schemas/backlog-tool-registry/1.1.0.schema.json` — never mutating an existing versioned schema in place per GSD compound learning `2026-08-30-157-s-149-f-schema-mutation-in-place-third-occurrence.md`).
   * Probe both installed `backlogit --version` (`backlogit_get_version`) and `backlogit_get_metadata_catalog` at Step 0.0; never assume the installed binary matches `backlogit` source `HEAD`. When `features.workflow_custody` is unsupported (e.g., `backlog-md` or older `backlogit <= 1.12.0`), report explicit `CUSTODY_CAPABILITY_UNSUPPORTED` rather than creating a silent parallel tracker in `.gsd/`.
5. **Sequencing Relative to Held Shipments `233-S` and `234-S`**:
   * **Step 1**: Remediate blocker `1D48B514` (GSD read-only `pre_claim` baseline fix + `backlogit` custody handoff/adoption support in `211-S`).
   * **Step 2**: Once the mechanism is active, run explicit operator choice-assisted adoption for the 9 Track A/B Stage artifacts (including the 7 revised docs and new review artifacts) on a non-`main` branch.
   * **Step 3**: Independently address hold `6B23AD1F` (direct `kbdb` ruling) on `233-S`. **Neither this plan nor any custody receipt implicitly releases holds on `233-S` or `234-S`.**

---

## 8. Failure-First Acceptance Matrix

| ID | Scenario | Expected Deterministic Outcome |
|---|---|---|
| **AC-01** | **Native Stage -> Ship Handoff (`Profile B`)**: Stage offers custody of workflow-created, reviewed deliberation/plan/review docs on `chore/stage-233-S`; Ship accepts on matching branch and digests with current-HEAD review `PASS`. | Receipt transitions `offered -> accepted` (`authority_profile: agent_workflow_handoff`, `revision: 1`); `backlogit custody verify` returns `HANDOFF_AGENT_VERIFIED`; GSD gate permits commit/merge without `FOREIGN_FILE_OVERLAP`. |
| **AC-02** | **Verified Operator Pre-Existing Adoption (`Profile A`, `R2`, `RD1`)**: Operator selects 2 of 3 pre-existing dirty files via GSD choice prompt; operator approval channel mints `CustodyApprovalToken` bound to the 2 files' `candidate_set_digest`; 3rd file is unselected. | `backlogit custody accept` verifies signature, role/issuer, scope, and CAS digests, consumes `nonce`, and records `ADOPTED_OPERATOR_VERIFIED` for the 2 files. The 3rd unselected file remains in `ForeignPaths` (untouched and never staged); any agent edit to the 3rd file still halts with `FOREIGN_FILE_OVERLAP`. |
| **AC-03** | **Mixed Staged + Unstaged Edits (`R4`)**: A file has both staged index changes and further unstaged worktree edits (`index_sha256 != worktree_sha256`). | Rejected by default (`MIXED_INDEX_WORKTREE_DIVERGENCE`) unless the operator explicitly reconciles/stages or issues a `Profile A` token binding `staged_diverges_accepted` and `contributor: mixed_reconciled` with exact dual hashes. |
| **AC-04** | **Post-Approval Content Drift / Stale Digest (`R5`, `R7`)**: File `X` is accepted at `worktree_sha256: H1`, then modified 1 byte to `H2` before commit. | `backlogit custody verify` returns `STALE_DIGEST` for `X`; GSD topology/pre-commit gate fails closed; any prior plan review covering `X` is marked stale until re-reviewed and re-accepted at `revision + 1`. |
| **AC-05** | **Wrong Branch / Default-Branch Rejection (`R3`, `R7`)**: Attempt to offer/accept mutable custody on `main`, or verify a `chore/stage-233-S` receipt while checked out on `feat/other`. | Offering/accepting on `main` fails closed (`ErrCustodyDefaultBranchForbidden`); cross-branch verification fails closed (`WRONG_BRANCH`). |
| **AC-06** | **Agent Self-Confirmation & Forged Token / Replay Rejection (`R7`, `RD1`)**: (a) Agent calls MCP `backlogit_custody_accept` for `preexisting_adoption` with `confirm: true` + fabricated `decision_ref` (no `CustodyApprovalToken`); (b) token has wrong issuer/role, wrong `candidate_set_digest`, wrong `workflow_id`/`shipment_id`/`scope_branch`, expired `not_after_utc`, or replayed `nonce`; (c) GSD gate encounters a confirmation-only receipt on a `ForeignPaths` file; (d) host lacks operator approval channel. | (a) Rejected fail-closed (`ErrAuthorizationRequired`); (b) rejected fail-closed (`ErrAuthorizationRejected` / `ErrApprovalWrongShipment` / `ErrApprovalWrongRequestIdentity` / `ErrApprovalReplay`) with zero state mutation; (c) GSD gate rejects as `UNVERIFIED_OPERATOR_PROVENANCE` (`FOREIGN_FILE_OVERLAP`); (d) halts with `APPROVAL_CHANNEL_UNAVAILABLE` (never downgrades to confirmation-only). |
| **AC-07** | **Authority Non-Expansion (`R7`)**: Caller presents a valid `accepted` custody receipt for `233-S` docs while hold `6B23AD1F` is active or `harness-surface` is missing. | Custody check passes for file cleanliness only; hold gate (`6B23AD1F`), resolver gate (`P-004`), and merge approval gate still halt independently. |
| **AC-08** | **Read-Only Preview / Advisory Gate Zero-Write (`R6`)**: Run `backlogit custody preview --dry-run` and `gsd gate pipeline-topology --phase pre_claim` (without `--capture-baseline`). | Zero files created or modified in `.backlogit/` or `.gsd/worktree-baseline.json`; verified via pre/post directory walk and SHA-256 snapshot. |
| **AC-09** | **Concurrent Writers & Crash Recovery (`R5`)**: Two processes race `custody accept` on `233-S`, or process crashes between writing `.backlogit/queue/233-S.md` and appending `logs/233-S.jsonl`. | Lock order `global -> A -> C -> B` serializes contenders (`ErrTaskBusy` / `ErrGateInProgress`); interrupted journal in `.backlogit/ops/` rolls back or rolls forward cleanly on next `recoverPendingShipmentOperations` barrier; `ErrWriteIndeterminate` never rolls back an applied write. |
| **AC-10** | **CLI / MCP Parity, Canonical-Direct Execution (Without SQLite `B3`), & Offline Bounded Output (`R8`, `RD4`)**: Run `backlogit custody` CLI and `backlogit_custody_*` MCP with SQLite projection absent/disabled. | Identical canonical JSON schemas and error codes operating directly on Markdown + JSONL; zero network calls; all responses bounded (`< 10 KiB`). |
| **AC-11** | **Rename, Case-Only Windows Rename, & Rename+Edit Custody (`R4`, `RD2`)**: (a) Exact rename `old_path -> path`; (b) rename + content edit (`worktree_sha256 != old_base_blob_sha256`); (c) Windows case-only rename (`Track-A.md -> track-a.md`); (d) partial selection of only `old_path` or only `path`. | (a, b) Accepted when both `old_base_blob_sha256` and new `index_sha256`/`worktree_sha256`/`patch_digest_sha256` match; (c) on-disk directory entry case verified and both `old_path` and `path` reconciled in `ClassifyWorktreeDeltas` without phantom `ForeignPaths` overlap; (d) rejected fail-closed with `ErrPartialRenameAdoptionRejected`. |

---

## 9. Risks, Caveats, Compatibility, and Rollback

1. **Backward Compatibility**: `custom_fields.workflow_custody` and `workflow_id` are optional on `shipment` artifacts. Existing workspaces and shipments without custody records behave identically to `v1.12.0`.
2. **No Automated Wholesale Adoption**: Migration never auto-adopts existing dirty files. Every pre-existing dirty path requires an explicit operator choice-assisted adoption receipt backed by a `Profile A` (`operator_verified`) approval proof.
3. **Rollback**: Because custody state lives in `.backlogit/queue/<shipment_id>.md` + `.backlogit/logs/<shipment_id>.jsonl` and the `custody_receipts` SQLite table (`U5` / `210.005-T`) is a disposable derived projection, rolling back the feature in `backlogit` only requires reverting the `211-S` merge commit (`git revert -m 1 <merge_sha>`) and running `backlogit sync`.

---

## Constitution Check

* **I. Safety-First Go**: `pass` — All production code is written in Go 1.24.0, uses zero `unsafe` packages, wraps all errors with `fmt.Errorf("context: %w", err)`, defines sentinel errors for known failures, and passes `go vet ./...`, `golangci-lint run`, and `gofmt -l .`.
* **II. Test-First Development (NON-NEGOTIABLE)**: `pass` — Every unit (`U1` through `U7`) starts with a colocated `*_test.go` RED harness written by `harness-architect` with **zero production files touched in the RED phase**, verified to compile cleanly (`go test -timeout=30m -run=^$ -count=1 ./...`) and observed failing before `build-feature` writes any production code.
* **III. Workspace Isolation and Security Boundaries**: `pass` — All candidate paths and `.backlogit/ops/` journal files resolve strictly within `ws.RootPath` via `filepath.Abs` + `filepath.EvalSymlinks` + component-by-component `os.Lstat` and `IsSymlinkOrReparsePoint` checks; no secrets or private keys are ever stored in `.backlogit/`.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE)**: `pass` — All CLI and MCP operations and tests operate strictly inside the target workspace directory (`ws.RootPath` / `t.TempDir()`).
* **V. Structured Observability**: `pass` — All custody mutations emit structured `slog` records and append correlated events to `.backlogit/logs/<shipment_id>.jsonl`.
* **VI. Single Responsibility**: `pass` — Zero new third-party dependencies added; reuses standard library `crypto/ed25519`, `crypto/sha256`, and existing `internal/canonical`, `internal/core`, and `internal/db` packages.
* **VII. Destructive Command Approval (NON-NEGOTIABLE)**: `pass` — `Profile A` (`operator_verified`) enforces cryptographic `CustodyApprovalToken` verification via `TrustAnchors` and `NonceLedger`, never allowing agents to self-confirm operator adoption.
* **VIII. Explicit Safety Modes for Elevated Risk**: `pass` — Plan hardening (`P-006`) and `strict-safety` `ProposedAction` / `ActionRisk` classifications are explicitly recorded below.
* **IX. Git-Friendly Persistence**: `pass` — Source of truth is human-readable Markdown frontmatter (`.backlogit/queue/<shipment_id>.md`) and append-only JSONL (`.backlogit/logs/<shipment_id>.jsonl`), with SQLite (`custody_receipts`) strictly a disposable cache rebuilt on `backlogit sync`.
* **X. Agent Context Efficiency**: `pass` — All CLI (`--json` / `--jsonrpc`) and MCP (`backlogit_custody_*`) responses are bounded (`< 10 KiB`), initialize slices as `[]` (never `null`), and support SQL querying via `custody_receipts`.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)**: `pass` — Shipment `211-S` will merge via a merge commit (`--merge`), never squash or rebase.
* `Constitution Check: pass`

---

## Plan Hardening Signals

* **Public API, schema, or contract change**: `present` — Adds `shipment.workflow_id` to `defaultHeaderDef()`, adds `custody_receipts` SQLite projection table, adds `backlogit custody` CLI subcommands and `backlogit_custody_*` MCP tools, and updates `.autoharness/backlog-registry.yaml`.
* **Security, auth, permission, or compliance-sensitive behavior**: `present` — Enforces `Profile A` (`operator_verified` `CustodyApprovalToken` via `TrustAnchors` + `NonceLedger`), `Profile B` (`agent_workflow_handoff`), Lstat symlink/reparse containment, and ungoverned `workflow_custody` mutation refusal.
* **Migration, backfill, destructive data/config action, or irreversible step**: `present` — Extends `upgradeLegacyGeneratedHeaderDef` bitmask for `shipment.workflow_id` and `EnsureSchema` / `Rehydrate` for `custody_receipts`.
* **External integration, operator checkpoint, or external dependency**: `present` — Defines the `workflow-custody/v1` contract consumed by GSD's worktree cleanliness gate (`1D48B514`).
* **High runtime, rollout, or rollback risk**: `present` — Participates in `shipment-lifecycle-global -> A -> C -> B` advisory lock ordering and `.backlogit/ops/` crash recovery journals.
* `Requires plan hardening: yes`

---

## Plan Hardening

### System-Wide Invariants

* **I1 (Canonical Lock Ordering & Ungoverned Entry-Point Refusal)**: `OfferCustodyHandoff` and `AcceptCustodyHandoff` MUST acquire `lockShipmentLifecycleGlobal(ctx, ws)` (running `recoverPendingShipmentOperations`) -> `lockShipmentMembership(ctx, ws, shipmentID)` (`A`) -> `events.LockItemLogCrossProcess(ctx, ws.LogsPath, shipmentID)` (`C`) -> `lockArtifactMutations(ctx, ws, []string{shipmentID})` (`B`) (`C` before `B`, matching `shipment_reconcile_transaction.go:577-592`), carrying the held-lock context (`withHeldItemLogLock` and `withHeldArtifactLocks`) through `persistArtifact` and event append / nonce consumption without re-locking inside the held span. Generic mutation entrypoints (`UpdateArtifactWithGate`, `MoveInQueue`, `writeArtifactFileGoverned` in `internal/core/artifacts.go`) MUST refuse any modification to `custom_fields["workflow_custody"]` unless the context carries the internal governed custody envelope marker.
* **I2 (Authenticate-Before-Filter & Non-Negotiable `Profile A` Token Floor — `RD1`, `OD-2`)**: `VerifyCustodyApprovalToken` MUST verify the Ed25519/HMAC cryptographic signature against the resolved operator `TrustAnchor`, algorithm, `authorized_role == "operator"`, validity window (`not_before_utc <= now <= not_after_utc`, `not_after_utc - not_before_utc <= 15m`), and domain-separated `CustodyRequestIdentityDigest` (`backlogit/workflow-custody/request-identity/v1`) **before** consulting dry-run, idempotency, or replay filters. A bare `--confirm: true` or `decision_ref` is NEVER accepted for `Profile A`; missing token or trust anchor fails closed with `ErrAuthorizationRequired` / `ErrApprovalChannelUnavailable`.
* **I3 (Handle-Pinned / Component-Walk Path Containment & Atomic Rename Pairing — `RD2`)**: `ValidateCustodyEntryPath` MUST normalize `ws.RootPath` with `filepath.Abs` and `filepath.EvalSymlinks` (supporting relative `--cwd .`), reject `..`, empty, backslash, drive-letter, UNC, and `.git/**` paths, walk every existing component with `os.Lstat` + `IsSymlinkOrReparsePoint`, and read worktree file bytes exclusively through a handle-pinned, no-follow open (`O_NOFOLLOW` / Windows `FILE_FLAG_OPEN_REPARSE_POINT`) that verifies the opened handle's final path and file identity against the `Lstat` pre-check (tested against post-validation symlink/junction swaps). `change_kind: "renamed"` entries MUST bind both `old_path` and `path` (plus `old_base_blob_sha256`); partial rename selection fails closed with `ErrPartialRenameAdoptionRejected`, and Windows `case_only_rename` (`strings.EqualFold(old_path, path) && old_path != path`) verifies on-disk directory entry casing via `os.ReadDir`.
* **I4 (Zero-Write `--dry-run` Preview & Canonical-Direct Read/Write Independence — `R6`, `RD4`)**: `PreviewCustodyCandidates`, `VerifyCustodyState`, and `--dry-run` invocations of `OfferCustodyHandoff` / `AcceptCustodyHandoff` MUST open `NewReadOnlyWorkspace`, verify `ensureNoPendingShipmentRecoveryReadOnly(ws)`, never consume a nonce in `NonceConsumer`/`NonceLedger`, never create `.backlogit/ops/` journals or lock sidecars, and operate directly on `.backlogit/queue/<shipment_id>.md` and `.backlogit/logs/<shipment_id>.jsonl` even when `.backlogit/backlogit.db` is absent. Live `OfferCustodyHandoff` and `AcceptCustodyHandoff` in MCP (`custodyWorkspace`) and CLI (`custody.go`) MUST also succeed via a DB-optional writable canonical workspace (`ws.DB == nil` with `HeaderDef` loaded and `.backlogit/ops/` recovery run under `lockShipmentLifecycleGlobal`) when the disposable SQLite cache is absent or unusable.
* **I5 (Two-Class Durability Contract — `ErrWriteNotApplied` vs `ErrWriteIndeterminate`)**: Any failure before the `.backlogit/ops/` journal commit point rolls back preimages and returns an error wrapping `blerrors.ErrWriteNotApplied` (`retryable: true`). Any failure at or after the commit point (such as post-commit journal update or parent-dir fsync) MUST NOT roll back applied mutations and MUST surface `blerrors.ErrWriteIndeterminate` (`retryable: false`) with an accurate `MutationEnvelope`.
* **I6 (Git Inspection Hardening & Unicode SimpleFold JSON/YAML Decoding)**: All git invocations in `PreviewCustodyCandidates` and `VerifyCustodyState` MUST pass `--no-replace-objects` and check `ctx.Err()` before classifying subprocess exit codes on Windows. All JSON and YAML custody payloads MUST reject duplicate keys (using `unicode.SimpleFold` orbit minimum for JSON and recursive `yaml.Node` mapping key checks + `KnownFields(true)` for YAML).
* **I7 (1:1 Registry Parity, Non-Nil Slices, & `-timeout=30m` Race Gate)**: All slice fields on custody result structs MUST be initialized to non-nil empty slices (`[]`) so JSON serialization never emits `null`. All 4 custody operations MUST be registered in `.autoharness/backlog-registry.yaml` and pass `TestRegistryParity_*`. Every `go test` invocation MUST include `-timeout=30m`.

### Learnings Consulted

* `docs/compound/2026-10-19-lock-held-side-effect-ordering.md`
* `docs/compound/concurrency-issues/2026-08-09-stable-lock-keys-and-heartbeat-refresh-for-unbounded-holds.md`
* `docs/compound/2026-08-18-shipment-shipped-prevention-envelope.md`
* `docs/compound/security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md`
* `docs/compound/security-issues/2026-08-09-authenticate-before-filter-security-check-ordering.md`
* `docs/compound/2026-10-17-evidence-validation-review-fix-classes.md`
* `docs/compound/2026-10-17-policy-engine-review-fix-classes.md`
* `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`
* `docs/compound/2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`
* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
* `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`
* `docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md`
* `docs/compound/2026-06-26-docline-frontmatter-contract.md`

### Strict-Safety Action Classifications (`strict-safety`)

| Unit | ProposedAction | ActionRisk | ActionResult | Safeguard & Rollback |
|---|---|---|---|---|
| `U1` (`210.001-T`) | Promote `shipment.workflow_id` in `internal/config/defaults.go` and `internal/config/headerdef.go`, gated by `TestU1_ShipmentWorkflowIDHeaderDefAndLegacyUpgrade` in `internal/core/artifacts_headerdef_test.go` | `moderate` (`approval_required: false`) | `planned` | Bitmask upgrade preserves operator-customized `header-def.yaml`; reversible by reverting commit |
| `U2` (`210.002-T`) | Declare `workflow-custody/v1` types, event constants, sentinel errors, and `ErrNotImplemented` stubs in `internal/core/custody_types.go` and `internal/core/custody_exec.go`, gated by AST test in `internal/core/custody_shape_test.go` | `low` (`approval_required: false`) | `planned` | Declarations and `ErrNotImplemented` stubs only; zero behavioral change |
| `U3` (`210.003-T`) | Implement `ValidateCustodyEnvelope`, `ValidateCustodyEntryPath`, `CustodyRequestIdentityDigest`, `MintCustodyApprovalToken`, and `VerifyCustodyApprovalToken` in `internal/core/custody_validate.go` | `high` (`approval_required: false`) | `planned` | Pure validation/crypto verification functions tested in `t.TempDir()` with symlink/junction and forged-token negative tests |
| `U4` (`210.004-T`) | Implement `PreviewCustodyCandidates`, `OfferCustodyHandoff`, `AcceptCustodyHandoff`, `VerifyCustodyState`, and `.backlogit/ops/` journal recovery in `internal/core/custody_exec.go`, `internal/core/shipment_ops.go`, `internal/core/shipment_recovery.go`, and `internal/core/artifacts.go` | `high` (`approval_required: false`) | `planned` | Serialized under `global -> A -> C -> B` locks with `shipment-operation/v1` preimage rollback and `NonceLedger` replay prevention |
| `U5` (`210.005-T`) | Add derived `custody_receipts` table in `internal/db/schema.go` and rebuild/live-upsert projection in `internal/db/rehydration.go`, `internal/db/queries.go`, and `internal/db/upsert_tx.go` | `moderate` (`approval_required: false`) | `planned` | Disposable SQLite read-model rebuilt on `backlogit sync` and updated in-transaction on `UpsertItem`; zero impact on canonical `.md`/`.jsonl` files |
| `U6` (`210.006-T`) | Register and implement `backlogit_custody_*` MCP tools in `internal/mcp/custody_handlers.go` and `internal/mcp/tools.go` with interim `mcp_only: true` registry rows in `.autoharness/backlog-registry.yaml` | `moderate` (`approval_required: false`) | `planned` | Read-only tools use `NewReadOnlyWorkspace`; `Profile A` rejects bare `confirm: true` |
| `U7` (`210.007-T`) | Wire `backlogit custody` CLI in `internal/cli/custody.go` and `internal/cli/root.go`, promote `.autoharness/backlog-registry.yaml` rows to `cli_command`, and regenerate `docs/cli-reference/` | `moderate` (`approval_required: false`) | `planned` | Verified by `TestRegistryParity_*` and `backlogit docs lint --path docs/cli-reference` |

### Review-Gate Capability Risks

* `plan-review` MUST probe subagent dispatch capability, emit literal `dispatch_mode: multi-agent-dispatch` (or `single-agent-declared-degradation` if any subagent fails to return), and record literal `decision: PASS` (or `ADVISORY` with `operator_authorization: approved`) before `harvest` is invoked.

---

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * Config & Core (`internal/config`, `internal/core`): `shipment.workflow_id` header-def field; `WorkflowCustodyEnvelope`, `CustodyReceipt`, `CustodyEntry`, `CustodyApprovalProof`, `PreviewCustodyCandidates`, `OfferCustodyHandoff`, `AcceptCustodyHandoff`, `VerifyCustodyState`, `MintCustodyApprovalToken`, `VerifyCustodyApprovalToken`.
  * SQLite Index (`internal/db`): `custody_receipts` derived projection table and `Rehydrate` rebuild.
  * CLI & MCP (`internal/cli`, `internal/mcp`, `.autoharness/backlog-registry.yaml`): `backlogit custody preview|offer|accept|verify` and `backlogit_custody_preview|offer|accept|verify`.
* **Runtime verification expectations**:
  * `go test -timeout=30m -count=1 -race ./internal/config/... ./internal/core/... ./internal/db/... ./internal/mcp/... ./internal/cli/...` passes with 0 failures.
  * `go vet ./...`, `golangci-lint run`, and `gofmt -l .` report 0 issues.
  * `go run ./cmd/gen-docs docs/cli-reference` produces zero `git diff` on `docs/cli-reference/`, and `go run ./cmd/backlogit docs lint` reports 0 violations.
* **Operational closure expectations**:
  * Ship produces `docs/closure/211-S-210-F-post-merge-closure.md` and runs post-merge `compact-context` (`P-020`).
  * **Monitoring & validation window**: Verify zero `Doctor` findings (`backlogit doctor`) and zero `TestRegistryParity_*` drift on merge to `main`.
  * **Rollback trigger and procedure**: If any custody operation or SQLite rehydration regresses on `main`, revert the `211-S` merge commit via `git revert -m 1 <merge_sha>` and run `backlogit sync`.

---

## 10. Review Status & Rubber-Duck Remediation Log (`Revision 2` Baseline)

- **Independent Rubber-Duck Review Status**: `rubber_duck_reviewed` — Remediated all 4 findings (`RD1`–`RD4`) from the independent rubber-duck review of `revision: 1`, and verified in scoped recheck (`2026-11-02T01:17:07Z`).
- **Scoped Recheck Result (`2026-11-02T01:17:07Z`, Reviewer `4f911545`)**:
  - **Verdict**: `READY FOR HANDOFF` as a report-only draft proposal awaiting `backlogit` owner acceptance (distinct from formal Stage harvest or release admission).
  - **Finding Verification**: `RD1` (P1) resolved at the design level (`operator_verified` proof floor, no confirmation-only fallback, explicit key-separation caveat); `RD2` (P2) rename/case-only rename semantics resolved; `RD3` (P2) unsupported percentage metric removed; `RD4` (P3) optional SQLite projection decoupled from canonical-direct MVP (`B1 -> B2 -> B4`). Zero blocking residuals found.
  - **Mandatory Bootstrap Prerequisite (`NOT IMPLEMENTED`)**: The concrete operator approval-channel issuer/adapter and key-isolation mechanism (`OD-2`) remains **NOT IMPLEMENTED** and is a mandatory owner/GSD bootstrap prerequisite (must be scheduled as the first dependent GSD task; callers and gates must never assume `Profile A` is available until implemented and verified, failing closed with `APPROVAL_CHANNEL_UNAVAILABLE`).

| Finding ID | Severity | Summary of Reviewer Finding | Disposition & Exact Section Remediated in `Revision 2` |
|---|---|---|---|
| **RD1** | **P1** | Confirmation-only `accept` (`--confirm: true` + `decision_ref`, and `OD-2` default `AuthEnforcementConfirmation`) cannot verify operator origin because an agent can self-author both. Must require content/scope-bound evidence from an operator approval/choice channel not callable as operator by agent tools, distinguish operator adoption (`Profile A`) from permitted agent-to-agent handoffs (`Profile B`), fail closed (`APPROVAL_CHANNEL_UNAVAILABLE`) without downgrade, and state key-separation trust assumptions and concrete negative/positive tests. | **Remediated (`Sections 5.1, 5.2 #4–#5, 5.3, 6, 7 #2–#3, 8 AC-01/AC-02/AC-06, 9.3 OD-2`)**: Removed confirmation-only as a valid operator custody path. Defined **Profile A (`operator_verified`)** requiring a signed, single-use `CustodyApprovalToken` (`approval_proof`) bound to `(issuer/key_id, principal, authorized_role: operator, workflow_id, shipment_id, scope_branch, base_commit_sha, candidate_set_digest, allowed_action, target_revision, not_before_utc, not_after_utc, nonce)` verified via `TrustAnchors` + `NonceLedger` under locks `global -> A -> C -> B`, and **Profile B (`agent_workflow_handoff`)** for routine agent-to-agent handoffs of workflow-created, in-budget, gate-verified files. Added fail-closed `APPROVAL_CHANNEL_UNAVAILABLE` and `UNVERIFIED_OPERATOR_PROVENANCE` rules, explicit key-separation assumptions, and concrete tests in `AC-01`, `AC-02`, and `AC-06`. |
| **RD2** | **P2** | `change_kind` (`added \| modified \| deleted \| staged_and_unstaged`) lacked rename semantics, risking split or lost custody across renames, Windows case-only renames, or rename+edit. | **Remediated (`Sections 5.1, 5.2 #2–#3, 6 Unit B1, 7 #2, 8 AC-11`)**: Added explicit `change_kind: "renamed"` (with required `old_path`, `path`, `old_base_blob_sha256`, `case_only_rename`, and destination `index_sha256`/`worktree_sha256`/`patch_digest_sha256`, or linked `rename_group_id` pairs) requiring atomic adoption/reconciliation of both paths, rejecting heuristic Git similarity as authority, handling Windows case-only renames via directory-entry/handle verification, and failing closed on partial renames (`ErrPartialRenameAdoptionRejected`) or unsupported status codes (`ErrUnsupportedCustodyChangeKind`). Added `AC-11`. |
| **RD3** | **P2** | Remove unsupported percentage substrate reuse metric; use qualitative, evidence-backed reuse statements only. | **Remediated (`Section 1.1`)**: Removed all percentage estimates and replaced with qualitative, source-cited descriptions of the reused `backlogit` subsystems. |
| **RD4** | **P3** | SQLite projection (`Unit B3`) is disposable yet was listed as a mandatory prerequisite for `Unit B4` (CLI/MCP). Make canonical receipt store + CLI/MCP work without SQLite projection and adjust the milestone DAG. | **Remediated (`Sections 4.1 #1, 5.3, 6 Milestone Table, 7, 8 AC-10, 9.2, 9.3 OD-3`)**: Decoupled `Unit B4` from `Unit B3` so `B4` depends only on `Unit B2` (`B1 -> B2 -> B4`), operating directly on canonical `.backlogit/queue/<shipment_id>.md`, `.backlogit/logs/<shipment_id>.jsonl`, and `.backlogit/ops/` via `readShipmentFromDiskReadOnly` / `NewReadOnlyWorkspace`. Marked `Unit B3` as an optional derived SQLite projection for `backlogit query` reporting. |

---

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: PASS
* attempt: 1
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer, Security Lens Reviewer
* plan_hardening: `Requires plan hardening: yes` satisfied by `## Plan Hardening` (System-Wide Invariants `I1`–`I7`, 13 institutional learnings consulted, and `strict-safety` `ProposedAction` / `ActionRisk` / `ActionResult` classifications across `U1`–`U7`).
* constitution_check: `Constitution Check: pass` verified across all Principles I–XI and Task Granularity rules.
* findings:
  * P0: 0
  * P1: 0
  * P2: 0
  * P3: 0
* gate_rationale:
  * **Constitution Reviewer (`PASS`, 0 findings)**: Confirmed Go 1.24.0 safety, P-002.1 cycle-31 RED-phase zero-production-files invariant across `U1`–`U7`, workspace path containment (`filepath.Abs` + `filepath.EvalSymlinks` + `os.Lstat` + `IsSymlinkOrReparsePoint`), `Profile A` `CustodyApprovalToken` enforcement, CQRS Markdown/JSONL canonical authority, and 2-hour rule compliance (`<= 3` primary files, `<= 5` functions, `<= 4` test scenarios per unit).
  * **Scope Boundary Auditor (`PASS`, 0 findings)**: Confirmed 1:1 alignment with deliberation `081-DL` (`OD-1`, `OD-2`, `OD-3`, `R1`–`R8`, `RD1`–`RD4`), explicit exclusion of external GSD repository tasks (`Section 7`) from `backlogit` units `210.001-T`–`210.007-T`, and single-domain width isolation.
  * **Architecture Strategist (`PASS`, 0 findings)**: Confirmed clean package boundaries (`internal/config` -> `internal/core` -> `{internal/db, internal/mcp}` -> `internal/cli`), canonical-direct MCP independence from SQLite (`U6` independent of `U5` per `RD4`), and canonical lock ordering (`shipment-lifecycle-global -> A -> C -> B`) with `shipment-operation/v1` journal recovery.
  * **Security Lens Reviewer (`PASS`, 0 findings)**: Confirmed `Profile A` (`operator_verified`) vs `Profile B` (`agent_workflow_handoff`) trust boundaries, authenticate-before-filter ordering (`I2`), `15m` token validity cap, `168h` receipt expiry default, fail-closed `ErrAuthorizationRequired` / `ErrApprovalChannelUnavailable` without confirmation-only downgrade, atomic rename / Windows `case_only_rename` verification (`RD2`), `--no-replace-objects` git inspection (`I6`), and ungoverned `UpdateArtifact` refusal (`I1`).
  * **Go Reviewer (`PASS`, 0 findings)**: Confirmed idiomatic Go type/function signatures, `%w` error wrapping, `ErrWriteNotApplied` vs `ErrWriteIndeterminate` (`MutationEnvelope`) durability classification, `ctx.Err()`-first subprocess handling on Windows, non-nil slice initialization, and `-timeout=30m -race` test commands.
  * **Agent-Native Parity Reviewer (`PASS`, 0 findings)**: Confirmed 1:1 CLI (`backlogit custody preview|offer|accept|verify`) and MCP (`backlogit_custody_preview|offer|accept|verify`) parity, `.autoharness/backlog-registry.yaml` registration (`preview_custody`, `offer_custody`, `accept_custody`, `verify_custody`, `features.workflow_custody: true`), zero-write `--dry-run` on cold/warm workspaces, and bounded `< 10 KiB` JSON responses.

