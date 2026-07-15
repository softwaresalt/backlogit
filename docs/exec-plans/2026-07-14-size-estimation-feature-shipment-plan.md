---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Optional size estimation for feature and shipment artifacts plan'
source: docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md
doc_type: plan
description: 'Contract/architecture spike charter (PR #241 refocus) for optional backlogit-owned size extensions on the docline base contract: structured composition, provenance/history atomicity, and ruleset ownership questions gated before any implementation.'
docline:
    date: 2026-07-14T23:40:00Z
    refocused: 2026-07-15T00:00:00Z
    time_box: "2h"
    conclusion: "pending"
    confidence: "low"
    linked_stash_ids:
        - D7B1B33D
    review_state: spike-chartered
    gate: SPIKE
    review_provenance: "plan-review skill RE-RUN 2026-07-15 by Stage against final plan bytes after PR #241 refocus to a contract/architecture spike; single-model multi-persona (cross-model unavailable per skill fallback); no implementation PASS recorded — spike exit criteria only"
---

# Size Extension Contract Architecture Spike — Feature and Shipment Sizing (D7B1B33D)

## Status: Refocused to a contract/architecture spike (PR #241)

This document was harvested as an implementation plan and previously recorded an
implementation-ready PASS. Bounded investigation for PR #241 found that
provenance/history atomicity and XS-XL aggregation semantics are **not yet
resolvable as an implementable contract** against current code. Per the Stage
honesty rule ("do not manufacture implementation readiness"), it is refocused
into a **time-boxed (2h) contract/architecture spike** (`108.001-T`) that gates
three blocked implementation follow-ups (`108.002-T`/`108.003-T`/`108.004-T`).
The resolvable parts (base-contract/extension ownership and the structured
composition sketch) are recorded below as spike inputs; the unresolved parts are
named explicitly as spike questions, not decisions.

## Problem Frame

Size estimation currently exists as an optional `size` field on **tasks only**
(enum XS-XL; `.backlogit/header-def.yaml`). Feature and shipment types define no
`size` field, and no `size_source`, `size_ruleset_version`, or estimate-history
concept exists anywhere in `internal/`, `schemas/`, or the header-def. Stash
`D7B1B33D` asks to extend optional size estimation to feature and shipment
levels **without** conflating human-authored estimates with machine-derived
composition.

**Base-contract / extension ownership (authoritative, PR #241).** Docline owns
only the base Markdown/frontmatter ingestion contract and its compatibility
rules. That base contract is open/extensible — like a base class that can be
extended with additional optional properties. `size`, `size_source`, and
`size_ruleset_version` are optional **backlogit-owned** extension properties.
Docline does **not** calculate, default, aggregate, validate domain semantics,
or emit size; consumers (graphtor, engram) tolerate/preserve or safely ignore
these extension keys per existing codec behavior. Any derived feature/shipment
composition is a backlogit runtime/query projection and is **never** persisted as
if human-authored.

## Bounded Investigation Findings (read-only, current HEAD)

| # | Finding | Evidence |
|---|---|---|
| F1 | The size mutation seam accepts only `(id, size)` and has no provenance inputs. | `internal/core/artifact_size.go:35` — `SetArtifactSize(ctx, ws, id, size string)`. |
| F2 | Both adapters route only `size` through that seam. | CLI `internal/cli/update.go:97-115`; MCP `internal/mcp/tools.go:747-755`. |
| F3 | `SetArtifactSize` intentionally emits **no** event (size-only changes bypass the hook chain). | `internal/core/artifact_size.go:32-34`. |
| F4 | Item history is append-only **JSONL** events, not YAML frontmatter; appends are best-effort (warn-on-failure). | `internal/core/commits.go:36-53` (`events.NewEventWriter`, `AppendEvent`, `slog.Warn` on failure). |
| F5 | Production containment/write path is lexical only and offers no realpath/rollback atomicity. | `internal/core/workspace.go:271-290` `SafeResolve` (lexical). |
| F6 | No canonical XS-XL aggregation ruleset exists anywhere in the codebase. | grep of `internal/`, `schemas/`, header-def: no `size_source`/`size_ruleset`/size-rollup/histogram code. |

## Requirements Trace (contract sketch → spike; implementation gated)

| ID | Requirement | Disposition |
|---|---|---|
| SE1 | Level-specific optional `size` semantics for feature vs shipment (schema/contract) | Resolvable; sketch in spike `108.001-T`, implemented by gated `108.002-T` after proceed. |
| SE2 | Typed provenance inputs `size_source`/`size_ruleset_version` and defaulting across the core seam and both adapters | **Unresolved** (F1–F3): the seam has no provenance input path today. Named spike question. |
| SE3 | Estimate-history behavior covering **every** provenance-field change, with a defined event/write failure ordering | **Unresolved** (F3–F5): overlaps the partial-core-mutation-rollback open question. Named spike question. |
| SE4 | Explicit **structured** derived composition (not a synthetic categorical aggregate) exposed at render with CLI/MCP parity | Composition shape resolvable (see below); membership/dedup/ruleset ownership must be ratified by the spike. |
| SE5 | CLI/MCP parity documentation and verification | Gated `108.004-T` after the contract is ratified. |

## Structured Composition Contract Sketch (spike input, must be ratified)

No canonical XS-XL aggregation ruleset exists (F6), so summing categorical
values into a single synthetic size would invent arithmetic. The spike's
preferred direction is an **explicit structured composition** response, computed
on read and never persisted:

```
composition:
  histogram: { XS: n, S: n, M: n, L: n, XL: n }   # counts per authored size
  unsized: n                                        # members with no size
  members: [ canonical member IDs counted ]         # exact, de-duplicated
  ruleset_version: <string|null>                    # null until a ruleset is owned
```

**Membership and de-duplication (resolves the double-count finding):**

* **Feature composition** counts a feature's **direct children by `parent_id`**
  (tasks/reviews), each canonical ID once.
* **Shipment composition** expands the shipment manifest, then **de-duplicates the
  union of `{feature, its child tasks}`** so a manifest listing both a feature and
  its child tasks counts each canonical work item exactly once. The `members`
  array makes the counted set auditable.
* **Missing/legacy handling:** members without an authored `size` increment
  `unsized`, never a default bucket. Absent `size_source` reads as unknown/legacy
  and is never rewritten as `human`.

This shape is implementable and avoids both invented categorical arithmetic and
feature+child double counting. It remains a **sketch** until `108.001-T` ratifies
membership rules, the `unsized` contract, and ruleset-version ownership.

## Named Spike Questions (unresolved — do not manufacture readiness)

1. **Typed mutation/defaulting seam.** `SetArtifactSize` is `(id, size)`-only
   (F1–F2). What is the typed signature that carries `size_source`/
   `size_ruleset_version`, and what are the defaulting rules, without duplicating
   logic across the CLI and MCP adapters? Both adapters must gain source/version
   inputs or a defined default; a `size_ruleset_version`-only change must be
   representable.
2. **Provenance/history atomicity.** History is best-effort JSONL (F3–F4) and
   `SetArtifactSize` emits no event today. What is the write/append ordering that
   guarantees **exactly one** history event per persisted provenance change
   (including `size_ruleset_version`-only changes), and the failure contract
   (fail-closed vs. rollback)? This overlaps the formal-gate spike's open
   partial-core-mutation-rollback question and must be resolved coherently, not
   assumed.
3. **Ruleset-version ownership.** Who owns and versions any future aggregation
   ruleset, and until one exists, is `ruleset_version` simply `null` with the
   structured histogram as the only composition output?
4. **Provenance vs. history storage split.** Provenance (`size_source`,
   `size_ruleset_version`) belongs in YAML frontmatter; estimate **history**
   belongs in append-only JSONL item-event logs. These are separate durability
   surfaces and the plan must state each guarantee honestly (JSONL append is
   best-effort warn-on-failure today).

## Task Map (spike + gated follow-ups)

### `108.001-T` — Run size extension contract architecture spike (2h max, QUEUED)

Strictly time-boxed (2h, investigation only) contract/architecture spike. Produce
one coherent contract sketch across the base-contract/extension ownership model,
the structured composition contract above, and the four named spike questions,
plus a `proceed`/`pivot`/`defer` conclusion with a confidence rating. No code,
schema, or CLI changes. If atomicity or aggregation semantics remain unresolved
at the box, the conclusion is `defer`/`pivot` with the unresolved questions named
— not a manufactured implementation contract.

### `108.002-T` — Provenance persistence and estimate history (BLOCKED on 108.001-T)

Gated implementation. Only after a `proceed` conclusion: extend the size-mutation
seam with typed provenance inputs and defaulting (resolving question 1), persist
`size_source`/`size_ruleset_version`, and append an estimate-history **JSONL**
event covering **every** provenance-field change (including
`size_ruleset_version`-only), with the failure/ordering contract from question 2.
Derived values MUST NOT be written with `size_source: human`. One shared core seam
consumed by both CLI and MCP; no adapter duplication.

### `108.003-T` — Structured composition at render (BLOCKED on 108.001-T)

Gated implementation. Only after `proceed`: implement the structured composition
response (histogram + `unsized` + de-duplicated `members`) via a shared
render/query helper consumed by BOTH the CLI (`get`/`queue`) and MCP
(`get_item`/`get_shipment`) surfaces, using the ratified membership/dedup rules.
Computed on read; never persisted as authored.

### `108.004-T` — Documentation and CLI/MCP parity (BLOCKED on 108.002-T, 108.003-T)

Gated docs. Document the ratified size-extension contract, provenance fields,
structured composition, and render-only projection; verify CLI/MCP parity.

## Sequencing

`108.001-T` (spike) runs first and gates everything. Only a `proceed` conclusion
unblocks `108.002-T` and `108.003-T` (parallel); `108.004-T` depends on both. A
`defer`/`pivot` conclusion keeps the follow-ups blocked and re-enters staging.

## Non-Goals

* No coupling to the formal-gate architecture spike or the docline open
  extension-key guard staged in the same PR.
* No mandatory sizing; the field stays optional at every level.
* No persisting of derived composition as human-authored estimates.
* No invented categorical arithmetic and no manufactured implementation readiness
  while atomicity/aggregation questions are open.

## Constitution Check

- **I (Safety-First Go):** All work is Go; the core seam and render helper keep
  wrapped errors and must pass `go vet ./...` and `golangci-lint run` with zero
  warnings before commit. No `unsafe` usage.
- **II (Test-First, NON-NEGOTIABLE):** Every task is labelled `test-first`; a
  failing test precedes implementation for the schema contract, the core
  persistence/history seam, and the render helper.
- **III/IV (Workspace isolation / CLI containment):** Size writes go through the
  existing body-preserving `core.SetArtifactSize` path (`atomicfile` within the
  workspace root); no path traversal, no writes outside the cwd tree.
- **V (Structured Observability):** Each provenance change is intended to emit
  exactly one append-only **JSONL** estimate-history event; the exact
  write/append ordering and failure contract is a named spike question, not an
  assumed guarantee.
- **VI (Single Responsibility):** No new dependencies; the design reuses the
  existing size-mutation seam and schema/registry rather than adding libraries.
- **VII (Destructive Approval, NON-NEGOTIABLE):** No destructive operations —
  writes are body-preserving and history is append-only; the schema change is
  additive and non-migrating.
- **VIII (Safety Modes):** Investigate-first + freeze-scope posture — the spike is
  read-only investigation confined to the size subsystem, explicitly decoupled
  from the formal-gate spike and docline guard.
- **IX (Git-Friendly Persistence):** Provenance fields (`size_source`,
  `size_ruleset_version`) serialize to human-readable YAML frontmatter; estimate
  **history** is separate append-only **JSONL** item-event logs (`events`
  package). These are distinct durability surfaces — the earlier "history as
  Markdown/YAML" wording is corrected here — and the JSONL append is best-effort
  (warn-on-failure) today, which the atomicity spike question must resolve.
- **X (Context Efficiency):** Rollups are computed on read and exposed through
  existing query/render surfaces; no bulk duplication.
- **XI (Merge Commit Preservation):** Not applicable at plan stage; Stage does
  not merge or ship.

Task Granularity: each of the four tasks is one concern (schema / core seam /
render / docs), targets well under three files and five functions, and covers
both CLI and MCP through a single shared seam or helper rather than duplicated
adapter logic — preserving both the 2-Hour Rule and width isolation. No
constitutional violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: SPIKE (no implementation PASS)

**Formal review provenance:** RE-RUN on 2026-07-15 by the Stage agent following
the `plan-review` skill against these exact final bytes after the PR #241 refocus.
The prior implementation PASS is **withdrawn** and replaced with a spike-charter
review, because bounded investigation (F1–F6) showed the provenance/history
atomicity and XS-XL aggregation semantics are not yet an implementable contract.
Recording an implementation PASS now would manufacture readiness. Cross-model
invocation was unavailable; per the skill's fallback, all personas ran with the
caller's model (single-model multi-persona, disclosed).

**Reviewer personas executed (against the refocused spike charter):**

| Persona | Trigger | Result |
|---|---|---|
| Constitution Reviewer | always-on | The refocus resolves the honesty violation (Principle II / no manufactured readiness): unresolved atomicity/aggregation are named as spike questions, not asserted contracts. `108.002-T`/`108.003-T`/`108.004-T` are blocked behind the spike. No P0/P1 on the charter. |
| Go Reviewer | always-on | Corrected factual claims: `SetArtifactSize` is `(id, size)`-only and emits no event (F1–F3); history is JSONL best-effort (F4). The charter no longer asserts a seam capability that does not exist. No P0/P1. |
| Scope Boundary Auditor | always-on | Width isolation intact; spike is read-only investigation, decoupled from formal-gate (105-F/106-F) and docline (107-F). No P0/P1. |
| Learnings Researcher | always-on | The atomicity question is correctly linked to the formal-gate spike's open partial-core-mutation-rollback question rather than re-deciding it in isolation. No P0/P1. |
| Architecture Strategist | always-on | Structured composition (histogram + `unsized` + de-duplicated `members`) is a sound direction that avoids invented categorical arithmetic and feature+child double counting; correctly held as a sketch pending spike ratification. No P0/P1. |
| Agent-Native Parity Reviewer | triggered (sizing touches MCP `update_item`/`get_item`/`get_shipment` and CLI) | Parity is designed into the gated tasks via one shared seam/helper; verification deferred to `108.004-T` after ratification. No P0/P1. |
| Security Lens Reviewer | not triggered | — |

**Findings disposition:** The prior latent P1s raised by Copilot — undefined
aggregation/membership (3585440713, 3585440748), missing provenance-input seam
(3585440776, 3585440806), and the YAML-vs-JSONL history contradiction
(3585440828) — are addressed structurally: the composition contract is made
explicit, the seam gap and history split are named as spike questions, and the
Constitution Check IX contradiction is corrected. None are resolved into an
implementation contract here; that is the spike's job.

**Plan hardening:** N/A for a read-only spike charter. If the spike concludes
`proceed`, the resulting implementation plan (touching the core mutation seam and
event durability) MUST be evaluated for `plan-harden` before the gated tasks are
unblocked.

### Spike Exit Criteria (in lieu of an implementation PASS)

The `108.001-T` spike is complete when EITHER:

* a coherent contract sketch resolves the base-contract/extension ownership, the
  structured composition membership/dedup/`unsized`/ruleset-ownership rules, and
  the four named spike questions, with a `proceed` conclusion and confidence
  rating — after which a bounded implementation plan is authored and re-reviewed
  (with `plan-harden` evaluated) before `108.002-T`/`108.003-T` unblock; OR
* the 2h box is reached with atomicity or aggregation still unresolved, in which
  case the conclusion is `defer`/`pivot`, the unresolved questions are named, and
  the follow-ups stay blocked.

**Disposition:** SPIKE chartered. Shipment `096-S` now carries the size extension
contract spike (`108-F` + queued spike `108.001-T` + blocked `108.002-T`/
`108.003-T`/`108.004-T`). No implementation is authorized. This work remains
decoupled from the formal-gate governance work and the docline open extension-key
guard staged in the same PR.
