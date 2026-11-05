---
chunk_strategy: h1-h2-h3
description: 'Pre-harvest block on atomic task-ID-bound exemption contracts.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-05/stage-docs-namespace-harvest-preflight-block.md
title: 'Stage Namespace Harvest Preflight Block'
docline:
  date: '2026-11-05'
  status: resolved
---

## Gate State

The final plan review is `ADVISORY`, with explicit operator authorization
recorded at `2026-11-05T07:13:15Z` and all four controls. Ship reported the
source-entrypoint authoring gate passed with `valid: true`, zero violations,
and native exit 0. No new review was performed.

## Preflight Block

The operator requires each docs-only exemption command and its complete
P-002.3 contract to be present in artifact fields **before task creation**.
The command must emit the exact `EXEMPT_VERIFY_OK:{task_id}` marker.
`backlogit_create_item` exposes no caller-selected ID or ID reservation
field, and `backlogit add --help` confirms IDs are assigned automatically.
The allocator also consults indexed and on-disk ID history. An ID is therefore
not guaranteed until the create call returns; guessing it would risk creating
a task whose pre-existing marker names a different task.

Stage halted before creating the covering parent or any child. Native
`feature` and `chore` to task edges are supported by `.backlogit/config.yaml`;
this is not a hierarchy-type blocker. MCP relationship metadata returned
`null`, and the active literal configuration was used instead. Backlog
search found no matching covering item.

Parent action is needed to identify a supported atomic ID-binding or
reservation operation, or to obtain an explicit operator decision on a safe
alternative that preserves P-002.3 and parent-first integrity. Do not
silently create placeholder commands or mark tasks harness-satisfied.

No maintenance backlog item, shipment, application source change, claim,
build, test, or lint was produced by Stage. The plan approval record is still
uncommitted and must not be mistaken for a shipment.

## Resolution

The operator clarified a safe native authoring sequence: create each ordinary
task blocked and without an exemption label, bind its generated ID into the
complete P-002.3 body, verify the closed set and metadata, then apply the label
and queue it. Stage used that sequence without claiming or executing tasks.

The resulting feature is `214-F` and the verified queued shipment is
`215-S`; see
`docs/memory/2026-11-05/stage-docs-namespace-shipment-handoff.md`. Authoring
and Markdown checks remain pending before a Stage planning commit.
