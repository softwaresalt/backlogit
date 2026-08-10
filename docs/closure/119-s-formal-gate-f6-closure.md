---
chunk_strategy: h1-h2-h3
description: 'Operational closure record for 119-S: F6 governed-operation CLI/MCP commit-association parity.'
doc_type: closure
schema_version: "1.0"
source: docs/closure/119-s-formal-gate-f6-closure.md
title: 119-S Closure — Formal Gate F6 Governed-Operation Parity
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
- Copilot review: 9 threads raised, all addressed and resolved

## Copilot Review Cycle

Review cycle 1 (commit ed42dbb8): 8 findings — all fixed in 2c00a2a2
Review cycle 2 (commit 2c00a2a2): 1 finding — fixed in c8f508a1 + 937f6237
Review cycle 3 (commit 937f6237): 1 finding — fixed in 937f6237 (conditional upsert)
Total: 2 review-fix cycles. P0 findings: 0 residual at merge.

## Releasability: READY

- No runtime-surface changes (internal library refactor)
- No schema migration required
- No observable behavior change for well-formed calls
- Rollback: revert PR #338 (no data migration needed)

## Follow-Up Items

- Track extending the governed set to other operations beyond commit-association
  (currently only commit-association has governed: true)
- Consider adding MCP-handler-level behavioral parity tests when package boundary
  allows it (currently tested at core function level due to import cycle)

## Archive Integrity (P-007)

Archive directory verified: no deletions detected. All 6 tasks + shipment archived.
