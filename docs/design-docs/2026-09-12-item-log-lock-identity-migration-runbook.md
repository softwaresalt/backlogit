---
chunk_strategy: h1-h2-h3
description: "Migration runbook for the item-log cross-process lock's stable, handle-validated sidecar identity (167.017-T, #423): the full-fleet quiescence precondition for legacy binaries that cannot honor the new runtime lease."
doc_type: design
status: draft
created: 2026-09-12
schema_version: "1.0"
source: docs/design-docs/2026-09-12-item-log-lock-identity-migration-runbook.md
title: "Item-Log Lock Identity Migration Runbook (167.017-T)"
---

# Item-Log Lock Identity Migration Runbook (167.017-T)

## Summary

The item-log cross-process lock (component **C**,
`events.LockItemLogCrossProcess`) previously derived its OS-level advisory
sidecar path from the *logs directory* (`itemLogLockSidecarPath(logsDir,
itemID)`). Because the logs directory is a swappable, reconfigurable input,
two lockers for the same item that observe different logs-directory values
(before and after a reconfiguration/replacement) opened **different inodes**
and therefore did **not** mutually exclude — a latent cross-process race.

167.017-T replaces this with a **stable, handle-validated** sidecar rooted at
a fixed, workspace-relative locks root (`.backlogit/.locks/itemlog/<encoded
itemID>`) that is completely independent of the logs directory. See
`internal/events/item_log_lock_identity.go` (`ItemLogLockPath`) for the
implementation and `internal/events/item_log_lock_identity_test.go` for the
behavioral proof (including a same-process, real-OS-lock mutual-exclusion
test that survives a simulated logs-directory swap).

## Why this migration cannot be transparently bridged

An old (pre-167.017-T) process is inode-bound to the **legacy** sidecar
location (inside whatever logs directory it currently observes). A new
(167.017-T+) process locks the **stable** sidecar location instead
(`.backlogit/.locks/itemlog/...`, independent of the logs directory). These
are two different files. If an old process and a new process are running
concurrently against the same item, **they do not share a lock** no matter
which one runs first — there is no dual-lock bridge that can fix this,
because:

* A dual-lock bridge (acquiring *both* the legacy and the stable sidecar)
  only helps processes that are running **within the same, unchanged, logs
  directory** — it does nothing once a logs-directory replacement happens,
  because the legacy sidecar's location moves with the (now-replaced) logs
  directory while an old process remains bound to the *original* directory's
  legacy sidecar.
* Legacy (pre-167.017-T) binaries **cannot be retrofitted** to additionally
  honor the new stable identity or a runtime lease protocol — they predate
  the change and have already been built/deployed.
* A runtime point-in-time check ("is a legacy-identity lock currently held?")
  is **insufficient**: an idle old-version process can pass the check and
  then acquire the legacy lock immediately afterward, reintroducing the race
  the migration is meant to close.

## Required operational precondition: full-fleet quiescence

Because of the above, the legacy → stable lock-identity migration **requires
full-fleet shutdown of every old-version process** as an
**externally-verified deployment precondition** — a stop-the-world
operational step enforced by the deployment process and the operator
runbook below, **not** by runtime code:

1. **Drain** all CLI invocations and MCP server instances running a
   pre-167.017-T binary against the target workspace(s). Confirm no
   pre-167.017-T process remains running (process list / orchestration
   tooling check) before proceeding.
2. **Deploy** the 167.017-T+ binary to every node/environment that touches
   the workspace.
3. **Only then** may any logs-directory reconfiguration/replacement (if one
   is also planned) be performed. Within a single-version (167.017-T+)
   fleet, the stable handle-bound identity holds across such a
   reconfiguration automatically — no additional coordination is required,
   because every process resolves the identical stable identity regardless
   of the logs directory's current value.
4. Resume normal operation.

A concurrent logs-directory replacement during **mixed-version** operation
(some processes pre-167.017-T, some post-) is explicitly **out of scope** and
must be **prevented by this quiescence step**, not compensated for by lock
code.

## Forward compatibility: the version marker

The stable lock namespace (`.backlogit/.locks/itemlog/`) carries a durable
version marker file (`.identity-version`, content
`events.ItemLogLockIdentityVersion`, currently `stable-handle-bound-v1`),
written on every lock-path resolution
(`internal/events/item_log_lock_identity.go`). This migration itself cannot
use a runtime lease (see above), but the marker lets a **future** migration
between two versions that both understand a lease protocol detect the
currently-active identity scheme and negotiate a transition automatically,
rather than requiring another stop-the-world step.

`TestItemLogLockPath_WritesVersionMarker`
(`internal/events/item_log_lock_identity_test.go`) asserts the marker is
written with the expected content on every resolution.

## Scope boundary (carried from PR #424 review, thread ft-pC)

This migration guarantees stability **across a logs-directory
reconfiguration/replacement** — the input that actually varies at runtime in
this codebase today. It does **not** defend against an adversarial rename or
symlink-swap of the `.backlogit/.locks` root itself. No lock in this codebase
defends that threat (membership lock A and artifact-mutation lock B,
`internal/core/shipment.go`, open their own roots by path too) — it is a
repository-wide threat-model boundary, not a gap specific to this lock.
