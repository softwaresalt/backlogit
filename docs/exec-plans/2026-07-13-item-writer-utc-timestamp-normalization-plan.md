---
chunk_strategy: h1-h2-h3
description: Normalize backlogit item-artifact created_at/updated_at emission to canonical UTC (Z) across EVERY item-artifact timestamp write site in internal/core (create, update, link, gate, queue, section-serializer, shipment, lifecycle, and migration paths), split into two ≤2h tasks.
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

## Complete implementation inventory (audited 2026-07-13)

The initial draft under-scoped this to `artifacts.go` alone. A full audit of
`internal/core` (revised in response to PR #234 Copilot review, threads
`3575108864` / `3575121017`) enumerates **every** item-artifact timestamp write
site. All are `time.Now()` (local); each must become UTC via a shared helper.

**Cluster 1 — core item writers (Task `103.001-T`):**

| File | Line(s) | Path |
|---|---|---|
| `internal/core/artifacts.go` | 225 (`now := time.Now()` → CreatedAt+UpdatedAt) | create |
| `internal/core/artifacts.go` | 552 | harness_status update restamp |
| `internal/core/artifacts.go` | 819 | AddLink restamp (`source.UpdatedAt`) |
| `internal/core/artifacts.go` | 856 | RemoveLink restamp (`source.UpdatedAt`) |
| `internal/core/artifact_references.go` | 105 | adopt / ID-rewrite deep-copy restamp |
| `internal/core/gate_transition.go` | 341 | `writeStatusDirect` restamp |
| `internal/core/queue.go` | 277 (`stamp := time.Now()` → assigned 293), 395 | queue mutation restamps |
| `internal/core/templates/service.go` | 124-127 (`now := time.Now()`; `fm["updated_at"] = now`) | **direct frontmatter serializer** (section-update path — bypasses the central writer) |

**Cluster 2 — shipment, lifecycle & migration writers (Task `103.002-T`):**

| File | Line(s) | Path |
|---|---|---|
| `internal/core/shipment.go` | 144, 200, 248 (`shipment.UpdatedAt`), 255 (`item.UpdatedAt`) | shipment create/add/mutate restamps |
| `internal/core/shipment_lifecycle.go` | 352, 491, 530 (`parent.UpdatedAt`), 555, 644 | shipment lifecycle transition restamps |
| `internal/core/migrate_links.go` | 167 | link-migration restamp |

**Explicitly OUT of scope (separate concern, not item frontmatter):** JSONL
event-log timestamps, e.g. `events.Event.Timestamp: time.Now()` at
`internal/core/shipment.go:299` (and peers), which are formatted by
`internal/core/logs.go` via RFC3339Nano into `.backlogit/logs/*.jsonl`. Those
are the event-history stream, not item artifacts, and are deferred as a distinct
follow-up if raised.

## Non-goals

* No backfill/rewrite of existing on-disk artifacts. Historical local-offset
  timestamps remain valid; the DB read path (`internal/core/queries.go:75-86`)
  already parses `RFC3339Nano` and `RFC3339` and is zone-agnostic.
* No change to JSONL event-log emission (`logs.go`) — see out-of-scope note.
* No schema, CLI-distribution, or `.tmpl` template-family changes.

## Blast-radius assessment

* **Touches:** `internal/core` production Go code only, now spanning 8 files
  (~19 assignment sites). `internal/core/templates/service.go` is the template
  **service** (section-serialization Go code), **not** a `.tmpl` template
  family.
* **Does NOT touch:** JSON schemas, CLI packaging/distribution, or any `.tmpl`
  template family. The elevated-blast-radius `plan-harden` triggers (schemas,
  distribution, multiple template families) are therefore absent, so
  `plan-harden` is **not** invoked. Mitigation for the broader-but-mechanical
  surface: a single shared `nowUTC()` helper centralizes the convention so
  every current and future writer inherits it. The change is additive
  normalization; mixed old/new zones round-trip cleanly through the zone-agnostic
  read path.

## Approach (test-first / TDD)

1. **Introduce a shared helper.** Add `nowUTC() time.Time { return
   time.Now().UTC() }` (or reuse the stash convention) in `internal/core`, so
   the UTC contract is enforced centrally.

2. **Write failing tests FIRST (exact `Z` assertion).** For each cluster, add
   regression tests that call the writer, read back the emitted Markdown
   **frontmatter string**, and assert the raw `created_at`/`updated_at` values
   **end with exactly `Z`** (canonical UTC) — not merely a zero numeric offset.
   Concretely: assert `strings.HasSuffix(value, "Z")` (equivalently, the
   serialized value equals the value formatted with `.UTC()`), and assert it
   does NOT contain a `+HH:MM`/`-HH:MM` offset. Confirm these tests **fail**
   against the current local-`time.Now()` emission (red phase). Parse-side
   tests may *separately* keep accepting historical `+/-HH:MM` offsets to prove
   backward compatibility — the tolerance is on read, the strict `Z` assertion
   is on write.

3. **Make the minimal change.** Replace every inventoried `time.Now()` write
   site (both clusters) with the shared `nowUTC()` helper.

4. **Green + regression.** Re-run the new tests plus the existing
   `internal/core` suite. Confirm round-trip parse still works; the `-07:00`
   fixtures in `artifact_size_test.go` are parser *inputs*, not emission
   assertions, so they remain valid.

5. **Full package gate.** `go test ./internal/core/...` (and `./internal/models/...`
   if touched) stays green.

## Task decomposition (2-hour rule / width isolation)

Because the audited surface is ~19 sites across 8 files with comprehensive
per-path TDD, it exceeds a single ~2-hour task and is split:

* **`103.001-T` — Core item writers + shared helper.** Introduce `nowUTC()`;
  normalize `artifacts.go` (create + update/link sites), `artifact_references.go`,
  `gate_transition.go`, `queue.go`, and the `templates/service.go` direct
  serializer. Exact-`Z` tests for create, harness-status update, link add/remove,
  adopt/reference-rewrite, gate status-direct, queue update, and section-update.
* **`103.002-T` — Shipment, lifecycle & migration writers.** Depends on
  `103.001-T` (consumes the shared `nowUTC()` helper). Normalize `shipment.go`,
  `shipment_lifecycle.go`, and `migrate_links.go`. Exact-`Z` tests for shipment
  create/add/mutate, lifecycle transitions, and link migration.

Both tasks belong to shipment `092-S`.

## Acceptance criteria

* A shared UTC-now helper exists and is used by **all** inventoried item-artifact
  write sites (both clusters); no item writer calls bare `time.Now()`.
* New item artifacts (features, tasks, subtasks, shipments) emit
  `created_at`/`updated_at` ending in exactly `Z`.
* Per-path regression tests assert exact-`Z` emission and **fail** on the
  pre-change code (red phase demonstrated); parse-side tests continue to accept
  historical offsets (backward compatibility proven).
* `go test ./internal/core/...` is green.

## Estimated effort

Two focused tasks, each within the 2-hour rule; single skill domain (Go code)
with no doc/template-family work bundled in.

## Plan review

**Provenance (honest): inline single-agent self-assessment + incorporation of
external automated review (GitHub Copilot code review on PR #234).** This is
**NOT** a formal multi-persona `plan-review` skill run. The formal
`plan-review` skill (`.github/skills/plan-review/SKILL.md`; required gate in the
AGENTS.md Release Unit Pipeline) **spawns independent reviewer persona
subagents**; the Stage agent in this environment cannot dispatch persona
subagents, so the formal gate could not be executed. It is not simulated or
claimed.

External review findings incorporated into this revision:

* **[B — high] Incomplete writer audit** (threads `3575108864`, `3575121017`):
  resolved by the Complete Implementation Inventory above (all 8 files / ~19
  sites enumerated and verified in-tree) and the two-task split.
* **[D — TDD precision] Assert canonical `Z`** (thread `3575108894`): resolved
  by the exact-`Z` write-side assertion in Approach step 2, with parse-side
  offset tolerance kept separate.

Self-assessment: scope/width isolation PASS (Go-only, no docs/schema/template
family); 2-hour rule PASS (split into two ≤2h tasks); test-first PASS (red phase
now demonstrable via exact-`Z` assertions); backward compatibility PASS
(zone-agnostic read path). Residual: none blocking.

**Recommended pre-build follow-up:** run the formal multi-persona `plan-review`
skill against this revised plan before Ship builds `092-S`, and append its real
result here.
