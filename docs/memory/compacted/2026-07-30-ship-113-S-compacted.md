---
chunk_strategy: h1-h2-h3
description: "Compacted Ship context for shipment 113-S complexity metadata."
doc_type: memory
schema_version: "1.0"
source: docs/memory/compacted/2026-07-30-ship-113-S-compacted.md
title: Ship 113-S compacted context
created_at: 2026-07-30T23:48:00Z
type: compacted-memory
release_unit: 113-S
input_memory:
  - docs/memory/2026-07-30/ship-113-S-pr-ready-memory.md
---

# Ship 113-S compacted context

Shipment 113-S adds optional task-only complexity metadata. The implementation mirrors the size seam across WIT metadata, body-preserving mutation, SQLite schema projection, query filtering, CLI, MCP, and generated CLI references.

All tasks `132.001-T` through `132.008-T` are done. The local review fixed legacy generated header-def upgrade behavior, task-only projection and filtering, MCP non-string input validation, and semantic wording so priority is documented as urgency.

Verification passed locally: `go test ./...`, `go vet ./...`, `golangci-lint run`, changed-file `gofmt -l`, and `go build ./cmd/backlogit`. Full `gofmt -l .` still reports pre-existing files outside the changed set.

Compaction assessment preserved the newest 113-S checkpoint and did not archive active or unrelated artifacts.
