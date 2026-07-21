---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Manually-created features leave abbreviated stash-harvest provenance; backfill the full canonical record the harvest path writes"
source: docs/compound/2026-07-20-manual-feature-harvest-provenance-backfill.md
doc_type: learning
description: "When a backlog feature/task is created directly (CreateArtifact / backlogit add) to represent work that originated from a stash entry — instead of promoting it through the canonical harvest_stash path — the harvest provenance is left incomplete in two places. (1) The artifact's custom_fields carries at most source_stash_id, missing the source_stash_kind, source_stash_text, source_stash_priority, and source_stash_path fields the canonical path writes (internal/core/stash.go:344-349). Because rehydration's stashRecordFromArtifact reads kind/text ONLY from source_stash_kind/source_stash_text (internal/db/rehydration.go:608-625), the harvested stash entry re-materializes with empty kind and text after sync, so list/query results that include harvested entries lose the original metadata. (2) If the stash was retired via the abbreviated `backlogit stash archive` CLI shortcut, the durable .backlogit/archive/stash.jsonl record is written with reason:archived and NO harvested_artifact_id, contradicting the actual harvest and diverging from canonical records produced by harvestStashEntryLocked (internal/core/stash.go:376-385). The DB link (stash_links) is still reconstructed correctly from the artifact's source_stash_id during sync, so agent-visible state can look fine while the durable history is inconsistent. The fix: write ALL five source_stash_* custom_fields on the artifact, set the archive record reason:harvested + harvested_artifact_id:<item-id>, then sync — after which stashRecordFromArtifact reconstructs a full harvested record (state=harvested, kind, priority, full text). Prefer harvest_stash for stash-originated work so this backfill is never needed."
docline:
    date: 2026-07-20T00:00:00Z
    severity: medium
    tags:
        - stash
        - harvest
        - provenance
        - rehydration
        - custom-fields
        - archive-record
        - backlog-hygiene
        - dogfooding
        - copilot-review
---

# Manual-Feature Harvest Provenance Backfill

## Symptom

A stash entry stays `active` (or lands as a bare `archived` record) even though
the work it describes was fully implemented and merged, because the feature/task
representing it was created directly rather than through `harvest_stash`. A code
reviewer (Copilot) flags: "Backfilling only `source_stash_id` leaves the
rehydrated harvested record with an empty kind and text" and "This durable
archive record still classifies the entry as merely `archived` and omits its
harvested target."

## Root cause

The canonical harvest path writes a complete provenance record in two places:

- **Artifact custom_fields** (`internal/core/stash.go:344-349`):
  `source_stash_id`, `source_stash_priority`, `source_stash_kind`,
  `source_stash_text`, `source_stash_path` (+ `source_deliberation_id` when set).
- **Durable archive record** (`harvestStashEntryLocked`,
  `internal/core/stash.go:376-385`): `reason: harvested` +
  `harvested_artifact_id: <item-id>` in `.backlogit/archive/stash.jsonl`.

Rehydration's `stashRecordFromArtifact`
(`internal/db/rehydration.go:600-625`) reconstructs the harvested stash entry
from the artifact's custom_fields. It requires `source_stash_id` (else it skips
the record) and reads kind/text ONLY from `source_stash_kind` /
`source_stash_text`. So an artifact with just `source_stash_id` produces a
harvested entry with **empty kind and text**.

The `backlogit stash archive <id>` CLI shortcut only marks the entry
`reason: archived` — it does not know the harvest target, so it omits
`harvested_artifact_id`.

## Fix

For each manually-created, stash-originated artifact:

1. Add the full canonical set to the artifact's `custom_fields`
   (4-space indent under `custom_fields:`):
   `source_stash_id`, `source_stash_kind`, `source_stash_priority`,
   `source_stash_path` (`stash.jsonl`), and `source_stash_text` (the full
   original stash text). If the stash entry had a deliberation, also copy
   `source_deliberation_id` — the canonical harvest path writes it whenever the
   entry has a deliberation ID (`internal/core/stash.go:351-352`) and rehydration
   reads it, so omitting it loses that provenance for deliberated stashes.
   `custom_fields` has no `backlogit` mutation — the
   `update` command only accepts modeled flags, so this backfill is inherently a
   **direct Markdown edit** followed by `backlogit sync`. Produce the
   `source_stash_text` value with a YAML-aware editor or a small codec-based
   utility (anything that quotes/escapes per the YAML spec) rather than
   hand-typing it. Hand-editing is safe only for simple single-line text: even a
   double-quoted scalar must additionally escape embedded `"` and `\`, and text
   with newlines or control characters needs a block scalar (`|`/`>`) or full
   escaping. Plain (unquoted) scalars are unsafe here because stash text
   frequently contains ` #` (issue refs → comment), `: ` (mapping), and `{}`
   (flow indicators). After `sync`, re-read the value (see step 4) to confirm it
   round-trips unchanged.
2. If the stash entry is still `active`, first run `backlogit stash archive
   <stash-id>` to move it out of the active store. `backlogit sync` alone does
   **not** remove an entry from `.backlogit/stash.jsonl`, and `FetchStash` reads
   that active file directly (`internal/core/stash.go:174`), so a merely
   index-rebuilt entry stays in the active stash. `stash archive` writes an
   archive record with `reason: archived` and flips the DB state to `removed`
   (`stash.go:723-758`); this is the step PR #272 performed for `A3C349DD`.
3. Update the durable archive record for the stash id in
   `.backlogit/archive/stash.jsonl` to canonical harvested form: `reason:
   harvested` and `harvested_artifact_id: <item-id>` (the `stash archive`
   shortcut leaves `reason: archived` with no `harvested_artifact_id`).
4. Run `backlogit sync`. Verify with a query that the stash entry is
   `state=harvested` with populated `kind`, `priority`, and `text` (plus
   `source_deliberation_id` when applicable), and that `stash_links` maps the
   stash to the item.

## Prevention

Promote stash-originated work through `harvest_stash` instead of creating the
feature/task directly. Reserve the manual path for genuinely non-stash work.

Note that `harvest_stash` is **not fully atomic** across the two records, so a
successful-looking harvest does not guarantee consistent provenance: the archive
append (`appendToStashArchive`) is best-effort and only logged on failure
(`internal/core/stash.go:387-389`), and a later DB or JSONL failure rolls back
the created artifact **without** removing the already-appended archive entry.
After any harvest, verify the stash entry is `state=harvested` with populated
`kind`/`text` and a `stash_links` row; backfill per the fix above if provenance
is incomplete.

## Note on false positives when auditing

When grepping `.backlogit/archive/stash.jsonl` for a stash id, a DIFFERENT
record can match because its `text` body mentions that id (e.g. one follow-up
task references another stash by hex id). Parse the JSON and match on the `id`
field, not a substring, before concluding a `harvested_artifact_id` is wrong.
