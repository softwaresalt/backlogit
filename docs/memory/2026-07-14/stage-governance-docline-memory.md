---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Stage memory: blocked planning governance and docline shipments'
source: docs/memory/2026-07-14/stage-governance-docline-memory.md
doc_type: memory
description: 'Corrected session continuity for blocked shipments 094-S and 095-S after authorization and Copilot review findings.'
docline:
    ms.date: 2026-07-15T01:03:00Z
    ms.topic: memory
---

# Stage Memory: Blocked Planning Governance and Docline Shipments

## Current State

- Shipment `094-S`, feature `105-F`, and tasks `105.001-T` through `105.013-T` are blocked.
- Shipment `095-S`, feature `106-F`, and tasks `106.001-T` through `106.007-T` are blocked.
- Both plans remain `review_state: blocked`; formal plan-review is NOT RUN and waiver authorization is NONE.
- This operator direction authorizes one bounded refinement pass only; it is not formal evidence or a waiver.
- No shipment was queued, claimed, implemented, removed, or archived.

## Twelve-thread Refinement

- Formal evidence now uses one unique final machine-validated record with exact canonical plan digest, complete attributed persona returns, and duplicate/stale/edit/trailing negatives.
- Formal and waiver records share one terminal parser but retain distinct schemas and verdict semantics.
- Waiver schema includes `intended_disposition`; every non-transition field is bound by an immutable authorization payload hash.
- Reservation returns the raw token once only to a long-lived in-process owner session, never to command/tool output; only its fingerprint persists and timing-safe comparison occurs under lock.
- Every mutation is a typed governed operation holding the plan lock/lease through revalidation and commit; validate-then-command is forbidden.
- `internal/atomicfile.WriteFileAtomic` is reused without fsync because Git supplies durability and governance requires atomic visibility.
- Plan-review produces evidence only. Stage owns stage-managed lifecycle; direct harvest owns direct lifecycle.
- Stage-managed ADVISORY requires durable plan-scoped confirmation; direct harvest accepts PASS or valid waiver only.
- Plan paths must be workspace-relative and reject traversal, symlink, junction, and reparse escapes before access.

## Task and Manifest Changes

Five blocked tasks were added to `094-S`:

- `105.009-T` — final formal review schema.
- `105.010-T` — immutable waiver schema.
- `105.011-T` — lock-held gate lease.
- `105.012-T` — governed core mutation broker.
- `105.013-T` — workspace plan-path containment.

Existing `105.001-T` through `105.008-T` were re-scoped for shared terminal parsing, token secrecy, immutable payload, correct lifecycle ownership, governed mutation, direct ADVISORY rejection, and expanded negative contracts. Every task remains at most two files and fewer than five production/test functions.

## Provenance Repair

Canonical source provenance now maps:

- `8CD8F46A -> 105-F`
- `CA877CD1 -> 105.004-T`
- `A4BE2FAD -> 106-F`

Targets carry standard `source_stash_*` metadata and archived source records carry harvested artifact IDs. Two consecutive repository-native syncs rebuilt exact `state: harvested` stash links. Repair stash `3E12DC97` was updated with verification evidence and archived only afterward.

## Docline Guard Refinement

`106.001-T`, its plan, and decision now require Git-tracked inventory filtered through exported `internal/docline.Scope()` rather than duplicated scope tables. Every selected path is checked for symlink/junction/reparse and resolved-target containment before read. Hermetic tests include an external-target escape and an always-run synthetic link-classification negative when real link creation lacks platform privilege.

## Upstream and Independent Intake

- Active stash `823BADF4` covers all four external generated templates and the refined formal/waiver, lifecycle ownership, governed mutation, ADVISORY, path, bypass, and Constitution contracts.
- Stash `DB1F9026` still tracks supported shipment requeue; generic `blocked -> active` remains forbidden.
- `D7B1B33D` remains active, unharvested, outside both shipments, and independently isolated in PR #240. This refinement does not modify PR #240.

## Integrity

- Use repository-native `backlogit.exe --cwd .`; MCP remains bound to the wrong installed-plugin workspace.
- Two final syncs indexed 859 artifacts and preserved all three harvested links; docline lint reports zero violations.
- Doctor reports only pre-existing orphan `016.001-R`; zero fixes were applied.
- Protected `docs/decisions/2026-07-13-scratch-spike.md` remains untracked, untouched, and unstaged.

## Exact Blocked Plan Digests

- `docs/exec-plans/2026-07-14-planning-governance-gates-plan.md`: `be37dbb4a00d34c286d290e0da13638717bd0fdab285bae207c782b31a1cb948`
- `docs/exec-plans/2026-07-14-docline-soft-key-regression-guard-plan.md`: `73bee3cc00e52d354655412c5fbcaee557a4cf12a012cc4afb739afcdcfdd40b`

These are canonical UTF-8/LF digests of the current exact blocked plan bytes. They are not approvals. Any later plan edit requires a new digest and new exact-plan evidence or authorization.

## Next Steps

1. Commit/push the bounded pass; reply to and resolve the exact twelve authorized threads.
2. Wait for fresh exact-head Copilot review and report any genuinely new substantive findings without another broad redesign.
3. Require formal multi-persona evidence or a separate exact-plan waiver before unblocking either plan.
4. Do not merge PR #239 or mark either shipment queued.
