---
chunk_strategy: h1-h2-h3
description: 'Governed repair: reconcile a legacy archived shipment to archived_status=shipped'
doc_type: reference
ingested_at: "2026-09-13T04:01:57Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_reconcile-shipped.md
title: backlogit shipment reconcile-shipped
---

## backlogit shipment reconcile-shipped

Governed repair: reconcile a legacy archived shipment to archived_status=shipped

### Synopsis

Reconcile an archived shipment whose released work predates the governed
ShipShipment envelope (144-F) into the durable, event-backed archived_status:
"shipped" (#423, 167-F).

This is a governed two-phase transaction (core.ReconcileShipmentToShipped): it
validates the shipment is archived with archived_status=active and every
manifest member reached a supported terminal status (done/accepted), verifies
the supplied merge commit and closure evidence against the configured trusted
refs, and writes the archive frontmatter to archived_status="shipped" before
durably appending the reconciliation event. Requests are idempotent: replaying the same
--idempotency-key is a no-op; a different request under the same key is
refused as a conflict.

A live (non-dry-run) mutation requires the shipment-specific confirmation
phrase "reconcile-shipped <shipment-id>" via --confirm, or typing that same
phrase at an interactive TTY prompt. --dry-run needs no confirmation and makes
no reconciliation writes (the archive file, index row, and event log are all
left unchanged).

```text
backlogit shipment reconcile-shipped <shipment-id> [flags]
```

### Examples

```text
  backlogit shipment reconcile-shipped 048-S --reason "legacy repair" --actor operator \
    --idempotency-key idem-048-s-1 --merge-sha 0123456789abcdef0123456789abcdef01234567 \
    --closure-evidence docs/closure/048-S.md --confirm "reconcile-shipped 048-S"
  backlogit shipment reconcile-shipped 048-S --dry-run --reason "preview" --actor operator \
    --idempotency-key idem-048-s-1 --merge-sha 0123456789abcdef0123456789abcdef01234567 \
    --closure-evidence docs/closure/048-S.md
```

### Options

```text
      --actor string               actor performing the reconciliation, recorded for audit (required)
      --closure-evidence string    workspace-relative path to closure evidence documenting the release (required)
      --confirm string             confirmation phrase "reconcile-shipped <shipment-id>", required for a live (non-dry-run) mutation unless stdin is an interactive TTY
      --dry-run                    evaluate every precondition and print the planned outcome without writing (needs no --confirm)
      --evidence-ref stringArray   additional evidence reference; repeatable
  -h, --help                       help for reconcile-shipped
      --idempotency-key string     idempotency key for this reconciliation request (required)
      --merge-sha string           merge commit SHA that delivered the released scope (required)
      --reason string              operator justification for the reconciliation (required)
      --second-approver string     optional second approver recorded for audit (must differ from --actor)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

