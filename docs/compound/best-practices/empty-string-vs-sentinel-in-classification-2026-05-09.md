---
title: "Empty-String vs Sentinel Discipline in Classification Functions"
problem_type: best_practice
category: best_practice
component: models
root_cause: type_mismatch
resolution_type: code_fix
severity: medium
message: "Classification functions should return empty string for empty input, not a sentinel; keep display labels at render boundaries only"
file_path: "internal/telemetry/records.go"
resolved: true
tags: [classification, sentinel, empty-string, aggregation, omitempty, telemetry, go]
date: 2026-05-09
---

## Empty-String vs Sentinel Discipline in Classification Functions

## Problem

A classification function (`DeriveModelClass`) returned the sentinel value `"other"` for
both empty input and non-empty unrecognised input. This collapsed two conceptually different
states — "no model data at all" and "a real but unknown model" — into the same bucket,
making it impossible for aggregation code to distinguish them.

## Symptoms

- `--by class` aggregation placed ghost/metadata-only sessions (zero token sessions with
  empty `TokensByModel`) in the `other` bucket instead of `(unknown)`
- Sessions with a genuine but unmapped model name and sessions with no model data at all
  displayed identically in reports
- `omitempty` JSON tags had no effect because the field was always populated with `"other"`
  even when there was nothing to record

## What Did Not Work

- **Returning `"other"` for empty input**: Both absence-of-data and unknown-but-real data
  collapsed to the same sentinel. Downstream code could not tell them apart.
- **Using `"(unknown)"` as the direct return value from the function**: This leaks a
  display-layer label into the data layer, coupling presentation format to the data model
  and causing the display string to appear in serialised JSON.

## Solution

Apply a three-layer separation:

1. **Data layer — empty input → `""`** (empty string signals "no data"; `omitempty` suppresses it from JSON)
2. **Data layer — non-empty unrecognised input → `"other"`** (a known sentinel for "real but unmapped")
3. **Render boundary — `""` key → `"(unknown)"`** (display label applied only when formatting output)

### Before

```go
// DeriveModelClass returns the model class for the given model name.
func DeriveModelClass(model string) string {
    switch {
    case strings.Contains(model, "sonnet"):
        return "sonnet"
    case strings.Contains(model, "haiku"):
        return "haiku"
    // ... other patterns ...
    }
    return "other" // ← returned for both "" and unmatched non-empty
}

// harvest.go — always set, even for ghost sessions
record.ModelClass = DeriveModelClass(primaryModel)
```

### After

```go
// DeriveModelClass returns the model class for the given model name.
// Returns "" for empty input (no model data); "other" for non-empty unrecognised names.
func DeriveModelClass(model string) string {
    if model == "" {
        return "" // ← explicit empty guard; omitempty will suppress from JSON
    }
    switch {
    case strings.Contains(model, "sonnet"):
        return "sonnet"
    case strings.Contains(model, "haiku"):
        return "haiku"
    // ... other patterns ...
    }
    return "other" // ← only for real but unmapped model names
}

// harvest.go — guard: only set fields when there is actual model data
if primaryModel != "" {
    record.ModelClass = DeriveModelClass(primaryModel)
    record.ReasoningLevel = DeriveReasoningLevel(primaryModel)
}

// reporter.go — render boundary: map empty key to display sentinel
key := s.ModelClass
if key == "" {
    key = "(unknown)"
}
```

## Why This Works

Classification functions have two distinct inputs: absence of data and presence of
unrecognised data. Treating them differently at the data layer — empty string for absence,
sentinel string for unrecognised — allows all downstream consumers (aggregators, JSON
serialisers, display formatters) to make the correct decision independently:

- **JSON serialisation**: `omitempty` suppresses `""` but preserves `"other"`, so ghost
  sessions emit neither field while sessions with real-but-unknown models emit `"other"`.
- **Aggregation**: a `""` key is routed to an explicit `(unknown)` display bucket; `"other"`
  goes to its own meaningful bucket.
- **Display layer**: `"(unknown)"` only ever appears as a formatted label, never in raw data.

## Prevention

- **Empty-input guard first**: Every classification function should handle `model == ""`
  (or equivalent) as its very first branch before any pattern matching.
- **Test empty input explicitly**: Write a dedicated unit test for the empty-string case on
  every classification function (`TestDeriveModelClass_Empty` expecting `""`).
- **Use `omitempty` on optional classification fields**: Struct fields that may legitimately
  have no value should use `json:",omitempty"` so absence is not serialised as a default.
- **Keep display sentinels at render boundaries**: Strings like `"(unknown)"`, `"-"`, `"N/A"`
  belong in formatting functions, not in data derivation or storage functions.
- **Match aggregation guard to data guard**: If the harvest layer only sets a field when
  `primaryModel != ""`, the aggregation layer should also skip records where that field
  is empty, or explicitly route them to a display-only bucket.

## Related Solutions

No closely related solutions exist yet in `docs/compound/`. Consider cross-referencing
this document when documenting the ghost-session aggregation guard pattern.
