---
doc_type: closure
schema_version: "1.0"
title: Shipment 121-S Closure — Default Workspace Dir Rename
created_at: 2026-08-10T21:55:00Z
shipment: 121-S
feature: 135-F
pr: "345"
merge_commit: 99e8ecc8
---

# Shipment 121-S Closure

## Merge Summary

- **PR**: [#345](https://github.com/softwaresalt/backlogit/pull/345)
- **Merge commit**: 99e8ecc8 (merge commit, P-009 compliant)
- **Merged**: 2026-08-10
- **Branch**: eat/135-f-workspace-default-dir-rename

## Scope Delivered

| Item | Title | Status |
| --- | --- | --- |
| 135-F | Default workspace directory rename to .backlog | archived |
| 135.001-T | Test-matrix scaffolding for dual-root resolver | archived |
| 135.002-T | Implement dual-root resolver + conflict detection | archived |
| 135.003-T | Path inventory and routing update | archived |
| 135.004-T | Init command update for .backlog default | archived |
| 135.005-T | Migration command --workspace-dir | archived |
| 135.006-T | Doctor root-conflict diagnostics | archived |
| 135.007-T | MCP server dual-root handling | archived |
| 135.008-T | Documentation and migration guide | archived |
| 135.009-T | Instructions and harness update | archived |

## Runtime Verification

All quality gates passed locally and in CI:

- go test ./... — PASS (all 5 CI runs green)
- go vet ./... — PASS
- golangci-lint run — PASS
- gofmt -l . — PASS (changed files only)
- CI checks: Detect code changes, Markdown lint, test, Docline frontmatter gate, CLI Reference Drift — all SUCCESS

## Copilot Review Gate (P-014)

Total Copilot review cycles: 5

- Review 1 (30e1bd3e): 6 unresolved threads → all addressed
- Review 2 (c0ca5045): 2 unresolved threads → addressed in d9cd012e
- Review 3 (0eeae380): 3 new threads → addressed in ce51ea26, a03b09f4
- Review 4 (ce51ea26): 1 new thread → addressed in 04bf082c
- Review 5 (04bf082c): 1 new thread → addressed in bcedc93e
- Review 6 (bcedc93e): CLEAN — 0 unresolved threads

Final gate: review at bcedc93e = HEAD, 0 unresolved Copilot threads, no pending reviews.

## Findings Addressed

| Thread | File | Fix commit |
| --- | --- | --- |
| PRRT_kwDORzozKM6X_g7J | internal/telemetry/workspace_root.go | d9cd012e |
| PRRT_kwDORzozKM6X_g7a | internal/db/workspace_root.go | d9cd012e |
| PRRT_kwDORzozKM6X_g7x | internal/core/workspace.go (partial) | 0eeae380 |
| PRRT_kwDORzozKM6X_g8V | internal/core/migrate_workspace_dir.go | d9cd012e |
| PRRT_kwDORzozKM6X_g8_ | internal/core/workspace_literal_guard_test.go | d9cd012e |
| PRRT_kwDORzozKM6X_rv8 | internal/core/canonical_scan.go | 0eeae380 |
| PRRT_kwDORzozKM6X_rwd | internal/core/shipment_verify.go | 0eeae380 |
| PRRT_kwDORzozKM6YBUHO | internal/cli/root.go | a03b09f4 |
| PRRT_kwDORzozKM6YCObM | internal/core/archive.go | ce51ea26 |
| PRRT_kwDORzozKM6YCsDf | internal/mcp/server.go | 04bf082c |
| PRRT_kwDORzozKM6YC3uj | internal/core/migrate_workspace_dir.go | bcedc93e |

## Rollback Trigger

If go test ./... fails on origin/main within 24 hours of merge, revert via
git revert 99e8ecc8 on a hotfix branch.

## Post-Merge Observation Window

Owner: ship agent. Duration: 24 hours after merge (until 2026-08-11T22:00Z).
Healthy signal: no test failures reported against origin/main.
