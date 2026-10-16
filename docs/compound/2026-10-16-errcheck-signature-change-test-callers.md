---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Changing an error-returning signature during lint remediation: grep test callers, because local golangci-lint v1 misses what CI v2 finds"
description: "When an errcheck remediation makes a helper return an error, its test-file callers become new errcheck findings. The local go1.24-built golangci-lint reported nothing, so only CI caught it; grep all callers, including tests, before pushing."
doc_type: learning
source: docs/compound/2026-10-16-errcheck-signature-change-test-callers.md
docline:
    date: 2026-10-16T00:00:00Z
    severity: low
    tags:
        - errcheck
        - lint
        - golangci-lint
        - 158-S
---

# errcheck remediation: signature changes leak into test callers

## Problem

Shipment `158-S` fixed unchecked errors by making `writeBulkStatusResult` return `error`.
The only remaining caller was an unowned test (`internal/cli/queue_bulk_status_output_test.go`),
which now discarded the new return value. Local `golangci-lint` (v1.64.8, built with go1.24
against go1.25+ dependencies) printed no findings even for unfixed code, so the new finding
surfaced only in CI's `test / Lint` job.

## Solution

Before pushing a lint remediation that changes a return type, run `git grep -n <helper>` over
all packages including `_test.go` files, and handle the error at each caller
(`require.NoError(t, ...)` in tests). Treat a clean local lint run as unproven until the
binary is known to analyze the code at all.

## Related

* `docs/closure/158-S-175-F-post-merge-closure.md`
