---
doc_type: memory
schema_version: "1.0"
title: "Compacted memory: 155-S / 174-F Ship session checkpoints (W1–W3, Wave 20, PR #450, closure)"
---

# Compacted Memory: 155-S / 174-F Ship Session Checkpoints

This file was compacted on 2026-09-29 during the P-020 compact-context run for
the `182-S` post-merge closure. It replaces 11 unreferenced Ship checkpoint
memories for shipment `155-S` (feature `174-F`). That shipment shipped in
PR #450 (merge `2c8759c3`), with closure PR #451. Originals were moved, not
deleted, to `docs/archive/memory/` under a date prefix:

* `2026-09-24-ship-155s-w1-harness-complete.md`
* `2026-09-24-ship-155s-w1-w3-blocked-174074-red-delta.md`
* `2026-09-24-ship-155s-w1-w3-blocked-174074-unexcluded-stash.md`
* `2026-09-25-ship-155s-w1-w3-complete-awaiting-suite-authorization.md`
* `2026-09-25-ship-155s-w2-complete-w3-class-halt.md`
* `2026-09-25-ship-155s-w2-halt-unrelated-gates.md`
* `2026-09-26-ship-155s-adversarial-review-blocked.md`
* `2026-09-26-ship-155s-pr450-blocked-on-required-gates.md`
* `2026-09-26-ship-155s-preclose-reconciliation-blocked.md`
* `2026-09-26-ship-155s-wave20-u20c1-role-boundary-halt.md`
* `2026-09-26-ship-155s-wave20-u20c5-p010-halt.md`

Other `155-S` memory files are still referenced by checkpoints, the `154-S`
banner, backlog records, or plans. They stay in place so those references keep
resolving.

## Outcome

* The W1–W3 tasks `174.073-T` through `174.076-T` completed, then Wave 20
  correctives `174.077-T` through `174.082-T`. PR #450 merged with a merge
  commit, and `155-S` was closed through the governed flat ship operation.
  The governed ship had to be retried: the first `shipment ship` call exceeded
  a 5-minute command timeout with no mutation. The second completed in about
  34 minutes.
* The operator withheld the governed full suite `go test -timeout=30m ./...`
  and authorized it one run at a time. PR CI was the confirmation run.

## Decisions and Halts (chronological)

* **174.074 red-deliverable vs claim (W1).** Claiming a task mutates its
  tracked queue file. Build-feature Step 0.5b's zero-delta gate had no
  lifecycle exclusion, so claiming broke the gate. Ship halted instead of
  inventing an exemption. The operator granted waiver `W-174074`, later
  extended as `W-174074-ext`. A mandatory P-021 capture then appended to
  `.backlogit/stash.jsonl`, which was out of the waiver surface, and Ship
  halted again. Entry `9CA03F5D` was captured with
  `DISCOVERY-STATUS: AMBIGUOUS` (candidate `7AA35A39`).
* **W2 (174.075-T).** First halt: an unrelated scratch doc failed doc lint
  (captured as `2F7FCA8B`), and `.backlogit/hooks_queue.jsonl` changed outside
  the waiver. The resolution admitted the hook journal only as an append-only
  suffix over a hashed prefix. After that, the exempt command passed with its
  marker.
* **W3 (174.076-T).** The first halt was P-002.4: the `docs-only` class does
  not cover `.autoharness/harness-manifest.yaml`. Stage reclassified the task
  as harness-required, with no relaxation of the exemption. The `^TestU19R3_`
  harness went red, then green.
* **Operator-owned edit preservation.** A `tools:` line in `_ship.agent.md`
  was never staged. Byte-exact backup and restore was verified by SHA-256
  across the W2/W3 index-blob swaps.
* **Wave 20.** A harness correction for U20C1 was committed on its own, citing
  §20.3.1. U20C3 and U20C6 completed. For U20C5, Ship edited source directly,
  a P-010 violation. The Orchestrator accepted it as procedural with no
  rewrite (commit `98367132`).
* **Review.** The standard review lacked full coverage of the 1.8 MB diff. The
  adversarial route was refused because of external source transfer. Readiness
  was later completed, and PR #450 went out `READY_WITH_FOLLOWUPS`.
* **PR #450 gates.** A P-008 markdownlint MD001 finding on archived task
  records was deferred as `C85386E6`. A lock-order test was stabilized in
  `85570a3b`.

## Learnings

* A red-deliverable zero-delta gate needs an explicit lifecycle-bookkeeping
  exclusion set. Without one, claiming the task and mandatory captures both
  trip it.
* A governed shipment ship over a large manifest can run far longer than
  5 minutes. Budget accordingly and never retry without disposition.
* Compaction was repeatedly deferred while `155-S` was active. It became
  eligible only after the shipment was archived, and was done here.
