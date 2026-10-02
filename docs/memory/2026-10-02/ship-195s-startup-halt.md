# Ship session checkpoint — 195-S startup halt

- Date: 2026-10-02
- Scope: shipment `195-S` only
- Outcome: halted before shipment validation, claim, branch creation, or build.

## Halt condition

The required unfiltered `backlogit_list_checkpoints(consumer_id: "ship")`
startup scan returned 57 summaries. One summary is malformed for recovery
purposes: `checkpoint-20260905-031054.json` is missing the required
`resume_hint` field. Its returned context was agent `ship`, session
`ship-137-s-complete`, phase `post-merge-closure-complete`, status `resolved`,
shipment `137-S`, feature `155-F`.

Per the fail-closed checkpoint recovery protocol, this anomaly requires
operator handoff before normal shipment validation. Do not resume, resolve,
prune, or otherwise rewrite this checkpoint as part of this session. Resume
shipment `195-S` only after the anomaly is resolved through the governed
checkpoint procedure and a new Ship session completes the startup scan.

## State and actions

- Initial branch: `main`; initial worktree was clean.
- Backlog registry was present. The read-only shipment availability probe
  returned `195-S` queued with explicit members `195-F` and
  `195.001-T`–`195.007-T`; `backlogit_sync_index` succeeded (`indexed: 1872`).
- No shipment claim, task status change, source edit, feature branch, PR, or
  build was performed.
- The repository's local-only `origin/main` tracking ref was refreshed and
  matched remote `main` at `38aebf61aa6aeca88209ff9d85a8fd889f0b7a83`.
- The bootstrap plan and E3 grant were read from that pinned `main` SHA after
  checking merged-PR provenance for the commits touching both paths. These
  reads do not clear the checkpoint-recovery halt.
- The CLI fallback binary's local metadata was verified:
  `vcs.revision=7c805f9baae7f74edd2b1eede47fcf35fbbc9066`,
  `vcs.modified=false`, SHA-256
  `F188FB4344CFCD701938BA3AE2DB964CECE958C9361F564DF6FFE951315FF46A`,
  with `6d233d21162a072ddbdfecb52ec62a8fb8a63793` an ancestor of the revision.
  No claim-time bootstrap contract check was completed.
- This file is the only repository change made by this session.

## Resume point

After the malformed checkpoint has been handled by an authorized recovery
path, rerun Ship startup recovery from the beginning, then revalidate all
`195-S` dispatch preconditions before any claim or workspace mutation.

## 2026-10-02 resumption attempt

The Orchestrator supplied an authoritative correction: checkpoint
`checkpoint-20260905-031054.json` is valid and conforming, with
`context.resume_hint` present; the unfiltered enumeration had zero active and
zero quarantine-required checkpoints. Do not mutate that resolved checkpoint.
The earlier startup-recovery halt is superseded.

The `ship_pre_branch` pipeline-topology invocation was run once as directed.
It exited 0 with `forced: true`; the underlying readiness token was
`PREDECESSOR_CLOSURE_INCOMPLETE` for shipped predecessor `154-S`, as covered by
the authorized bootstrap grant. The `ship_pre_claim` invocation was not run.

Resumption stopped at the branch-creation cleanliness gate. The current branch
is `main`, and `git status --short` reports this session's untracked memory
file: `docs/memory/2026-10-02/ship-195s-startup-halt.md`. The installed Ship
procedure requires a clean worktree before creating a shipment branch. No
branch, claim, task transition, or build was performed. Preserve this file;
do not bypass the clean-worktree gate or commit it to `main`. A safe,
authorized way to carry the continuity file onto the shipment branch is
required before restarting from the branch gate.

## 2026-10-02 second resumption stop

The Orchestrator confirmed the prior `ship_pre_branch` grant consumption and
directed creating the feature branch while the memory file remains untracked.
On recheck, the worktree is still on `main` at
`38aebf61aa6aeca88209ff9d85a8fd889f0b7a83` (equal to `origin/main`) with that
single untracked path.

The installed P-011 branch-creation gate requires `git status --short` to be
empty before creating a shipment branch and says to halt on any output. The
standing E2 carry-forward instruction permits these continuity files not to
block branch push/publish, but it does not replace the explicit clean-worktree
precondition for branch creation. Therefore, no `git switch -c` was run and no
claim was made. The pre-branch topology label was not repeated; it already
passed and was consumed. `ship_pre_claim` remains unused.

Resume only after the worktree is clean through an authorized path that
preserves this continuity note. Do not commit it on `main`, and do not bypass
P-011.
