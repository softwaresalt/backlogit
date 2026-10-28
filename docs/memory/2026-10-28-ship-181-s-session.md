---
title: "Ship 181-S session checkpoint"
source: docs/memory/2026-10-28-ship-181-s-session.md
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Ship 181-S session

* Shipment 181-S (feature 180-F, tasks 180.001-T..180.006-T) implemented on branch `feat/181-s-governed-test-budget-e4-constitution-and-top-level-guidance`.
* 180.001-T: RED contract test `TestGovernedFullSuiteBudget_Guidance` (12 anchored subtests, all RED before the migration) then GREEN.
* 180.002-T..180.005-T: constitution (MINOR 1.1.0, rationale + sync impact), AGENTS.md, README.md, copilot-instructions.md, go.instructions.md, copilot-review-instructions.md migrated to `go test -timeout=30m ./...`.
* 180.006-T: manifest checksums rotated (LF-normalized); `TestU180006_ManifestChecksumsCurrent` added.
* copilot-instructions.md has a pre-existing doubled code fence swallowing `## Commands`; the test anchors through `## Project Structure` (the file fix is out of scope: AC1 limits the changed lines).
* Local review (code-review, report-only) at a75624a3: READY, P0/P1/P2/P3 = 0/0/0/0.
* Local gates: `go vet ./...` and `go build ./cmd/backlogit` pass. `golangci-lint` is unusable locally (toolchain export-data mismatch). The full suite had only intermittent `TestU175_*_LintFix` failures under package-parallel load; each passes in isolation (environmental, unrelated to this diff).
* Next: push, PR, CI + P-018 gate, merge (merge commit), post-merge closure.
