---
chunk_strategy: h1-h2-h3
description: "Ship session memory — shipment 094-S (formal-gate architecture spike, findings artifact) shipped end-to-end: PR #248 merged 98b0522, post-merge closure PR #249 merged 66a143e after 5 bullet-proof Copilot review cycles; SHA backfilled to full ship-path parity via LinkCommit; commit_links durability + gitignore model documented."
doc_type: memory
schema_version: "1.0"
docline:
  ms.date: 2026-07-17T00:00:00Z
  ms.topic: reference
source: docs/memory/2026-07-17/094-S-ship-closure-memory.md
title: "094-S Ship closure — session memory"
---

## Task IDs completed

* Shipment **094-S** shipped/archived — formal-gate architecture spike
  (findings artifact, NOT code). Conclusion: PIVOT / medium confidence.
* Members archived: `105-F` (Research base-contract vs backlogit extension
  ownership), `105.001-T`.

## Key SHAs

* Main PR **#248** merged → `98b0522` (true merge commit, parents `e32c4a5` +
  `c02ccc3`). P-014 operator-approved, P-009 merge-commit-only.
* Post-merge closure PR **#249** merged → `66a143e` (parents `98b0522` +
  `610e1af`). Local `main` == `origin/main` @ `66a143e`.

## Review cycles

PR #248: 8 Copilot review-fix cycles (operator "continue until bullet-proof").
Thread trend 5→6→6→4→1→2→1→1→0. Final §1.9 on HEAD `c02ccc3`, CI 4/4 green.

PR #249 (closure): 5 cycles, each one distinct valid doc-accuracy finding,
converged to 0 threads:

* `2472031` — completed SHA backfill to ship-path parity (real `core.LinkCommit`
  + `updated_at` restamp, provenance-safe).
* `83b238d` — "Five" → "Seven" foundational-gap count (matches Q1–Q7 table).
* `45e27dc` — added `.backlogit/queue/094-S.md` queued→active claim to PR #248
  file inventory.
* `610e1af` — qualified backfill event as *equivalent* (event shape + commit
  metadata), NOT byte-identical (`LinkCommit` stamps `time.Now()`).
* Final §1.9 on HEAD `610e1af`, 0 unresolved Copilot threads, CI 4/4 green.

## Key learnings (this session)

* **`commit_links` is append-only runtime state.** `db.Rehydrate` Phase 2 clears
  `items`/`item_deps`/`item_links`/`item_logs` but **NOT** `commit_links`, and no
  path rebuilds it from JSONL `commit_tracked` events (`indexEventTx`/`IndexEvent`
  write only `item_logs`+`item_log_entries`). So on a full `backlogit.db`
  delete+sync, `commit_links` is empty for **all** items. This is a real
  parity/durability gap (Q7/F6) the spike itself documents.
* **Version-controlled vs local-only commit records.** Both
  `.backlogit/backlogit.db` and `.backlogit/logs/` are gitignored. The **only**
  version-controlled record of a commit association is the frontmatter `commit`
  scalar (which `backlogit get` reads). `commit_links` + `commit_tracked` are
  local runtime/log projections a normal `shipment ship --sha` also produces
  locally but never commits.
* **Provenance-safe backfill on archived items** = `core.LinkCommit`
  (`internal/core/commits.go:27-57`; INSERT `commit_links` + append
  `commit_tracked` JSONL, never touches frontmatter) + a **direct body-preserving**
  `updated_at` edit. Never use `backlogit update --commit` on archived items — the
  typed codec round-trip drops `archived_from`/`archived_status` AND does not call
  `LinkCommit`.

## Files modified

* `.backlogit/archive/{094-S,105-F,105.001-T}.md` — `commit: 98b0522…` scalar +
  restamped `updated_at`; `archived_from`/`archived_status` preserved.
* `docs/closure/2026-07-17-094-S-ship-closure.md` — traceability section expanded
  (version-controlled scalar vs gitignored index/log projections; event
  equivalence not byte-identical); Q1–Q7 gap count; PR #248 file inventory.
* `.backlogit/backlogit.db` + `.backlogit/logs/*.jsonl` (both gitignored) —
  `commit_links` rows + `commit_tracked` events for the 3 archived items.

## Prior-request artifacts (confirmed present)

* Compound learning:
  `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`.
* Product-bug stash `50C90A1B` (kind: bug, archived-item mutation safety).

## Next steps

* Queue: 0 active, 2 queued — `095-S` (guard tracked docline soft keys, fold stash
  `A4BE2FAD`) and `096-S` (size-extension contract architecture spike, stash
  `D7B1B33D`). Await operator "ship next" (095-S next).
* Keep `start.ps1` (M) + `docs/decisions/2026-07-13-scratch-spike.md` (untracked)
  UNSTAGED.
