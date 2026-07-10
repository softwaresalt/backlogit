---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 079-S CLI/MCP command parity phase 2.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-079-S-cli-mcp-parity-phase2-compacted.md
title: Compacted memory - 079-S CLI/MCP parity phase 2
---
## Summary

Shipment `079-S` built the deferred CLI fallbacks worth building from phase 1. Stage promoted `6C6ACE00`, corrected the phase-1 deliberation record via ride-along `2827CB5F`, and Ship delivered link, hook, memory, comment, and metadata CLI surfaces with registry/docs updates.

## Archived originals

* `docs/archive/memory/2026-07-03-stage-6C6ACE00-phase2-cli-mcp-parity-shipment-complete.md`
* `docs/archive/memory/2026-07-03-ship-079-S-session.md`

## Decisions and outcomes

* Build families: `link add/remove/list`, `hooks poll/ack`, `memory save`, `comment add`, and `metadata types/wit/templates`.
* Defer `merge_sync` to phase 3 because it writes by default and needs dry-run guardrails; keep `log_telemetry` permanently MCP-only.
* Registry flag parity is load-bearing; the drift test must assert required flags and positional/flag names, not only command path existence.
* PR #172 merged by true merge commit `a8e07ea38f8e153e9a29def264538bcab8222868`; post-merge closure shipped and archived all 15 items plus source deliberation `051-DL` and `079-S`.

## Files and verification

* CLI link, hooks, memory, comment, and metadata subcommands landed with tests.
* `core.AppendComment` extracted shared behavior; MCP passes shared `EventWriter`, CLI uses one-shot default.
* Copilot caught a valid concurrency issue: a fresh `EventWriter` per `AppendComment` call would drop in-process append serialization. The shared-writer fix was merged.
* Final gates passed: build, vet, tests, lint, generated docs idempotency, CI, runtime verification, Copilot freshness, and §1.9.
* New compound learning `2026-07-04-core-extraction-shared-eventwriter-append-serialization.md` captured the shared-writer pattern.
