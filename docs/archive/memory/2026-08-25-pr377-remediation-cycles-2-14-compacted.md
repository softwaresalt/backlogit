---
chunk_strategy: h2
description: "Compacted rollup of PR #377 remediation cycles 2-14 for shipment 130-S — decisions, structural changes, and superseded claims, with archive pointers"
doc_type: memory
schema_version: "1.0"
source: compact-context
title: "PR #377 Remediation Cycles 2-14 Compacted"
---

# PR #377 Remediation Cycles 2-14 Compacted

**Compacted**: 2026-08-25 (cycle-16 session)
**Release unit**: shipment `130-S`, feature `147-F`, PR #377, branch `chore/stage-130-s`
**Originals**: `docs/archive/memory/stage-pr377-remediation-cycle-{2,3,4,6,7,8,9,10,11,12,13,14}-memory.md`

## Why these were compacted

Twelve per-cycle memory artifacts for one still-open release unit. Each cycle superseded the one
before it, and the authoritative record of every split, retirement, and narrowing lives in the
plan's own `PR #377 ... remediation` appendices, not in these files. Cycle-15 and cycle-16 memory
artifacts remain **live** because they carry the current gate context. Nothing in the active plan,
the cycle-15 `Plan Review` `FAIL` record, the cycle-16 appendix, the canonical checkpoint, or
`.backlogit/memories.json` was touched by this compaction.

## Structural outcome across cycles 2-14

Tasks added to hold the mandatory `<3 files` / `<4 scenarios` granularity limits:

| Cycle | Added | Unit | Reason |
|---|---|---|---|
| 3 | `147.023-T`, `147.024-T`, `147.025-T`, `147.026-T` | U6d, U7c, U7d, U10b | width splits from U6, U7b, U7, U10 |
| 4 | `147.027-T` | U8c | CLI `checkpoint get` projection that U8b's parity table required and no unit owned |
| 14 | `147.028-T`, `147.029-T` | U2g, U7e | context-member duplicate detection; `domainError` mappings split out of U7 and U7d |

Retirement: `147.010-T` / U5b archived in cycle 10 because its production delta — changing the
`QuarantineCheckpoint` refusal sentinel for non-active conforming targets — contradicted the origin
decision's explicit exclusion of the state-conflict class. Inventing a delta to satisfy P-004's red
gate is not a valid justification.

## Decisions that still bind

* **Refuse, do not preserve.** The checkpoint top level is closed at create (146.011-T), so a
  top-level preservation carrier would make one namespace closed inbound and open outbound.
  `context.Extra` is the sanctioned open carrier.
* **`ParseCheckpoint` stays lenient.** Making it strict would sweep the whole on-disk corpus into
  quarantine candidacy. The conformance check is caller-invoked at two write and two read
  boundaries.
* **Round-trip safety, not "no unknown keys", is the predicate.** `status` plus `Status` has zero
  unknown keys and still loses a member on rewrite — this is what makes U2c non-optional.
* **Multi-`%w` for the resolve validity gate.** Copying `AbandonCheckpoint`'s shipped `%v` idiom
  would drop `ErrCheckpointInvalid` and lose the one mapping that classifies the refusal.
* **`conforming` is a new reported field, never a redefinition of `valid`.** `GetCheckpoint`'s
  `valid` already has consumers.
* **U6 stays read-only and computes the verdict before the filter block**; whether a quarantine
  candidate also becomes filter-exempt is U6d's separate, published contract.
* **MCP `get` keeps its validation-class refusal** for schema-invalid documents. Routing a read
  through a mutation-shaped error path buys no safety — `list` already reports
  `needs_quarantine: true` for the same file.
* **Cycle-4 teardown ownership move.** U10 hands the scratch workspace over intact; U10b owns
  teardown, because U10b consumes U10's archive, fixtures, and branch-built binary.

## Claims corrected during these cycles

* U7d's `validation_failed` mapping for the abandoned-resolve case was wrongly called
  "pre-existing" — `domainError` had no case for it and fell to `default: InternalError`.
* U7's `domainError` claim that `ErrCheckpointInvalid` was unmapped was false;
  `internal/mcp/errors.go` already maps it to `validation_failed`.
* The `RemediationCommand` shell contract is **POSIX-safe only**, not PowerShell-safe for arbitrary
  filenames; the generator's `checkpoint-YYYYMMDD-HHMMSS.json` shape happens to be paste-safe in
  `pwsh` as well.
* U8's cross-reference to a CLI/MCP error-shape asymmetry "restated in U9b" pointed at text U9b did
  not contain; repaired by naming the follow-up record that does exist (stash `63E810D9`).

## Superseded by cycles 15 and 16

Read these only as history:

* Cycle 14 placed context-duplicate detection in `CheckpointContext.UnmarshalJSON` behind a new
  `ErrCheckpointContextDuplicateKey`. **Withdrawn in cycle 15** — the verdict lives in the
  caller-invoked read-boundary conformance helper and reports through the existing
  `ErrCheckpointNonConforming` contract.
* Cycles 10-14 built up a body-preserving hand-repair and post-quarantine restore runbook in U9b.
  **Withdrawn in cycle 16** on security grounds and deferred to stash `35A27CD0`.
* Cycle 14's U7e carried three `domainError` rows. **Narrowed in cycle 16** to the one reachable
  row after a source audit of the handler routing.
* Cycle-10-through-15 topology figures (26-28 tasks, 40-48 edges) are all superseded by the
  cycle-16 measurement: 29 queued tasks, 52 executable edges, 30 shipment members, ready set
  `{147.001-T}`, historical total 53.

## Pointers

* Current gate and remediation record: `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
* Current session memory: `docs/memory/2026-08-24/stage-pr377-remediation-cycle-16-memory.md`
* Immediately prior session memory: `docs/memory/2026-08-24/stage-pr377-remediation-cycle-15-memory.md`
* Origin decision: `docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md`
