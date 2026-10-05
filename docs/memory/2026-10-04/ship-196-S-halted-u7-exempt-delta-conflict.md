# Ship Halt — 196-S U7 Exemption Delta Conflict

## Scope and state

- Shipment: `196-S` only; dark-mode scope remains unchanged.
- Branch: `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`
- HEAD when halted: `746217be83b7cb15184dbd242f73f6b493b37daa`
- Administrative checkpoint-resolution commit made after halting: `c8412ad1` (`chore(harness): resolve 196-S resume checkpoint`). The U7 test remains staged and was not included.
- Task: `196.007-T` (U7, `verification-only`), still `active`.
- Resume point: obtain a reviewed Stage contract amendment or Orchestrator direction for handling the resolved-checkpoint file in U7's P-002.4 delta range. Do not commit the U7 test or mark U7 done before that gate is resolved.

## Completed resumption checks

- The sole selected checkpoint, `checkpoint-20261004-235817.json`, was restored and its bounded prune/gate completed after engram reported Ready. It was resolved only after U7 was successfully resumed; the resolve operation succeeded at `2026-10-05T00:48:23.760377Z`.
- The plan's closed harness-exempt set includes `196.007-T` as `verification-only`, with no owner.
- U7's exact must-fail-before-deliverable command was run before creating its test file. It exited `1` with `USR7 test file missing`, as required.
- A clean pre-claim exemption baseline was captured as `746217be83b7cb15184dbd242f73f6b493b37daa`.
- R14 was repeated at U7 claim: metadata roots remained the supplied workspace/storage roots, and the standalone `SELECT name,file FROM pragma_database_list;` query returned exactly one `main` row for `.backlogit/backlogit.db`.
- U7's item log was read under a session-scoped P-012 declaration, with path containment, reparse-point checks, corrected `GetFileInformationByHandle` declaration, no-follow open, and opened-path verification. Before append, the latest shipment-claim epoch had zero valid `WORK_STARTED: 196-S` records. `backlogit_append_comment` succeeded; the safe re-read confirmed exactly one valid record.
- The selected prior checkpoint is now resolved. Its tracked working-tree status changed from `active` to `resolved`.
- The resolved checkpoint change was committed separately at `c8412ad1`; the P-002.4 exemption baseline remains the original pre-claim SHA and was not changed.

## Halt: `EXEMPT_DELTA_EXCEEDS_CLASS`

U7's P-002.4 baseline is the required pre-claim SHA above. The required changed-file set was checked against it:

- Working-tree diff: `.backlogit/checkpoints/checkpoint-20261004-235817.json`, `internal/mcp/served_root_self_report_test.go`
- Index diff: `internal/mcp/served_root_self_report_test.go`
- Untracked repository files: none at that check

The checkpoint-resolution file is outside U7's `verification-only` delta surface, which permits only the named new `*_test.go` file. The U7 test is staged but has not been run through its post-work verification command, committed, or associated with task completion. No production Go was changed. P-005 telemetry recorded `P-002.4_EXEMPT_DELTA_EXCEEDS_CLASS`.

This is not safe to work around by changing the captured baseline, omitting the checkpoint path from the required diff, reverting the resolved checkpoint, or treating the checkpoint mutation as U7 test content. The test file and checkpoint status must be preserved pending Stage/Orchestrator resolution. The active task's existing single `WORK_STARTED: 196-S` record must not be appended again.

## Shipment progress

- Done: `196.001-T`, `196.003-T`, `196.004-T`.
- Active/claim-assigned: `196.002-T`, `196.005-T`, `196.006-T`, `196.007-T`, `196.008-T`, `196.009-T`.
- U7 is not individually complete and wave 1 has not converged.
- Open red selectors remain `^TestUSR1_` (green-maker `196.002-T`, wave 2), `^TestUSR3_` (green-maker `196.005-T`, wave 2), and `^TestUSR8_` (green-maker `196.009-T`, wave 3).
- No PR, merge, shipment archive, or closure artifact exists.

## Next safe action

Stage must decide, through a reviewed amendment, how a required resumption-checkpoint resolution that changes a tracked file is separated from the U7 exemption delta measured from its pre-claim baseline. Resume only after that contract decision is explicit. Preserve shipment scope, the staged U7 test, the resolved checkpoint, the single start record, and the recorded baseline.
