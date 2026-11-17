---
title: "217-S durable outcomes and residual risks"
description: "Durable architectural outcomes and tracked residual risks for Shipment 217-S (Feature 216-F / CAP-3)."
doc_type: memory
source: docs/memory/2026-11-17-217-s-session-summary.md
ingested_at: "2026-11-17T18:55:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 217-S
  feature_id: 216-F
  branch: feat/217-s-zero-flake-concurrency-and-fast-ship-closure-engine
---

## Durable Outcomes

Shipment `217-S` (`216-F`: Zero-Flake Concurrency and Fast Ship-Closure Engine,
CAP-3), governed by [the CAP-3 implementation plan](../plans/2026-11-15-cap3-zero-flake-concurrency-ship-closure-plan.md)
and deliberations `096-DL`..`101-DL` ([CAP-3 deliberation](../decisions/2026-11-15-cap3-zero-flake-concurrency-ship-closure-deliberation.md)),
delivered Units U1, U2, U3a, U3b, U4, U5, U6, and U7 (`216.001-T`..`216.008-T`):

* **Commit-writer lock ordering (`U1`)**: `LinkCommit` and `AssociateCommit` acquire `lifecycle-global -> C (item-log)` before mutating `commit_links` or appending JSONL events, failing closed with `*ShipmentLifecycleLockOrderError` when `C` is preheld without verified `lifecycle-global` ownership.
* **Event-reader buffer sizing (`U2`)**: `events.ReadAllEvents` allocates a `64 KiB` initial scanner buffer growing up to `1 MiB` max, checks `ctx.Err()`, and wraps `scanner.Err()`.
* **Two-pass contained lookup (`U3a`)**: `findArtifactWithPath` probes `{id}.md` across search directories via `ensureArtifactLookupContained` before falling back to `filepath.WalkDir`, eliminating redundant directory walks in `snapshotShipArtifacts`.
* **Immutable ancestry memoization (`U3b`)**: `(*Workspace).isAncestor` enforces `--no-replace-objects`, normalizes `nil` contexts, sets `WaitDelay`, and memoizes verified 40/64-hex `(ancestor, descendant)` verdicts in a bounded 4,096-entry `Workspace` cache strictly restricted to complete, ungrafted repositories (`.git/shallow` and `.git/info/grafts` absent both before and after `cmd.Run()`).
* **SQLite WAL Page-1 initialization (`U4`)**: `db.Open` places `_pragma=busy_timeout(30000)` first, materializes Page 1 WAL header bytes `[2, 2]` on empty databases, and `EnsureSchema` / `EnsureSchemaWithExtensions` verify `commit_links` and `WAL` readiness.
* **Bounded custody Git execution & OID cache (`U5`)**: `custodyGit` enforces a `15s` timeout, `WaitDelay`, `-c core.fsmonitor=false -c gc.auto=0 --no-replace-objects --no-optional-locks`, and sanitized `custodyGitEnv()` (`GIT_TERMINAL_PROMPT=0`), while `custodyBlobDigest` validates object names and memoizes immutable blob OIDs under canonicalized workspace roots.
* **Deterministic test seeding & lock-hook sync (`U6`–`U7`)**: `newAuthSeamFixture` shares an immutable `sync.Once` seed, `TestUR1B` bounds its child subprocess with `CommandContext` and `WaitDelay`, `lockArchiveGovernance` emits `"attempt_membership"` and `"attempt_item_log"`, and `TestU172` / `TestArchiveItemGovernance` synchronize on deterministic contention signals.

## Tracked Residual Risks and Deferred Scope

* Unnamed `df4a2df9` `internal/core` panic (`63909DAE` / `101-DL`) remains explicitly unresolved as a tracked diagnostic risk (unproven origin; unrelated to `custodyGit` or `TestUR1B`).
* P-021 C2 deferred stash entries captured for Stage triage: `451486E7` (`detect git worktree` `10s` timeout under load), `FDFC52FF` (`confineToStorageRoot` symlink fallback / lookup TOCTOU), `DA6809A0` (standalone non-member `AssociateCommit` pre-lock read serialization), and `D1CA631D` (`ValidateCustodyEnvelope` `base_commit_sha` strict SHA validation).
