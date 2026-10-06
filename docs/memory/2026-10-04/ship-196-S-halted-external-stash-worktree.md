# 196-S halted — external stash file prevents a clean U7 baseline

- **Shipment scope:** `196-S` only
- **Feature:** `196-F`
- **Branch:** `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`
- **HEAD at halt:** `3afdf11f` (`chore(backlog): complete 196-S U4 task`)
- **Halt point:** Before the claim-time gate, pre-work probe, or any mutation for
  `196.007-T` (U7).
- **Halt reason:** `P-002.1_CLEAN_BASELINE_REQUIRED` — an unrelated local change
  in `.backlogit/stash.jsonl` makes the worktree dirty. U7's exemption protocol
  requires a clean worktree before capturing its baseline. Continuing would
  measure a mixed state or would require Ship to alter an out-of-scope stash.

## Preserved external state

`git status --short` showed `.backlogit/stash.jsonl` modified. Its new trailing
entry is `69B0B3F0`, created at `2026-10-04T23:50:40.1430262Z`, titled “Define,
document, and enforce the work-item lifecycle state machine for backlogit
mutation commands.” It is unrelated to `196-S` and was not created by Ship as a
P-021 capture. Ship did not edit, stage, commit, delete, archive, or reclassify
this stash entry and did not modify `.git/info/exclude`.

The U4 completion record is committed separately at `3afdf11f`; the stash
change was explicitly excluded from that commit. The uncommitted stash state
must remain untouched until the operator/Orchestrator determines its safe
ownership and disposition. Do not reset, stash, clean, or selectively rewrite
the file.

## Shipment progress

- `196.001-T` U1 — `done`, red selector `^TestUSR1_` remains open for
  `196.002-T` in wave 2.
- `196.003-T` U3 — `done`, red selector `^TestUSR3_` remains open for
  `196.005-T` in wave 2.
- `196.004-T` U4 — `done`; commits
  `0d094fc755b4e880641482aec2fb0cd12ef830e8` and
  `ebf010d41cff8f0971c90394abd18c11e9f8214e` are associated to the task.
  Current-HEAD report-only review readiness at `ebf010d...` was `READY`, with
  no unresolved findings.
- `196.007-T` U7 — queued; no claim, start record, or implementation attempt
  has occurred in this resumed segment.
- `196.008-T` U8 — queued; its red selector `^TestUSR8_` remains open for
  `196.009-T` in wave 3.

Wave 1 is not converged. No task in waves 2–4 has been admitted. No PR has been
opened. The full wave convergence gates, including `go vet ./...`, remain
pending.

## Resume conditions

Resume only after the operator/Orchestrator safely restores a clean working
tree without Ship editing or discarding the external stash state. Then re-read
the checkpoint and live shipment/task state, repeat the R14 served-root
attestation including `pragma_database_list` before U7's raw item-log read, and
continue U7's exact verification-only contract. Keep the release scope limited
to `196-S`.
