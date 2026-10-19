# 196-S resume remains blocked — engram unavailable during checkpoint restore

- **Shipment scope:** `196-S` only
- **Selected checkpoint:** `.backlogit/checkpoints/checkpoint-20261004-235817.json`
- **Checkpoint owner/state:** valid, conforming, `agent: ship`, `status: active`
- **Branch:** `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`
- **HEAD at restore attempt:** `4c3f7e8347ef23c7b8590ab35daefbb14a519555`
- **Halt point:** after operator-confirmed checkpoint selection and served-root
  attestation, but before prune-on-restore, task claim, or U7 execution.

The Orchestrator selected this checkpoint and explicitly confirmed owner-exclusive
resume. Ship enumerated all checkpoint summaries without filters: 110 records,
zero quarantine/validation anomalies, and exactly one active checkpoint, this
Ship-owned `196-S` checkpoint. The checkpoint read was valid and conforming.

Backlog index sync succeeded (`indexed: 1887`). The R14 served-root attestation
also passed: `backlogit_get_metadata_catalog` reported workspace root
`C:\Source\GitHub\backlogit` and storage root
`C:\Source\GitHub\backlogit\.backlogit`; the read-only
`pragma_database_list` query returned exactly one `main` row at
`C:\Source\GitHub\backlogit\.backlogit\backlogit.db`. Git confirmed the main
worktree and a clean tree at `4c3f7e83`. The `196-S` queued manifest was present
only in the queue, and its ID, active status, update timestamp, and ordered item
list matched `backlogit_get_shipment`.

**Resume blocker:** the `agent-engram` MCP daemon failed readiness twice with
`readiness_timeout`. The installed backlogit checkpoint-recovery protocol
requires the owning agent to perform bounded prune-on-restore before resuming
when engram is installed; if engram is unreachable, it explicitly requires
operator handoff, with no prune and no resume. Ship therefore did not resolve or
alter the selected checkpoint, did not claim `196.007-T`, and did not read or
append its item log.

## Resume instruction

Restore engram daemon/workspace reachability, then have the Orchestrator
re-dispatch and confirm this same active checkpoint. Resume the checkpoint
restore/prune gate before execution. Keep the checkpoint unresolved until that
resume gate succeeds. Then repeat the R14 task-claim
`pragma_database_list` attestation before U7's first raw item-log read and
continue `196-S` only. The previous dirty-stash halt is resolved by Orchestrator
commit `4c3f7e83`; stash `69B0B3F0` remains outside this shipment and belongs to
Stage.
