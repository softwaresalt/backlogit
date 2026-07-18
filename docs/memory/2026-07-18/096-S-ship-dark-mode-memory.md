---
chunk_strategy: h1-h2-h3
description: 'Session memory — DARK_MODE dark-factory pipeline: shipped 096-S size-extension architecture spike (PR #252, merge 86aa6ec). Records the two-model adversarial re-review that flipped the conclusion pivot->proceed and hardened exactly-once durability and lexical containment, the eight Copilot review cycles, the post-merge closure, and the next step (stage-next of five stash entries).'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-07-18/096-S-ship-dark-mode-memory.md
title: 096-S ship (dark-mode) — session memory
---

# 096-S Ship (DARK_MODE) — Session Memory

## Scope

DARK_MODE (P-017) dark-factory pipeline, operator AFK. Ordered scope:
(1) ship 095-S ✅ (prior), (2) ship 096-S ✅ (this session), (3) stage-next
(5 stash entries) — PENDING.

## 096-S outcome

- **Shipped**: PR #252 merged as merge commit `86aa6ec` (P-009). Shipment
  `096-S` shipped; `109-F` + `109.001-007-T` archived; pre/post reconcile PROCEED.
- **Deliverable**: `docs/decisions/2026-07-18-size-extension-contract-architecture-spike.md`
  (read-only spike) + `internal/core/docline_codec_roundtrip_test.go` (3 tests).
- **Conclusion**: `proceed` — canonical artifact-size home = `custom_fields.size`
  (Model-A delegated option); carrier bridge rejected; all 3 proceed-gates resolved.

## Review provenance (key decisions)

- Two-model adversarial re-review on current HEAD: Gemini clean; GPT-5.6-sol
  1 P0 + 3 P1 + 1 P2, **all accepted** after cross-checking the charter:
  - P0: conclusion `pivot`->`proceed` — charter `exec-plan:652-664` makes proceed
    the sole impl-authorization once all 3 gates resolve (they are).
  - P1: exactly-once history events required (`exec-plan:183-193`) -> durability
    policy changed to **event-before-write, fail-closed** (was event-after-write).
  - P1: `QueueLayout.RootDir` lexical `..` escape (`schema.go:99-104` vs the
    `reg.Directories`-only guard `loader.go:77-85`) -> two-layer lexical+realpath fix.
  - P1: read-parity matrix corrections (queue/list default divergence, get_queue
    already projects custom_fields, ExitError{Code:4} mis-citation).
  - P2: provenance-scope coherence.
- Eight Copilot cycles total; final cycle-8 on `e845145` clean. `time_box` synced
  to charter 14h (`exec-plan:11,18`).

## Files modified this session

- `docs/decisions/2026-07-18-size-extension-contract-architecture-spike.md`
  (commits `c0257c3`, `d7e377a`, `dc6bc78`, `ed016e3`, `e845145`).
- `internal/core/docline_codec_roundtrip_test.go` (`d7e377a`).
- `docs/closure/2026-07-18-096-S-size-extension-spike-post-merge-closure.md` (this closure).
- Backlog: `096-S` queue->archive; member archive metadata; `hooks_queue.jsonl`.

## Failed approaches / corrections

- Original spike labeled `pivot` on the mistaken basis of "approach changed";
  the charter defines proceed/pivot by downstream effect (gates resolved +
  implementation authorized = proceed). Corrected.
- Original durability policy "fail-surface, event-after-write, no-rollback"
  violated the chartered exactly-once requirement. Corrected to fail-closed.

## Next steps

1. **stage-next**: triage 5 active stash entries -> backlog + shipment(s), ship if
   safe. Proceed conclusion authorizes 108-F blocked->active restaging in that cycle.
2. Parked untracked (LEAVE ALONE): `docs/decisions/2026-07-13-scratch-spike.md`,
   `docs/memory/2026-07-17/094-S-ship-closure-memory.md`.
