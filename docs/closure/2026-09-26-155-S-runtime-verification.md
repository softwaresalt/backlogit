---
chunk_strategy: h1-h2-h3
description: "Post-merge CLI verification of the governed 155-S shipment close on the merged main binary."
doc_type: closure
docline:
  date: 2026-09-26T20:31:10Z
  tags:
    - runtime-verification
    - post-merge
    - 155-S
    - 174-F
schema_version: "1.0"
source: docs/closure/2026-09-26-155-S-runtime-verification.md
title: "155-S CLI runtime verification"
---

## Verification context

* **Shipment:** `155-S`, feature `174-F`
* **Implementation PR:** #450, merged to `main` at
  `2c8759c3f7583d678ef674b3c6566541b4945375`
* **Surface:** CLI shipment lifecycle, backed by the core transaction and
  archive store
* **Mode:** Manual dogfood with the workspace-local `.\bin\backlogit.exe`
  rebuilt from merged `main`
* **Profile:** `runtime_validation.validator_manifest.surfaces` and
  `surfaces_expected` are empty; `validation_expectations.required` is false.
  The governed CLI close and its post-close evidence provide the relevant
  manual verification for this surface.

## Environment prechecks

* The existing worktree was on `post-merge/155-s-closure`; the merge commit was
  present on synchronized `main`.
* `.\bin\backlogit.exe shipment get 155-S` reported `active` before the retry,
  and `.backlogit\ops` contained no pending shipment-operation journal.
* The pre-close reconciliation for the exact 45-member manifest passed:
  `.backlogit/reconcile/155-S-pre-20260926T194250Z.md`.
* No HTTP service, external deployment, or browser surface is involved.

## Scenario and expected behavior

Run the governed close for `155-S` with the implementation merge SHA. The
expected result is a `shipped` result envelope, no returned members, durable
archive provenance for each explicit manifest member and the shipment record,
and no mutation to unlisted artifacts.

The first invocation was terminated by a five-minute command timeout during
per-member read-only evidence validation. Orchestrator lock and event evidence
showed sequential progress and no artifact writes before termination; the
timeout was classified as too short, not as a hung transaction. The
performance follow-up is captured as stash `D116AF58`.

## Execution and evidence

The second authorized invocation completed within the 45-minute gate/build
timeout class:

```text
.\bin\backlogit.exe shipment ship 155-S --sha 2c8759c3f7583d678ef674b3c6566541b4945375
Started: 2026-09-26T19:57:19.5013856Z
Ended:   2026-09-26T20:31:10.3690700Z
Native exit code: 0
```

Observed result: `shipment_status: shipped`, `returned_ids: []`, and merge SHA
`2c8759c3f7583d678ef674b3c6566541b4945375`. The result listed 42 newly
archived IDs: 40 tasks (`174.039-T`–`174.068-T` and
`174.073-T`–`174.082-T`), feature `174-F`, and shipment `155-S`. Tasks
`174.069-T`–`174.072-T` already had valid formal archive provenance and
remained archived.

The follow-up `shipment get` returned the shipment control record as
`status: archived`, `archived_status: shipped`, with the merge SHA recorded.
The post-close reconciliation verified all 45 manifest members exactly once
under `.backlogit/archive/`, each with valid `archived_from` provenance,
`status: archived`, and `archived_status: done`. All 44 task parent IDs remain
`174-F`; the feature remains the root member. No queue duplicates, missing
members, status mismatches, or returned members were found.

Reconciliation evidence:
`.backlogit/reconcile/155-S-post-20260926T203148Z.md`.
The governed CLI stdout, stderr, exit code, and timing metadata are captured
under `logs/diagnostics/155-s-shipment-ship-attempt2.*`.

Unlisted deliberations `067-DL`–`073-DL` remain queued. No semantic links or
`source_deliberation_id` were present on the explicit manifest, and source
stash `808E4323` remains untouched.

## Verdict

**PASS WITH FOLLOW-UP.** The governed CLI close and post-close invariants
passed. The 33m50s runtime remains an operational performance follow-up under
`D116AF58`; the command completed within the authorized 45-minute budget.
Manual observation expectations and rollback triggers are recorded in the
post-merge closure artifact.
