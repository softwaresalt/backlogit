---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "json omitempty on a slice field silently defeats an arrays-always-[] contract, and a populated-only test fixture will not catch it"
source: docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md
doc_type: learning
description: "When an API advertises that a collection field is always present as a JSON array (never null, never absent), a `json:\"field,omitempty\"` struct tag breaks that contract for the empty case: encoding/json omits ANY length-0 slice under omitempty regardless of whether the Go value is a non-nil empty slice (`[]string{}`) or nil. So a rollup that carefully initializes `Skipped: []string{}` in its constructor still serializes to a MISSING `skipped` key, not `\"skipped\": []`, when there are no skipped members. The trap is that the sibling collection fields (`histogram`, `members`) had NO omitempty and correctly emitted `{}` / `[]`, making the one field with omitempty an easy-to-miss inconsistency. Worse, the canonical-shape parity tests only used a fixture WITH populated members, so they exercised the present-and-non-empty path and passed green while the advertised empty-case shape was absent on every surface (CLI list --json, MCP list_items, get/queue). Fix: drop omitempty from any collection field that is part of an always-an-array contract, and add an explicit empty-case assertion on EVERY transport that requires the key to be present as a JSON array (type-assert to []any and assert len 0), not just the populated case."
docline:
    date: 2026-07-21T00:00:00Z
    severity: medium
    tags:
        - json
        - encoding-json
        - omitempty
        - api-contract
        - arrays-always
        - mcp
        - cli-mcp-parity
        - test-fixture-gap
        - size-composition
        - dogfooding
---

# `omitempty` on a slice defeats an arrays-always-`[]` contract

## Context

`SizeCompositionResult` is the computed-on-read size rollup projected onto
feature/shipment aggregates across four read surfaces: CLI `get` / `list --json`
/ `queue view --json` and MCP `get_item` / `list_items` / `get_queue`. The
compound "arrays always `[]`, never `null`" rule (Rule 3) is meant to give agent
consumers a stable shape.

## The bug

The struct was:

```go
type SizeCompositionResult struct {
    Histogram map[string]int          `json:"histogram"`         // no omitempty → emits {}
    Members   []SizeCompositionMember `json:"members"`           // no omitempty → emits []
    Skipped   []string                `json:"skipped,omitempty"` // OMITTED when empty ← bug
}
```

Every result flowed through a constructor (`rollupFromMembers`) that initialized
`Skipped: []string{}` (non-nil, len 0). The team reasoned "the slice is always
non-nil, so the contract holds." It does not: `encoding/json` omits any **len-0**
slice under `omitempty`, nil or not. So aggregates with no skipped members
serialized `{"histogram":{...},"members":[...]}` — the `skipped` key simply
vanished, breaking the always-an-array promise for the common case.

## Why the tests missed it

The canonical-shape parity tests seeded a fixture WITH members and asserted the
populated histogram. No test seeded an aggregate with zero skipped members and
asserted `skipped == []`. A populated-only fixture exercises the present path and
stays green while the empty-case shape is silently absent on every surface.

## The fix

1. Drop `omitempty` from the collection field:
   `Skipped []string \`json:"skipped"\`` — now emits `[]` when empty, matching
   `histogram` / `members`.
2. Add an explicit empty-case assertion on EVERY transport, not just one:

   ```go
   skipped, isArr := comp["skipped"].([]any)
   require.True(t, isArr, "skipped must always be a JSON array (never omitted/null)")
   assert.Empty(t, skipped)
   ```

   The `.([]any)` type assertion fails for both an absent key (nil interface) and
   a JSON `null`, so it locks "present AND is an array" in one check.

## Rules

- For any field governed by an "always an array/object" contract, do NOT use
  `omitempty`. Initializing a non-nil empty slice is necessary but NOT sufficient
  — `omitempty` still elides it.
- Audit sibling collection fields for tag consistency; one field with `omitempty`
  among several without it is a red flag.
- Lock the EMPTY case in tests on every surface that advertises the contract. A
  populated-only fixture is a false-green; the empty case is where shape contracts
  break.
- This is agent-facing (MCP) parity, so an absent-vs-`[]` drift directly degrades
  agent consumers that expect a stable key.
