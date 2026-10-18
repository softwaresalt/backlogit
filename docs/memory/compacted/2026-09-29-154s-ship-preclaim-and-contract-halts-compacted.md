---
schema_version: "1.0"
doc_type: memory
title: "Compacted Memory: 154-S Ship pre-claim halt, contract halt, and resume (2026-09-29)"
description: "Compacted record of the 154-S Ship pre-claim worktree halt, the wave-schedule red-deliverable contract halt, and the resume after Stage's contract amendment."
chunk_strategy: h1-h2-h3
compacted_at: "2026-10-17"
---

# 154-S Ship pre-claim and contract halts

Shipment `154-S` (feature `173-F`, shipment-claim scheduler baseline marker) had
seven task members: `173.006-T`, `173.007-T`, `173.008-T`, `173.009-T`,
`173.001-T`, `173.003-T`, and `173.005-T`. Branch:
`feat/154-s-shipment-claim-scheduler-baseline-marker`.

Originals are archived under `docs/archive/memory/`:
`2026-09-29-ship-154s-preclaim-blocked.md`,
`2026-09-29-ship-154s-wave-schedule-contract-halt.md`, and
`2026-09-29-ship-154s-resumed.md`.

## Pre-claim halt

* Intake checks passed: config validated, backlogit and engram reachable, no
  active Ship checkpoint, no other active shipments, and the `pre_claim`
  topology gate passed.
* The P-011 branch-creation gate halted the run because the worktree on `main`
  was not clean (modified `.backlogit/stash.jsonl`, untracked checkpoints and
  `155-S` reconcile files). Nothing was staged, stashed, cleaned, or reset, and
  no claim or implementation began.
* Resume: operator provides a clean-worktree feature branch without altering
  the protected files, then Ship intake is re-run.

## Wave-schedule contract halt

* After the operator allowlisted the carry-forward artifacts, Ship created the
  branch, committed them as `cdf11cc9` and claimed `154-S`. The claim call
  timed out, but the `post_claim` gate and `backlogit_get_shipment` confirmed
  the shipment was `active`.
* Halt `WAVE_RED_MAPPING_UNRESOLVED` (P-002.2): `173.006-T`, `173.007-T`, and
  `173.008-T` declared RED deliverables with no canonical
  `red-deliverable-contract` block. Ship does not infer the mapping from prose.
  Repeated section reads crossed the three-failure circuit breaker.
* `154-S` was governedly blocked with a structured checkpoint as its resume
  reference. Resume required a Stage amendment plus an operator-selected
  checkpoint, then reconciliation and topology and wave-schedule re-checks.

## Resume after Stage amendment

* The operator authorized the governed unblock. Stage commit `b49350d7` added
  the missing red-deliverable contracts to the three tasks.
* Unblock used `confirm: true` and `target: active`. The recorded member status
  snapshot matched; no task statuses were rewritten by hand.
* Frozen schedule `M` (seven tasks, `173-F` excluded) produced five acyclic
  waves: (`.006`, `.007`, `.008`), `.009`, `.001`, `.003`, `.005`.
  Red-deliverable mappings: `.006` and `.007` close at wave 3 via `.001`;
  `.008` closes at wave 2 via `.009`. Green-regression arrays were `[]`.
* The `wave-scheduler-sim` check returned `WAVE_SIM_OK: 186/186`.
* The approved bootstrap exception was limited to the recorded claim-activated
  status; dependency ordering and all gates stayed in force.
