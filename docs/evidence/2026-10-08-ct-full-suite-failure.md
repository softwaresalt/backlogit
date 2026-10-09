---
title: "CT full-suite failure evidence: 2026-10-08"
description: "Sanitized durable summary of the full-suite failure that motivated CT test-suite health planning."
doc_type: reference
source: docs/evidence/2026-10-08-ct-full-suite-failure.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
ingested_at: "2026-10-09T04:49:50Z"
---

# CT full-suite failure evidence: 2026-10-08

## Provenance and command

This is an archival summary of the Orchestrator's 2026-10-08 run on Windows
at main commit `d2f3ff97`, not a new test execution or a green closure record.
The original local full-suite capture is untracked; this summary preserves
the failure evidence required by the CT deliberation without depending on
that scratch file.

Command recorded by the deliberation:

```text
go test ./... -count=1 -timeout 30m
```

## Observed result

- Failing test: `TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters`.
- Assertion location: `internal/core/gate_evidence_formal_test.go:476`.
- Failure: a concurrent caller encountered the bounded gate-counter lock
  for `001.001-T`; the returned error ended in
  `backlogit: gate in progress for item`.
- Package: `github.com/softwaresalt/backlogit/internal/core`.
- Package duration: `1656.232s` (approximately 92% of the 1800s timeout).
- Exit status: `EXIT=1`.
- Whole-command elapsed time: `00:28:41.1696794`.
- No other failing test or failed package was present in the capture.

## Sanitized excerpt

The absolute temporary gateproof path is replaced with a redaction marker.
No absolute user paths, credentials, or secrets are retained.

```text
--- FAIL: TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters (11.79s)
    gate_evidence_formal_test.go:476:
        Error: Received unexpected error:
        backlogit: formal gate evidence required but could not be satisfied:
        gate evidence counter locked for 001.001-T:
        [temporary gateproof path omitted]: backlogit: gate in progress for item
        Test: TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters
        Messages: goroutine 2
FAIL
FAIL github.com/softwaresalt/backlogit/internal/core 1656.232s
EXIT=1 ELAPSED=00:28:41.1696794
```

## Planning use and limits

The CT deliberation selects test-only stabilization and a later full-suite
verification after CX merges. This failed run does not prove those fixes,
does not authorize changes to production lock semantics, and does not close
the earlier isolation-fix entries.

Related planning artifacts:

- `docs/decisions/2026-10-08-ct-test-suite-health-deliberation.md`.
- `docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md`.
