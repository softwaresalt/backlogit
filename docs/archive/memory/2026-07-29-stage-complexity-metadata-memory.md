---
chunk_strategy: h1-h2-h3
description: Stage memory for stash D46F3B0C complexity metadata planning, review, harvest, and shipment assembly.
doc_type: guide
schema_version: "1.0"
source: docs/memory/2026-07-29/stage-complexity-metadata-memory.md
title: Stage memory — complexity metadata
---

## Summary

Stage processed exactly one stash entry: `D46F3B0C`.

Output shipment:

* `113-S` — Add optional task complexity metadata

Created backlog hierarchy:

* `132-F` — Add optional task complexity metadata
* `132.001-T` — Define complexity WIT metadata
* `132.002-T` — Add body-preserving complexity setter
* `132.003-T` — Add complexity schema evolution
* `132.004-T` — Project complexity into SQLite index
* `132.005-T` — Add complexity query filtering
* `132.006-T` — Add CLI complexity surfaces
* `132.007-T` — Add MCP complexity surfaces
* `132.008-T` — Document complexity semantics

## Artifacts

* Deliberation:
  `docs/decisions/2026-07-29-complexity-metadata-deliberation.md`
* Implementation plan:
  `docs/exec-plans/2026-07-29-complexity-metadata-plan.md`

## Decisions

* Recommended complexity vocabulary: `trivial`, `low`, `medium`, `high`
* Task-only optional field with no default
* Store as `custom_fields.complexity`, with SQLite projection to
  `items.complexity`
* No first-slice complexity provenance fields or audit event
* No default queue-ordering change
* Complexity semantics:
  * size = implementation volume
  * complexity = implementation difficulty and uncertainty
  * priority = urgency and importance

## Review Outcome

Plan-review was run with persona subagents and appended to the plan.
Initial P1 findings were addressed in the plan before harvest:

* schema evolution before projection
* strict red/green test posture
* shared direct and transactional upsert projection
* size/complexity mutual exclusion
* clear/unset semantics
* final Go quality gates
* safety-mode and approval boundaries for destructive index rebuilds

Final plan-review decision: `PASS`.

## Traceability

* `D46F3B0C` is harvested and linked to `132-F`.
* `7F0A6E89` remains active and untouched.
* `6FA0829B` remains active and untouched.

## Validation

* `backlogit docs lint --path docs/decisions/2026-07-29-complexity-metadata-deliberation.md`
  returned valid with zero violations.
* `backlogit docs lint --path docs/exec-plans/2026-07-29-complexity-metadata-plan.md`
  returned valid with zero violations after review updates.
* `backlogit sync` completed after all backlog and shipment mutations.
* SQLite verification confirmed shipment `113-S` contains exactly `132-F` plus
  the eight harvested tasks.
