---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "json omitempty on a slice field silently defeats an arrays-always-[] contract, and asserting sibling fields while never inspecting the empty collection lets it pass green"
source: docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md
doc_type: learning
description: "When an API advertises that a collection field is always present as a JSON array (never null, never absent), a `json:\"field,omitempty\"` struct tag breaks that contract for the empty case: encoding/json omits ANY length-0 slice under omitempty regardless of whether the Go value is a non-nil empty slice (`[]string{}`) or nil. So a rollup that carefully initializes `Skipped: []string{}` in its constructor still serializes to a MISSING `skipped` key, not `\"skipped\": []`, when there are no skipped members. The trap is that the sibling collection fields (`histogram`, `members`) had NO omitempty and correctly emitted `{}` / `[]`, making the one field with omitempty an easy-to-miss inconsistency. The test gap was NOT a populated-only fixture: the parity fixtures already had zero skipped members, so `skipped` was already empty — the tests simply asserted the populated `histogram` and never inspected `skipped`, and the canonical DeepEqual guard could not catch it because the canonical value is built from the same omitempty struct (both sides omit the key identically and compare equal). Fix: drop omitempty from any collection field that is part of an always-an-array contract, and assert the empty case (type-assert to []any, assert len 0) so a missing/null key fails. Commit `b2a3d1f9` added that assertion to the two new flat-list surfaces (CLI `list --json` and MCP `list_items`); the get/queue transports share the same struct and are fixed by the tag change but were not given their own empty-case assertions."
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
feature/shipment aggregates across six read surfaces — three CLI commands
(`get`, `list --json`, `queue view --json`) and three MCP tools (`get_item`,
`list_items`, `get_queue`). The compound "arrays always `[]`, never `null`"
rule (Rule 3) is meant to give agent consumers a stable shape.

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

The parity fixtures already seeded aggregates with zero skipped members (no
dangling member), so `skipped` was already empty in the fixture. The bug escaped
because the tests asserted the populated `histogram` but never inspected
`skipped`. The canonical-shape DeepEqual guard could not catch it either: the
canonical value is produced from the same omitempty struct, so both sides omitted
the key identically and compared equal. The same fixtures now catch the
regression once an explicit `skipped` array assertion is added — the gap was the
missing assertion, not a missing empty-case fixture.

## The fix

1. Drop `omitempty` from the collection field:
   `Skipped []string \`json:"skipped"\`` — now emits `[]` when empty, matching
   `histogram` / `members`.
2. Assert the empty case so a missing/`null` key fails. Commit `b2a3d1f9` added
   this to the two new flat-list surfaces (CLI `list --json` and MCP
   `list_items`); the `get` and `queue` transports share the same struct and are
   fixed by the tag change, but were not given their own empty-case assertions:

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
- Lock the EMPTY case in tests: assert the collection field itself, not just its
  siblings. Asserting a populated `histogram` while never inspecting `skipped` is
  a false-green, and a DeepEqual against a canonical value built from the same
  struct will not catch an omitempty break because both sides elide the key
  identically.
- This is agent-facing (MCP) parity, so an absent-vs-`[]` drift directly degrades
  agent consumers that expect a stable key.
