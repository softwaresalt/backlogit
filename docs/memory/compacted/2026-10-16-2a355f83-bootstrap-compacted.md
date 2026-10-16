---
schema_version: "1.0"
doc_type: memory
title: "Compacted memory: 2A355F83 bootstrap staging and 195-S startup halt (2026-10-01..03)"
description: "Dense summary of six Stage/Orchestrator/Ship checkpoints about the claimed-vs-started bootstrap plan, harvest under 195-F/195-S, and the 195-S startup halt."
chunk_strategy: h1-h2-h3
---

# Compacted memory: 2A355F83 bootstrap (2026-10-01 to 2026-10-03)

## Outcome

* Plan review for stash `2A355F83` failed three times and ended at `ESCALATION_DEGRADED`, an operator halt. A single operator-authorized fourth attempt returned ADVISORY (0 P0, 0 P1, ten P2 findings).
* After explicit operator approval, a feature-root harvest created `195-F`, seven tasks `195.001-T` to `195.007-T` and shipment `195-S`. A chore root was avoided because of an ID collision with archived `001-C` tasks. Staging commit `888fccc9`. Execution stayed gated.
* The engram daemon Ready handshake hit the circuit breaker twice (three failures each) during checkpoint recovery.
* Ship's startup halt for `195-S` (checkpoint `checkpoint-20260905-031054.json`, reported as lacking `resume_hint`) was **superseded** on 2026-10-02: the Orchestrator's authoritative correction showed the checkpoint was valid with `context.resume_hint` present and zero active/quarantine-required checkpoints. The checkpoint was not mutated.
* Final outcome (2026-10-03): PR #470 (operator bootstrap grant) merged; topology gate passed; Ship was dispatched. PR #471 (repaired Ship/policy text) merged at `58f5bdbac22d2c051aea41a9b0a2d508b5a7b9d0`. First closure attempt halted `RECONCILE_FAIL` (195-F still active); after Stage moved 195-F to done, reconcile pre/post passed and `195-S` was shipped and archived. Closure PR #472 merged at `1ccecd946de3577c31f4018166d1ea05a7faf848`; closure `READY_WITH_CONDITIONS`, compaction status `degraded`.
* Remaining follow-ups for Stage triage: `227A2930` (pass authoritative served roots to Ship), `B83081F5`, `359D8F32`, `731CE551`, `41FE00A1`. Lesson: Ship's breaker batch kept issuing requests after the 3rd failure.

## Decisions

* A checkpoint was never rewritten to clear an anomaly.
* The harvest packaging deviation was approved only for that scope.

## Archived originals

Moved to `docs/archive/memory/2026-10-01/` and `docs/archive/memory/2026-10-02/`:
`stage-claimed-vs-started-bootstrap-memory.md`, `circuit-break-engram-daemon-ready.md`, `orchestrator-195s-staging-gates.md`, `ship-195s-startup-halt.md`, `stage-2a355f83-attempt4-advisory.md`, `stage-2a355f83-harvest-complete.md`.
