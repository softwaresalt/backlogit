---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "PowerShell fixture runners need an exhaustive up-front input-validation checklist"
description: "In 176-S, ten Copilot review rounds each found one more validation gap in the PowerShell lint-runner bootstrap scripts. Validate strictly up front against a complete checklist, and run the Go suite with explicit package roots because go test ./... walks a huge logs/ directory."
doc_type: learning
source: docs/compound/2026-10-15-powershell-fixture-runner-strict-input-validation.md
docline:
    date: 2026-10-15T21:55:00Z
    severity: medium
    tags:
        - powershell
        - validation
        - copilot-review
        - testing
        - 176-S
---

# PowerShell fixture runners need an exhaustive up-front input-validation checklist

## Problem

Shipment 176-S (PR #504) added `scripts/verify-task-lint.ps1`, `scripts/verify-baseline-lint.ps1`, and `scripts/verify-terminal-lint.ps1`, with Go harnesses in `tests/runner_bootstrap_*_175_099_test.go`, `..._175_100_test.go`, and `..._175_101_test.go`. Copilot review took ten rounds. Each round found one more gap in how the runners parse and compare fixture JSON. Patching only the flagged line each round is the whack-a-mole loop described in `2026-07-16-copilot-review-loop-complete-class-enumeration.md`.

## Checklist to apply before the first review request

* **Strict types.** Reject any fixture member whose JSON type differs from the contract, including null members and arrays where objects are expected.
* **Duplicate and case-colliding keys.** `ConvertFrom-Json` silently collapses them. Detect them in the raw token stream and fail.
* **Ordinal comparison.** Fingerprints and case-sensitive identifiers use ordinal, case-sensitive comparison. Default PowerShell `-eq` and `-contains` are case-insensitive.
* **Exact numeric tokens.** Compare the numeric token text, not a parsed double. Parsing hides precision loss and `1` vs `1.0` differences.
* **Throwing UTF-8 decoder.** Use `UTF8Encoding($false, $true)` so invalid bytes raise instead of becoming U+FFFD.
* **Single read for hash and parse.** Read the file bytes once, then hash and parse those same bytes. Hashing one read and parsing another is a TOCTOU gap.
* **Relative path resolution.** Resolve fixture paths against an explicit root and reject escapes. Do not rely on the current directory.
* **No pipe-joined keys.** Joining fields with `|` for map keys can collide. Use a structured key or an unambiguous length-prefixed encoding.

## Process rule

Write the checklist into the task's acceptance criteria and test each item in the Go harness before requesting review. When a reviewer flags one item, sweep the whole checklist across all three scripts in the same commit.

## Test command on this repository

`go test ./...` walks the roughly 160k-file `logs/` directory, which is very slow. Run `go test -count=1 ./cmd/... ./internal/... ./tests/...` instead. Use `-count=1` to avoid cached results. This also follows `2026-10-11-unfiltered-suite-evidence-under-load.md` on quiet-host gate evidence.

## Related

* `docs/compound/2026-07-16-copilot-review-loop-complete-class-enumeration.md`
* `docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md`
