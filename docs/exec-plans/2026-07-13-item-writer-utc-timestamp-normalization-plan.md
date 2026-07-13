---
chunk_strategy: h1-h2-h3
description: Normalize backlogit item-artifact created_at/updated_at emission to UTC (Z) across the internal/core item writers so item frontmatter matches the UTC style already used by stash entries.
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

backlogit currently writes two timestamp styles into `.backlogit/`:

* **Stash entries** are written UTC-normalized. `internal/core/stash.go`
  uses `time.Now().UTC()` at every write site (lines 279, 286, 395, 592,
  658, 758, 868), so `stash.jsonl` timestamps end in `Z`.
* **Item artifacts** (features, tasks, shipments, subtasks) are written with a
  local offset. `internal/core/artifacts.go` stamps `now := time.Now()`
  (line 225) into `CreatedAt`/`UpdatedAt`, and the update paths at lines 552,
  819, and 856 use `time.Now()` as well. When the frontmatter map (lines
  290-296) is serialized, the `time.Time` retains its local zone, producing
  offsets such as `-07:00` (see the fixtures in
  `internal/core/artifact_size_test.go:28,34`).

The mixed styles are cosmetically inconsistent and can surprise lexical
sorting and cross-artifact auditing tooling that assumes a single canonical
zone.

## Goal

Every item-artifact writer emits `created_at`/`updated_at` in UTC (`Z`),
matching the stash convention, with no change to timestamp precision
(RFC3339Nano) and full backward compatibility for already-written artifacts.

## Non-goals

* No backfill/rewrite of existing on-disk artifacts. Historical local-offset
  timestamps remain valid; the DB read path already parses both forms.
* No change to the JSONL event-log timestamp emission in `internal/core/logs.go`
  (out of scope for this stash entry; it is a separate concern if raised).
* No schema, CLI-distribution, or template-family changes.

## Blast-radius assessment

* **Touches:** `internal/core` production code only (item writer + update
  paths). Behavior change is confined to the zone of newly emitted item
  timestamps.
* **Does NOT touch:** JSON schemas, CLI packaging/distribution, or any
  template family. Therefore `plan-harden` is **not** invoked (the elevated
  blast-radius triggers — schemas, distribution, multi-template — are all
  absent). The change is additive normalization: `internal/core/queries.go`
  (lines 75-86) already falls back across `RFC3339Nano` and `RFC3339` when
  parsing, and neither variant is zone-restricted, so mixed old/new zones
  round-trip cleanly.

## Approach (test-first / TDD)

1. **Write the failing test first.** Add a regression test in
   `internal/core` (e.g. `artifacts_utc_timestamp_test.go`) that:
   * calls `CreateArtifact` for an item type,
   * reads back the written Markdown frontmatter,
   * asserts the raw `created_at` and `updated_at` strings are UTC — the
     serialized value ends in `Z` (or has a `+00:00` offset), i.e. equals the
     value formatted with `.UTC()`.
   * Add a parallel assertion for an update path (e.g. a field/status update
     that restamps `UpdatedAt`) so lines 552/819/856 are covered too.
   Confirm the test fails against current `time.Now()` emission.

2. **Make the minimal change.** Replace the item-writer `time.Now()` calls
   with `time.Now().UTC()` at the emission sites:
   * `internal/core/artifacts.go:225` (create),
   * `internal/core/artifacts.go:552`, `:819`, `:856` (update restamps),
   * audit `internal/core/artifact_references.go:105` and
     `internal/core/gate_transition.go:341` (both restamp `UpdatedAt =
     time.Now()`) and normalize any that write item frontmatter.
   Prefer a single shared helper (e.g. `nowUTC()` or reuse the stash pattern)
   so the convention is centrally enforced and future writers inherit it.

3. **Green + regression.** Re-run the new test plus the existing
   `internal/core` suite. Confirm round-trip parse still works and no existing
   fixture assertion breaks (the `-07:00` fixtures in `artifact_size_test.go`
   are *inputs* to the parser, not emission assertions, so they remain valid).

4. **Full package gate.** `go test ./internal/core/...` (and the wider suite
   if touched) must stay green.

## Acceptance criteria

* New item artifacts written by the CLI/MCP emit `created_at`/`updated_at` in
  UTC (`Z`).
* A regression test asserts UTC emission for both the create and an update
  path and fails on the pre-change code.
* Existing artifacts with local-offset timestamps still parse and round-trip
  (no forced backfill).
* `go test ./internal/core/...` is green.

## Estimated effort

Single focused change in one package with test-first coverage. Well within the
2-hour rule; single skill domain (Go code) — no doc/template work bundled in.

## Plan review

**Provenance: inline single-agent self-assessment by the Stage agent — NOT a
formal multi-persona `plan-review` skill run.** The Stage agent operates as a
single agent in this environment and cannot spawn the independent reviewer
personas the formal gate requires, so this is labeled honestly as a self-check.

* **Scope / width isolation:** PASS. Single package (`internal/core`), single
  concern (timestamp zone). No docs/schema/template work folded in.
* **2-hour rule:** PASS. Contained edit + focused regression test.
* **Test-first:** PASS. Failing test precedes the change; both create and
  update paths covered.
* **Backward compatibility:** PASS. Read path already parses mixed zones; no
  backfill required.
* **Residual risk:** LOW. If a later concern wants event-log (`logs.go`) or
  a historical backfill, that is explicitly deferred as a separate item.

Self-assessment disposition: **proceed to harvest.** No `plan-harden` needed.
