---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Commit the shipment archive record after backlogit shipment ship rewrites it"
description: "If the closure commit stages .backlogit/archive/<id>-S.md before the governed ship finishes, main gets a stale status: active record. Stage and commit archive records only after shipment ship returns, and verify archived_status: shipped on main."
doc_type: learning
source: docs/compound/2026-10-21-commit-archive-record-after-governed-ship.md
docline:
    date: 2026-10-21T00:00:00Z
    severity: medium
    tags:
        - shipment
        - post-merge-closure
        - archive
        - ship-agent
        - workflow
---

# Commit the shipment archive record after the governed ship rewrites it

## Problem

In the 184-S and 194-S closures, the closure PR carried `.backlogit/archive/<id>-S.md`
as it was before `backlogit shipment ship` finished. After merge, `main` held
`status: active` with no `archived_status: shipped` or merge `commit`. A third
correction PR was needed each time (PR 557 for 184-S, PR 563 for 194-S).
Copilot's review of the 184-S closure PR flagged the stale record, and the agent
dismissed it as unreproducible without checking `main`.

## Root Cause

The governed ship rewrites the archive record on disk, and that can take 3 to 15
minutes under load. The closure branch was staged while the ship was still
running or before the rewritten file was re-read, so the commit captured the
pre-rewrite copy.

## Resolution

Wait until `backlogit shipment ship <id> --sha <merge>` returns `shipped`, then
stage the archive records and commit. Afterwards, run
`git show origin/main:.backlogit/archive/<id>-S.md` and confirm
`status: archived`, `archived_status: shipped` and the merge `commit`. If the
record is stale, open a small correction PR.

## Prevention

* Treat a review comment about a stale archive record as a claim to check on `main`, not to dismiss.
* The Ship closure step should name this ordering explicitly: ship first, then stage archive records.
* The 189-S, 190-S, 191-S, 192-S, 193-S and 185-S closures followed this order and needed no correction.
