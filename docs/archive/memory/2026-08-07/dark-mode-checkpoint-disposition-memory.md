---
type: dark-mode-cycle
timestamp: 2026-08-07T23:35:00-07:00
agent: Stage
skill: direct
policy: P-017 (dark factory mode) — amended scope reactivation
operation: checkpoint administrative disposition staging (stash FDEDE39A)
---

# Stage session memory — amended dark scope (checkpoint disposition)

- **Date:** 2026-08-07
- **Worktree:** `.copilot/session-state/ecebe820-92d7-4253-852a-0c3c23f8aea9/files/dark-factory-worktree`
- **Branch:** `admin/dark-stage-formal-gate`, reactivated from `3aed7bbe`
- **Prior cycle:** halted under `DARK_MODE_HALTED` for scope expansion; all prior
  work preserved and untouched.

## DARK_MODE_ACTIVE — reactivation

All prior activation approvals and stop conditions carry forward unchanged:
`merge_approval_pre_authorized: true`, `admin_fallback_pre_authorized: true` for
confidently classified review/conversation branch-protection blocks only, P-009
merge commits, P-014 Copilot gate, P-016 single-worktree topology, and the full
armed stop-condition list. Visibility remains **degraded** — `agent-intercom` and
`agent-engram` are still unavailable; this record replaces the broadcast.

## DARK_MODE_SCOPE (amended)

| Order | Unit | Status this cycle |
|---|---|---|
| 1 | `117-S` F1 | unchanged, staged in the prior cycle |
| 2 | `118-S` F4 | unchanged |
| 3 | `119-S` F6 | unchanged |
| 4 | `120-S` F5 | unchanged |
| 5 | **`122-S` checkpoint disposition** | **staged this cycle** |
| 6 | `121-S` workspace-dir rename | unchanged, re-sequenced after `122-S` |
| — | `054.001-R` | already completed and archived; **not reopened** |

**Explicit exclusion honored:** `B5D7E401` (checkpoint JSON HTML-escaping) was not
consumed, harvested, planned, or archived. Verified: the string `B5D7E401` appears
in **zero** files on this branch.

## Step 1 — stash import with ID preservation

`backlogit stash add` has **no `--id` flag** (verified via `--help`), so the CLI
cannot preserve a caller-provided stash ID. Per the operator's instruction, the
sanctioned fallback was used: a surgical Stage-owned edit wrote the **verbatim**
`FDEDE39A` JSONL line — read read-only from the dirty primary root — into the
linked worktree's empty `.backlogit/stash.jsonl`, followed by `backlogit sync`.

Safety checks executed during the import:

* asserted exactly **one** matching line was captured;
* asserted the captured line contained neither `B5D7E401` nor `9370A18C`;
* wrote with an explicit LF terminator and a BOM-free UTF-8 encoder;
* the primary worktree was **read only** — never modified, staged, stashed, or
  restored.

Original ID, kind, priority, text, and `created_at` were all preserved byte-for-byte,
so traceability from stash to `136-F` to `122-S` is intact.

## Steps 1.5–1.8 — classification, grouping, learnings

`FDEDE39A` is **feature-shaped** (`kind: feature`, declares a coherent capability
with multiple implied tasks), so Step 1.5 contextual grouping was **skipped by
rule** — the Stage contract skips grouping for feature-shaped entries. Evaluation
logged rather than silently omitted: the one natural grouping partner,
`B5D7E401`, is in the same checkpoint subsystem but is **excluded by operator
directive**, so no grouping was possible or attempted.

Learnings retrieval returned **high** confidence for exact-target-vs-broad-sweep,
evidence preservation, auditable operator/reason metadata, CLI/MCP parity, and
atomic single-file writes, and an explicit **gap statement** for checkpoint-specific
prior art: *"There is NO compound learning and NO decision record dedicated to
checkpoint lifecycle, retention/cleanup, quarantine, or validation errors."* All
cited precedents are therefore analogs and are labeled as such in the deliberation.

## Steps 2–4 — deliberate, plan, harden, review

| Artifact | Path |
|---|---|
| Deliberation | `docs/decisions/2026-08-07-checkpoint-administrative-disposition-deliberation.md` |
| Plan | `docs/exec-plans/2026-08-07-checkpoint-administrative-disposition-plan.md` |

`BRAINSTORM_HANDOFF_READY`: decision recorded as `decided`, promoted to plan.
Unresolved questions carried as advisory follow-ups, none blocking — whether
`abandon` should later archive rather than leave in place; whether the sidecar is
projected into the SQLite index; whether a bulk maintenance verb is ever added for
reported `NeedsQuarantine` files.

P-006: `Requires plan hardening: yes` with a full `## Plan Hardening` section —
eleven protected invariants, eleven consulted learnings, five classified risky
actions, deepened verification, and unresolved decisions.

### Plan review

`dispatch_mode: multi-agent-dispatch`. Personas: Constitution Reviewer
(`claude-opus-4.6`), Go Reviewer with an architecture lens
(`gemini-3.1-pro-preview`), Scope Boundary Auditor with an agent-native parity lens
(`gpt-5.6-sol`), plus the Learnings Researcher pass feeding the deliberation.

| Cycle | Decision | P0 | P1 | P2 | P3 |
|---|---|---|---|---|---|
| 1 | **FAIL** | 2 | 2 | 5 | 3 |
| 2 | **FAIL** | 0 | 5 | 3 | 0 |
| 3 (post-remediation) | **PASS** | 0 | 0 | — | — |

Two remediation cycles used, within the 3-attempt circuit breaker.

**Cycle 1 P0s — both real and both structural:**

1. **Import cycle.** The draft placed the verbs, target confinement, and audit in
   `internal/events` while reusing `internal/core` helpers. Verified at HEAD:
   `internal/core` imports `internal/events` in **13** files; `internal/events`
   imports `internal/core` in **zero**. Fixed by settling placement — types stay in
   `internal/events`, all operational logic moves to `internal/core` — and
   recording it as a protected invariant.
2. **Contradictory audit design.** A fail-closed audit combined with "the CLI
   passes `nil`" for the `EventWriter` would have made every CLI disposition fail
   permanently. Fixed: both surfaces construct a real writer; no caller passes
   `nil`; core never mints one.

**Cycle 2 P1s — the substantive design corrections:**

1. The deliberation still recorded the superseded "converge the implicit
   auto-quarantine" decision while the plan removed it → the deliberation was
   **amended** with an explicit supersession record.
2. `quarantine` accepted **every** checkpoint, overlapping `abandon` and exceeding
   the stash's malformed-only wording → it now classifies **in memory** and refuses
   a valid target with `ErrCheckpointUseAbandon`, without ever rewriting bytes.
3. `disposition_operator` provenance was undefined, and MCP has no authenticated
   identity → CLI resolves `--operator` → `BACKLOGIT_OPERATOR` → OS user and
   **never** defaults to `backlogit`; MCP requires `operator` as a tool parameter.
4. "A failed audit means unmoved" conflicted with the F5 rule that an indeterminate
   write must never be compensated → the audit was **reordered to run before the
   move**, so the invariant holds in *both* error classes without compensating
   anything.
5. A `governed: true` marker alone cannot establish behavioral parity → `U12` now
   registers valid **and** malformed parity fixtures with the existing F6 assertion,
   and halts rather than building a second framework.

Also removed as YAGNI: `disposition_source`, which became a constant once the only
automatic producer was deleted.

## Step 5 — harvest

Covering feature **`136-F`** (high) plus 13 tasks `136.001-T` … `136.013-T`, all
`queued`, each ≤2 hours, each width-isolated, each with acceptance criteria.

| Task | Unit | Domain |
|---|---|---|
| `136.001-T` | U1 harness stubs + disposition invariants | tests |
| `136.002-T` | U2 metadata contract, `abandoned` status, sidecar type, listing fields | code |
| `136.003-T` | U3 typed sentinels in `internal/errors` | code |
| `136.004-T` | U4 confinement and refusal matrix | tests |
| `136.005-T` | U5 basename-only target resolution (thin adapter) | code |
| `136.006-T` | U6 `AbandonCheckpoint` | code |
| `136.007-T` | U7 `QuarantineCheckpoint` (malformed-only, byte-preserving) | code |
| `136.008-T` | U8 audit before the move, class-specific handling | code |
| `136.009-T` | U9 make `ListCheckpoints` read-only | code |
| `136.010-T` | U10 CLI subcommands | code |
| `136.011-T` | U11 MCP tools and structured errors | code |
| `136.012-T` | U12 registry rows, governed markers, parity fixtures | config |
| `136.013-T` | U13 documentation | docs |

**14 dependency edges** wired: the internal chain
`U1→U2→U3→U4→U5→{U6,U7}→U8→U9→U10→U11→U12→U13`, plus `136.001-T ← 106.031-T` so
the unit cannot start before the F5 foundation it consumes has landed.

## Step 5.5 — shipment assembly and ordering

**`122-S`** — "Administrative checkpoint disposition (abandon and quarantine)",
priority `high`, `queued`, manifest `136-F` + all 13 tasks. It is a **complete**
feature shipment, so the covering feature is included; this is not a partial-feature
shipment and the P-015 exclusion that applies to `117-S`/`118-S`/`119-S` does not
apply here.

Shipment-level ordering edges:

* `122-S` blocks on `120-S`
* `121-S` blocks on `122-S`
* additionally `135.001-T` blocks on `136.013-T`, so the ordering is enforced at
  task level too, mirroring how the F-series order was encoded.

Final order: `117-S → 118-S → 119-S → 120-S → 122-S → 121-S`. All six are `queued`;
**none is active or claimed**.

## Step 5.6 — consumed stash archival

`FDEDE39A` archived **after** successful harvest and shipment assembly. The branch
stash is now empty. `B5D7E401` was never imported and appears in zero files.

## Files changed this cycle

* `.backlogit/stash.jsonl`, `.backlogit/archive/stash.jsonl` — `FDEDE39A` imported
  verbatim, then archived after harvest
* `.backlogit/queue/136-F.md`, `136.001-T.md` … `136.013-T.md` — new feature and 13 tasks
* `.backlogit/queue/122-S.md` — new queued shipment
* `.backlogit/queue/120-S.md`, `121-S.md`, `135.001-T.md` — dependency edges added
* `.backlogit/hooks_queue.jsonl` — tool-managed append-only event stream
* `docs/decisions/2026-08-07-checkpoint-administrative-disposition-deliberation.md`
* `docs/exec-plans/2026-08-07-checkpoint-administrative-disposition-plan.md`
* `docs/memory/2026-08-07/dark-mode-checkpoint-disposition-memory.md` — this file

## Handoff

Ship claims in strict order `117-S → 118-S → 119-S → 120-S → 122-S → 121-S`, reusing
this same linked worktree to preserve P-016. `122-S` declares **blocking
dependencies** on the F5 `MutationEnvelope` and the F6 governed-operation contract
plus its fixture-registration seam — if any is absent at claim time, **halt rather
than reimplement**. Risky action `A1` (relocating checkpoint evidence) is classified
`destructive` and requires explicit operator approval; `A2`, `A3`, and `A4` are
`high` and also require approval.

The stash merge hazard recorded in the prior cycle still stands: this branch's
committed stash is empty, while the operator's primary worktree holds
`9370A18C`, `FDEDE39A`, and `B5D7E401` as **uncommitted** changes. `B5D7E401` in
particular must survive the merge — it is a live, unharvested bug entry for a future
cycle.
