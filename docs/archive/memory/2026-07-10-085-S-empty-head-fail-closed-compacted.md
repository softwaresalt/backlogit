---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 085-S shipment-gate empty-head fail-closed hardening.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-085-S-empty-head-fail-closed-compacted.md
title: Compacted memory - 085-S empty head fail-closed hardening
---
## Summary

Shipment `085-S` hardened empty shipment/member `head_sha` handling so enforced real-repo shipment gates fail closed instead of silently skipping lineage checks. Stage bundled `B85DAEE8` and `1AEA2B0E`; Ship merged PR #185, shipped `085-S`, and prepared closure PR work.

## Archived originals

* `docs/archive/memory/2026-07-07-stage-shipment-gate-empty-head-fail-closed-checkpoint.md`
* `docs/archive/memory/2026-07-07-085-S-ship-session-checkpoint.md`

## Decisions and outcomes

* The discriminator is a bounded fail-closed repo-presence probe (`git rev-parse --is-inside-work-tree`), not `ev.Enforced`, because test brokers can fake git probes.
* Enforced plus real worktree plus empty shipment or member head fails closed; no-repo, non-enforcement, or non-autoharness paths preserve legacy skip behavior.
* Forced evidence has no empty-head exception because forced-in-real-repo records a head.
* PR #185 merged by true merge commit `7c129b0`; post-merge `shipment ship 085-S --sha 7c129b0` archived six artifacts with clean P-007 reconcile.

## Files, review, and verification

* `internal/core/shipment_gate.go` gained `inGitWorktreeBounded`, empty shipment-head fail-closed, and empty member-head fail-closed behavior.
* Tests added git fixture coverage, flipped the real-repo empty-member-head acceptance test to refusal, and preserved no-repo skip regression.
* Adversarial review blocked on broken `.git` pointer fail-open, then follow-up fixes made broken-repo handling message-independent and fail-closed.
* Copilot rounds fixed indeterminate `.git` stat fail-closed behavior and guarded no-repo tests against ambient git state.
* Full quality gates passed; runtime verification and operational closure were recorded. New compound knowledge captured empty-head fail-closed behavior and cross-referenced 084-S.
