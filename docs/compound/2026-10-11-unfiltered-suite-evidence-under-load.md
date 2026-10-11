---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Unfiltered full-suite evidence is valid only on a quiet workspace, and an unnamed panic is not a pass"
description: "In 197-S, an unfiltered go test ./... run concurrent with review subagents failed on lock-timing tests, and a quiet run failed on an unnamed panic. Gate evidence needs a quiet host and a named failing test before any classification."
doc_type: learning
source: docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md
docline:
    date: 2026-10-11T01:48:00Z
    severity: medium
    tags:
        - testing
        - full-suite
        - ship
        - 197-S
---

# Unfiltered full-suite evidence under load

## Observation

In 197-S, the unfiltered `go test -timeout=30m ./...` gate gave three kinds of result at different HEADs:

* A run concurrent with review subagents failed two lock-timing tests. Both passed in isolation.
* A quiet run failed one package (`internal/core`) with a panic line. The test name was not captured, so the origin is unproven. The same package passed alone.
* Two later quiet runs exited 0.

## Rule

1. Run the unfiltered suite on a quiet workspace, with no concurrent review or build subagents.
2. Save the full output with `-v` or `-json`, so a panic carries its failing test name. Do not classify a run whose failing test is unnamed.
3. A failure that passes in isolation is a load-sensitivity finding to capture. It is not a waiver. Capture it with the test names and the run context.
4. Count only a run whose exit code and package list are both recorded for the exact HEAD under gate.

## Why it matters

Gate evidence is only as good as its attribution. A classified failure in an unnamed panic can hide a real regression. The 197-S closure carries the unresolved case as follow-up `63909DAE`.
