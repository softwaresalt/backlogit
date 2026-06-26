---
chunk_strategy: h1-h2-h3
description: Use a manually curated schema registry for production API surfaces; guard it with a reflection-based drift-detection test to prevent stale entries
doc_type: learning
docline:
    ms.date: 2026-05-22T00:00:00Z
    ms.topic: reference
    source_shipment: 063-S
    tags:
        - go
        - testing
        - schema
        - reflection
        - telemetry
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/go-patterns/manual-schema-registry-drift-detection-2026-05-22.md
title: Manual Schema Registry with Drift-Detection Test
---

## Manual Schema Registry with Drift-Detection Test

## Problem

When exposing structured schema metadata as a programmatic API (e.g., telemetry fact
table descriptors for the `backlogit telemetry schema` command), two approaches are
tempting but suboptimal:

* **Reflection at runtime**: expensive per-call, produces verbose output, hard to add
  descriptions or custom metadata per field.
* **Hardcoded JSON/YAML file**: loses compile-time verification; drifts silently as
  structs evolve.

## Pattern

**Production path — manual registry**: Define `DescribeX()` functions that return
a typed slice of descriptor structs populated by hand. Descriptions, optional flags,
and custom metadata are set explicitly:

```go
// DescribeFactTables returns the schema reference for all telemetry JSONL fact tables.
func DescribeFactTables() []FactTableSchema {
    return []FactTableSchema{
        {
            Name:       "tool-calls",
            File:       "tool-calls.jsonl",
            RecordType: "ToolCallFact",
            Fields: []FactFieldDescriptor{
                {Name: "session_id", JSONKey: "session_id", Type: "string"},
                {Name: "tool_name",  JSONKey: "tool_name",  Type: "string"},
                // ...
            },
        },
        // ...
    }
}
```

**Test path — drift detection**: Use reflection to enumerate the JSON tags on the
corresponding Go structs and compare them against the registry. The test acts as a
compile-time + test-time contract:

```go
func TestDescribeFactTables_DriftDetection(t *testing.T) {
    tables := DescribeFactTables()
    // for each table, reflect on its RecordType struct
    // compare registry field JSONKeys to struct json tags
    // fail if any tag is missing from or extra in the registry
}
```

## Rules

* The `DescribeX()` function is the production contract — it must not use reflection.
* The drift-detection test is the correctness gate — run it in CI.
* When a struct field is renamed, the JSON tag changes → the test fails → update the
  registry. This makes drift visible immediately rather than silently at runtime.
* Add `Optional: true` on registry entries for fields marked `omitempty` in their JSON tag.

## Scope of Applicability

Use this pattern when all of the following are true:

* The schema is consumed by CLI output, JSON API responses, or agent instruction surfaces.
* Fields need human-readable descriptions or optional metadata beyond the raw JSON tag.
* The underlying Go structs are in the same package, making reflection feasible in tests.

## Evidence

* `internal/telemetry/schema_ref.go` — `DescribeFactTables`, `DescribeTelemetrySQLTables`
* `internal/telemetry/schema_ref_test.go` — drift-detection tests
* `063-S` — Schema Discoverability (2026-05-22), PR #123
