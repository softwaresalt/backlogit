---
chunk_strategy: h1-h2-h3
description: Normalize backlogit item-artifact created_at/updated_at emission to canonical UTC (Z) across EVERY item-artifact timestamp write site in internal/models, internal/core, internal/core/templates and internal/cli via a shared exported models.NowUTC() helper, decomposed into eleven heuristic-compliant tasks (each one production file plus its colocated test, and fewer than 5 functions per task).
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

Introduce an **exported, doc-commented** helper in **`internal/models`**, written
born-compliant with Go's exported-identifier documentation convention (and
`golangci-lint`'s `revive`/`golint` exported-comment rule):

```go
// NowUTC returns the current wall-clock time normalized to UTC, so that every
// item-artifact writer serializes created_at/updated_at with a canonical
// trailing "Z" instead of a machine-local offset.
func NowUTC() time.Time { return time.Now().UTC() }
```

Rationale: the write sites live in two packages —
`internal/core` and the separate `internal/core/templates` package
(`templates/service.go` is `package templates`). `internal/core/templates`
imports `internal/models` (for `SerializeFrontmatter`), and `internal/core`
imports `internal/models` too; `internal/models` imports **neither**, so placing
the helper there is importable by both writer packages with **no import cycle**.
An unexported `core.nowUTC` would NOT be callable from the `templates` package
(PR #234 review, thread `3575231557`), so the helper is exported from `models`.

## Complete implementation inventory → task mapping (audited 2026-07-13, revised for PR #234 cycle-3)

Exhaustive sweep of every item-artifact timestamp write site, produced by
grepping `internal/core/**`, `internal/cli/**`, `internal/db/**`,
`internal/models/**`, `internal/telemetry/**`, `internal/events/**` for
`time.Now()`, `created_at`/`updated_at`, `.UpdatedAt =`/`.CreatedAt =`, and every
`models.SerializeFrontmatter` caller. Each **in-scope** site currently stamps
`time.Now()` (local zone) into an item artifact's frontmatter and becomes
`models.NowUTC()`. Sites are grouped one production file per task so each task
modifies exactly **two files** (the production file + its colocated
`*_test.go`), strictly fewer than the constitution's 3-file heuristic.

| Task | Production file | Site line(s) | Write path |
|---|---|---|---|
| `103.001-T` | `internal/models/frontmatter.go` | add `NowUTC`; 42, 43 | shared helper + defensive `ArtifactFromFrontmatter` defaults (used when a parsed artifact omits timestamps) |
| `103.002-T` | `internal/core/artifacts.go` | 225 (create `now`), 552, 819, 856 | create + update + adopt/section restamps (serialized 328, 726) |
| `103.003-T` | `internal/core/queue.go` | 277 (`stamp`)→293, 395 | queue move/update restamps |
| `103.004-T` | `internal/core/shipment.go` | 144, 200, 248, 255 | shipment create/add/mutate item restamps (299 = event log, OUT) |
| `103.005-T` | `internal/core/shipment_lifecycle.go` | 352, 491, 530 | `attachCommitToItems` / `setArtifactStatus` / `cascadePersistedParentStatuses` (3 funcs) |
| `103.006-T` | `internal/core/gate_transition.go` | 341 | gate status-direct restamp |
| `103.007-T` | `internal/core/artifact_references.go` | 105 | adopt/ID-rewrite reference restamp |
| `103.008-T` | `internal/core/migrate_links.go` | 167 | link-migration restamp |
| `103.009-T` | `internal/core/templates/service.go` | 124 (`now`)→126, serialized 129 | direct section-update frontmatter serializer (`package templates`) |
| `103.010-T` | `internal/cli/update.go` | 240 (`now`)→241, 259, serialized 243 | CLI `update --section` direct frontmatter serializer |
| `103.011-T` | `internal/core/shipment_lifecycle.go` | 555, 644 | `clearParentID` / `AdoptItem` (2 funcs) |

Eleven tasks over ten production files (`shipment_lifecycle.go` is split across
`103.005-T`/`103.011-T` by **function** — its five writer sites span five
distinct functions, which breaches the `<5 functions` heuristic for a single
task). `103.002`–`103.011-T` depend on `103.001-T` (they consume the shared
`models.NowUTC()` helper).

**Per-task function-count audit (`<5 functions` heuristic, verified):**
`103.002-T` artifacts.go = 4 funcs (`CreateArtifact`, `updateArtifactUngated`,
`AddArtifactLink`, `RemoveArtifactLink`); `103.003-T` queue.go = 2
(`MoveInQueue`, `BulkUpdateStatus`); `103.004-T` shipment.go = 3
(`moveShipmentStatusWithTopLevel`, `AddItemToShipment`, `ReturnBlockedItem`);
`103.005-T` = 3; `103.011-T` = 2; all others = 1 function. Every task is `<5`
functions and `<3` files.

**Explicitly OUT of scope (verified, not item-artifact frontmatter):**

* **JSONL event-log / history timestamps** (`events.Event.Timestamp`,
  `ArchiveRecord.ArchivedAt`, commit/gate-evidence events): `shipment.go:299`,
  `archive.go:277,315`, `commits.go:40`, `gate_evidence.go:43`,
  `mcp/tools.go:573`, `db/logs.go:39,60` — the event-history stream formatted by
  `internal/core/logs.go`, not item frontmatter.
* **Already-UTC writers** (stash, checkpoints, hooks, telemetry, doctor,
  docline clock): `internal/core/stash.go` (all sites `.UTC()`),
  `internal/stash/stash.go:110`, `internal/db/stash.go`, `internal/events/**`
  (checkpoint/hook/memory), `internal/hooks/**`, `internal/telemetry/harvest.go`,
  `internal/core/doctor.go:158`, `internal/cli/checkpoint.go:185`,
  `internal/cli/docs.go:106`, `internal/mcp/docs_tools.go:73` — these already
  emit `Z`.
* **Non-stamping re-serializers** (preserve the existing `updated_at`, add no new
  timestamp): `archive.go:217,714` (archive/restore re-serialize),
  `mcp/tools.go:1087` (`writeSectionsToFile` re-serializes parsed `fm` without a
  new stamp — the MCP section path does not bump `updated_at`).
* **Non-write comparisons**: `archive.go:887,903`, `task_lock.go:*` (deadlines).

## Non-goals

* No backfill/rewrite of existing on-disk artifacts. Historical local-offset
  timestamps remain valid; the DB read path (**`internal/db/queries.go:75-86`**)
  already parses `RFC3339Nano` with an `RFC3339` fallback and is zone-agnostic.
* No change to JSONL event-log emission (`logs.go`) — see out-of-scope note.
* No schema, CLI-distribution, or `.tmpl` template-family changes.

## Blast-radius assessment

* **Touches:** production Go code in `internal/models` (one small helper +
  defensive defaults), `internal/core`, the separate `internal/core/templates`
  package, and `internal/cli` (~20 assignment/serializer sites across 10 files).
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
   `models.NowUTC()`; convert the defensive `ArtifactFromFrontmatter` defaults.
2. **Write failing tests FIRST with a DETERMINISTIC, environment-independent red
   phase.** The naive assertion "emitted `updated_at` ends with `Z`" is NOT a
   reliable red phase: CI runs on `ubuntu-latest` where the local zone is
   normally **UTC**, so the pre-change `time.Now()` writers already serialize a
   trailing `Z` and the test would pass before implementation (PR #234 cycle-3
   thread `3575266998`). Each writer test MUST therefore **force a non-UTC local
   zone** so the pre-change code deterministically fails on any runner:
   * **Preferred (in-process):** at the top of the test, override the process
     local zone and restore it on cleanup:
     `orig := time.Local; time.Local = time.FixedZone("TESTNONUTC", -8*3600);
     t.Cleanup(func(){ time.Local = orig })`. These tests MUST be **serial**
     (no `t.Parallel`) because `time.Local` is process-global. Under this zone
     the pre-change `time.Now()` path serializes `-08:00` (red); the post-change
     `models.NowUTC()` path serializes `Z` (green).
   * **Alternative (hermetic):** re-exec the writer assertion in a subprocess
     with `TZ=America/Los_Angeles` set, for packages where mutating `time.Local`
     is undesirable.
3. **Assert canonical `Z` on the emitted frontmatter string.** Read back the
   written Markdown, extract the raw `created_at`/`updated_at` values, and assert
   each **ends with exactly `Z`** (`strings.HasSuffix(value, "Z")`, equivalently
   equals the value formatted with `.UTC()`) and contains **no** `+HH:MM`/`-HH:MM`
   offset. Parse-side tests may *separately* keep accepting historical
   `+/-HH:MM` offsets to prove backward compatibility — tolerance is on read; the
   strict `Z` assertion is on write.
4. **Make the minimal change.** Replace each inventoried `time.Now()` write site
   with `models.NowUTC()`. Confirm the test now passes (green).
5. **Green + regression.** Re-run new tests plus the existing package suite. The
   `-07:00` fixtures in `artifact_size_test.go` are parser *inputs*, not emission
   assertions, so they remain valid.
6. **Full quality-gate sequence (constitution).** The complete gate MUST pass:
   `go test ./...`, `go vet ./...`, `golangci-lint run`, and `gofmt -l .` (no
   output). The new `models.NowUTC()` helper is authored born-compliant with the
   exported-doc-comment lint rule (see Shared helper).

## Task decomposition (2-hour rule — constitution §Task Granularity)

The constitution heuristic (NON-NEGOTIABLE) caps a task at **fewer than 3 files
modified**, **fewer than 5 functions changed**, fewer than 4 test scenarios.
PR #234 cycle-3 thread `3575267006` correctly noted a colocated `*_test.go`
**counts toward the file limit**; cycle-4 thread `3575772360` correctly noted the
**function** count also binds (five writer sites in `shipment_lifecycle.go` span
five functions). The decomposition is therefore **one production file + its one
colocated `*_test.go` per task (2 files, `<3`)** AND **`<5` functions per task**,
across eleven tasks:

* **`103.001-T`** — `internal/models/frontmatter.go`: add `models.NowUTC()` +
  convert defensive defaults; `frontmatter_test.go`. *(foundation)*
* **`103.002-T`** — `internal/core/artifacts.go` writers (4 funcs) + test. *(dep: 103.001-T)*
* **`103.003-T`** — `internal/core/queue.go` writers (2 funcs) + test. *(dep: 103.001-T)*
* **`103.004-T`** — `internal/core/shipment.go` writers (3 funcs) + test. *(dep: 103.001-T)*
* **`103.005-T`** — `internal/core/shipment_lifecycle.go` `attachCommitToItems`/`setArtifactStatus`/`cascadePersistedParentStatuses` (3 funcs) + test. *(dep: 103.001-T)*
* **`103.006-T`** — `internal/core/gate_transition.go` writer + test. *(dep: 103.001-T)*
* **`103.007-T`** — `internal/core/artifact_references.go` writer + test. *(dep: 103.001-T)*
* **`103.008-T`** — `internal/core/migrate_links.go` writer + test. *(dep: 103.001-T)*
* **`103.009-T`** — `internal/core/templates/service.go` serializer + test. *(dep: 103.001-T)*
* **`103.010-T`** — `internal/cli/update.go` `update --section` serializer + test. *(dep: 103.001-T)*
* **`103.011-T`** — `internal/core/shipment_lifecycle.go` `clearParentID`/`AdoptItem` (2 funcs) + test. *(dep: 103.001-T)*

`103.002`–`103.011-T` depend on `103.001-T` because they consume the shared
helper. `103.005-T` and `103.011-T` both edit `shipment_lifecycle.go` but touch
**disjoint functions**, so they are independently reviewable/testable (a build
agent sequences them to avoid a merge conflict). All eleven belong to shipment
`092-S`. The per-file substitutions are mechanical one-line `models.NowUTC()`
swaps (not independent logic changes), so each task stays well within ~2 hours of
human-equivalent effort. Width isolation holds: every task is a single skill
domain (Go code); the colocated unit test is the atomic-milestone verification
artifact for that same production change, not separate test-infrastructure work —
no docs/schema/template-family work is bundled.

## Acceptance criteria

* An exported `models.NowUTC()` helper exists and is used by **all** inventoried
  item-artifact write sites across all eleven tasks; no item writer calls bare
  `time.Now()`.
* New item artifacts (features, tasks, subtasks, shipments) emit
  `created_at`/`updated_at` ending in exactly `Z`.
* Per-task regression tests force a non-UTC local zone, assert exact-`Z`
  emission, and **fail** on the pre-change code (deterministic red phase on any
  runner, including UTC CI); parse-side tests continue to accept historical
  offsets (backward compatibility proven).
* The full constitution quality-gate sequence passes: **`go test ./...`**,
  **`go vet ./...`**, **`golangci-lint run`**, and **`gofmt -l .`** (empty output).
  The exported `models.NowUTC()` helper carries its required Go doc comment so the
  linter's exported-comment rule is satisfied.

## Estimated effort

Eleven focused tasks, each within the constitution's per-task heuristic (2 files,
&lt;5 functions, &lt;4 test scenarios); single skill domain (Go code) with no
doc/template-family work bundled in. Individually mechanical; the split satisfies
the strict file- and function-count heuristics rather than reflecting large
per-task effort.

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
  exact-`Z` write-side assertion (Approach step 3), parse-side tolerance kept
  separate.
* **[pass 2 — helper package boundary]** (thread `3575231557`): resolved — the
  helper is an **exported `models.NowUTC()`** callable from both `internal/core`
  and the separate `internal/core/templates` package (no import cycle).
* **[pass 2 — 2-hour heuristic]** (thread `3575231580`): resolved — split into
  discrete per-file tasks per constitution §Task Granularity.
* **[pass 2 — wrong path]** (thread `3575231596`): resolved — corrected
  `internal/core/queries.go` → **`internal/db/queries.go:75-86`** (Non-goals +
  blast-radius).
* **[pass 3 — missed CLI writer]** (thread `3575266981`): resolved — added
  `internal/cli/update.go:240` (`update --section` direct serializer) to the
  inventory as `103.010-T`, after an exhaustive re-sweep of `internal/cli/**`,
  `internal/db/**`, `internal/models/**`, `internal/telemetry/**`,
  `internal/events/**` in addition to `internal/core/**`.
* **[pass 3 — non-deterministic red phase]** (thread `3575266998`): resolved —
  Approach step 2 forces a non-UTC local zone (`time.Local` override or `TZ`
  subprocess) so the pre-change code fails on UTC CI runners.
* **[pass 3 — test files count toward file limit]** (thread `3575267006`):
  resolved — re-split to **one production file + one colocated test = 2 files**
  per task, strictly fewer than 3.
* **[pass 4 — function-count heuristic]** (thread `3575772360`): resolved —
  `shipment_lifecycle.go`'s five writer sites span five distinct functions, so
  the task is split by function into `103.005-T` (3 funcs) and `103.011-T`
  (2 funcs); a per-task `<5 functions` audit of every task was added to the
  inventory (all tasks verified `<5`).

Self-assessment: scope/width isolation PASS (Go-only); 2-hour rule PASS (eleven
2-file / `<5`-function tasks); test-first PASS (deterministic non-UTC red phase +
exact-`Z`); backward compatibility PASS (zone-agnostic read path). Residual: none
blocking.

**Recommended pre-build follow-up:** run the formal multi-persona `plan-review`
skill against this revised plan before Ship builds `092-S`, and append its real
result here.
