---
id: TASK-001.07.02
title: Implement telemetry stream writer
status: Done
assignee: []
created_date: '2026-03-30 01:42'
labels: []
dependencies: []
parent_task_id: TASK-001.07
priority: high
ordinal: 7200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/events/telemetry.go` with:

1. `TelemetryWriter` struct — holds file path and `sync.Mutex`
2. `NewTelemetryWriter(path string) *TelemetryWriter`
3. `LogTelemetry(ctx context.Context, entry TelemetryEntry) error` — appends JSON line to `telemetry.jsonl`
4. `TelemetryEntry` struct: Timestamp (time.Time), EventType (string), Payload (map[string]any)

Create `internal/events/telemetry_test.go` with tests.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 LogTelemetry writes JSON line to telemetry.jsonl with timestamp, event_type, payload
- [ ] #2 Telemetry entries capture arbitrary payload as map[string]any
- [ ] #3 Tests verify JSONL format and payload serialization
<!-- AC:END -->
