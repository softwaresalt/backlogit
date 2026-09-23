---
name: shipment-reconcile
description: "GI/GR reconciliation gate for flat shipment manifests — validates explicit members before and after governed closure while preserving P-007 archive integrity and evidence-backed blocked lifecycle handling."
---

# Shipment Reconcile

Shipment membership is flat and explicit. This skill provides a double-entry
integrity gate for shipment closure and reconciles only the IDs in the
shipment manifest; parentage, descendants, references, and linked deliberations
do not add scope. The shipment control record is the only non-manifest
transactional record included in closure.

Unlisted descendants and linked deliberations remain untouched. They are not
expected to archive, return to backlog, or appear as reconciliation orphans.
An explicitly listed feature remains a normal explicit member and receives the
same governed completion and archive handling as every other listed ID.

## When to Use

* **Ship Step 0.5 intake**: run `mode: pre` with `expected_status: queued` or
  `active`
* **Ship Step 6 closure**: hold the shipment lock across `mode: pre`,
  `mode: safe-close`, and `mode: post`
* **Blocked resume audit**: run `mode: pre` to validate a governed resumable
  pause before Ship considers unblock or normalization
* **Ad-hoc audit**: compare the manifest with current queue/archive state
* **Mixed-role diagnosis**: run `mode: detect-mixed-role` as a read-only scan
  over explicit task members

## Inputs

| Parameter | Required | Values | Notes |
|---|---|---|---|
| `mode` | yes | `pre` \| `post` \| `safe-close` \| `detect-mixed-role` | Selects the reconciliation phase |
| `shipment_id` | yes except all-shipment detection | e.g. `155-S` | Shipment control record to inspect |
| `expected_status` | pre only | `queued` \| `active` \| `done` | Expected status for each explicit member |
| `merge_commit_sha` | safe-close and post | git SHA | Commit metadata recorded by governed closure |

The phase is derived from `mode` and `expected_status`. Pre-mode with
`queued|active` is intake/resume; pre-mode with `done` is pre-close;
safe-close performs the governed flat closure; post-mode verifies archives.

## Output

Write a structured report to
`.backlogit/reconcile/{shipment_id}-{mode}-{timestamp}.md`.

Each explicit manifest member receives exactly one classification:

| Classification | Pre-Mode | Post-Mode |
|---|---|---|
| `matched` | Queue record exists and status equals `expected_status` | Archive record exists with valid provenance |
| `pre-archived` | Queue record is absent and one valid archive record exists | N/A |
| `missing` | No unique queue or archive record exists | Required archive record is absent |
| `status-mismatch` | Queue status differs from `expected_status` | N/A |

Do not classify an unlisted artifact. In particular, an unlisted child,
sibling, ancestor, source stash, or linked deliberation is neither a missing
member nor an orphan.

### Shipment Record Classification

Pre-mode also classifies the shipment control record:

| Classification | Condition |
|---|---|
| `record-consistent` | `queued`, `active`, or terminal state is consistent with the requested phase |
| `record-queued-with-active-work` | Record is `queued` while at least one explicit task member is `active` or `done` |
| `record-blocked-resumable` | Record is `blocked` with a canonical envelope, exact-manifest snapshot, and correlated committed block/normalize evidence |
| `record-blocked-invalid` | Record is `blocked` without complete canonical evidence |

Only explicit task members contribute to aggregate task-status checks. A
feature listed in the manifest remains in reconciliation scope but is excluded
from task aggregation.

### Recommendations

* `PROCEED`: every explicit member is `matched` or `pre-archived` and the
  shipment record is consistent
* `PAUSED — governed unblock required`: intake/resume found
  `record-blocked-resumable`
* `HALT — operator reconcile required`: an explicit member is missing or has a
  status mismatch, or the shipment record is inconsistent/invalid
* `HALT — blocked shipment unexpected for the requested reconciliation phase`:
  pre-close, safe-close, or post-close encountered a valid blocked shipment
* `HALT — shipped-event reconciliation required`: governed ship halted before
  archival because shipped-event durability was indeterminate
* `HALT — restore archives`: P-007 archive integrity failed
* `HALT — non-member mutation detected`: closure changed a queue/archive
  artifact outside the explicit manifest and shipment control record
* `CLOSED`: governed flat closure and postconditions passed

## Behavioral Constraints

* **Flat explicit scope.** Let `M` be the exact de-duplicated manifest item
  list. The only transactional closure set is `M` plus `shipment_id`.
* **No inferred membership.** Never enumerate children, descendants, siblings,
  ancestors, references, source artifacts, or linked deliberations to enlarge
  `M`.
* **Explicit feature handling.** A feature in `M` is completed and archived as
  that one explicit member. Its relationships confer no scope.
* **Non-member stays untouched.** Any queue/archive mutation outside
  `M` plus `shipment_id` is a hard failure. Do not reinterpret, return, orphan, or
  auto-add the affected ID.
* **Governed closure only.** `mode: safe-close` invokes
  `backlogit_ship_shipment`; it does not synthesize per-item transitions or a
  second shipment-close path.
* **Blocked is nonterminal.** An evidence-backed blocked shipment pauses intake
  and cannot enter close/post phases until explicit governed unblock succeeds.
* **No blocked auto-repair.** This skill reports blocked evidence. Ship owns
  lifecycle recovery through declared operations.
* **P-007 remains authoritative.** Preserve the halted shipped-event branch,
  deleted-archive guard, and archive-provenance checks.
* **Single-writer lock.** Ship Step 6 holds the shipment record lock across
  pre, safe-close, and post.
* **Read-only detection.** `detect-mixed-role` never mutates backlog state.

## Required Protocol

### Pre-Mode

1. **Acquire the lock for closure.** Ship Step 6 invokes `file-lock` for
   `.backlogit/queue/{shipment_id}.md`. Intake audits are read-only and do not
   retain a lock.
2. **Load the shipment.** Call `backlogit_get_shipment(shipment_id)`, retain the
   exact ordered manifest list `M`, reject duplicate IDs, and record the
   shipment status.
3. **Inspect only explicit members.** For each ID in `M`, resolve exactly one
   queue/archive record, read `status` and `artifact_type`, and assign the
   per-member classification. Do not scan for related unlisted artifacts.
4. **Classify the shipment record.** Aggregate statuses only from explicit task
   members and apply the Shipment Record Classification table.
5. **Validate the canonical blocked envelope.** A blocked record is resumable
   only when it has a non-empty `blocked_reason`, RFC3339 `blocked_at`, an
   exact-manifest `member_status_snapshot`, canonical optional fields, and
   correlated committed block or normalize lifecycle evidence.
6. **Write the report.** Include `M`, each explicit member classification, the
   shipment-record classification, and the recommendation.
7. **Decide.**
   * Intake/resume plus `record-blocked-resumable` returns `PAUSED` and releases
     any lock; it is not `RECONCILE_FAIL`.
   * Pre-close plus any blocked classification halts and releases the lock.
   * Any explicit-member failure or inconsistent record halts and releases the
     lock.
   * `PROCEED` during Ship Step 6 retains the lock for safe-close/post.

### Safe-Close Mode

1. **Re-read under lock.** Load the shipment and require its manifest to equal
   the pre-mode `M` exactly. Manifest drift halts before mutation.
2. **Require close-ready state.** Reject blocked or other nonterminal shipment
   state and require every non-pre-archived explicit member to be `done`.
3. **Capture the baseline.** Record `git status --short -- .backlogit/queue/
   .backlogit/archive/` and the unique queue/archive location, status,
   `parent_id`, and content identity of every ID in `M` plus `shipment_id`.
   Existing unrelated worktree changes are retained as baseline, not absorbed
   into this transaction.
4. **Invoke governed closure.** Call
   `backlogit_ship_shipment(shipment_id, merge_commit_sha)` (CLI fallback:
   `backlogit shipment ship <shipment_id> --sha <merge_commit_sha>`).
5. **Validate the result envelope.** Require `shipment_status: shipped` and
   `returned_ids: []`. Let `A` be `archived_ids`; require
   `A` to be a subset of `M` plus `shipment_id`, and require every
   member/control record that was not
   already validly archived at baseline to appear in `A`.
6. **Enforce non-member-stays-untouched.** Compare queue/archive changes after
   closure with the baseline. Any newly changed artifact ID outside
   `M` plus `shipment_id` yields `HALT — non-member mutation detected`, a P-005
   event, and no commit. Do not derive an expected non-member set recursively;
   the closed allowed set is sufficient.
7. **Preserve explicit-member parentage.** Re-read every explicit member and
   require `parent_id` to equal its baseline value. Closure must not flatten
   hierarchy metadata.
8. **Apply P-007.** Inspect `.backlogit/archive/` for unexpected tracked
   deletions. If deletion recovery is required, halt with
   `HALT — restore archives` and use the approved P-007 path before commit.
9. **Write the report.** Record `M`, the control record, result envelope,
   allowed/observed changed-ID sets, parentage comparison, P-007 result, and
   `CLOSED` or the precise halt reason.
10. **Continue to post-mode.** Keep the lock held when safe-close returns
    `CLOSED`; release it on every halt.

### Post-Mode

1. **Classify a partial mutation first.** If `backlogit_ship_shipment` returned
   `mutation_partial`, `classification: indeterminate`, and
   `failed_step: shipped-event-append`, emit
   `HALT — shipped-event reconciliation required`. Do not run archive restore;
   the governed operation halted before archival. Direct the operator to
   `backlogit doctor --check-shipped-event-completeness` and P-007.
2. **Reject nonterminal shipment state.** A live blocked, queued, or active
   control record cannot satisfy post-close reconciliation.
3. **Verify the shipment archive.** Require one archived control record with
   `archived_status: shipped` (or a pre-existing normalized legacy `done`
   provenance accepted by the installed lifecycle contract).
4. **Verify explicit member archives.** For each ID in `M`, require one archive
   record with valid provenance. Do not inspect or expect any unlisted artifact
   to archive.
5. **Repeat the non-member change-set check.** The observed queue/archive delta
   from the pre-close baseline must contain no artifact ID outside
   `M` plus `shipment_id`.
6. **Run the P-007 deleted-file guard.** If tracked archive deletions exist,
   emit `HALT — restore archives`; otherwise emit `PROCEED`.
7. **Write the post report and release the lock.** Release failures are warnings
   because stale locks are operator-recoverable.

### Mixed-Role Detection Mode

This mode is operator-invoked and read-only.

1. List shipments with `backlogit_list_shipments`; optionally select one ID.
2. Load each shipment with `backlogit_get_shipment`.
3. Treat a canonical evidence-backed `blocked` record as
   `blocked-resumable`. Treat malformed blocked state as `malformed-legacy`.
4. For a queued candidate, classify only task IDs explicitly present in its
   manifest as `live-queued`, `live-active`, or
   `archived-completed(done)`. Report duplicate, conflicting, missing,
   malformed-provenance, unsupported-status, or torn-partial anomalies for
   those explicit IDs only.
5. Report `REPORTED` when a queued shipment has at least one explicit task in
   `live-active` or `archived-completed(done)`, or when an explicit-member
   anomaly exists. Report `DETECTED` when no signature/anomaly exists. Report
   `DEGRADED` and halt when backlogit is unreachable.
6. Write the diagnostic under
   `.backlogit/reconcile/{shipment_id-or-all}-detect-mixed-role-{timestamp}.md`
   and emit the configured audit/telemetry event. Never mutate a shipment or
   member.

### Blocked Lifecycle Recovery Guidance

When reconciliation reports a valid governed pause, Ship may use only these
registered operations:

* `backlogit_block_shipment` to enter blocked state through the governed
  lifecycle
* `backlogit_unblock_shipment` to resume with explicit `confirm: true`; the
  registry intentionally has no automatic CLI fallback because the CLI uses a
  presence-only `--confirm` flag
* `backlogit_normalize_blocked_shipment` to normalize malformed legacy blocked
  state when the required snapshot evidence exists

The normalizer is intentionally MCP-only. Do not invent a CLI fallback, edit
blocked fields directly, treat dependency satisfaction as implicit unblock, or
let Orchestrator perform Ship's lifecycle mutation.

## Lock Failure Handling

If closure pre-mode cannot acquire the shipment lock:

1. Retry once after 30 seconds.
2. If the retry fails, count a session stall and report
   `Shipment lock conflict on {shipment_id}. Another process holds the lock.`
3. Do not invoke `backlogit_ship_shipment` without the lock.

## Quality Criteria

* Frontmatter parses and the required skill sections remain present
* Intake pre-mode validates only explicit manifest members
* Closure holds one lock across pre, governed flat close, and post
* `mode: safe-close` invokes the governed native transaction exactly once
* The allowed transactional set is exactly the manifest plus shipment control
  record
* Explicit feature members complete and archive without expanding scope
* Unlisted descendants, siblings, ancestors, source artifacts, and linked
  deliberations remain untouched and are never classified as orphans
* Post-mode requires archives only for explicit members and the control record
* Non-member changes halt before commit
* P-007 halted-event, archive-deletion, and provenance checks remain enforced
* Canonical blocked state is resumable and nonterminal
* Blocked normalization is MCP-only and unblock remains explicitly confirmed
* Detection mode remains read-only and manifest-scoped

## Related Artifacts

* `.github/agents/_ship.agent.md` — execution and recovery integration
* `.github/agents/_orchestrator.agent.md` — routes blocked shipments to Ship
* `.github/policies/workflow-policies.md` — P-007 and P-015
* `.autoharness/backlog-registry.yaml` — governed operation mappings
* `internal/core/shipment_lifecycle.go` — flat release/archive implementation

## Model Routing

This skill operates at **Tier 2 (Standard)** — bounded manifest comparison and
lifecycle evidence validation do not require frontier-level reasoning.

Generated by autoharness | Template: shipment-reconcile/SKILL.md.tmpl
