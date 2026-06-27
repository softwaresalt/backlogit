---
chunk_strategy: h1-h2-h3
description: 'Migration closure runbook for shipment 067-S: repairing legacy self-referential archived_from records with doctor --fix-archived-from, including census, verification, and rollback'
doc_type: closure
docline:
    ms.date: 2026-06-26T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-06-26-archived-from-migration-closure.md
title: 067-S archived_from Migration Closure Runbook
---

## Scope

Operational runbook for the legacy-record data migration shipped inside
shipment `067-S` ("ArchiveItem archived_from Integrity"). It documents how to
repair the historical archive records whose `archived_from` frontmatter field
self-references the archive path instead of the canonical
`.backlogit/queue/<id>.md` restore path, using the CLI-only
`backlogit doctor --fix-archived-from` repair introduced by task `067.005-T`
(core) and `067.006-T` (CLI).

This is the operator-run portion of `067-S`. The code fixes (resolver, archive
stamping, unarchive self-heal, doctor audit, repair, CLI flag) merge with the
feature pull request. The data migration described here rewrites real archive
records in `.backlogit/archive/` and is committed as an isolated commit,
separate from the code.

## Background — the defect

`core.ArchiveItem` stamped `archived_from` with the record's *current* path. For
items already in the archive directory (archived at done-time, then re-stamped
during `shipment ship`), the current path equalled the archive path, so
`archived_from` self-referenced the archive location. `UnarchiveItem` then
skipped the queue restore (it saw source == destination), leaving those items
un-restorable. The code fixes correct the stamping and add a read-time self-heal,
but historical records already on disk carry the bad value and require this
one-time repair.

## Baseline census

Measured against the working tree on 2026-06-26 with the freshly built
`067-S` binary:

```
backlogit doctor --check-orphans=false --check-duplicates=false --check-archived-from --format json
```

| Category | Count | Migration disposition |
|---|---|---|
| Self-referential (`archived_from` resolves to the record's own archive path) | 130 | Rewritten to canonical queue path |
| Malformed (`archived_from` present but not a `.md` path, e.g. `done`) | 2 | Flagged only — never auto-repaired |
| Canonical (`archived_from` is a valid `.backlogit/queue/<id>.md` path) | 259 | Untouched |
| Fieldless (no `archived_from`) | 217 | Untouched |
| Total archive `.md` records | 608 | — |

The two malformed records (`038-DL`, `039-DL`, value `archived_from: done`) are
flagged by the audit but intentionally NOT auto-repaired in this shipment
(operator decision: flag-only). The exact canonical/fieldless totals depend on
the archive population at migration time; the binding invariant is the
self-referential count (130) and the post-migration self-referential count (0).

## Preconditions

1. The `067-S` feature pull request is merged (or, when running the migration as
   part of the same PR, all code commits for tasks `067.001-T` through
   `067.006-T` are present on the branch).
2. The binary used to run the repair is built from the branch HEAD that includes
   tasks `067.001-T` (resolver) through `067.006-T` (CLI flag). A stale binary
   without `--fix-archived-from` cannot perform the repair.
3. The `.backlogit/archive/` working tree is clean except for the intended
   migration. Run `git status --short -- .backlogit/archive/` and confirm there
   are no unrelated pending changes, so the migration lands as an isolated,
   reviewable commit.
4. A full backup or a known-good git ref exists for rollback (the feature merge
   commit, or the commit immediately preceding the migration commit).

## Migration procedure

### 1. Dry-run detection

Confirm the self-referential population before mutating anything:

```
backlogit doctor --check-orphans=false --check-duplicates=false --check-archived-from --format json
```

Expect exactly `130` findings of type `archived_from_self_ref` and `2` of type
`archived_from_malformed`. If the self-referential count differs from the
expected baseline, STOP and re-establish the baseline before proceeding — a
drift indicates the archive population changed and the migration scope must be
re-confirmed.

### 2. Run the repair

```
backlogit doctor --fix-archived-from
```

The repair rewrites ONLY the `archived_from` frontmatter field of each
self-referential record to its canonical `.backlogit/queue/<id>.md` path. Body
bytes (including CRLF line endings and horizontal rules) are preserved verbatim.
Each repair is reported as a `fix:archived_from_repaired` action. The repair is
continue-on-error per record (a single unreadable or symlinked record is logged
and skipped, not fatal), and refuses to follow symlinks or rewrite records that
escape the workspace storage root.

### 3. Post-migration verification

Re-run the census and confirm zero self-referential records remain:

```
backlogit doctor --check-orphans=false --check-duplicates=false --check-archived-from --format json
```

Acceptance criteria:

* `archived_from_self_ref` count is `0`.
* `archived_from_malformed` count is unchanged at `2` (still flagged, not fixed).
* The canonical record count increased by exactly the number of repaired records
  (`+130`): the rewritten records are now classified canonical.
* `git diff --stat -- .backlogit/archive/` shows exactly the repaired records
  changed, and a content spot-check confirms only the `archived_from` line
  differs (body bytes unchanged).

### 4. Idempotency check

Re-run the repair and confirm it is a no-op:

```
backlogit doctor --fix-archived-from
```

Expect zero `fix:archived_from_repaired` actions and a byte-stable working tree
(`git status --short -- .backlogit/archive/` reports no changes). A repaired
record no longer classifies as self-referential, so a second run cannot rewrite
it.

### 5. Commit the migration

Stage and commit ONLY the repaired archive records as an isolated commit,
separate from the code commits:

```
git add -u .backlogit/archive/
git commit -m "chore(core): repair 130 legacy self-referential archived_from records"
```

## Rollback

The migration touches only the `archived_from` frontmatter field and is fully
reversible:

* If verification fails or an unexpected record was modified, revert the
  migration commit: `git revert <migration_commit_sha>` (or
  `git checkout <pre_migration_sha> -- .backlogit/archive/` for an uncommitted
  state), then re-run the dry-run census to confirm the baseline is restored.
* Because the repair preserves body bytes and only rewrites one field, a revert
  is byte-exact — no data beyond `archived_from` was ever at risk.

Rollback trigger conditions:

* Post-migration self-referential count is not `0`.
* `git diff` shows any body-byte change in a repaired record.
* Any record outside the expected 130 was modified.

## Operator sign-off

Before considering the migration complete, confirm and record:

- [ ] Dry-run baseline matched the expected self-referential count (`130`).
- [ ] `--fix-archived-from` reported `130` repaired records.
- [ ] Post-scan self-referential count is `0`.
- [ ] Malformed count unchanged at `2` (flag-only, not repaired).
- [ ] `git diff` confirms only `archived_from` lines changed (no body bytes).
- [ ] Idempotency re-run was a byte-stable no-op.
- [ ] Migration committed as an isolated commit.

Once all boxes are checked, the `067-S` data migration is closed. Post-merge
closure of the shipment (`shipment ship 067-S`, archival of `067-F` and tasks)
proceeds as a separate operator step.
