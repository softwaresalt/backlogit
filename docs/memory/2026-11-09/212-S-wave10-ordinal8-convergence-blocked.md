---
title: "212-S Wave 10 ordinal 8 convergence blocked"
shipment_id: "212-S"
feature_id: "211-F"
phase: "wave10-unfiltered-convergence-failed"
---

## Outcome

The single authorized post-U7 unfiltered full-suite run was consumed
and failed. Wave 10 Step 4.6 and Step 5 item 1 remain **BLOCKED**.
No retry, repair, local build, branch-wide standard review, adversarial
review, push, PR, merge, closure or successor claim followed this result.
The build and review steps were conditional on full-suite PASS.

All nineteen frozen M19 tasks remain done. Shipment 212-S and feature
211-F remain active. No task was reclaimed, restarted or reimplemented.
The open-red set remains empty; this failure is not tolerated red.

## Recovery and clean execution identity

The operator explicitly selected and confirmed recovery of
`checkpoint-20261109-083105.json`. Full enumeration inspected 171
summaries without validation or quarantine anomalies. Two active Ship
checkpoints were disclosed; only the selected checkpoint was restored
and resolved.

Bounded Engram CLI memory retrieval succeeded but returned unrelated
history. The validated checkpoint and exact pre-suite memory supplied
the restored cursor. No active cursor, gate verdict or historical
checkpoint was pruned. No counter was reset.

Fresh Ship routing validation, MCP served-root attestation, index sync
and lifecycle topology passed. Local visibility was used because the
intercom pack is not installed. Hook event 4648, U7's done transition,
was processed and acknowledged.

The received handoff record is:

```json
{
  "handoff_status": "PASS",
  "shipment_id": "212-S",
  "checkpoint_filename": "checkpoint-20261109-083105.json",
  "served_workspace_root_rel": ".",
  "served_storage_root_rel": ".backlogit",
  "manifest_path_rel": ".backlogit/queue/212-S.md",
  "index_file_rel": ".backlogit/backlogit.db",
  "timestamp_utc": "2026-11-09T08:33:00Z"
}
```

The selected resolved checkpoint was already UTF-8 without BOM and
without CR bytes. The hook queue and memories file also satisfied those
encoding checks and required no change. Only the selected checkpoint
was staged and committed:

```text
ddc57037d321185246652c3f1f93075622bd488f
chore: resolve 212-S pre-convergence checkpoint
```

The working tree was completely empty under `git status --short`
before receipt issuance, before launch and after the failed run.
The exact baseline EOL test passed before launch; the recorder's own
EOL preflight also passed.

Protected checkpoint `checkpoint-20261106-210317.json` remains
untouched and unstaged, independently verified before and after the run:

```text
235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70
```

## Authorization and package census

The Orchestrator reconciled 46 packages as the complete unfiltered
first-party repository census. Historical prose counted those same
46 packages plus four no-test scratch packages. No source package was
filtered or invented.

Under explicit coordinator issuance authority, external receipts were
created in ignored `bin/tmp/`, bound to the exact clean HEAD above:

* Operation: `unfiltered-convergence`
* Scope: `212-S`
* Ordinal: 8
* Prior failed count: 6
* Budget: 2,700 seconds
* Issuer: `coordinator`
* Receipt ID: `212-S-wave10-unfiltered-convergence-ordinal-8`

The token itself is not recorded here or in the checkpoint.
After `FULL_SUITE_BEGIN`, the coordinator marked the external receipts
consumed-running. After completion, they were marked consumed-failed,
with final failed count 7 and the result digest. Their running-state
bytes were retained in ignored backup files. No receipt is reusable.

The driver was launched exactly once under `bin/tmp/212-go-env.ps1`:

```text
python bin/tmp/212-full-suite.py
  --authorization-token-file bin/tmp/212-ordinal8.token
  --authorization-receipt bin/tmp/212-ordinal8.authorization.json
  --counter-receipt bin/tmp/212-ordinal8.counter.json
  --output-dir bin/_rrc7e4b298/wave10-ordinal8
```

Its unfiltered Go command was:

```text
go test -count=1 -p=1 -timeout=30m -json ./...
```

Ship waited on the original shell with 300-second reads.
Ship did not stop or rerun it. The recorder's own deadline handling
terminated its owned job and returned exit 124.

## Persisted evidence

Evidence directory: `bin/_rrc7e4b298/wave10-ordinal8`.
The ignored evidence is local and is not guaranteed by Git.

`result.json` SHA-256:

```text
cc19c50e076ff8d36ca954078e0a695c7d4fd7f27f42961632c3a9efc6d68130
```

The persisted result was independently read and checked:

| Field | Observed value |
|---|---|
| `suite_native_exit` | 124 |
| `evidence_valid` | false |
| `reported_failure_count` | 4 |
| Discovered packages | 46 |
| Completed packages | 8 pass |
| Missing terminal package results | 38 |
| Top-level passes observed | 1,373 |
| Test cycle consumed | true |
| Recorder evidence guard | FAIL |
| Timed out | true |
| Cleanup errors | none reported |

Both `head_before` and `head_after` equal the clean execution HEAD.
Driver bytes were unchanged before and after execution, SHA-256:

```text
9aead967085d5a9f45a2705ba4d2035e15b4cf71e160f8dacd4ecf1b500b957e
```

## Observed failures and timing anomaly

Four failure events represent two subtest failures and their parent
failures in `internal/core`:

* `TestUR1B_UnblockIsTargetAwareAndPreservesResumptionEvidence/active_restores_exact_snapshot`
  failed while replacing a shipment-operation journal: `Access is denied`,
  with durable write outcome indeterminate
* Its parent `TestUR1B_UnblockIsTargetAwareAndPreservesResumptionEvidence`
  also reported FAIL
* `TestP021ClaimSerialization_RepeatedMembershipAndGenericWritersContend/iteration_01`
  failed in a generic writer replacing an artifact: `Access is denied`,
  with durable write not applied
* Its parent
  `TestP021ClaimSerialization_RepeatedMembershipAndGenericWritersContend`
  also reported FAIL

The last unfinished test context was
`TestReconcileResumeAuthorization/live:_fresh_valid_token_matching_the_prepared_approver_appends`.
This is an observed unfinished location, not a proven hang culprit.
The five critical test receipts were not observed; none is claimed PASS.
No panic candidate was reported.

The supervisor recorded:

* Start: `2026-11-09T08:37:44Z`
* Finish: `2026-11-09T15:48:43Z`
* Elapsed: 25,860.603 seconds
* Maximum polling gap: 23,946.461 seconds
* Polling gaps over five seconds: 1
* Idle-sleep hold acquired and cleared on the same thread

The gap's cause is unproven. Do not infer machine sleep, application
deadlock, antivirus interference or a particular filesystem race from
these observations. The recorded elapsed time exceeded the nominal
budget; this run provides neither complete-suite evidence nor clearance.
Raw output retention was bounded to 1 MiB, with 4,259,529 bytes omitted.
The structured result retained all four reported failure events.

## Counters and next step

Ordinal 8 consumed one full-suite allowance and added one failed outcome:
historical failed count 6 becomes cumulative failed count 7.
All other inherited counters and exhausted task budgets remain preserved
in the structured checkpoint; this session attempted zero new tasks.
There is no remaining authorization to rerun convergence.

Existing residuals `0D073C21`, `D2FCA0FB`, native POSIX runtime and
process-group validation, incomplete new-symbol graph coverage and the
previously incomplete adversarial pool remain open. No new readiness
record or false review clearance was produced.

Operator or Orchestrator disposition is required before investigation,
repair or a separately authorized run. Preserve the observed failure
evidence and classify any proposed fix against the authorized contract
before execution. Do not widen scope, reset counters, change tests to
hide failure, infer PR readiness, or claim shipments 213-S through 215-S.
The next build and branch-wide review gates remain pending until valid
unfiltered convergence is established.

The final active Ship recovery pointer is
`checkpoint-20261109-155217.json`. It preserves the original counters,
records this run's delta separately and requires explicit selection and
confirmation before recovery. The protected historical pointer was not
resolved. No review-readiness outcome is claimed for this HEAD.
