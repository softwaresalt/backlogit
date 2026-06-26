---
chunk_strategy: h1-h2-h3
description: Correct pattern for reading arbitrarily large JSONL lines without scanner token-too-long panics, including the EOF last-line gotcha.
doc_type: learning
docline:
    feature: 052-F
    ms.date: 2026-05-09T00:00:00Z
    ms.topic: reference
    pr: "92"
    tags:
        - go
        - bufio
        - jsonl
        - telemetry
        - runtime-error
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/runtime-errors/bufio-scanner-readline-eof-pattern-2026-05-09.md
title: bufio.Reader + ReadString EOF handling pattern for unbounded JSONL lines
---

## Problem

`bufio.NewScanner` has a default per-line limit of 64KB. While `sc.Buffer`
can raise that limit, backlogit's bounded JSONL readers use a 1 MiB cap as a
practical ceiling. Copilot CLI `events.jsonl` files contain full conversation
context dumps that can exceed even that cap, causing:

```text
bufio.Scanner: token too long
```

This crashed `backlogit telemetry harvest`: the scanner error was returned and
the harvest loop exited early without processing the remaining sessions.

## Root Cause

`ParseSessionEvents` in `internal/telemetry/session_events.go` used the
default `bufio.NewScanner` with no `sc.Buffer` call. Any single JSONL line
longer than the scanner's internal limit triggered the error.

## Fix Pattern

Replace `bufio.NewScanner` with `bufio.NewReader` + `ReadString('\n')`:

```go
reader := bufio.NewReader(f)
for {
    line, readErr := reader.ReadString('\n')
    isEOF := errors.Is(readErr, io.EOF)
    if readErr != nil && !isEOF {
        return nil, fmt.Errorf("reading %s: %w", path, readErr)
    }

    line = strings.TrimSpace(line)
    if line != "" {
        // process line
    }

    if isEOF {
        break
    }
}
```

**Critical gotcha**: `ReadString('\n')` returns the last line AND `io.EOF`
simultaneously when the file has no trailing newline. Always check `isEOF`
AFTER processing the line content, not before. Otherwise the last line of
every file without a trailing newline is silently dropped.

## When to Use sc.Buffer Instead

`sc.Buffer(make([]byte, 1<<20), 1<<20)` is adequate for files where lines
are known to be bounded (e.g., event queues, stash JSONL). It is NOT adequate
for `events.jsonl` / session telemetry files which contain conversation history
of unbounded length.

Rule of thumb:
- Use `sc.Buffer` for files that store structured short records (events, stash)
- Use `bufio.NewReader` + `ReadString` for files that may contain large blobs
  (session context, conversation history)

## Affected Files in backlogit

| File | Fix applied |
|---|---|
| `internal/telemetry/session_events.go` | `bufio.NewReader` + `ReadString` loop |
| `internal/events/reader.go` | `sc.Buffer(1<<20, 1<<20)` |
| `internal/stash/jsonl.go` | `sc.Buffer(1<<20, 1<<20)` |

Already hardened (no change needed):
- `internal/telemetry/hook_events.go`: has `sc.Buffer(1<<20, 1<<20)`
- `internal/telemetry/correlator.go`: has `sc.Buffer(1024*1024, 1024*1024)`
- `internal/db/telemetry_schema.go`: uses `bufio.NewReader`

## Detection Strategy

When fixing a `bufio.Scanner: token too long` crash in one file, always grep
the entire codebase for other `bufio.NewScanner` instances:

```bash
grep -rn "bufio.NewScanner" --include="*.go" .
```

Each site must be evaluated: is the input bounded? If not, apply the
`bufio.NewReader` + `ReadString` pattern or add `sc.Buffer`.

## Validation

End-to-end harvest of 97 JSONL files (480MB, 64 sessions, 295M tokens)
completed without any scanner overflow. PR #92, commit `b408cea`.
