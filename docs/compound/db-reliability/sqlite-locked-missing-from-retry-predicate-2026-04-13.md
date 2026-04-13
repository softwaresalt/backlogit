---
title: "SQLITE_LOCKED must be included alongside SQLITE_BUSY in retry predicates"
description: "isSQLiteBusy initially omitted SQLITE_LOCKED, meaning shared-cache lock contention would bypass the retry wrapper entirely"
tags: [sqlite, retry, concurrency, db]
date: 2026-04-13
severity: p1
pr: "https://github.com/softwaresalt/backlogit/pull/34"
commit: e3fd907
---

## Problem

The `isSQLiteBusy` predicate in `internal/db/retry.go` was written to match
`"SQLITE_BUSY"` and `"database is locked"` but omitted `"SQLITE_LOCKED"`.

SQLite raises `SQLITE_LOCKED` (error code 6) for shared-cache lock contention —
distinct from `SQLITE_BUSY` (error code 5), which is inter-connection/inter-process
lock contention. The PR description and `RetryWrite` doc comment claimed both codes
were handled, but the predicate only matched `SQLITE_BUSY`.

Callers using shared-cache mode or hitting intra-process lock contention would
receive a non-retryable error even though the write should have been retried.

## Fix

```go
func isSQLiteBusy(err error) bool {
    if err == nil {
        return false
    }
    msg := err.Error()
    return strings.Contains(msg, "SQLITE_BUSY") ||
        strings.Contains(msg, "SQLITE_LOCKED") ||
        strings.Contains(msg, "database is locked")
}
```

## Prevention

- When writing string-based error predicates for SQLite, enumerate all related
  error codes from the SQLite docs (SQLITE_BUSY = 5, SQLITE_LOCKED = 6).
- Add a table-driven `TestIsSQLiteBusy` that explicitly tests each string variant.
- Document the covered codes in the function comment.
