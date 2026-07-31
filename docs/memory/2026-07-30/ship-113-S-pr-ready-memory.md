---
chunk_strategy: h1-h2-h3
description: "Ship session memory for shipment 113-S complexity metadata through local review readiness."
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-07-30/ship-113-S-pr-ready-memory.md
title: "Ship 113-S PR-ready session memory"
type: session-memory
timestamp: 2026-07-30T23:45:00Z
agent: ship
task: shipment-113-S
---

# Ship 113-S memory

## Outcome

Implemented shipment 113-S on branch `feat/132-F-complexity-metadata` through local review readiness. All shipment tasks `132.001-T` through `132.008-T` are done and associated with commits.

## Files modified

* `.backlogit/header-def.yaml`, `.backlogit/queue/132-F.md`, `.backlogit/queue/113-S.md`, `.backlogit/archive/132.001-T.md` through `.backlogit/archive/132.008-T.md`
* `internal/config/defaults.go`, `internal/config/headerdef.go`, and complexity/default tests
* `internal/core/artifact_complexity.go`, `internal/core/artifact_size.go`, and complexity tests
* `internal/db/schema_gen.go`, `internal/db/queries.go`, `internal/db/upsert_projection.go`, `internal/db/upsert_tx.go`, and projection/query tests
* `internal/cli/list.go`, `internal/cli/update.go`, generated CLI reference docs, and CLI tests
* `internal/mcp/tools.go` and MCP complexity tests

## Decisions

* Complexity is task-only optional planning metadata with allowed values `trivial`, `low`, `medium`, `high`.
* Complexity is stored in `custom_fields.complexity` and projected to `items.complexity` only for task artifacts.
* Legacy generated `header-def.yaml` loads with complexity injected in memory; customized schemas are preserved.
* `--complexity ""` and MCP `complexity: ""` explicitly clear task complexity, but still require the task schema to define complexity.

## Verification

* Red tests were observed for legacy header-def upgrade, task-only DB projection/filtering, non-string MCP complexity input, non-task clears, and priority wording.
* Targeted package tests passed after fixes.
* Full gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`, changed-file `gofmt -l`, and `go build ./cmd/backlogit`.
* Full `gofmt -l .` still lists pre-existing unformatted files outside this shipment; changed Go files are gofmt-clean.

## Review

Local review found P1/P2 issues around legacy config upgrade, task-only projection, MCP non-string mutation clearing, and priority wording. Commit `12ea3e36` fixed those findings. Current local review readiness is `READY` with no unresolved P0/P1 findings.

## Next steps

Push the branch, open the PR against `main`, request Copilot review, poll the four required checks, resolve Copilot threads if any, run the GitHub readiness gate, then halt for merge approval. Do not merge without operator approval.
