---
chunk_strategy: h1-h2-h3
description: Running `backlogit update` (e.g. `--commit`) on an already-archived item silently drops the archive-only provenance fields `archived_from` and `archived_status`, because the typed Artifact codec does not model them. The result is a non-invertible archive record (`UnarchiveItem` hard-fails) and a lost original status. Backfill commits on archived items with a direct frontmatter edit, or attach the SHA at ship time via `shipment ship --sha`.
doc_type: learning
docline:
  date: 2026-07-17T00:00:00Z
  severity: high
  tags:
    - backlogit
    - archive
    - provenance
    - codec
    - data-integrity
    - cli
schema_version: "1.0"
source: docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md
title: backlogit update on archived items silently drops archive provenance (archived_from/archived_status)
---

## Problem

During 097-S post-merge closure the shipment had been shipped without `--sha`, so
the merge commit was not attached to the archived scope items. Backfilling it with
`backlogit update <id> --commit <sha>` on the already-archived records **silently
dropped** the `archived_from` and `archived_status` frontmatter keys that
`ArchiveItem` stamps. The closure-PR (#246) Copilot review caught it: the mutated
archive records were now non-invertible (`UnarchiveItem` hard-fails with
`archived item <id> is missing archived_from metadata`) and the original pre-archive
status (`done`) was lost. The same round-trip also reorders the `links` sequence and
restamps `updated_at`.

## Root cause

`backlogit update` loads the record through the **typed generic Artifact codec** and
writes it back through `WriteArtifactFile`. That path only round-trips the fields the
`models.Artifact` struct models — and the struct has **no** `ArchivedFrom` /
`ArchivedStatus` fields:

* `internal/models/artifact.go:34-55` — the `Artifact` struct enumerates id, title,
  status, artifact_type, parent_id, sprint, priority, description, assigned_to,
  owner, labels, dependencies, links, references, commit, custom_fields, created_at,
  updated_at, level, hierarchy_path. There is no carrier for the archive-only keys.
* `internal/core/artifacts.go:681-725` — `WriteArtifactFile` builds the output
  frontmatter map by **explicitly enumerating recognized keys**. Any key not in that
  enumeration (`archived_from`, `archived_status`) is not re-emitted, so it is
  dropped on write.
* `internal/core/archive.go:675-681` — `UnarchiveItem` reads `archived_from` from the
  raw frontmatter and returns a hard error when it is absent. Dropping the field
  makes the archive record non-invertible.

This is the same class of gotcha as "the generic artifact codec carries only
`custom_fields` and drops unmodeled top-level keys" — archived records carry
provenance keys that live **outside** the typed model, so any typed round-trip is
lossy for them.

## Guidance

* **Do not use `backlogit update`'s generic field flags on already-archived items.**
  The lossy path is specifically the field-update flags (`--commit`, `--status`,
  `--priority`, `--owner`, `--labels`, ...) that route through
  `UpdateArtifactWithGate` → `UpdateArtifact` → `WriteArtifactFile` (the typed
  round-trip). By contrast, `--section`-only updates reserialize the **raw**
  frontmatter map (`internal/cli/update.go:188-260`, via `models.ParseFrontmatter`)
  and `--size` uses the raw `mdfront` path (`internal/core/artifact_size.go:35-70`),
  so both **preserve** archive-only keys. Caveat: combining `--section` with a
  generic field flag in the same invocation is still lossy, because the field-flag
  branch runs the typed rewrite.
* **Attach the commit at ship time** with a single command:
  `backlogit shipment ship <id> --sha <merge-sha> --message ... --author ...`.
  `attachCommitToItems` reloads each artifact from its **Markdown source**
  (`findArtifact`) — not from the DB index — so `item_links` and archive provenance
  survive the re-persist. Already-archived members are skipped entirely (no
  re-stamp). This is a single sequential workflow (persist frontmatter, then
  `LinkCommit`), **not** a transaction — there is no rollback if the second step
  fails. Note `shipment ship` cannot be re-run once the shipment is `shipped`
  (it guards on `status: active`). For details on the DB projection gap and the
  reload-from-Markdown fix, see
  `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`
  (PR #312, shipment 110-S).
* **If a post-hoc backfill on an archived record is unavoidable**, treat the
  frontmatter `commit` scalar and the commit *traceability* records as two
  independent concerns:
  * **Frontmatter-only** — edit the frontmatter **directly** (body-preserving
    text edit): restore/keep `archived_from` and `archived_status`, add the
    `commit` line, and set `updated_at` to the edit time. Then `backlogit sync`
    reprojects the `commit` scalar into `items.commit`. This is **all** a direct
    edit does: it does **not** create a `commit_links` row, does **not** append a
    durable `commit_tracked` log event, and fires **no** hooks — so `sync` does
    not "complete" the backfill, and there is no hook event to align `updated_at`
    against (set it directly). Verify the diff is exactly the intended lines and
    that provenance keys survive.
  * **Full traceability (archive-safe)** — to record the commit in the durable
    traceability surface, use `backlogit_track_commit` (MCP) →
    `LinkCommit` (`internal/core/commits.go:25-57`). It inserts the `commit_links`
    row **without rewriting the artifact markdown**, so it preserves
    `archived_from`/`archived_status`. The accompanying `commit_tracked` log event
    is **best-effort**: an append/index failure is only logged as a warning and
    `LinkCommit` still returns success (`internal/core/commits.go:50-56`), so
    **verify** the `commit_tracked` event landed when durable rehydration matters.
    Note it populates the traceability tables/log, **not** the frontmatter `commit`
    scalar — the two are independent, and only the ship-time `shipment ship --sha`
    path writes both (in the single sequential workflow above).

## Evidence

* Observed in shipment 097-S post-merge closure (PR #246); the lossy backfill was
  reverted and repaired via direct frontmatter edits (restore provenance, re-add
  commit, restamp `updated_at`).
* Product-bug follow-up: the `update`/`WriteArtifactFile` path should preserve
  unmodeled archive-only frontmatter keys (or refuse to mutate archived items)
  rather than silently dropping provenance.
