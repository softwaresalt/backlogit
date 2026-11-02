---
chunk_strategy: h1-h2-h3
description: 'Deliberation and owner decision resolution (OD-1, OD-2, OD-3) for the Backlogit Workflow Custody, Pre-Existing Change Adoption, and Stage-to-Ship Handoff native domain extension (blocker 1D48B514).'
doc_type: decision
docline:
  decision_id: '081-DL'
  decision_status: decided
  depth: deep
  linked_artifacts:
    - docs/plans/2026-11-02-backlogit-workflow-custody-handoff-plan.md
    - .backlogit/archive/081-DL.md
  promoted_to: both
  source_stash_id: 6DA6A627
  tags:
    - workflow-custody
    - change-adoption
    - handoff
    - authorization-token
    - cli-mcp-parity
  topic: 'Backlogit Workflow Custody, Pre-Existing Change Adoption, and Stage-to-Ship Handoff (1D48B514)'
schema_version: "1.0"
source: docs/decisions/2026-11-02-backlogit-workflow-custody-handoff-deliberation.md
title: 'Deliberation — Backlogit Workflow Custody, Pre-Existing Change Adoption, and Stage-to-Ship Handoff (1D48B514)'
---

## Problem Frame

In the two-agent Stage-to-Ship architecture (`1D48B514`), legitimate Stage-authored planning documents (`docs/decisions/*`, `docs/plans/*`, `docs/reviews/*`) and operator-approved pre-existing workspace edits collide with worktree cleanliness gates when:

* worktree baseline capture records only raw file digests (`ForeignPaths`) without workflow identity (`workflow_id`), branch scope (`scope_branch`), producer role (`operator | stage | ship`), or adoption/handoff status
* `backlogit` lacks a native, governed **Workflow Custody & Change-Set Handoff/Adoption** domain contract (`R1`–`R8`) capable of binding `workflow_id`, `scope_branch`, `base_commit_sha`, and per-path/rename/patch SHA-256 digests (`base_blob_sha256`, `index_sha256`, `worktree_sha256`, `patch_digest_sha256`) inside `.backlogit/`
* pre-commit gates face a bootstrap deadlock (`R6`) if adoption receipts must be committed before the adopted files can pass pre-commit inspection

We must decide how `backlogit` persists, validates, authorizes, and projects workflow custody and change-set handoff receipts across CLI and MCP while preserving `backlogit`'s constitutional guarantees (Safety-First Go, Test-First Development, Workspace Isolation, CLI Containment, Destructive Command Approval, Git-Friendly Persistence, and Context Efficiency).

* **Originating Stash Entry**: `6DA6A627` (`priority: high`, `kind: feature`)
* **Target Implementation Plan**: `docs/plans/2026-11-02-backlogit-workflow-custody-handoff-plan.md`

## Research Findings

### Codebase Substrate Inventory

1. **Header Schema & Upgrade Path (`internal/config/defaults.go`, `internal/config/headerdef.go`)**:
   * `defaultHeaderDef()` (`internal/config/defaults.go:142-158`) defines `shipment` with `status`, `branch` (`type: string, optional: true`), and `items`.
   * `upgradeLegacyGeneratedHeaderDef` (`internal/config/headerdef.go:63-97`) uses a bitmask loop over prior generated defaults to transparently add new default fields without clobbering operator-customized `header-def.yaml` files.
2. **Authenticated Operator Approval & Replay Ledger (`internal/core/authorization_token.go`, `internal/core/nonce_ledger.go`, `internal/core/shipment_reconcile_authorization.go`)**:
   * `AuthorizationToken` (`internal/core/authorization_token.go:10-48`), `VerifyAuthorizationToken`, `AuthorizeApproval` (`internal/core/shipment_reconcile_authorization.go:118-210`), `TrustAnchors`, and `NonceLedger.RecordBound` (`internal/core/nonce_ledger.go:75-115`) already implement Ed25519/HMAC signature verification, validity window checks (`not_before`, `not_after`), request-identity digest binding, and atomic nonce consumption (with zero-write dry-run verification).
3. **Lock Hierarchy, Crash Recovery Journals, and Read-Only Workspaces (`internal/core/shipment.go`, `internal/core/shipment_reconcile_transaction.go`, `internal/core/workspace_readonly.go`)**:
   * Governed shipment operations serialize under `shipment-lifecycle-global -> lockShipmentMembership(A) -> events.LockItemLogCrossProcess(C) -> lockArtifactMutations(B)` and persist `shipment-operation/v1` journals in `.backlogit/ops/` recovered by `recoverPendingShipmentOperations`.
   * `NewReadOnlyWorkspace` and `NewReadOnlyIndexWorkspace` guarantee zero disk/SQLite/journal writes during `--dry-run` previews and read-only verifications.
4. **Disposable SQLite Projection (`internal/db/schema.go`, `internal/db/rehydration.go`, `internal/db/gate_evidence.go`)**:
   * `EnsureSchema` (`internal/db/schema.go:258`) creates derived read-model tables idempotently (such as `gate_evidence`), and `Rehydrate` (`internal/db/rehydration.go:281-285`) rebuilds derived projections in-memory from already-collected Markdown artifacts and item logs without a second filesystem walk.

### Institutional Learnings Consulted (`docs/compound/`)

* `docs/compound/2026-10-19-lock-held-side-effect-ordering.md` — Enforce strict lock order `shipment-lifecycle-global -> A -> C -> B`; never invoke secondary locks inside the held span.
* `docs/compound/2026-10-27-pin-the-handle-not-the-path.md` — Resolve workspace paths once, inspect every path component via `Lstat` + `IsSymlinkOrReparsePoint`, and use directory-entry/handle verification for Windows case-only renames.
* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` — Classify pre-commit errors as `ErrWriteNotApplied` (`retryable: true`) and post-commit fsync/projection errors as `ErrWriteIndeterminate` (`retryable: false`, commit-then-surface with `MutationEnvelope`).
* `docs/compound/2026-10-17-evidence-validation-review-fix-classes.md` — Enforce `unicode.SimpleFold` duplicate-key rejection on JSON/YAML decode paths and bind full operation parameters into token request-identity digests.
* `docs/compound/2026-10-17-policy-engine-review-fix-classes.md` — Pass `--no-replace-objects` to git subprocesses and check `ctx.Err()` first on Windows.
* `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` and `docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md` — Register every new CLI command and MCP tool in `.autoharness/backlog-registry.yaml`, initialize slices to `[]` (never `null`), and dispatch parity tests through the authoritative registry.

## Options Evaluated

### Option A: Pure Configuration or Existing API Composition

Attempt to record custody using existing `backlogit update` / `backlogit_update_item` and `backlogit_append_comment`.

* **Pros**: Zero Go code changes.
* **Cons**: Impossible on current codebase — public update surfaces do not expose `custom_fields`, `references`, or `branch`, and `append_comment` writes untyped prose with no digest verification, no CAS, no rename pairing, and no `Profile A` token verification. Violates `R2`, `R4`, `R5`, and `R7`.
* **Effort**: Low (infeasible).

### Option B: Narrow Native Workflow Custody Domain Extension on Shipment Artifacts (Recommended)

Add a dedicated Workflow Custody domain (`internal/core/custody*.go`, `internal/db/custody.go`, `internal/cli/custody.go`, `internal/mcp/custody_handlers.go`) that persists `workflow-custody/v1` envelopes in `.backlogit/queue/<shipment_id>.md` (`custom_fields.workflow_custody` + top-level `workflow_id` and `branch`), appends typed custody events to `.backlogit/logs/<shipment_id>.jsonl`, journals multi-file mutations under `.backlogit/ops/`, and projects a derived `custody_receipts` SQLite table on `backlogit sync`.

* **Pros**: Reuses `backlogit`'s existing hashing, locking, journal, `TrustAnchors`, `NonceLedger`, and read-only workspace primitives; eliminates the pre-commit bootstrap deadlock (`R6`); preserves canonical-direct CLI/MCP operation without SQLite (`RD4`); provides SQL queryability via `Unit B3`.
* **Cons**: Requires 4 bounded implementation units across `internal/config`, `internal/core`, `internal/db`, `internal/cli`, and `internal/mcp`.
* **Effort**: Medium.

### Option C: Overload `ReconstituteShipments` (`210-S`) or `ReconcileShipmentToShipped` (`167-F`)

Extend manifest reconstitution or post-merge archive reconciliation to carry arbitrary working-tree file custody.

* **Pros**: Reuses existing CLI/MCP command namespaces.
* **Cons**: Violates Single Responsibility and domain separation (`R7`). `ReconstituteShipments` redistributes backlog items across shipments (and restricts paths to `.backlogit/`), while `ReconcileShipmentToShipped` repairs archived shipments post-merge.
* **Effort**: Medium-High.

### Option D: Store Custody State Only in External `.gsd/` Files

Keep all custody and adoption receipts in `.gsd/worktree-baseline.json` or `.gsd/workflows/`.

* **Pros**: No `backlogit` changes required.
* **Cons**: Violates `R1` and `R8` by splitting the system of record into a parallel non-Git-friendly database and fails to bind custody to shipment lifecycles.
* **Effort**: Medium.

## Trade-off Comparison

| Criterion | Option A (Config Only) | Option B (Native Custody Extension) | Option C (Overload Reconstitute/Reconcile) | Option D (External `.gsd/` Store) |
|---|---|---|---|---|
| Satisfies R1–R8 & RD1–RD4 | No | Yes | Partial (violates R7) | No (violates R1, R8) |
| Complexity | N/A | Bounded (4 units) | High (couples unrelated state machines) | Medium |
| Risk | High (unsafeprose) | Low-Moderate (reuses proven primitives) | High (regression risk on 167-F / 210-S) | High (split brain) |
| Alignment with Constitution | Fails | Full compliance | Violates Principle VI | Violates Principle IX |

## Decision

We select **Option B: Narrow Native Workflow Custody Domain Extension on Shipment Artifacts**, and explicitly resolve Open Owner Decisions `OD-1`, `OD-2`, and `OD-3` as follows:

1. **Resolution of `OD-1` (Header-Def Fields vs `custom_fields.workflow_custody`)**:
   * Store the full structured `workflow-custody/v1` envelope under `custom_fields.workflow_custody` on `shipment` Markdown artifacts so nested receipts and entries serialize cleanly through `models.Artifact.ToFrontmatterMap()`.
   * Promote `workflow_id` (`type: "string"`, `Optional: true`) alongside `branch` (`type: "string"`, `Optional: true`) as top-level `shipment` fields in `defaultHeaderDef()` (`internal/config/defaults.go:142-158`), and extend `upgradeLegacyGeneratedHeaderDef` (`internal/config/headerdef.go:63-97`) so generated `header-def.yaml` configs automatically gain `shipment.workflow_id` and `items.workflow_id` SQLite column projection without overwriting operator-customized schemas.
2. **Resolution of `OD-2` (Operator Approval Channel & Token Minting for `Profile A`)**:
   * Enforce the non-negotiable `RD1` floor: `Profile A` (`operator_verified`, required for `kind: preexisting_adoption`, any `ForeignPaths` entry, or `contributor: operator | mixed_reconciled`) ALWAYS requires a signed, single-use `CustodyApprovalToken` (`approval_proof`) verified against an operator `TrustAnchor` and `NonceLedger` under locks `shipment-lifecycle-global -> A -> C -> B`, binding `(key_id, alg, principal, authorized_role == "operator", workflow_id, shipment_id, scope_branch, base_commit_sha, candidate_set_digest, allowed_action, target_revision, not_before_utc, not_after_utc, nonce)` under domain separator `backlogit/workflow-custody/request-identity/v1`.
   * Ship the CLI/host-supervisor signer support (`MintCustodyApprovalToken` / `VerifyCustodyApprovalToken` using an operator trust-anchor key isolated outside the agent tool surface). Confirmation-only (`--confirm: true` or `decision_ref`) is NEVER accepted for `Profile A`; when no trust anchor or token is available, fail closed with `APPROVAL_CHANNEL_UNAVAILABLE` / `ErrAuthorizationRequired`.
   * Support `Profile B` (`agent_workflow_handoff`) for routine `stage_to_ship_handoff` and `ship_task_custody` when `producer_role != accepting_role` and all paths are workflow-created after `base_commit_sha` (or already adopted at that exact baseline digest by an earlier valid `Profile A` receipt in the same `workflow_id`).
3. **Resolution of `OD-3` (Default Receipt Expiry Window & Full 4-Unit Scope `B1`–`B4`)**:
   * Default receipt `expires_at_utc` to `168h` (7 days) from `accepted_at_utc` (or until superseded by `custody_revision + 1` or invalidated by content-digest drift), while enforcing a default `15m` max validity window (`not_after_utc - not_before_utc <= 15m`) on single-use `CustodyApprovalToken` claims.
   * Deliver all four units in Shipment `211-S` (`Feature 210-F`): the canonical-direct MVP (`Unit B1 -> Unit B2 -> Unit B4`, which reads and writes `.backlogit/queue/<shipment_id>.md`, `.backlogit/logs/<shipment_id>.jsonl`, and `.backlogit/ops/` without requiring SQLite) plus `Unit B3` (derived SQLite `custody_receipts` table in `EnsureSchema` and `Rehydrate` rebuild on `backlogit sync` + live upsert on custody mutations) so `backlogit query` SQL reporting works out of the box while preserving canonical-direct independence (`RD4`).

## Rejected Alternatives

* **Option A (Pure Config / Comments)**: Rejected because untyped comments cannot enforce cryptographic digest binding, rename pairing, CAS, or `Profile A` operator approval verification.
* **Option C (Overload `ReconstituteShipments` / `ReconcileShipmentToShipped`)**: Rejected to preserve strict domain separation between manifest item redistribution, post-merge archive repair, and pre-commit working-tree file custody.
* **Option D (External `.gsd/` Custody Store)**: Rejected because `backlogit` is the single system of record for shipment and workflow state (`R1`, `R8`).

## Unresolved Questions

None blocking `backlogit` implementation (`210-F` / `211-S`). Downstream GSD gate integration (`ClassifyWorktreeDeltas` overlay and interactive choice prompt) is scoped to the GSD repository once `211-S` ships.

## Risks and Mitigations

* **Lock-Order Inversion Risk**: Mitigated by strictly acquiring `shipment-lifecycle-global -> lockShipmentMembership(A) -> events.LockItemLogCrossProcess(C) -> lockArtifactMutations(B)` and never acquiring `stashMu` or secondary locks inside the held span (`2026-10-19-lock-held-side-effect-ordering.md`).
* **Symlink / Reparse-Point / Case-Only Rename Escape on Windows**: Mitigated by component-by-component `Lstat` + `IsSymlinkOrReparsePoint` validation followed by handle-pinned, no-follow file opens (`O_NOFOLLOW` / Windows `FILE_FLAG_OPEN_REPARSE_POINT` with opened-path/file-ID identity verification against the `Lstat` pre-check before hashing worktree bytes, plus a deterministic post-validation symlink/junction swap regression test in `U3`/`U4`) and directory-entry case verification (`os.ReadDir`) for `case_only_rename` entries (`2026-10-27-pin-the-handle-not-the-path.md`).
* **Post-Commit Durability Indeterminacy**: Mitigated by enforcing `ErrWriteNotApplied` before the journal/rename commit point and commit-then-surface `ErrWriteIndeterminate` with `MutationEnvelope` after the commit point (`2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`).
* **Registry & CLI/MCP Parity Drift**: Mitigated by registering `preview_custody`, `offer_custody`, `accept_custody`, and `verify_custody` in `.autoharness/backlog-registry.yaml` and verifying via `TestRegistryParity_*` (`2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`).
