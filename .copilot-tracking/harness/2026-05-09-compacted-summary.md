---
type: compacted-summary
date: 2026-05-09
source_count: 2
source_date_range: "2026-04-04 to 2026-04-05"
---

# Compacted Summary: Test Harness Artifacts

## Sources

- `F013-harness.md` (archived to `archive/harness/`)
- `F015-harness.md` (archived to `archive/harness/`)

## Key Decisions

- F013 used characterization-first testing for YAML workflow infrastructure (ci.yml, release.yml)
- F015 used standard TDD harness with 44 tests across 7 test files covering 6 packages

## Outcomes

### F013: Release Pipeline Fix

- 5 work items, 4 test functions in `tests/integration/ci_compliance_test.go`
- Branch: `013-release-pipeline-fix`
- Validated: tag filter glob, Go version matrix, SHA pins, persist-credentials

### F015: Two-Agent Workflow Refactor

- 44 tests across 7 files: shipment config defaults, shipment errors, core CRUD, CLI, stash JSONL, contract tests, integration tests
- Branch: `015-two-agent-workflow-refactor`
- Key packages: `internal/config`, `internal/errors`, `internal/core`, `internal/cli`, `internal/stash`, `tests/contract`, `tests/integration`

## Preserved Context

- F013 harness was characterization-first (tests describe existing behavior before fixing)
- F015 harness covered the full shipment lifecycle including JSONL-backed stash
