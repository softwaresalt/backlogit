---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Optional size estimation for feature and shipment artifacts plan'
source: docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md
doc_type: plan
description: 'Architecture spike charter (PR #241 refocus) of four sequenced <=2h research/decision tasks for optional backlogit-owned size extensions on the docline base contract: base/extension ownership, mutation/provenance/JSONL history durability and the unresolved size-write containment boundary, structured-composition membership/dedup/ruleset, and an explicit proceed/pivot exit decision — no implementation and no root-containment claim.'
docline:
    date: 2026-07-14T23:40:00Z
    refocused: 2026-07-15T00:00:00Z
    time_box: "8h"
    conclusion: "pending"
    confidence: "low"
    linked_stash_ids:
        - D7B1B33D
    review_state: spike-chartered
    gate: SPIKE
    review_provenance: "plan-review skill RE-RUN 2026-07-15 by Stage against final plan bytes after PR #241 refocus AND the correction repurposing the feature into four sequenced <=2h research/decision tasks (none blocked) and adding the size-write containment boundary as spike question 5; single-model multi-persona (cross-model unavailable per skill fallback); no implementation PASS recorded — spike exit criteria only"
---

# Size Extension Contract Architecture Spike — Feature and Shipment Sizing (D7B1B33D)

## Status: Refocused to a contract/architecture spike (PR #241)

This document was harvested as an implementation plan and previously recorded an
implementation-ready PASS. Bounded investigation for PR #241 found that
provenance/history atomicity, XS-XL aggregation semantics, and even the
workspace-containment boundary of the size-write path are **not yet resolvable as
an implementable contract** against current code. Per the Stage honesty rule
("do not manufacture implementation readiness"), it is refocused into a genuine
**architecture spike** composed **only** of four sequenced research/decision
tasks — `108.001-T` (base-contract/extension ownership + typed surface
inventory), `108.002-T` (mutation/provenance/JSONL history durability, failure
ordering, and containment), `108.003-T` (structured-composition membership/dedup/
missing/ruleset), and `108.004-T` (CLI/MCP parity, decision synthesis, and the
explicit `proceed`/`pivot` exit record). Each task is strictly **≤2h** and
independently verifiable; the spike is capped at **8h total**. This feature
contains **no implementation tasks** and asserts **no implementation readiness**.
The resolvable parts (base-contract/extension ownership and the structured
composition sketch) are recorded below as spike inputs; the unresolved parts —
including the size-write containment boundary — are named explicitly as spike
questions, not decisions. Any size implementation may be planned, harvested, and
reviewed **only after** `108.004-T` records an explicit `proceed` decision.

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
| F5 | The size-write path is **not proven root-contained**: `SetArtifactSize` resolves the path via `FindArtifactPath` (`artifactSearchDirs` + `filepath.WalkDir`) and writes via `atomicfile.WriteFileAtomic` **without ever calling `SafeResolve`**. `SafeResolve` itself is lexical-only (no realpath/rollback) and is not on this path. | `internal/core/artifact_size.go:35-81` (`FindArtifactPath` → `atomicfile.WriteFileAtomic`, no `SafeResolve`); `internal/core/artifacts.go:646-679` (`FindArtifactPath` WalkDir); `internal/core/workspace.go:271-290` `SafeResolve` (lexical, off-path). |
| F6 | No canonical XS-XL aggregation ruleset exists anywhere in the codebase. | grep of `internal/`, `schemas/`, header-def: no `size_source`/`size_ruleset`/size-rollup/histogram code. |

## Requirements Trace (stash requirement → spike investigation; no implementation authorized)

| ID | Requirement | Disposition |
|---|---|---|
| SE1 | Level-specific optional `size` semantics for feature vs shipment (schema/contract) | Ownership/surface researched in `108.001-T`; implementable contract deferred to a future plan authored only after a `proceed` decision. |
| SE2 | Typed provenance inputs `size_source`/`size_ruleset_version` and defaulting across the core seam and both adapters | **Unresolved** (F1–F3): the seam has no provenance input path today. Investigated in `108.002-T` as a named spike question. |
| SE3 | Estimate-history behavior covering **every** provenance-field change, with a defined event/write failure ordering | **Unresolved** (F3–F5): overlaps the partial-core-mutation-rollback and containment-boundary questions. Investigated in `108.002-T`. |
| SE4 | Explicit **structured** derived composition (not a synthetic categorical aggregate) exposed at render with CLI/MCP parity | Composition shape sketched (see below); membership/dedup/ruleset ownership investigated in `108.003-T`, not ratified here. |
| SE5 | CLI/MCP parity documentation and verification | Parity surfaces verified and the exit decision recorded in `108.004-T`; documentation follows a future implementation plan, not this spike. |

## Structured Composition Contract Sketch (spike input, must be ratified)

No canonical XS-XL aggregation ruleset exists (F6), so summing categorical
values into a single synthetic size would invent arithmetic. The spike's
preferred direction is an **explicit structured composition** response, computed
on read and never persisted:

```yaml
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
5. **Workspace-containment boundary of the size-write path (must be resolved
   before `proceed`).** `SetArtifactSize` reaches `atomicfile.WriteFileAtomic`
   via `FindArtifactPath` (`filepath.WalkDir` over the artifact search dirs) and
   **does not invoke `SafeResolve`** (F5). Size writes are therefore **not
   proven root-contained** by this seam. The spike MUST determine the correct
   containment seam (where/whether `SafeResolve` or a realpath boundary belongs
   on the write path) and record the evidence. A `proceed` outcome is **not
   permitted** until this containment boundary is resolved. This is a named open
   question and evidence item — not a decision, and not something implemented in
   this Stage turn.

## Task Map (four sequenced research/decision tasks, all QUEUED)

This spike contains **no implementation tasks**. All four tasks are strictly
≤2h, investigation/decision only, and independently verifiable (8h cap total).

### `108.001-T` — Base-contract/extension ownership + typed surface inventory (2h max, QUEUED)

Research/decision. Inventory the typed size surface (task-only `size` enum; no
feature/shipment `size`; no `size_source`/`size_ruleset_version`/history anywhere;
`SetArtifactSize(ctx, ws, id, size)` `(id, size)`-only seam; CLI `update --size`
and MCP `update_item size` route only `size`). Record the base-contract/extension
ownership boundary: docline owns only the base ingestion contract; the size keys
are optional backlogit-owned extensions docline never calculates/defaults/
aggregates/validates/emits. Deliverable: written ownership boundary + surface
inventory feeding `108.002-T`/`108.003-T`. No code, schema, or CLI changes.

### `108.002-T` — Mutation/provenance/JSONL history durability, ordering, and containment (2h max, QUEUED)

Research/decision (depends on `108.001-T`). Investigate the durability questions a
future typed provenance seam must answer: the missing provenance-input path and
absent mutation event (F1–F3), the best-effort JSONL history surface (F4), the
write/append ordering and failure contract (fail-closed vs. rollback) needed for
exactly one history event per persisted provenance change, and the **unresolved
workspace-containment boundary** (F5 — size writes reach `atomicfile` without
`SafeResolve`). Deliverable: durability/ordering/containment findings and named
open questions as evidence, not decisions. No implementation.

### `108.003-T` — Structured-composition membership/dedup/missing/ruleset (2h max, QUEUED)

Research/decision (depends on `108.001-T`). Investigate the candidate structured
composition (histogram + `unsized` + de-duplicated `members`), feature membership
(direct children by `parent_id`) vs. shipment manifest expansion with
`{feature, its child tasks}` dedup, missing/legacy handling (`unsized`; absent
`size_source` reads unknown/legacy), and the ruleset-version ownership question
(`null` until owned). Deliverable: membership/dedup/missing/ruleset findings as an
investigation feeding `108.004-T`, never persisted as authored. No implementation.

### `108.004-T` — CLI/MCP parity, decision synthesis, and explicit proceed/pivot exit record (2h max, QUEUED)

Research/decision (depends on `108.002-T`, `108.003-T`). Verify the CLI/MCP
response-parity surfaces (CLI `get`/`queue`, MCP `get_item`/`get_shipment`) a
future projection must populate identically, synthesize the three prior
investigations, and record an **explicit `proceed`/`pivot`/`defer` exit decision**
with confidence in this plan's `docline.conclusion`. The containment-boundary
question (`108.002-T`) MUST be resolved before any `proceed`. A `proceed` decision
is the ONLY authorization for a later, separately planned, harvested, and reviewed
implementation. Deliverable: the recorded decision + exit-criteria confirmation.
No implementation.

## Sequencing

`108.001-T` runs first and grounds the ownership/surface inventory. `108.002-T`
and `108.003-T` then investigate durability/containment and composition in
parallel (both depend on `108.001-T`); `108.004-T` synthesizes both and records
the exit decision. All four are queued research tasks — none carries a `blocked`
status or implementation readiness. A future implementation plan is authored only
after `108.004-T` records a `proceed` decision.

## Non-Goals

* No coupling to the formal-gate architecture spike or the docline open
  extension-key guard staged in the same PR.
* No mandatory sizing; the field stays optional at every level.
* No persisting of derived composition as human-authored estimates.
* No invented categorical arithmetic and no manufactured implementation readiness
  while atomicity/aggregation questions are open.

## Constitution Check

This is a read-only research spike; the checks below describe the **spike's**
conduct, not an authorized implementation.

- **I (Safety-First Go):** No production Go is written by this spike. Any future
  implementation (authored only after a `proceed` decision) must keep wrapped
  errors and pass `go vet ./...` and `golangci-lint run` with zero warnings; no
  `unsafe` usage. Not exercised here.
- **II (Test-First, NON-NEGOTIABLE):** No implementation and therefore no
  test-first obligation in this spike. Each of the four tasks is
  investigation/decision only. Test-first applies to the future implementation
  plan, not to these research tasks.
- **III/IV (Workspace isolation / CLI containment):** The size-write path is
  **not proven root-contained** — `SetArtifactSize` reaches
  `atomicfile.WriteFileAtomic` via `FindArtifactPath`/`filepath.WalkDir` and does
  **not** invoke `SafeResolve` (F5). This spike therefore makes **no** claim that
  size writes are root-contained; instead it records the containment boundary as
  named spike question 5, which MUST be resolved before any `proceed` outcome.
- **V (Structured Observability):** The desired guarantee — exactly one
  append-only JSONL estimate-history event per persisted provenance change — is a
  named spike question (`108.002-T`), not an assumed guarantee. `SetArtifactSize`
  emits no event today.
- **VI (Single Responsibility):** The spike adds no dependencies and writes no
  code; it inventories the existing size seam and schema/registry only.
- **VII (Destructive Approval, NON-NEGOTIABLE):** No destructive operations —
  the spike is read-only investigation plus this plan's recorded findings.
- **VIII (Safety Modes):** Investigate-first + freeze-scope posture — read-only
  investigation confined to the size subsystem, explicitly decoupled from the
  formal-gate spike and docline guard.
- **IX (Git-Friendly Persistence):** Provenance fields (`size_source`,
  `size_ruleset_version`) would serialize to human-readable YAML frontmatter;
  estimate **history** is separate append-only **JSONL** item-event logs
  (`events` package). These are distinct durability surfaces — the earlier
  "history as Markdown/YAML" wording is corrected here — and the JSONL append is
  best-effort (warn-on-failure) today, which the atomicity spike question must
  resolve before implementation.
- **X (Context Efficiency):** Any future rollups are computed on read through
  existing query/render surfaces; no bulk duplication. Not implemented here.
- **XI (Merge Commit Preservation):** Not applicable at spike stage; Stage does
  not merge or ship.

Task Granularity: each of the four tasks is a single **research/decision**
concern (ownership + surface inventory / durability + ordering + containment /
composition membership + dedup + ruleset / parity + decision synthesis) and is
strictly ≤2h and independently verifiable, capped at 8h total. These are **not**
implementation tasks, so the plan makes **no** claim that they fit "under three
files and five functions" — that granularity heuristic applies to a future
implementation plan, not to investigation. Width isolation is preserved (the
spike is decoupled from CLI, schema, and template work). No constitutional
violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: SPIKE (no implementation PASS)

**Formal review provenance:** RE-RUN on 2026-07-15 by the Stage agent following
the `plan-review` skill against these exact final bytes after the PR #241 refocus
**and** the subsequent correction that repurposes the entire feature into four
sequenced ≤2h research/decision tasks (no gated implementation tasks) and adds the
size-write containment boundary (F5) as an explicit spike question. The prior
implementation PASS is **withdrawn** and replaced with a **spike-charter review**
with explicit exit criteria — not an implementation PASS — because bounded
investigation (F1–F6) showed the provenance/history atomicity, XS-XL aggregation
semantics, and workspace-containment boundary are not yet an implementable
contract. Recording an implementation PASS now would manufacture readiness.
Cross-model invocation was unavailable; per the skill's fallback, all personas
ran with the caller's model (single-model multi-persona, disclosed).

**Reviewer personas executed (against the refocused spike charter):**

| Persona | Trigger | Result |
|---|---|---|
| Constitution Reviewer | always-on | The refocus resolves the honesty violation (Principle II / no manufactured readiness): unresolved atomicity/aggregation/containment are named as spike questions, not asserted contracts. All four tasks are **queued research/decision** tasks (none `blocked`, none implementation), so the shipment can complete under recursive release-scope semantics. The III/IV containment overreach is corrected (no root-containment claim). No P0/P1 on the charter. |
| Go Reviewer | always-on | Corrected factual claims: `SetArtifactSize` is `(id, size)`-only and emits no event (F1–F3); history is JSONL best-effort (F4); the size-write path reaches `atomicfile` via `FindArtifactPath`/`WalkDir` and never calls `SafeResolve` (F5). The charter no longer asserts seam capabilities or containment guarantees that do not exist. No P0/P1. |
| Scope Boundary Auditor | always-on | Width isolation intact; the spike is read-only investigation, decoupled from formal-gate (105-F/106-F) and docline (107-F). The charter no longer claims implementation granularity ("three files/five functions") for research tasks. No P0/P1. |
| Learnings Researcher | always-on | The atomicity question is correctly linked to the formal-gate spike's open partial-core-mutation-rollback question rather than re-deciding it in isolation. No P0/P1. |
| Architecture Strategist | always-on | Structured composition (histogram + `unsized` + de-duplicated `members`) is a sound direction that avoids invented categorical arithmetic and feature+child double counting; correctly held as an investigation input pending the decision task. No P0/P1. |
| Agent-Native Parity Reviewer | triggered (sizing touches MCP `update_item`/`get_item`/`get_shipment` and CLI) | Parity is verified as an investigation in `108.004-T`; no implementation parity is claimed. No P0/P1. |
| Security Lens Reviewer | triggered (F5 raises a workspace-containment/trust-boundary question) | The unresolved containment boundary is recorded honestly as spike question 5 and is a hard precondition for any `proceed`. No claim of proven containment remains. No P0/P1. |

**Findings disposition:** The prior latent P1s raised by Copilot — undefined
aggregation/membership (3585440713, 3585440748), missing provenance-input seam
(3585440776, 3585440806), and the YAML-vs-JSONL history contradiction
(3585440828) — plus the current review's containment-boundary finding (F5, size
writes not proven root-contained) are addressed structurally: the composition
sketch is an investigation input, the seam gap, history split, and containment
boundary are named spike questions, and the Constitution Check III/IV and IX
claims are corrected. None are resolved into an implementation contract here;
that is the spike's job.

**Plan hardening:** N/A for a read-only spike charter. If `108.004-T` records a
`proceed` decision, the resulting implementation plan (touching the core mutation
seam, event durability, and the containment seam) MUST be evaluated for
`plan-harden` before any implementation is harvested.

### Spike Exit Criteria (in lieu of an implementation PASS)

Each research/decision task has an explicit, independently verifiable exit:

* **`108.001-T`** — exits when the base-contract/extension ownership boundary and
  the typed size-surface inventory are recorded.
* **`108.002-T`** — exits when the mutation/provenance path, JSONL history
  durability/ordering options, failure contract, and the **containment boundary**
  (F5) are documented as findings and open questions.
* **`108.003-T`** — exits when feature/shipment structured-composition membership,
  dedup, missing/legacy handling, and ruleset-version ownership are documented as
  findings.
* **`108.004-T`** — exits when CLI/MCP parity surfaces are verified and an
  explicit `proceed`/`pivot`/`defer` decision with confidence is recorded in
  `docline.conclusion`. A `proceed` outcome is **only** permitted if the
  containment boundary from `108.002-T` is resolved. A `proceed` decision is the
  sole authorization for a later, separately planned and re-reviewed (with
  `plan-harden` evaluated) implementation; `pivot`/`defer` keeps size
  implementation out of scope and re-enters staging.

**Disposition:** SPIKE chartered. Shipment `096-S` now carries the size extension
contract architecture spike (`108-F` + four **queued research/decision** tasks
`108.001-T`/`108.002-T`/`108.003-T`/`108.004-T`, none `blocked`). No
implementation is authorized. This work remains decoupled from the formal-gate
governance work and the docline open extension-key guard staged in the same PR.
