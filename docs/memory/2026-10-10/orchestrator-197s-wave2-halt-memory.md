---
schema_version: "1.0"
doc_type: memory
title: Orchestrator 197-S dark run halted at wave 2 admission (U15 premise refuted)
description: P-017 dark run for scope [197-S] stopped after wave 1 converged. The U6 benchmark shows no redundant work for U15 to remove, so wave 2 cannot be admitted without an operator ruling on U15 and the manifest.
timestamp: "2026-10-10T05:37:53Z"
---

# Orchestrator 197-S dark run halted at wave 2 admission

## Status: DARK_MODE_HALTED

Scope `[197-S]`. Gate: Ship wave 2 admission (P-002.6). Outcome: no PR, no merge. 197-S is `active` on
`feat/cx-ship-closure-protocol-and-gate-correctness`. Next action: operator ruling (see Decisions needed).

## What was done

- **Staging PR #490** (merge `d9d0237b`, Copilot clean on its second pass, CI green): declared 197-S a `dag-root`.
  The pre-claim topology gate then passed with `predecessor_source: declared_root`.
- **Ship claimed 197-S** and halted at Step 3 with `WAVE_RED_MAPPING_UNRESOLVED`: seven RED tasks lacked
  `red-deliverable-contract` blocks. Stage added the P-002 contracts (three adversarial reviewers: Constitution,
  Scope Boundary, Correctness; all APPROVE_WITH_NOTES). The correctness review caught a real defect: the plan's
  `TestCXS*` selectors do not satisfy the P-002.6 `^TestU<unit>_` anchoring rule. Stage renamed them to
  `TestUCXS1_`..`TestUCXS6_`.
- **Ship ran wave 1** (9 of 19 tasks: 001-008 and 017): seven RED harnesses scaffolded and verified failing for the
  right reason, U6 benchmark and U7 closure registration done, convergence checks passed. Commits are on the
  pushed shipment branch (HEAD `778e8bce`).

## Why it stopped

Wave 2 admission needs a numeric U6-derived target for 197.016-T (U15). Stage read the code to derive one and found
none exists: one `validateMemberGateEvidence` call already reads each member's event log and artifact once, so the
redundant share is 0%. The 93.8% that U6 attributes to log reads and lookups is essential cost. The only duplicate is
the deliberate second gate pass in `ShipShipment` (`shipment_lifecycle.go:607` and `:627`), which U15 may not change
(verdicts, lock order, one-file scope). U6 measured about 0.9-3.4 ms per member, against about 10 s per member
reported in stash `D116AF58`. The real cost sits in `snapshotShipArtifacts` and `attachCommitToItems`, which U6 did
not profile. The plan says a refuted U15 is re-planned through Stage. P-002.6 allows no partial wave, so waves 2-4
(10 tasks) are blocked. The full analysis is in `stage-197s-red-contract-amendment.md`.

## Decisions needed (operator)

Changing membership mid-flight is a Stage manifest decision that earlier runs (196-S) escalated to the operator, so
I did not decide it autonomously.

1. **Descope U15 (recommended).** Stage amends the manifest to drop 197.016-T and removes the 197.018-T -> 197.016-T
   edge (U16b's "same file" ordering is not a real dependency). Capture a follow-up stash for a new profile of the
   real ship path plus the 1 MiB events-reader buffer. The remaining 18 tasks ship R10's "profiled" and "reports
   progress" parts; "fixed" moves to the follow-up. Needs an explicit ruling that M is re-frozen at 18 tasks.
2. Re-plan U15 as a new profiling spike plus a retargeted fix (files change to `shipment_lifecycle.go`; needs plan
   review and likely more cycles).
3. Keep U15 as written and accept no RED harness: not possible under P-002.1 and P-002.2.

After a ruling: Stage amends, then re-invoke Ship (it resumes from repository state at Step 3; checkpoint
`checkpoint-20261010-053048.json` is a pointer only, restore needs your confirmation).

## State and judgment calls to review

- The operator's parked `.autoharness/config.yaml` edit sits in `stash@{0}`; backup in
  `logs/config.yaml.operator-edit-2026-10-09.bak` (sha256 `6D5E5614...661F5`). Restoring it onto the working tree
  blocks Ship's clean-tree gates, so restore it only when no Ship run is in progress.
- Stale checkpoint `checkpoint-20261008-073934.json` (153-S) is still `active`; PR #485/#486 are merged.
  Untouched. Ship's earlier 197-S checkpoint `checkpoint-20261010-024921.json` was left unresolved as well.
- Ship classified a P-002.5 screen hit on `--apply` inside a read-only regex literal (197.008-T probe) as a false
  positive and ran it as a read-only probe.
- Ten members remain claim-assigned while 197-S is active; nothing else was claimed.
- Only PR #490 was opened and merged. No `--admin`.
