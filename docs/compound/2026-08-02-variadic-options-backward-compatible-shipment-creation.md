---
title: "Variadic options pattern for backward-compatible core factory function extension"
source: docs/compound/2026-08-02-variadic-options-backward-compatible-shipment-creation.md
doc_type: learning
description: "When a core factory function (e.g. CreateShipment) needs a new optional parameter without breaking existing callers, use a variadic ...Option slice. Thread caller opts first, then apply validated required fields last to prevent override of invariants."
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
    date: 2026-08-02T00:00:00Z
    severity: medium
    tags:
        - go
        - patterns
        - options
        - backward-compatible
        - shipment
        - factory
        - tdd
---

# Variadic Options Pattern for Backward-Compatible Core Factory Extension

## Context

Graduated from shipment 116-S / feature 134-F (PR #330, merged `f3c6f76a`). `core.CreateShipment` needed a `priority` parameter without breaking the 20+ call sites that already passed `title` and `itemIDs` only. The function also needed an ordering invariant: the validated `items` field must not be overridable by caller options.

## Problem

Adding a new required parameter to `CreateShipment(ctx, ws, title, itemIDs, priority)` would break all existing callers. Adding an optional `priority string` parameter with a sentinel empty value ("" = default) is fragile and does not compose well when more optional parameters are needed later.

## Solution — Variadic `...Option`

```go
// internal/core/shipment.go

// CreateShipment creates a new shipment artifact.
// opts are applied before the validated items field so invariants cannot be overridden.
func CreateShipment(ctx context.Context, ws *Workspace, title string, itemIDs []string, opts ...Option) (*models.Artifact, error) {
    if err := validateShipmentItemIDs(ctx, ws, itemIDs); err != nil {
        return nil, fmt.Errorf("validate shipment items: %w", err)
    }
    // Apply caller opts first, then items field last — this ordering invariant
    // prevents a caller-supplied WithFields({items}) from bypassing validateShipmentItemIDs.
    createOpts := []Option{WithStatus(ShipmentQueued)}
    createOpts = append(createOpts, opts...)
    createOpts = append(createOpts, WithFields(map[string]any{"items": itemIDs}))
    return CreateArtifact(ctx, ws, "shipment", title, createOpts...)
}
```

Existing callers compile unchanged. New callers pass `core.WithPriority("high")`:

```go
// CLI thread-through
shipment, err := core.CreateShipment(ctx, ws, title, itemIDs, core.WithPriority(priorityFlag))
```

## Ordering Invariant

The key insight: apply caller options **before** validated required fields. This means:

1. `createOpts = append(createOpts, opts...)` — caller opts applied, including any `WithPriority`
2. `createOpts = append(createOpts, WithFields({"items": itemIDs}))` — items field applied LAST

Since `CreateArtifact` applies options left-to-right and later options win for the same field key, validated fields always win over caller options. A caller cannot bypass `validateShipmentItemIDs` by passing `WithFields({"items": ...})`.

## Parity Test — Denylist Lock

This pattern pairs with the denylist parity test (see `2026-07-23-cli-mcp-filter-param-denylist-parity-test.md`) to ensure the CLI `--priority` flag and MCP `create_shipment.priority` param stay in sync:

```go
// internal/cli/shipment_test.go
var shipmentCreateOutputOnlyDenylist = map[string]bool{
    "json":       true,
    "output":     true,
    "quiet":      true,
    "no-confirm": true,
}

func TestShipmentCreateCLIMCPParityLock(t *testing.T) {
    // Derive CLI filter set from live pflag.FlagSet.VisitAll minus denylist
    // Cross-check against live srv.ToolDefs() for backlogit_create_shipment
}
```

## Applicability

Use this pattern when:

1. A factory function has existing callers that should not be broken.
2. The new parameter is optional or has a sensible zero value.
3. More optional parameters may be added in the future (composes well).
4. Some fields are invariants that must be applied after caller opts (ordering discipline).

Do NOT use if:
- The new parameter is required and all callers MUST be updated (use required param instead)
- There is only one call site (inline is simpler)
- The options type is not already defined in the package (`Option` is a pre-existing type in `internal/core`)

## Related

- `2026-07-23-cli-mcp-filter-param-denylist-parity-test.md` — denylist parity test pairs with this
- `2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` — options must not bypass the commit-then-surface ordering
