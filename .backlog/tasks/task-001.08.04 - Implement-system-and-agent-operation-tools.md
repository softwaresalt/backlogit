---
id: TASK-001.08.04
title: Implement system and agent operation tools
status: Done
assignee: []
created_date: '2026-03-30 01:43'
labels: []
dependencies: []
parent_task_id: TASK-001.08
priority: high
ordinal: 8400
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add to `internal/mcp/tools.go`:

1. `backlogit_sync_index` tool — no parameters. Handler: calls `db.Rehydrate` to force-rebuild index.db. Returns JSON with indexed count and timing.
2. `backlogit_append_comment` tool — parameters: item_id (required), comment (required). Handler: calls `events.AppendEvent` with event_type "comment".
3. `backlogit_log_telemetry` tool — parameters: event_type (required), payload (required). Handler: calls `events.LogTelemetry`.
4. `backlogit_save_memory` tool — parameters: key (required), summary (required). Handler: calls `events.SaveMemory`.
5. `backlogit_create_checkpoint` tool — parameter: state_dump (required). Handler: calls `events.CreateCheckpoint`. Returns checkpoint file path.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit_sync_index triggers full rehydration and returns indexed item count
- [ ] #2 backlogit_append_comment writes event to events.jsonl
- [ ] #3 backlogit_log_telemetry writes entry to telemetry.jsonl
- [ ] #4 backlogit_save_memory updates memories.json
- [ ] #5 backlogit_create_checkpoint writes state dump to checkpoints/
<!-- AC:END -->


## Implementation Notes

Completed in commit a49b9dd. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.