---
chunk_strategy: h1-h2-h3
description: Compound library review for shipment 063-S — evaluating entries affected by schema discoverability shipping
doc_type: closure
docline:
    mode: propose
    ms.date: 2026-05-22T00:00:00Z
    ms.topic: reference
    shipment: 063-S
ingested_at: "2026-06-26T02:32:32Z"
schema_version: "1.0"
source: docs/closure/2026-05-22-063-s-schema-discoverability-compound-refresh.md
title: 'Compound Refresh: 063-S Schema Discoverability'
---

## Compound Refresh: 063-S Schema Discoverability

**Scope**: `recent` — entries in `docs/compound/` relevant to 063-S shipped surfaces
**Context**: `063-S` shipped `db.IntrospectSchema`, `core.MetadataCatalog.SQLSchema`, and
`backlogit telemetry schema` CLI subcommand. No prior compound entries existed for these
surfaces, so this refresh focuses on confirming existing db and runtime entries remain
accurate and identifying whether a new entry should be captured.
**Mode**: `propose`

## Entries Reviewed

### Candidate: `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`

**Relevance**: The 063-S work adds new PRAGMA-based read queries against the SQLite
database. The atomic rehydration entry covers transactional integrity for write operations.

**Classification**: **keep** — the guidance on BEGIN IMMEDIATE transactions for write
durability is unaffected by the addition of read-only PRAGMA introspection. No overlap.

### Candidate: `docs/compound/runtime-errors/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md`

**Relevance**: This entry covers OOM crashes when running a stale binary against an
updated schema. The 063-S introspection is additive (new query function, no schema
migration), so the core guidance — rebuild binary before restarting the MCP server after
schema changes — remains accurate and relevant.

**Classification**: **keep** — no change needed. The guidance actually applies to the
new `allowedPragmas` entries too: if an operator deploys code with new PRAGMA gates but
runs an old binary, the old binary will reject the new PRAGMA queries. The entry's
existing guidance covers this.

### Candidate: `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`

**Relevance**: 063.005-T regenerated CLI reference docs using `go run ./cmd/gen-docs`.
This entry covers the pattern of how manual edits bypass the gen-docs tool and cause
CI drift failures.

**Classification**: **keep** — still accurate. The telemetry schema CLI reference at
`docs/cli-reference/backlogit_telemetry_schema.md` was generated via the correct path
and passed the CI drift check.

### Candidate: `docs/compound/go-patterns/f015-shipment-stash-patterns.md`

**Relevance**: Not directly relevant to 063-S schema surfaces.

**Classification**: **keep** — out of scope for this refresh.

### New Entry Candidates

**Should a new compound entry be captured for 063-S?**

Assessment: Two patterns from 063-S are worth capturing as new compound knowledge:

1. **PRAGMA introspection gate pattern** — The pattern of maintaining a `allowedPragmas`
   set before interpolating PRAGMA names from `sqlite_master` query results is a
   security-correctness pattern specific to this codebase. It's non-obvious and
   prevents SQL injection via PRAGMA string interpolation. **Worth capturing.**

2. **Manual schema registry with drift-detection test** — The pattern of maintaining a
   manually curated Go registry (`schema_ref.go`) and guarding it with a reflection-based
   drift test (`schema_ref_test.go`) that compares registry field names to struct JSON
   tags. This pattern is reusable for any future telemetry or schema reference surface.
   **Worth capturing.**

## Recommendations

| Entry | Classification | Action |
|---|---|---|
| `atomic-rehydration-sqlite-transaction-2026-04-08.md` | keep | No change needed |
| `stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md` | keep | No change needed |
| `cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` | keep | No change needed |
| `f015-shipment-stash-patterns.md` | keep | Out of scope |
| **New: PRAGMA introspection gate pattern** | new | Capture in `docs/compound/database-issues/` |
| **New: manual schema registry drift-detection pattern** | new | Capture in `docs/compound/go-patterns/` |

## New Entry: PRAGMA Introspection Gate Pattern

**Recommended file**: `docs/compound/database-issues/pragma-introspection-allowlist-gate-2026-05-22.md`

**Summary**: When interpolating PRAGMA names derived from `sqlite_master` query results,
validate the name against an explicit `allowedPragmas` set before constructing the
query string. This prevents an adversarial or corrupt `sqlite_master` entry from
injecting arbitrary PRAGMA commands. The gate is enforced in `internal/db/gate.go`.
New PRAGMA types (e.g., `index_list`, `index_info`) must be added to `allowedPragmas`
before they can be used in schema introspection.

**Evidence**: `internal/db/schema.go` (`IntrospectSchema`), `internal/db/gate.go`
(`allowedPragmas`), shipped in 063-S (2026-05-22).

## New Entry: Manual Schema Registry with Drift-Detection Test

**Recommended file**: `docs/compound/go-patterns/manual-schema-registry-drift-detection-2026-05-22.md`

**Summary**: When exposing a schema reference for structs as a programmatic API (e.g.,
telemetry fact table descriptors), maintain the registry manually rather than using
reflection for the production path. Guard it with a reflection-based drift-detection
test that compares registry field names to struct JSON tags. This separates runtime
behavior (deterministic, no reflection cost) from correctness assurance (test-time
reflection). Pattern: `DescribeFactTables()` registry + `TestDescribeFactTables_DriftDetection`.

**Evidence**: `internal/telemetry/schema_ref.go`, `internal/telemetry/schema_ref_test.go`,
shipped in 063-S (2026-05-22).

## Follow-up

Create the two new compound entries above during this closure pass or delegate to a
future maintenance session. They are advisory captures, not blockers for this closure PR.
