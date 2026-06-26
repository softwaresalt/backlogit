---
chunk_strategy: h1-h2-h3
description: 'Post-merge closure record for shipment S001 and PR #8.'
doc_type: closure
docline:
    author: Copilot
    estimated_reading_time: 4
    keywords:
        - closure
        - shipment
        - feature-016
        - numeric-ids
        - migration
    ms.date: 2026-04-07T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T02:32:32Z"
schema_version: "1.0"
source: docs/closure/2026-04-07-f016-s001-numeric-ids-closure.md
title: F016 S001 numeric IDs closure
---

## Post-merge closure

This closure record captures the merge, shipment closeout, and operational
readiness state for shipment `S001` and PR `#8`.

| Field | Value |
|-------|-------|
| Feature | `F016` |
| Shipment | `S001` |
| Branch | `ship/s001-f016-numeric-ids` |
| PR | `#8` |
| Merge commit | `2ec3e57e7831e0b66ae986398d796a02d820da26` |
| Result | `READY` |

## Healthy signals

* PR `#8` merged cleanly into `main` with a merge commit.
* Both CI matrix jobs passed on the final pushed branch state.
* Shipment `S001` transitioned to `shipped`.
* Released scope archived cleanly:
  `F016.T001` through `F016.T007`, `DL003`, `F016`, and `S001`.
* Unreleased follow-on work returned to backlog explicitly:
  `F016.T008` through `F016.T014` and `F016.R001`.

## Failure signals

Watch for these signals during the first migration and first normal queue use:

* migration script errors while rewriting backlog artifacts or logs
* missing or stale cross-file references after migration
* queue queries showing orphaned or missing returned items unexpectedly
* MCP tool lookup regressions for shipment or adoption operations

## Monitoring plan

Use the first post-merge validation window to check:

* the migration path on the target workspace before routine backlog work resumes
* queue visibility for returned items `F016.T008` through `F016.T014`
* archive visibility for the shipped scope
* normal MCP and CLI use for `get`, `move`, `shipment`, and adoption surfaces

## Rollback trigger

Rollback is warranted if the post-merge migration corrupts workspace references,
breaks normal artifact lookup, or makes returned backlog items inaccessible.

If that happens:

* stop further migration attempts
* restore the workspace from Git or backup state taken before migration
* revert the merge commit on `main` if the runtime surface itself is the source
  of failure rather than the workspace data

## Validation window

Validation window: the first migration run and first normal backlog session
after upgrading to the merged code.

Owner: Derek Williams

## Follow-up and notes

No unresolved review or CI items remain from PR `#8`.

The migration script is intended to run after upgrade, not before. Upgrade to
the merged code first, then run migration once against the workspace.

The unrelated local-only follow-up experiments were intentionally moved off the
shipment branch into git stash `wip/s001-local-followups` so the merge branch
could close cleanly.

## Related artifacts

* Review artifact: `.backlogit/queue/F016.R001-branch-review.md`
* Compound learning:
  [`docs/compound/workflow-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md`](../compound/workflow-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md)
* PR: <https://github.com/softwaresalt/backlogit/pull/8>
