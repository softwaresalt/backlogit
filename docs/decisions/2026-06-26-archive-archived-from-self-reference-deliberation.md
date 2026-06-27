---
chunk_strategy: h1-h2-h3
description: 'Deliberation on fixing core.ArchiveItem self-referential archived_from for pre-archived items, restoring invertible unarchive, and repairing ~130 legacy records'
doc_type: decision
docline:
    decision_status: decided
    depth: deep
    linked_artifacts:
        - internal/core/archive.go
        - internal/core/archive_test.go
        - docs/exec-plans/2026-06-26-archive-archived-from-integrity-plan.md
    promoted_to: plan
    stash_ids:
        - 53F22794
    tags:
        - archive
        - unarchive
        - data-integrity
        - data-migration
        - invertibility
        - doctor
        - regression-test
    topic: 'core.ArchiveItem self-referential archived_from for pre-archived items'
ingested_at: "2026-06-26T20:15:36Z"
schema_version: "1.0"
source: docs/decisions/2026-06-26-archive-archived-from-self-reference-deliberation.md
title: 'Repair self-referential archived_from in core.ArchiveItem and restore invertible unarchive'
---

## Problem Frame

`core.ArchiveItem` (`internal/core/archive.go:181`) unconditionally stamps:

```go
fm["archived_from"] = workspaceRelativePath(ws.RootPath, currentPath)
```

For a **pre-archived** item — one that already resides in `.backlogit/archive/`
before `ArchiveItem` runs — `currentPath == archivePath`. This happens when the
registry routes a terminal status (e.g. `done`) to the archive directory at
done-time, and the item is then re-stamped during `shipment ship`. In that case
`archived_from` is written as the item's **own archive path**
(`.backlogit/archive/<id>.md`) instead of the canonical restore location
(`.backlogit/queue/<id>.md`).

Consequences:

1. **Contract violation.** The ArchiveItem contract test
   (`internal/core/archive_test.go:70`) asserts
   `archived_from == ".backlogit/queue/001-T.md"` for the normal queued→archive
   path. There is **no test** for the pre-archived path, which is exactly how the
   defect slipped through.
2. **Broken invertibility.** `UnarchiveItem`
   (`internal/core/archive.go:358-409`) reads `archived_from` as the restore
   target. When it equals the archive path, `originalPath == archivePath`; the
   compensating branch at lines 401-409 *accepts* this by skipping removal and
   leaving the file in the archive directory. **Net effect: unarchiving a
   pre-archived item cannot return it to the queue.** Unarchive is not invertible
   for this class of item.
3. **Polluted historical data.** 130 already-archived records carry the
   self-referential `archived_from` (measured below). The 12 in-diff 065 records
   were corrected in PR #138 (commit `166c5a53`), but the systemic code defect and
   the legacy records remain.

This is a production data-correctness defect: systemic in `core.ArchiveItem`,
surfaced for 065 but general to any artifact archived at done-time.

## Research Findings

### Code evidence (grounded against `main` @ `0036a7ee`)

* **`archive.go:103-108`** — when `FindArtifactPath` returns an archive-dir path
  but a queue copy also exists, the queue copy is preferred as the canonical
  source (060.002-T half-archive recovery). The self-reference therefore only
  triggers when **no** queue copy exists and the item lives solely in the archive
  dir.
* **`archive.go:135`** — `archivePath := filepath.Join(archiveDir, filepath.Base(currentPath))`.
  For a pre-archived item `archivePath == currentPath`.
* **`archive.go:181`** — the defect: stamps `currentPath`, not the canonical
  restore path.
* **`archive.go:183`** — `archived_status` already preserves the pre-archive
  status (060.003-T), which is the key the registry router needs to resolve the
  canonical restore directory.
* **`archive.go:196-205`** — the source-removal guard already special-cases
  `currentPath == archivePath`, so it knows the file may pre-exist in the archive.
* **`archive.go:401-409`** (UnarchiveItem) — the compensating branch that accepts
  the self-reference. Any fix to `archived_from` must keep these two halves
  consistent.
* **`artifacts.go:522-568`** (`artifactSearchDirs`) and `archive.go:457-462`
  (`queueRootDir`) — the canonical restore directory is the configured queue
  layout root (`ws.Config.QueueLayout.RootDir`, default `queue`), and the registry
  `Directories` rules describe status-based relocations. This is the existing
  machinery for resolving where an artifact "should live" when active.

### Doctor surface

`backlogit doctor` on the current tree reports **"No issues found."** — it does
**not** currently audit `archived_from` invertibility. The 066 work added a
doctor audit infrastructure (`internal/core/doctor.go`), so an `archived_from`
self-reference audit is a natural extension of an existing surface, not net-new
scaffolding.

### Legacy record census (measured this session)

Scan of 602 `.md` files in `.backlogit/archive/`:

| Class | Count | Meaning |
|---|---|---|
| Self-referential (`archived_from` → `.backlogit/archive/...`) | **130** | The defect target — must be repaired |
| Canonical (`archived_from` → `.backlogit/queue/...`) | 258 | Correct; leave alone |
| No `archived_from` field | 211 | Pre-feature archives; out of scope |
| Other | 3 | See anomalies below |

The 130 matches the stash's "~130" estimate exactly.

**Anomalies in the "other" bucket (important for migration scoping):**

* `036-DL.md` → `archived_from: .backlogit/deliberations/036-DL.md` — its
  `archived_from` value is simply **not self-referential** (it does not equal the
  record's own archive path), so the v1 self-reference comparator
  (`archived_from == <record's archive path>`) **excludes** it automatically and the
  migration leaves it untouched. This is **not** evidence that
  `.backlogit/deliberations/` is a valid restore location: this workspace's registry
  routes artifacts only between `queue/` and `archive/` by *status* (there are no
  type-based directory rules), so no resolver reproduces a `deliberations/` path and
  none needs to. The record is preserved purely because the comparator skips it — its
  exact value is out of scope for the self-reference repair.
* `038-DL.md`, `039-DL.md` → `archived_from: done` — **malformed**: a status value
  leaked into the field. These are a distinct corruption class the audit should
  flag (v1 detect/report only, no auto-repair), but they are not part of the
  130 self-reference set.

### Prior learnings

The compound library has no archive/unarchive prior art (Step 1.8 confidence:
low). It does have a directly relevant docline-frontmatter contract learning
(`docs/compound/2026-06-26-docline-frontmatter-contract.md`) — the staging docs
authored here are born-compliant and lint-verified per that pattern.

## Options Evaluated

### Decision 1 — Migration delivery vehicle

#### Option A: Pure code fix + dedicated one-shot migration

Fix `ArchiveItem`/`UnarchiveItem`, then ship a one-shot migration command (e.g.
`backlogit migrate archived-from`) or a script that rewrites the 130 records once.

* **Pros**: Narrow blast radius; the migration code can be deleted after it runs;
  no coupling to doctor's audit model.
* **Cons**: One-shot tools rot and provide no ongoing regression detection; the
  066 doctor audit infrastructure would be duplicated; handling the DL-subdir and
  malformed-`done` edge cases means re-implementing registry path resolution
  outside doctor.

#### Option B: Code fix + `doctor` audit with `--fix`

Fix `ArchiveItem`/`UnarchiveItem`, then add an `archived_from` invertibility audit
to `backlogit doctor` (detection by default; repair under `--fix`), reusing the
066 doctor audit infrastructure.

* **Pros**: Doctor is already the integrity surface; the audit gives **ongoing**
  detection so the class can never silently regress; one code path resolves the
  canonical restore directory via the registry for queue, DL-subdir, and malformed
  records alike; idempotent + reviewable batches fit doctor's model; verifiable via
  a post-`doctor` scan showing zero findings.
* **Cons**: Slightly larger surface than a throwaway script; must scope the audit
  carefully so it doesn't touch the 258 already-canonical or 211 fieldless records.

### Decision 2 — Canonical restore-path seeding (cross-cutting)

#### Option R1: Hardcode `.backlogit/queue/<basename>`

Simple; matches the contract test literal. **But** it corrupts the `036-DL`
deliberation record (which restores to `deliberations/`) and ignores configurable
queue layouts.

#### Option R2: Registry/queue-layout resolved restore path

Resolve the canonical restore directory from the artifact type and the registry
`Directories` rules / `QueueLayout.RootDir`, falling back to `queue/<basename>`.
Honors artifact-type routing and configurable layouts; the same resolver serves
both the code fix and the migration.

### Decision 3 — Unarchive consistency (cross-cutting)

Once `archived_from` points at the queue/restore path again, `UnarchiveItem`'s
`originalPath != archivePath`, so the rename-into-queue + remove-archive-copy path
(lines 388-409) restores correctly. The lines 401-409 special case stays as a
defensive net for any residual self-referential records not yet migrated, but the
**primary** path must restore pre-archived items to the queue. A round-trip
regression test (archive → unarchive → file back in queue) locks this in.

## Trade-off Comparison

| Criterion | A: one-shot | B: doctor audit+fix |
|---|---|---|
| Reuses 066 doctor infra | No | Yes |
| Ongoing regression detection | No | Yes |
| Edge-case (DL/malformed) handling | Re-implement | Single resolver |
| Post-run verification | Manual scan | `doctor` clean |
| Code longevity | Throwaway | Durable surface |
| Blast radius | Smaller | Slightly larger |

## Decision

**Chosen direction (recommended, pending operator confirmation at merge):**

1. **Code fix (R2 + Decision 3).** In `ArchiveItem`, when the item is pre-archived
   (`filepath.Clean(currentPath) == filepath.Clean(archivePath)`), set
   `archived_from` to the **`.backlogit/`-prefixed canonical restore path** derived
   from `ws.Config.QueueLayout.RootDir` (default `.backlogit/queue/<basename>`)
   instead of `currentPath`. The path must be repo-root-relative and `.backlogit/`-prefixed
   to match `workspaceRelativePath` / `archive_test.go:70` and to satisfy
   `UnarchiveItem`'s F-006 guard (a prefix-less `queue/...` would be rejected). Leave
   the normal queued→archive behavior byte-for-byte unchanged so contract test
   `archive_test.go:70` still passes. Add a regression test mirroring `:70` for the
   pre-archived path.
2. **Unarchive consistency.** Confirm/adjust `UnarchiveItem` so the new
   `archived_from` restores pre-archived items to the queue; keep lines 401-409 as
   a defensive net. Add an archive→unarchive round-trip test for the pre-archived
   case.
3. **Migration (Option B — recommended, flagged for operator sign-off).** Add an
   `archived_from` invertibility audit to `backlogit doctor` (detect by default,
   repair under a **CLI-only** `--fix-archived-from` flag, not exposed on the MCP
   `backlogit_doctor` tool per Principle VII). Detection is by path-equality
   (`archived_from` equals the record's own archive path); repair rewrites only those
   130 self-referential records to `.backlogit/<queue-root>/<basename>` derived from
   `QueueLayout.RootDir`, using the **same restore-path resolver** as the code fix.
   Idempotent, reviewable batches, post-scan verifies zero self-ref records remain.
   The audit also flags the 2 malformed `archived_from: done` records (detect/report
   only in v1) and must **not** touch the 258 canonical, 211 fieldless, or 1
   legitimate DL-subdir records (the latter is excluded by the self-ref comparator).

Rationale: R2 (a `QueueLayout.RootDir`-derived resolver) is preferred over R1 (a
hardcoded `queue/`) so the restore path honors a workspace's configured queue root
rather than assuming the default — the resolver stays correct if `QueueLayout.RootDir`
is ever customized. Option B reuses the existing integrity surface, converts a
one-time repair into durable regression detection, and shares one resolver across
fix + migration. The work decomposes cleanly into 2-hour, single-domain, TDD-first
tasks.

> **Refinement (plan-review, attempt 1 → 2).** Plan-review corrected three framing
> errors carried in the earlier draft of this deliberation, now reflected above:
> (1) the restore path must be **repo-root-relative and `.backlogit/`-prefixed**
> (a prefix-less `queue/...` is rejected by `UnarchiveItem`'s F-006 guard);
> (2) the backlogit registry routes archive/queue placement by **status**, not
> artifact **type** — there is no type→directory map, so the resolver is pure over
> `QueueLayout.RootDir` and the `036-DL` record is retained because it is
> **non-self-referential** (excluded by the comparator), not because a resolver
> reproduces a type-specific path; (3) the migration `--fix` is **CLI-only**
> (Principle VII), not exposed on the MCP doctor tool. See the `## Plan Review`
> section of the reviewed plan for the full attempt-1 FAIL → attempt-2 PASS record.

## Rejected Alternatives

* **Option A (one-shot migration)** — rejected as the primary vehicle because it
  duplicates 066 doctor infrastructure, rots after use, and provides no ongoing
  detection. Retained as a fallback if the operator wants to decouple the repair
  from doctor's release cadence.
* **Option R1 (hardcoded `queue/`)** — rejected: corrupts the legitimate
  `036-DL` deliberation record and ignores configurable queue layouts.
* **Leaving UnarchiveItem's 401-409 as the sole behavior** — rejected: it
  permanently traps pre-archived items in the archive directory on unarchive.

## Unresolved Questions

1. **Doctor-audit vs one-shot (Decision 1).** Recommendation is Option B (doctor
   audit + `--fix`). **Flagged for operator confirmation** — if the operator
   prefers to keep doctor lean, fall back to a dedicated one-shot migration command
   (Option A) using the same R2 resolver.
2. **Malformed `done` records (038-DL, 039-DL).** Should the audit auto-repair
   these via registry resolution, or only flag them for manual review? Recommend
   **flag-only in `--fix` v1** (they are a different corruption class) with a
   follow-up if auto-repair is wanted.
3. **Migration of the 211 fieldless records.** Out of scope for this feature
   (pre-`archived_from` archives). Confirm we are not expected to back-fill them.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Migration corrupts the legitimate DL-subdir record or canonical records | Audit matches **only** records whose `archived_from` resolves to their own archive path; explicit unit fixtures for DL-subdir and canonical "leave-alone" cases |
| Code fix regresses the normal queued→archive contract | Keep the normal branch unchanged; `archive_test.go:70` is the guard; add pre-archived test alongside, not in place of |
| Unarchive fix and `archived_from` fix drift apart | Round-trip test exercises both halves together; plan-harden gate required |
| Non-idempotent migration leaves a moving corpus | Idempotent rewrite + post-scan asserting zero self-ref; re-run is a no-op |
| Elevated blast radius (correctness + 130-record data migration) | Plan-harden + plan-review gates before harvest; TDD-first tasks; reviewable batches |
