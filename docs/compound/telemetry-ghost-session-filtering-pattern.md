---
title: Ghost session filtering pattern in Go telemetry reporters
description: How to exclude zero-activity sessions from telemetry aggregation while preserving them for auditability
ms.date: 2026-05-08
ms.topic: reference
tags: [telemetry, go, reporter, ghost-session, filtering]
---

## Problem

Telemetry trend reports were inflated by "ghost sessions" — sessions that were initialized but had no model interaction (zero tokens, zero model calls, zero tool calls). These sessions distorted per-session averages and made trend data unreliable.

## Solution

Define a single exported predicate and apply it at report-generation time, not at harvest time:

```go
// IsGhostSession reports whether s is a fully inactive (ghost) session.
// A ghost session has zero total tokens, zero model calls, and zero tool calls.
func IsGhostSession(s SessionSummaryRecord) bool {
    return s.TotalTokens == 0 && s.ModelCalls == 0 && s.ToolCalls == 0
}
```

In `GenerateTrendReport`, guard **both** the aggregation loop and the finalisation loop:

```go
// Aggregation loop
for _, s := range sessions {
    if IsGhostSession(s) {
        continue
    }
    // ... accumulate group stats
}

// Finalisation loop (taskCounts / peakCounts)
for _, s := range sessions {
    if IsGhostSession(s) {
        continue
    }
    // ... accumulate counts for optional fields
}
```

For session list formatters, show ghost sessions but mark them visually:

```go
sessionDisplay := s.SessionID
if IsGhostSession(s) {
    sessionDisplay = s.SessionID + " [empty]"
}
```

## Why Not Filter at Harvest?

Ghost sessions remain in JSONL for auditability. They are evidence that a session was opened but produced no work — useful for diagnosing tooling issues or abandoned sessions. Filtering at display time is the right boundary.

## Distinction from Partial Sessions

`ValidateSessionSummary` (harvest gate) rejects sessions with `ToolCalls > 0 && TotalTokens == 0` — partial sessions where tool calls occurred but no tokens were recorded (likely a harvesting or instrumentation gap). Ghost sessions are different: they have zero everything and are valid but empty.

## Two-Loop Requirement

`GenerateTrendReport` iterates sessions twice: once to accumulate group metrics and once to accumulate `taskCounts`/`peakCounts` for optional average fields. Both loops must guard against ghost sessions or the optional-field averages will use a different (inflated) denominator than the main `Sessions` count.

## Test Pattern

```go
func writeGhostTrendJSONL(t *testing.T, workspacePath string) {
    // 3 active sessions + 2 ghost sessions, same date group
    // Verify: g.Sessions == 3, g.AvgTokensSession == 1000 (not 600)
}
```

Key assertion: `g.Sessions` must equal the count of non-ghost sessions only. The average must use that count as the denominator.
