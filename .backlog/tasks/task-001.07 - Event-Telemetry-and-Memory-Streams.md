---
id: TASK-001.07
title: 'Event, Telemetry, and Memory Streams'
status: To Do
assignee: []
created_date: '2026-03-30 01:37'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.03
parent_task_id: TASK-001
priority: high
ordinal: 7000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement the JSONL append-only event stream for state changes and comments, the telemetry stream for agent execution metrics, efficient tail-reading for recent events by item ID, and memory/checkpoint persistence.

Per review P1-07: Includes `SaveMemory` (writing to `memories.json`) and `CreateCheckpoint` (writing timestamped files to `checkpoints/`).
Per review P2-04: Uses `sync.Mutex` for goroutine-level concurrent append safety and `O_APPEND` flag for process-level safety.
<!-- SECTION:DESCRIPTION:END -->
