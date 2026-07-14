---
chunk_strategy: h1-h2-h3
description: Normalize backlogit item-artifact created_at/updated_at emission to canonical UTC (Z) across EVERY item-artifact timestamp write site in internal/core via a shared exported models.NowUTC() helper, decomposed into five heuristic-compliant (<3 files each) tasks.
doc_type: plan
docline:
    stash_ref: 9B38A09E
    conclusion: proceed
    confidence: high
schema_version: "1.0"
source: docs/exec-plans/2026-07-13-item-writer-utc-timestamp-normalization-plan.md
title: Normalize item-writer created_at/updated_at emission to UTC
---

## Normalize item-writer created_at/updated_at emission to UTC

## Context

Stash entry `9B38A09E` (task, medium): *"Normalize backlogit CLI
created_at/updated_at timestamp emission to UTC (Z) across item artifact
writers."* Surfaced by the PR #230 Copilot review.

backlogit writes two timestamp styles into `.backlogit/`:

* **Stash entries** are UTC-normalized. `internal/core/stash.go` uses
  `time.Now().UTC()` at every write site, so `stash.jsonl` timestamps end in
  `Z`.
* **Item artifacts** (features, tasks, subtasks, shipments) are written with a
  local offset. The writers stamp `time.Now()` (local zone) into
  `CreatedAt`/`UpdatedAt`; the YAML serializer (`internal/models/frontmatter.go`
  `SerializeFrontmatter` → `yaml.Marshal`) renders the `time.Time` in RFC3339Nano
  **preserving its zone**, producing offsets such as `-07:00` (confirmed on
  disk, e.g. `created_at: 2026-07-13T16:46:33.0672771-07:00`).

The mixed styles are cosmetically inconsistent and can surprise lexical sorting
and cross-artifact auditing that assumes a single canonical zone.

## Goal

**Every** item-artifact writer emits `created_at`/`updated_at` in canonical UTC
(trailing `Z`), matching the stash convention, with no change to timestamp
precision (RFC3339Nano) and full backward compatibility for already-written
artifacts.

## Shared helper (package-boundary aware)

Introduce an **exported** `func NowUTC() time.Time { return time.Now().UTC() }`
in **`internal/models`**. Rationale: the write sites live in two packages —
`internal/core` and the separate `internal/core/templates` package
(`templates/service.go` is `package templates`). `internal/core/templates`
imports `internal/models` (for `SerializeFrontmatter`), and `internal/core`
imports `internal/models` too; `internal/models` imports **neither**, so placing
the helper there is importable by both writer packages with **no import cycle**.
An unexported `core.nowUTC` would NOT be callable from the `templates` package
(PR #234 review, thread `3575231557`), so the helper is exported from `models`.

## Complete implementation inventory → task mapping (audited 2026-07-13)

Full audit of every item-artifact timestamp write site (revised in response to
PR #234 Copilot review). Each is `time.Now()` (local) and becomes
`models.NowUTC()`.

| Task | File(s) (<3 per task) | Site(s) | Path |
|---|---|---|---|
| `103.001-T` | `internal/models/*.go` (add `NowUTC`); `internal/core/artifacts.go` | 225 (create), 552, 819, 856 | helper + core create/update/link restamps |
| `103.002-T` | `internal/core/queue.go`; `internal/core/gate_transition.go` | queue 277/293, 395; gate 341 | queue mutation + gate status-direct restamps |
| `103.003-T` | `internal/core/artifact_references.go`; `internal/core/templates/service.go` | refs 105; service 124-127 | adopt/ID-rewrite restamp + direct section serializer |
| `103.004-T` | `internal/core/shipment.go`; `internal/core/migrate_links.go` | shipment 144/200/248/255; migrate 167 | shipment create/add/mutate + link migration |
| `103.005-T` | `internal/core/shipment_lifecycle.go` | 352/491/530/555/644 | shipment lifecycle transition restamps |

**Explicitly OUT of scope (separate concern, not item frontmatter):** JSONL
event-log timestamps, e.g. `events.Event.Timestamp: time.Now()` at
`internal/core/shipment.go:299` (and peers), formatted by `internal/core/logs.go`
via RFC3339Nano into `.backlogit/logs/*.jsonl`. Those are the event-history
stream, not item artifacts, and are deferred as a distinct follow-up if raised.

## Non-goals

* No backfill/rewrite of existing on-disk artifacts. Historical local-offset
  timestamps remain valid; the DB read path (**`internal/db/queries.go:75-86`**)
  already parses `RFC3339Nano` with an `RFC3339` fallback and is zone-agnostic.
* No change to JSONL event-log emission (`logs.go`) — see out-of-scope note.
* No schema, CLI-distribution, or `.tmpl` template-family changes.

## Blast-radius assessment

* **Touches:** production Go code in `internal/models` (one small helper),
  `internal/core`, and `internal/core/templates` (~19 assignment sites).
  `internal/core/templates/service.go` is the template **service**
  (section-serialization Go code), **not** a `.tmpl` template family.
* **Does NOT touch:** JSON schemas, CLI packaging/distribution, or any `.tmpl`
  template family. The elevated-blast-radius `plan-harden` triggers (schemas,
  distribution, multiple template families) are absent, so `plan-harden` is
  **not** invoked. Mitigation for the broader surface: the single exported
  `models.NowUTC()` helper centralizes the convention so every current and
  future writer inherits it. The change is additive normalization; mixed old/new
  zones round-trip cleanly through the zone-agnostic read path
  (`internal/db/queries.go:75-86`).

## Approach (test-first / TDD)

1. **`103.001-T` introduces the shared helper.** Add exported
   `models.NowUTC()`; then swap the `artifacts.go` write sites.
2. **Write failing tests FIRST (exact `Z` assertion).** For each task, add
   regression tests that call the writer, read back the emitted Markdown
   **frontmatter string**, and assert the raw `created_at`/`updated_at` values
   **end with exactly `Z`** (canonical UTC): assert `strings.HasSuffix(value,
   "Z")` (equivalently, equals the value formatted with `.UTC()`) and assert it
   contains no `+HH:MM`/`-HH:MM` offset. Confirm the tests **fail** against the
   current local-`time.Now()` emission (red phase). Parse-side tests may
   *separately* keep accepting historical `+/-HH:MM` offsets to prove backward
   compatibility — tolerance is on read; the strict `Z` assertion is on write.
3. **Make the minimal change.** Replace each inventoried `time.Now()` write site
   with `models.NowUTC()`.
4. **Green + regression.** Re-run new tests plus the existing `internal/core`
   suite. The `-07:00` fixtures in `artifact_size_test.go` are parser *inputs*,
   not emission assertions, so they remain valid.
5. **Full package gate.** `go test ./internal/...` stays green.

## Task decomposition (2-hour rule — constitution §Task Granularity)

The constitution heuristic caps a task at **<3 files, <5 functions, <4 test
scenarios**. The audited surface (9 files including the helper, ~19 sites) is
therefore split into **five** tasks, each modifying **≤2 files** with **≤3 test
scenarios** (see the inventory table). The per-file site counts are mechanical
one-line `models.NowUTC()` substitutions (not independent logic changes), so
each task stays well within ~2 hours of human-equivalent effort.

* **`103.001-T`** — shared `models.NowUTC()` helper + `artifacts.go` writers.
* **`103.002-T`** — `queue.go` + `gate_transition.go` writers. *(dep: 103.001-T)*
* **`103.003-T`** — `artifact_references.go` + `templates/service.go` serializer. *(dep: 103.001-T)*
* **`103.004-T`** — `shipment.go` + `migrate_links.go` writers. *(dep: 103.001-T)*
* **`103.005-T`** — `shipment_lifecycle.go` writers. *(dep: 103.001-T)*

`103.002-005-T` depend on `103.001-T` because they consume the shared helper.
All five belong to shipment `092-S`. Width isolation holds: every task is a
single skill domain (Go code); no docs/schema/template-family work is bundled.

## Acceptance criteria

* An exported `models.NowUTC()` helper exists and is used by **all** inventoried
  item-artifact write sites across all five tasks; no item writer calls bare
  `time.Now()`.
* New item artifacts (features, tasks, subtasks, shipments) emit
  `created_at`/`updated_at` ending in exactly `Z`.
* Per-task regression tests assert exact-`Z` emission and **fail** on the
  pre-change code (red phase demonstrated); parse-side tests continue to accept
  historical offsets (backward compatibility proven).
* `go test ./internal/...` is green.

## Estimated effort

Five focused tasks, each within the constitution's per-task heuristic (<3 files,
<4 test scenarios); single skill domain (Go code) with no doc/template-family
work bundled in.

## Plan review

**Provenance (honest): inline single-agent self-assessment + incorporation of
external automated review (GitHub Copilot code review on PR #234, two passes).**
This is **NOT** a formal multi-persona `plan-review` skill run. The formal
`plan-review` skill (`.github/skills/plan-review/SKILL.md`; required gate in the
AGENTS.md Release Unit Pipeline) **spawns independent reviewer persona
subagents**; the Stage agent in this environment cannot dispatch persona
subagents, so the formal gate could not be executed. It is not simulated or
claimed.

External review findings incorporated:

* **[B — pass 1] Incomplete writer audit** (threads `3575108864`,
  `3575121017`): resolved by the Complete Implementation Inventory (all sites
  enumerated and verified in-tree).
* **[D — pass 1] Assert canonical `Z`** (thread `3575108894`): resolved by the
  exact-`Z` write-side assertion (Approach step 2), parse-side tolerance kept
  separate.
* **[pass 2 — helper package boundary]** (thread `3575231557`): resolved — the
  helper is an **exported `models.NowUTC()`** callable from both `internal/core`
  and the separate `internal/core/templates` package (no import cycle).
* **[pass 2 — 2-hour heuristic]** (thread `3575231580`): resolved — split into
  five tasks, each ≤2 files / ≤3 test scenarios per constitution §Task
  Granularity.
* **[pass 2 — wrong path]** (thread `3575231596`): resolved — corrected
  `internal/core/queries.go` → **`internal/db/queries.go:75-86`** (Non-goals +
  blast-radius).

Self-assessment: scope/width isolation PASS (Go-only); 2-hour rule PASS (five
≤2-file tasks); test-first PASS (exact-`Z` red phase); backward compatibility
PASS (zone-agnostic read path). Residual: none blocking.

**Recommended pre-build follow-up:** run the formal multi-persona `plan-review`
skill against this revised plan before Ship builds `092-S`, and append its real
result here.
