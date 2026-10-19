---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Holding a lock across a side effect changes its lock order"
description: "CY-B (200-S, PR #540): moving item-log lock C before B made nested archives hold C through a stash side effect, which Copilot found could deadlock against CorrectStashProvenance; queue nested side effects for the outermost owner."
doc_type: learning
source: docs/compound/2026-10-19-lock-held-side-effect-ordering.md
docline:
    date: 2026-10-19T00:00:00Z
    severity: medium
    tags:
        - concurrency
        - locking
        - 200-S
---

# Holding a lock across a side effect changes its lock order

## Problem

CY-B reordered ShipShipment and ArchiveItem so item-log lock C is taken before batch lock B. Nested archives inherit the outer C, so a post-archive stash effect that took the process-wide `stashMu` now ran while C was held. `CorrectStashProvenance` takes `stashMu` and then requests C, so the two orders could deadlock. Three local persona reviews missed it; Copilot found it (P1, PR #540).

## Learnings

* **Extending a lock's hold time creates new edges.** When you widen the span a lock is held, list every mutex and lock the code in that span can reach, not only the one you are reordering.
* **Queue effects for the outermost owner.** Nested operations register the effect on a context-carried queue. The owner runs it from the first-registered defer, so it runs after every lock release. Keep the execution-time CAS check.
* **Test the inversion with the real mutex.** Hold the other lock in the test, run the operation, then request the first lock under a short timeout, so a regression fails fast instead of hanging.
* **Scope the lock set to the real write set.** A recursive descendant set added unrelated locks for a flat manifest (Copilot, same PR). Derive the set from what the operation actually writes.
* **Capture, do not fix, a pre-existing inversion on another surface.** Commit writers take C then global; stash 703835DC.
