---
chunk_strategy: h1-h2-h3
description: "Stage session memory for PR #377 Copilot review remediation cycle 3 — eight unresolved threads reconciled by splitting four oversized tasks and repairing five implementation contracts across the 147-F / 130-S planning and backlog artifacts"
doc_type: memory
docline:
    date: 2026-08-24T00:00:00Z
    status: accepted
    tags:
        - session-memory
        - D3CE9E81
        - 147-F
        - 130-S
        - checkpoint
        - dark-mode
        - review-remediation
schema_version: "1.0"
source: docs/memory/2026-08-24/stage-pr377-remediation-cycle-3-memory.md
title: "Stage Session Memory — PR #377 Review Remediation Cycle 3"
---

# Stage Session Memory — PR #377 Review Remediation Cycle 3

**Date**: 2026-08-24
**Agent**: Stage
**Session scope**: PR #377 review remediation only, bounded to stash `D3CE9E81` / feature `147-F` / shipment `130-S`
**Dark mode**: `DARK_MODE_ACTIVE` under P-017, `admin_fallback_pre_authorized=false`
**Branch / worktree**: `chore/stage-130-s` in the dedicated Stage planning worktree
**Reviewed HEAD**: `d6c11c5ef55d2a053cf1c05f488feb70743a4359` (CI green)
**Cycle position**: third and final automatic cycle under the standard review-fix limit of 3

## Summary

Third Copilot review remediation cycle on staging PR #377. The fresh review
`PRR_kwDORzozKM8AAAABKsDFyQ` covers the current head and raised eight unresolved
threads under the summary "several implementation contracts are inconsistent, and
multiple tasks violate mandatory granularity limits". All eight were confirmed
against live Go source, all eight were **valid**, and all eight were fixed.
Nothing was declined or deferred. Two carried a partly incorrect premise while
being right about the remedy; in both cases the premise was corrected in the
owning task rather than silently adopted.

Unlike cycles 1 and 2, this cycle **changes the backlog shape**. Four tasks were
appended — `147.023-T`, `147.024-T`, `147.025-T`, `147.026-T` — because four
existing tasks breached the mandatory granularity limits and no amount of prose
editing fixes an oversized unit. The graph goes from 22 tasks and 34 edges to 26
tasks and 40 edges; shipment `130-S` goes from 23 to 27 manifest members. No
existing task ID was renumbered and no unit was removed.

## Granularity splits (the four that changed the graph)

Every split obeys append-only ID assignment and preserves the original task's
identity — the parent task keeps the smaller, more cohesive half.

### `147.013-T` → `147.025-T` (plan U7 → U7d)

U7 mixed two skill domains: sentinel-to-code mapping in
`internal/mcp/errors.go` and handler routing in `internal/mcp/tools.go`. Width
isolation forbids that. U7 now owns `errors.go` only; `147.025-T` owns the
`handleResolveCheckpoint` routing change.

The split also forced a **correctness** fix. `147.013-T` claimed `domainError`
has "no case for `ErrCheckpointInvalid`". It has one —
`internal/mcp/errors.go:188-193` groups it with `ErrValidation` and
`ErrCheckpointCorrupt` → `validation_failed`. Only `ErrCheckpointUseQuarantine`
and `ErrCheckpointNonConforming` are genuinely absent, so U7 adds two cases, not
three.

The routing in `147.025-T` must be **by class**, never wholesale:

```text
if events.QuarantineIsRemedy(err) { checkpointDispositionError(...) }
else                              { domainError(...) }
```

A wholesale swap to `checkpointDispositionError` would regress
`ErrCheckpointCannotResolveAbandoned` and `ErrCheckpointCorrupt` from
`validation_failed` to `internal`, because that helper's `default` arm is
`InternalError`. `ErrCheckpointNotFound` is already handled there
(`internal/mcp/errors.go:358`), so `NotFound` is preserved either way.

### `147.014-T` → `147.024-T` (plan U7b → U7c)

Five tool descriptions across two skill domains in one task. `147.014-T` keeps
the **two read-surface** rows (`backlogit_list_checkpoints`,
`backlogit_get_checkpoint`); `147.024-T` takes the **three mutation-surface**
rows (`resolve`, `abandon`, `quarantine`). `147.014-T`'s dependency retargets
from `147.013-T` to `147.022-T`, because the read descriptions promise the field
that `147.022-T`'s MCP `get` projection actually emits.

### `147.019-T` → `147.026-T` (plan U10 → U10b)

Five verification rows spanning refusal, acceptance, restore, and a recovery
sweep. `147.019-T` keeps three refusal-path rows; `147.026-T` takes acceptance,
restore, and the recovery sweep.

### `147.011-T` → `147.023-T` (plan U6 → U6d)

Not a size split — a missing unit. See below.

## Contract fixes

### The `list_checkpoints` filter hole

`147.011-T` asserted schema-invalid documents are "already surfaced" by
`ListCheckpoints`. They are not.
`internal/events/checkpoint_lifecycle.go:27-101` has two failure branches with
**different** filter behaviour: the parse-failure path (`:46-57`) appends and
`continue`s **before** the filter block, so it is filter-exempt; the `valErr`
path (`:69-73`) sets `NeedsQuarantine` and then **falls through** into the filter
block (`:76-92` — Agent / Status / ShipmentID / FeatureID / MaxAge). A quarantine
candidate with an empty `agent` is therefore filtered out of the very listing
that advertises its remedy.

`147.023-T` closes it with a **blanket** exemption for `NeedsQuarantine: true`,
matching the shipped parse-failure precedent. A lifecycle/identity two-tier split
was rejected: a schema-invalid file may have an empty `agent`, so any
identity-preserving filter can still hide it.

### MCP `get` keeps its validation-class refusal

`147.022-T`'s third acceptance criterion asserted MCP `get_checkpoint` returns
`code: checkpoint_use_quarantine`. Unreachable: `handleGetCheckpoint`
(`internal/mcp/tools.go:1194-1212`) routes through `domainError`, whose signature
is `domainError(op string, err error)` — no filename — so it structurally cannot
produce a `checkpointDispositionErrorResponse`. The criterion retargets to the
**reachable, pre-existing** `validation_failed`, and `147.022-T` drops its
dependency on `147.013-T`.

Recorded as a decision rather than a patch: routing a **read** through a
mutation-shaped error path would widen the unit into U7's file set and change a
shipped contract, and the quarantine remedy is already discoverable from
`list_checkpoints` (`needs_quarantine: true` plus a remediation command). So the
safety value is nil and the churn is real. **Disposition codes stay on the
mutation verbs.**

The same impossible code appeared one surface over in `147.016-T`'s parity table
(`legacy-shaped` row for MCP `get`). The review did not flag it; it was found
while reconciling and fixed in the same pass.

### U2f commits to enumeration

`147.021-T` offered two mechanisms — a write-site enumeration test **or** an
exported `events.RewriteCheckpointFile` seam. Its real file set was therefore
either one new test file or five files across `internal/events` and
`internal/core` and two skill domains. A unit whose size cannot be known before
work starts cannot be scoped to two hours.

The seam is **withdrawn**. Enumeration is the only mechanism. A halt condition
replaces the fallback: if the enumeration proves unimplementable, `147.021-T`
goes `blocked` and the seam is re-planned under a **new** ID rather than absorbed.
The task still covers both packages, because the allow-list must enumerate
`internal/core`'s `AbandonCheckpoint` write site too.

### Offender classification in the repair path

`147.018-T`'s entry point (a) told operators to "move unmodeled keys into
`context.legacy_top_level`" without saying which offenders that actually works
for. It gained a four-row classification table:

| Reported offender | Repair |
|---|---|
| `<key>` | Move into `context.legacy_top_level` |
| `duplicate:<key>`, one side modeled | Keep the modeled spelling; move the variant |
| `duplicate:<key>`, neither modeled | Move **both**, preserving both spellings |
| `duplicate:progress.<key>` | **Not repairable this way** — `progress` is modeled and `legacy_top_level` is top-level; operator decision or quarantine |

Plus a **termination rule**: re-run the conformance check after each repair and
stop after one round-trip. Without it a blind move can create a new duplicate and
loop.

### The U10 contradiction

`147.019-T` required every file under `.backlogit/checkpoints/` to be
byte-unchanged **and** a recovery sweep that "succeeds on every other file" in
that same directory. Success means a rewrite. Mutually exclusive.

The sweep moves to a **copied mirror** at
`docs/scratch/checkpoint-verification/mirror/`. Live bytes stay read-only and
hash-guarded; mirrored filenames are preserved so the nine-name discrimination
assertion still means what it says.

## Backlog shape after this cycle

| Dimension | Cycle 2 | Cycle 3 |
|---|---|---|
| Tasks under `147-F` | 22 | 26 |
| `blocks` edges | 34 | 40 |
| `130-S` manifest members | 23 | 27 |
| Ready set | `147.001-T` | `147.001-T` |

New edges: `147.023-T ← 147.011-T`; `147.025-T ← 147.001-T, 147.013-T`;
`147.024-T ← 147.013-T, 147.025-T`; `147.026-T ← 147.019-T`; `147.016-T` gains
`147.024-T`. Retargeted: `147.014-T` from `147.013-T` to `147.022-T`. Removed:
`147.022-T ← 147.013-T`. Seven added, one removed, net +6. No cycles.

## Files changed

* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
* `.backlogit/queue/147.005-T.md`, `147.011-T.md`, `147.012-T.md`, `147.013-T.md`,
  `147.014-T.md`, `147.016-T.md`, `147.018-T.md`, `147.019-T.md`, `147.021-T.md`,
  `147.022-T.md`
* `.backlogit/queue/147.023-T.md`, `147.024-T.md`, `147.025-T.md`,
  `147.026-T.md` (new)
* `.backlogit/queue/147-F.md`, `.backlogit/queue/130-S.md`
* `.backlogit/checkpoints/checkpoint-20260824-191617.json`
* `.backlogit/memories.json`
* `docs/memory/2026-08-24/stage-pr377-remediation-cycle-3-memory.md` (this file)

## Not done by Stage

Push, review-thread replies, thread resolution, review re-request, shipment
claim, and merge are all forbidden by the Stage Role Boundary (PR row: Allowed
"—", Forbidden "Create, push, or merge pull requests"), and the fail-closed rule
in `.github/instructions/role-enforcement.instructions.md` treats the unlisted
PR-mutating operations the same way. Ship or the operator owns all of them. The
exact reply bodies for each of the eight comment IDs were produced in the session
handoff.

No Go source was written and no build, test, lint, or format command was run —
all four are in the Stage Role Boundary's Forbidden column.

## Next steps

1. Ship or the operator pushes `chore/stage-130-s`.
2. Post the cycle-3 replies against the eight thread IDs and resolve them.
3. Re-request Copilot review and re-run the §1.9 readiness gate against the new
   head before presenting the PR for the P-014 merge approval.
4. This was standard cycle 3. If a fourth review raises new P0/P1 findings, the
   circuit breaker applies: accept remaining findings as backlog items or
   escalate for an explicit operator decision rather than looping automatically.
5. Refresh the SQLite index before trusting query output — these edits were made
   in a dedicated planning worktree.
