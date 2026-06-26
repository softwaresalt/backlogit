---
chunk_strategy: h1-h2-h3
description: ""
doc_type: learning
docline:
    date: 2026-05-07T00:00:00Z
    severity: high
    tags:
        - telemetry
        - tokens
        - allocation
        - arithmetic
        - float
        - integer
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/2026-05-07-integer-arithmetic-largest-remainder.md
title: Integer arithmetic in largest-remainder token allocation
---

# Integer Arithmetic in Largest-Remainder Token Allocation

## Problem

When distributing `totalTokens` proportionally across N servers using float64
arithmetic, the largest-remainder algorithm can compute a negative `remaining`
count. This occurs because float division introduces rounding errors that push
`floored` sums slightly above `totalTokens`.

```go
// WRONG — float arithmetic
for _, c := range calls {
    floor := int(float64(totalTokens) * float64(c) / float64(totalCalls))
    // float truncation error: sum of floors may > totalTokens
    // remaining = totalTokens - sum(floors) can be < 0
}
```

A negative `remaining` means the final loop that distributes bonus tokens
iterates a negative number of times — allocating no bonus at all — yet the
total already exceeds `totalTokens`. The invariant `sum(allocated) == totalTokens`
is silently violated.

## Solution

Use integer arithmetic throughout. Multiply numerator by `totalTokens` first,
then divide to preserve the full integer remainder:

```go
// CORRECT — integer arithmetic
type ranked struct {
    key      string
    floor    int
    rem      int
}
items := make([]ranked, 0, len(calls))
sumFloors := 0
for k, c := range calls {
    numerator := totalTokens * c
    floor := numerator / totalCalls
    rem   := numerator % totalCalls
    items = append(items, ranked{key: k, floor: floor, rem: rem})
    sumFloors += floor
}
// remaining is guaranteed 0 ≤ remaining ≤ len(items)
remaining := totalTokens - sumFloors
sort.Slice(items, func(i, j int) bool {
    return items[i].rem > items[j].rem
})
for i := range items {
    bonus := 0
    if i < remaining {
        bonus = 1
    }
    out[items[i].key] = items[i].floor + bonus
}
```

## Invariants guaranteed by integer arithmetic

- `0 ≤ remaining ≤ len(items)` — mathematically guaranteed
- `sum(out values) == totalTokens` — exact, no epsilon
- No negative allocation

## Where applied

`internal/telemetry/correlator.go` — `Correlate()` function, session summary
`tokens_by_server` map construction.

## References

- PR #89: feat(telemetry): attribution analytics & trend reporting (049-S)
- Copilot review thread PRRT_kwDORzozKM6AeViT
