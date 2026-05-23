---
title: "PRAGMA Introspection Allowlist Gate"
description: "Validate PRAGMA names against an explicit allowlist before string interpolation — prevents injection via corrupted sqlite_master entries"
ms.date: 2026-05-22
ms.topic: reference
tags: ["sqlite", "security", "pragma", "schema-introspection"]
source_shipment: 063-S
---

# PRAGMA Introspection Allowlist Gate

## Problem

When using `sqlite_master` to enumerate table names and then constructing
`PRAGMA table_info(?)`, `PRAGMA index_list(?)`, or similar queries by interpolating
the table/index name into the query string, the name comes from the database itself
rather than from user input. While unlikely to be adversarially corrupted, a bug or
corruption in `sqlite_master` could cause an unexpected PRAGMA command to execute.
More practically, using a gate makes the set of permitted PRAGMA operations explicit
and reviewable.

## Pattern

Maintain an explicit `allowedPragmas` set in `internal/db/gate.go`:

```go
var allowedPragmas = map[string]bool{
    "table_info":  true,
    "index_list":  true,
    "index_info":  true,
    // add new PRAGMA types here before using them in IntrospectSchema
}

func pragmaAllowed(name string) bool {
    return allowedPragmas[name]
}
```

Before constructing a PRAGMA query string, validate against the gate:

```go
if !pragmaAllowed("index_list") {
    return nil, fmt.Errorf("db: PRAGMA index_list not in allowedPragmas")
}
rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_list(%s)", tableName))
```

## Rules

* Every PRAGMA type used in `IntrospectSchema` must appear in `allowedPragmas`.
* When adding a new introspection query that uses a new PRAGMA, add the PRAGMA name
  to the gate first and get it reviewed.
* Table names from `sqlite_master` are validated by checking them against the
  `sqlite_master` result set before interpolation — do not interpolate arbitrary
  user-supplied strings into PRAGMA queries.

## Evidence

* `internal/db/schema.go` — `IntrospectSchema` function
* `internal/db/gate.go` — `allowedPragmas` gate
* `063-S` — Schema Discoverability (2026-05-22), PR #123
