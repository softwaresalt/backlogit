---
chunk_strategy: h1-h2-h3
doc_type: closure
schema_version: "1.0"
source: docs/closure/118-S-closure.md
title: "118-S closure — F4 durable dependency type persistence"
---

# 118-S Operational Closure — F4 Durable Dependency Type Persistence

## Summary

Shipment 118-S shipped as PR #335, merged as `39a3dbaf` on 2026-08-09.

## Scope

Items 106.012-T through 106.018-T (F4/U1–U7). No successor shipments touched.

## Gate Outcomes

| Gate | Outcome |
|---|---|
| Characterization tests (U1) | PASS — RED at HEAD, GREEN after implementation |
| Build (`go build ./...`) | PASS |
| Tests (`go test ./...`) | PASS — all 29 packages |
| Vet (`go vet ./...`) | PASS |
| Lint (`golangci-lint run`) | PASS |
| Format (`gofmt -l .`) | PASS on changed files |
| CI (GitHub Actions) | All 5 checks PASS |
| Copilot review | 2 rounds; 8 threads addressed and resolved |
| P-014 §1.9 gate | PASS (review covers HEAD b827ade4, 0 unresolved threads) |
| Merge strategy | Merge commit (P-009 compliant) |

## Copilot Review Rounds

**Round 1 (on a5ad99c1)**: 7 findings addressed:
- P0: fix `dep.ID` format arg in artifact_references (go vet clean)
- P1: validate depType in AddDependency before mutation
- P2: update frontmatter when re-adding edge with different type
- P2: reject unsupported dep entry types (integers) in toDependencyEdges
- P2: validate non-string type field in dependencyEdgeFromMap
- P2: backward-compat SQLite row explanation (already addressed in queries.go)
- P3: parity test CLI surface acknowledgment

**Round 2 (on b827ade4)**: 2 threads:
- Resolved `PRRT_kwDORzozKM6Xscrg` (default branch was in f8187c33)
- Fixed `scripts/migrate-ids.go` rewriteIDSlice to handle typed dep objects

## Runtime Verification

- All 29 packages pass locally and in CI
- Parity contract test (tests/contract/dep_type_parity_test.go) exercises relates_to edge through both CLI and MCP surfaces
- Characterization tests confirm dep_type survives Rehydrate cycle

## Files Modified

**New files:**
- `internal/errors/dependency_errors.go` — ErrInvalidDependencyType, ValidDependencyType
- `internal/db/rehydration_deptype_test.go` — U1 characterization tests
- `tests/contract/dep_type_parity_test.go` — U6 parity contract test
- `docs/design-docs/dependency-type-durability.md` — U7 design doc

**Modified files (production):**
- `internal/models/artifact.go` — DependencyEdge type, Dependencies field, serializeDependencies
- `internal/models/frontmatter.go` — toDependencyEdges, dependencyEdgeFromMap, load-edge validation
- `internal/db/rehydration.go` — upsertDependencyTx reads type from DependencyEdge
- `internal/db/merge_sync.go` — dep loop migrated to DependencyEdge
- `internal/db/queries.go` — backward-compat JSON fallback for legacy string arrays
- `internal/core/dependencies.go` — AddDependency/RemoveDependency typed edges
- `internal/core/artifacts.go` — createOptions, WithDependencies, UpdateArtifact
- `internal/core/shipment.go` — clone dependencies
- `internal/core/artifact_references.go` — dep rewrite; removed obsolete SQLite pre-fetch
- `internal/cli/migrate.go` — legacy import dep mapping to DependencyEdge
- `scripts/migrate-ids.go` — rewriteIDSlice handles typed dep objects

**Modified files (instruction/config):**
- `.github/instructions/backlogit-yaml-header-tooling.instructions.md`
- `.github/instructions/backlogit-sql-schema.instructions.md`

## Post-Merge State

- 118-S: shipped
- 106.012-T through 106.018-T: done
- origin/main: 39a3dbaf
- Successor: 119-S (queued, not claimed)
