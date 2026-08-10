---
chunk_strategy: h1-h2-h3
description: "Operational closure for 119-S (Formal Gate F6 — governed-operation CLI/MCP parity): implementation, Copilot review cycle, merge, and post-merge archival."
doc_type: closure
docline:
    date: 2026-08-10T00:00:00Z
    status: accepted
    tags:
        - operational-closure
        - formal-gate
        - 119-S
        - governed-operation-parity
schema_version: "1.0"
source: docs/closure/119-s-formal-gate-f6-closure.md
title: "119-S Formal Gate F6 — Operational Closure"
---

# 119-S Closure — Formal Gate F6

## Outcome: SHIPPED

**PR**: https://github.com/softwaresalt/backlogit/pull/338
**Merge SHA**: 5b6e7779a723eecd918a749f5e3ded3ac2ec15ba
**Merged at**: 2026-08-10T03:37:36Z
**Post-merge branch**: post-merge/119-s-formal-gate-f6

## Tasks Shipped

| Task | Title | Status |
|---|---|---|
| 106.019-T | F6/U1: characterize three-surface commit-association divergence | shipped |
| 106.020-T | F6/U2: shared commit-association core function as discrete steps | shipped |
| 106.021-T | F6/U3: route all three surfaces through the shared function | shipped |
| 106.022-T | F6/U4: registry governed markers and honest CLI mapping | shipped |
| 106.023-T | F6/U5: behavioral parity assertion for governed operations | shipped |
| 106.024-T | F6/U6: document the governed-operation parity contract | shipped |

## Files Changed

- `internal/core/commits.go` — AssociateCommit (new), LinkCommit deprecated
- `internal/cli/update.go` — --commit routed through AssociateCommit
- `internal/mcp/tools.go` — handleTrackCommit + handleUpdateItem routed through AssociateCommit
- `internal/cli/commit_association_parity_test.go` — characterization + parity tests (new)
- `internal/cli/registry_parity_test.go` — U5 governed behavioral parity + force-gates assertion
- `.autoharness/backlog-registry.yaml` — governed markers, cli_only_flags
- `docs/design-docs/governed-operation-parity.md` — U6 design doc (new)
- `.github/instructions/backlogit-yaml-header-tooling.instructions.md` — commit row updated

## Quality Gates

- `go test ./...`: all 29 packages pass
- `go vet ./...`: clean
- `golangci-lint run`: clean (5 min timeout)
- `gofmt -l .`: clean on changed files
- CI: all checks green (Docline, Markdown lint, CLI Reference Drift, test)
- Copilot review: 10 threads raised (8+1+1), all addressed and resolved

## Copilot Review Cycle

- Review cycle 1 (commit ed42dbb8): 8 findings — all fixed in 2c00a2a2
- Review cycle 2 (commit 2c00a2a2): 1 finding — fixed in c8f508a1
- Review cycle 3 (commit c8f508a1): 1 finding — fixed in 937f6237 (conditional upsert)
- Total: 3 review-fix cycles. P0/P1 residual at merge: 0.

## Releasability: READY

- No runtime-surface changes (internal library refactor)
- No schema migration required
- No observable behavior change for well-formed calls
- Rollback: revert PR #338 (no data migration needed)

## Follow-Up Items

- `4CF89803`: Extend `governed: true` to other registry operations beyond commit-association

## Archive Integrity (P-007)

Archive directory verified: no deletions detected. All 6 tasks + shipment archived.
