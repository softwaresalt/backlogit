---
id: TASK-001.07.01
title: Implement JSONL event stream writer
status: Done
assignee: []
created_date: '2026-03-30 01:42'
labels: []
dependencies: []
parent_task_id: TASK-001.07
priority: high
ordinal: 7100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/events/stream.go` with:

1. `EventWriter` struct — holds file path and `sync.Mutex` for goroutine-safe writes (per P2-04)
2. `NewEventWriter(path string) *EventWriter` — constructor
3. `AppendEvent(ctx context.Context, event Event) error` — marshals event to JSON, appends as single line to `events.jsonl` using `O_APPEND` flag. Event schema: `{timestamp, actor, item_id, event_type, delta}`
4. `Event` struct with Timestamp (time.Time), Actor (string), ItemID (string), EventType (string: "state_change", "comment", "checkpoint"), Delta (map[string]any)

Create `internal/events/stream_test.go` with tests for append, JSONL format, and concurrent write safety.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 AppendEvent writes a single JSON line to events.jsonl with timestamp, actor, item_id, event_type, delta
- [ ] #2 Multiple sequential appends produce valid JSONL (one JSON object per line)
- [ ] #3 Concurrent goroutine appends do not corrupt the file (sync.Mutex protection)
- [ ] #4 File is opened with O_APPEND flag for process-level safety
- [ ] #5 Tests verify JSONL format and concurrent write safety
<!-- AC:END -->


## Implementation Notes

Completed in commit 90b24e6. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.