---
chunk_strategy: h1-h2-h3
description: 'attachCommitToItems (shipment_lifecycle.go) re-persists artifacts after stamping a commit SHA. Any DB-first load at the re-persist seam silently drops item_links (not in the items table) and archived_status (not in selectCols). Fix: use findArtifact (Markdown reload) for both the archived-skip guard and the mutate-then-persist step. Precedent: MoveInQueue, serializer_provenance_hardening.'
doc_type: learning
docline:
    date: 2026-07-28T00:00:00Z
    severity: high
    tags:
        - shipment
        - attach-commit
        - re-persist
        - item-links
        - archive-provenance
        - db-projection
        - reload-from-markdown
        - findArtifact
        - loadArtifact
        - field-drop
        - shipment-lifecycle
ingested_at: "2026-07-28T21:00:00Z"
schema_version: "1.0"
source: docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md
title: 'attachCommitToItems re-persist seam: use findArtifact (Markdown) not loadArtifact (DB) — DB projection omits item_links and archived_status'
---

# attachCommitToItems Re-Persist Seam: Reload from Markdown

## Problem

`attachCommitToItems` in `internal/core/shipment_lifecycle.go` stamps the merge
commit SHA on each shipment member, then calls `persistArtifact` to write the
updated Markdown. Before fix (PR #312), it called `loadArtifact` (DB fast-path
via `bldb.GetItem`) to load the artifact. That load is lossy:

1. **`item_links` are dropped** — the SQLite `items` table has no `links` column;
   semantic links live in the normalized `item_links` table. `loadArtifact` returns
   an `Artifact` with `Links == nil`, so `persistArtifact` emits no `links:` block
   in the frontmatter, silently destroying `spike_ref`, `related_to`, and any
   other typed links the author set (129.002-T / bug `7A965F8A`).

2. **Archived linked deliberations abort the ship** — `collectArchiveCandidateIDs`
   appends `linkedDeliberationIDs` without an archived filter. An already-archived
   linked deliberation lands in the archive candidate set. The DB-loaded artifact
   has `Status: archived` but empty `ArchivedFrom`/`ArchivedStatus` (both absent
   from `selectCols`). `persistArtifact` → `WriteArtifactFileWithOptions` sees an
   archived artifact without provenance and refuses: `refusing to write archived
   artifact without provenance`. This aborts the ship (129.001-T / bug `D04D63D0`).

## Root Cause: Two DB Projection Gaps

### Gap 1 — `item_links` table separation

The `items` table stores scalar artifact fields only. Typed semantic links
(`spike_ref`, `related_to`, `informs`, etc.) are modeled in the separate
`item_links` table. `loadArtifact` (`bldb.GetItem`) reads only from `items`, so
`Artifact.Links` is always nil for any DB-loaded artifact. Any re-persist path
that starts from a DB load destroys the links frontmatter block.

### Gap 2 — `archived_status` absent from `selectCols`

`selectCols` / `scanArtifactRow` enumerate the projected DB columns explicitly.
`archived_from` and `archived_status` are not in that list (they are stamped
only in the Markdown file by `ArchiveItem`). `loadArtifact` is index-first and
falls back to disk only on `ErrNotFound` — a found-but-stale row returns a
zero-valued `ArchivedStatus`. See the same gap in
`docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`.

## Fix: Single findArtifact (Markdown) Load

Use `findArtifact` (walks Markdown dirs, reads full frontmatter) for:
1. The **archived-skip guard** — if `artifact.Status == StatusArchived`, skip
   this item entirely (do not re-stamp, do not re-persist). An archived linked
   deliberation is correctly detected and skipped.
2. The **mutate-then-persist step** — stamp the commit SHA on the Markdown-loaded
   artifact and call `persistArtifact`. `Links` is populated from the real
   frontmatter, so they survive the write.

The earlier split-load approach (`loadArtifact` for the guard, `findArtifact` for
the persist) was discarded after Copilot review (PR #312, cycle 1): a stale DB
row with `status: archived` could skip an artifact whose Markdown is actually
`active`, or vice versa. A single `findArtifact` call is authoritative and
eliminates the race.

```go
// attachCommitToItems — post-fix seam
artifact, err := findArtifact(ctx, ws, id)
if err != nil {
    return fmt.Errorf("attach commit: load artifact %s: %w", id, err)
}
if artifact.Status == models.StatusArchived {
    continue  // already archived — skip re-stamp
}
artifact.Commit = commitSHA
if err := persistArtifact(ctx, ws, artifact); err != nil {
    return fmt.Errorf("attach commit: persist artifact %s: %w", id, err)
}
```

## Test Coverage (PR #312)

`internal/core/shipment_repersist_test.go`:

- **`TestShipShipment_SkipsAlreadyArchivedLinkedDeliberation`** (129.001-T) — full
  `ShipShipment` flow with a linked deliberation that is already archived. Asserts:
  (a) ship completes without error; (b) shipment manifest is archived; (c) the
  deliberation's `commit` SHA and `archived_from`/`archived_status` provenance are
  **unchanged** after ship (proves the skip happened; prevents false-green where
  reload alone would restore provenance and let the stamp succeed).

- **`TestAttachCommitToItems_PreservesItemLinks_AfterRepersist`** (129.002-T) —
  table-driven (populated `spike_ref` link + nil/empty links). Targets a stamped
  non-archived candidate directly. Asserts `item_links` (`spike_ref`) survive
  re-persist; also asserts the nil/empty case produces no `links:` key (omitempty
  guard).

## The omitempty Guard

`models.Artifact.Links` is tagged `omitempty`. Serialization emits `links:` only
when the slice is non-nil and non-empty. The nil/empty test case confirms that an
artifact with no links produces no `links: []` noise in frontmatter — an important
round-trip invariant when cross-checking with artifacts authored without links.

## Rules

- **Any re-persist seam that calls `loadArtifact` (DB fast-path) is lossy for
  `item_links`.** Audit every callsite that calls `loadArtifact` then
  `persistArtifact` — each is a candidate field-drop.
- **Any re-persist seam that uses `loadArtifact` is lossy for `archived_status`.**
  The same DB projection gap applies to all archive-provenance fields.
- **Prefer `findArtifact` (Markdown reload) at any re-persist seam.** It is the
  source-of-truth loader and carries all frontmatter fields. Precedents:
  `MoveInQueue`, `serializer_provenance_hardening`, now `attachCommitToItems`.
- **A single `findArtifact` load for both guard and mutate is safer than a split
  load.** Two loads from different sources (DB for guard, Markdown for persist)
  create a race window where DB and Markdown diverge in status.
- **Skip archived items at re-persist seams, do not re-stamp them.** An archived
  artifact belongs to a prior lifecycle stage; re-stamping it aborts the write guard
  (`refusing to write archived artifact without provenance`). Note: `ArchiveItem`
  stamps `archived_from`/`archived_status` but does NOT write the `commit`
  frontmatter scalar — `WithCommitSHA` attaches the SHA only to the archive event
  (`archive.go:74-76, 281-284`), not to the artifact's frontmatter `commit` field.
  The skip is correct because the archived artifact belongs to earlier work, not
  because archiving already populated its commit scalar.

## Precedent

- `MoveInQueue` (`internal/core/artifacts.go`) — reloads from Markdown before
  rewriting so serialize-then-write does not lose unmodeled fields.
- `serializer_provenance_hardening` — same pattern applied to provenance keys
  during status transitions.
- `archivedFromNonTerminalStatus` in `shipment_gate.go` — reads `archived_status`
  from Markdown source because DB projection omits it (same gap, different context).

## Evidence

- Bugs `D04D63D0` (ship abort on already-archived deliberation) and `7A965F8A`
  (spike_ref drop on re-persist); surfaced during shipment 110-S planning
  (exec-plan `docs/exec-plans/2026-07-28-archive-repersist-projection-drop-plan.md`).
- Fix: PR #312 (merge commit `f32c9f7847f9cee9428ff8b76f0d7778748f0944`);
  shipment 110-S (feature `129-F`, tasks `129.001-T` / `129.002-T`).
- `internal/core/shipment_lifecycle.go`: `attachCommitToItems` (re-persist seam).
- `internal/core/shipment_repersist_test.go`: regression tests for both bugs.
