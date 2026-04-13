---
title: "Batch operations that fail silently (log + nil return) prevent callers from detecting incomplete state"
description: "Rehydrate logged a warning and returned nil when a batch tx failed after retries, giving callers no signal that the index was incomplete"
tags: [reliability, error-handling, rehydration, db, anti-pattern]
date: 2026-04-13
severity: p1
pr: "https://github.com/softwaresalt/backlogit/pull/34"
commit: e3fd907
---

## Problem

`Rehydrate` iterated batches of 100 artifacts. When a batch transaction failed
after exhausting retries, the original code did:

```go
if batchErr != nil {
    slog.Warn("rehydration batch failed, index may be partial", ...)
    // no error returned — falls through to next batch / returns nil
}
```

Callers received a nil error even though the index was known to be incomplete.
The returned `count` also excluded failed batches, but callers had no way to
distinguish "complete rebuild" from "partial rebuild" without inspecting the
warning log — which agents do not do.

## Fix

Return immediately on the first batch failure:

```go
if batchErr != nil {
    return count, fmt.Errorf("rehydration batch at offset %d: %w", i, batchErr)
}
count += batchCount
```

Since `backlogit.db` is an ephemeral cache, callers can re-invoke `Rehydrate`
on error and the partial state is acceptable — but they must be informed of it
so they can retry rather than operate on a stale/empty index.

## Principle

Any operation that builds shared state in phases MUST propagate phase failures
to callers. Log-and-continue is only appropriate when:

1. The failure is truly ignorable (cosmetic, non-functional), AND
2. The caller explicitly documents that partial results are acceptable.

A partial SQLite index is neither.
