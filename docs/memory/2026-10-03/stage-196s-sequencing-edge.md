---
schema_version: "1.0"
doc_type: memory
title: Stage 196-S sequencing edge resolved as dag-root
description: The 196-S pre-claim topology gate blocked as UNSEQUENCED_SHIPMENT; a blocks edge onto 195-S failed PREDECESSOR_CLOSURE_INCOMPLETE, so the Orchestrator decided, and Stage executed, a scoped dag-root waiver with a non-gating related_to link to 195-S.
timestamp: "2026-10-04T05:45:00Z"
---

# Stage 196-S sequencing edge resolved as dag-root

## Session status

Complete under P-017 dark mode (scope `[196-S]`) on branch `chore/stage-196-S-sequencing`
(from `main` at `ce95c229`). The Orchestrator made the decision; **Stage executed every
backlog mutation** (`backlogit_remove_dependency`, label update, `backlogit_add_link`,
body edit, `backlogit_sync_index`) under its own role boundary. Dark mode authorized
autonomy within scope; it did not waive role boundaries.

## Timeline

* **Initial block.** The `pre_claim` topology gate exited 1 with `UNSEQUENCED_SHIPMENT`:
  `196-S` declared neither a predecessor nor a root.
* **Blocks-edge attempt.** `backlogit_add_dependency(196-S, 195-S, blocks)` made the gate
  exit 1 with `PREDECESSOR_CLOSURE_INCOMPLETE`: the `195-S` closure
  (`docs/closure/195-S-claim-start-proof-post-merge-closure.md`) is `READY_WITH_CONDITIONS`
  with no machine-readable satisfied `conditions:` block.
* **Commit attribution (F2, resolved by the Orchestrator with git).** `47dfcc93` merges
  PR #327 (`feat/133-shipshipment-cascade-fix`, 2026-08-01, 133-F). Plan line 708 of
  `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md` anchors the
  binary-ancestry closure guard to 133-F on purpose (minimum-version check), so the guard
  is correct; `47dfcc93` is simply not a `195-S` deliverable and orders nothing on `195-S`.
  `195-S` merged as `58f5bdba` (PR #471, 2026-10-02).

## Decision: reasoned exception

* Unlike `182-S`, which had no predecessor, `196-S` follows shipped `195-S`. The
  `dag-root` label waives the predecessor-closure check for `196-S` only.
* Why: the `195-S` conditions include `227A2930` and `B83081F5`, which only `196-S`
  discharges, so a `blocks` edge cannot clear until `196-S` itself ships.
* `195-S` obligations: `227A2930` and `B83081F5` are discharged by `196-S`; `359D8F32`
  remains separately open and is not covered by `196-S`. It is non-gating: P-020 treats
  incomplete compaction as non-blocking `degraded`, so leaving it open is a recorded P-017
  dark-mode Orchestrator decision under the `196-S` scope.
* Mutations: the edge was removed (dependency list empty), label `dag-root` and a
  non-gating `related_to` link `196-S` → `195-S` were added.
* The `196-S` body paragraph and Ship post-merge closure note were edited directly in
  markdown (`backlogit_update_item` only replaces the full description), then
  `backlogit_sync_index` ran; no hook event exists for the body edit.

## Final gate result

The passing run was on the `main` working tree carrying the then-uncommitted edits: exit 0,
`blocked: false`, `shipment_readiness` with `predecessor_source: declared_root` and empty
`predecessor_ids`. The gate passes on clean `main` only after the staging PR merges; the
Orchestrator re-runs it post-merge.

## Next step

The Orchestrator owns the staging PR; Ship may claim `196-S` once it merges and the
post-merge gate re-run passes.
