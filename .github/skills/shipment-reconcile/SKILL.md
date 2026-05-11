---
name: shipment-reconcile
description: "GI/GR reconciliation gate for shipment manifests — verifies every manifest item exists in queue (pre-mode) or archive (post-mode) with the expected status before or after backlogit_ship_shipment runs."
---

# Shipment Reconcile

Provides a double-entry (GI/GR) integrity check for shipment manifests. Run
`mode: pre` before calling `backlogit_ship_shipment` and `mode: post` after
the archive + restore steps complete.

## When to Use

* **Ship Step 6** (mandatory): pre-mode immediately before `backlogit_ship_shipment`;
  post-mode immediately after the `git restore .backlogit/archive/` step.
* **Ship Step 0.5** (sanity check): pre-mode at intake with `expected_status: queued`
  (or `active` if the shipment was already claimed in a prior session)
  to catch Stage-side over-inclusion before any build work begins.
* **Ad-hoc audit**: any time an operator suspects manifest drift.

## Inputs

| Parameter | Required | Values | Notes |
|---|---|---|---|
| `mode` | yes | `pre` \| `post` | Controls which check phase runs |
| `shipment_id` | yes | e.g. `004-S` | The shipment to reconcile |
| `expected_status` | pre-mode only | `queued` \| `active` \| `done` | `queued` for fresh intake; `active` when shipment already claimed in a prior session; `done` for pre-ship check |
| `merge_commit_sha` | post-mode only | git SHA | The merge commit that closed the PR |

## Output

A structured **reconciliation report** stored at
`.backlogit/reconcile/{shipment_id}-{mode}-{timestamp}.md`.

Every item in the manifest is classified as one of:

| Classification | Pre-Mode Meaning | Post-Mode Meaning |
|---|---|---|
| `matched` | Queue file present AND declared status matches `expected_status` | Archive file present for this item |
| `pre-archived` | No queue file found but archive file exists — item already archived before this shipment ran; treated as valid | N/A (all items are expected in archive; use `matched` / `missing`) |
| `missing` | No queue or archive file found for this manifest item | Archive file not found for this manifest item |
| `status-mismatch` | Queue file present but declared status does not match `expected_status` | N/A (post-mode does not check status fields) |
| `orphan` | Queue file declares this `shipment_id` in its frontmatter but is NOT in the manifest | N/A (post-mode does not scan queue files) |

> Classification semantics are mode-dependent. Pre-mode checks the queue
> for status correctness; post-mode checks the archive for file presence only.

The report ends with a `recommendation`:

* `PROCEED` — all items are `matched` or `pre-archived`; no action needed
* `HALT — operator reconcile required` — one or more missing, status-mismatch, or orphan items

## Behavioral Constraints

* **Report-and-halt only.** This skill NEVER modifies the shipment manifest, queue
  files, or archive contents. Operator must manually reconcile via existing
  backlog tools and re-invoke Ship Step 6.
* **No prune mode in v1.** Auto-mutation of the manifest is reserved for a future
  version after upstream backlog tool validation surface is confirmed.
* **Single-writer lock.** When invoked from Ship Step 6, this skill holds the
  `.backlogit/queue/{shipment_id}.md` file lock (via the `file-lock` skill) for
  the duration of pre-mode → post-mode. See lock protocol in the Required Protocol
  section below.
* **Halt on RECONCILE_FAIL.** Do not proceed to `backlogit_ship_shipment` unless
  pre-mode returns `PROCEED`. Surface the report path to the operator.

## Required Protocol

### Pre-Mode

1. **Acquire single-writer lock** (Ship Step 6 invocations only, not intake):
   Invoke the `file-lock` skill to acquire `.backlogit/queue/{shipment_id}.md`.
   If lock acquisition fails, count as a session stall (circuit-breaker protocol)
   and prompt the operator.

2. **Load manifest** via `backlogit_get_shipment(shipment_id)`.
   Extract the `items` list.

3. **Check each manifest item**:
   * Attempt to locate the file at `.backlogit/queue/{id}.*`
   * If found, read its frontmatter and compare `status` to `expected_status`
     — classify as `matched` or `status-mismatch`
   * If NOT found in queue, check `.backlogit/archive/{id}.*`
     — if archive file exists, classify as `pre-archived` (valid; item already shipped)
     — if no file in either location, classify as `missing`

4. **Orphan scan**:
   Scan `.backlogit/queue/` for any files whose YAML frontmatter declares
   `shipment_id: {shipment_id}` but whose ID is NOT present in the manifest `items` list.
   Classify each such file as `orphan`.

5. **Produce report** and store at
   `.backlogit/reconcile/{shipment_id}-{mode}-{timestamp}.md`.

6. **Gate decision**:
   * If all items are `matched` or `pre-archived` and no orphans exist → `recommendation: PROCEED`
   * If any `missing`, `status-mismatch`, or `orphan` items exist →
     `recommendation: HALT — operator reconcile required`
   * On `HALT`: emit the report path, release the lock, and halt with
     `RECONCILE_FAIL`. Do NOT call `backlogit_ship_shipment`.
   * On `PROCEED` from Ship Step 6: retain the lock until post-mode completes.

### Post-Mode

1. **Verify archive presence**:
   List `.backlogit/archive/` and confirm a file exists for the shipment itself
   (`{shipment_id}.*`).

2. **Per-item archive check**:
   For every item in the manifest, verify a corresponding archive file exists.
   If any are absent, flag them in the report.

3. **Deleted-file guard** (known `backlogit_ship_shipment` quirk — see P-007):
   Run `git status -- ".backlogit/archive/"` and inspect for deletions.
   If any archive files are reported as deleted, recommend
   `git restore .backlogit/archive/` before the commit step.

4. **Produce post-mode report** per the same schema.

5. **Gate decision**:
   * If all archive files present and no deletions detected → `recommendation: PROCEED`
   * If missing archive files or unrestored deletions detected →
     `recommendation: HALT — restore archives`
   * On `HALT`: release the lock and report. Ship must restore archives before committing.

6. **Release lock** (acquired in step 1 of pre-mode):
   Invoke `file-lock` release for `.backlogit/queue/{shipment_id}.md`.
   If release fails, log a warning — stale locks are operator-recoverable.

### Lock-Conflict Scenario

If pre-mode cannot acquire the lock because another process holds it:

1. Retry once after 30 seconds.
2. If retry also fails, count as a session stall and prompt the operator:
   `Shipment lock conflict on {shipment_id}. Another process holds the lock.`
3. Do NOT proceed without the lock. Do NOT call `backlogit_ship_shipment`.

## Quality Criteria

* `mode: pre` runs before every `backlogit_ship_shipment` call in Ship Step 6
* `mode: pre` with `expected_status: queued` (or `active` for already-claimed shipments) runs at Ship Step 0.5 intake
* `mode: post` runs after every archive + restore sequence in Ship Step 6
* All five item classifications are represented in the schema
* Lock is acquired before pre-mode and released after post-mode (or on any halt)
* Report-and-halt is the only mutation path; no auto-prune

## Related Artifacts

* `.github/skills/file-lock/SKILL.md` — lock acquisition/release primitives
* `.github/agents/ship.agent.md` — integration points (Step 0.5, Step 6)
* `.github/agents/stage.agent.md` — scope guard (Step 5.5)
* `.github/policies/workflow-policies.md` — P-007 archive integrity policy

## Model Routing

This skill operates at **Tier 2 (Standard)** — file scanning and frontmatter
comparison do not require frontier-level reasoning.

Generated by autoharness | Template: shipment-reconcile/SKILL.md.tmpl
