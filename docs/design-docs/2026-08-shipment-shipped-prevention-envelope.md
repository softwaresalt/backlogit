---
chunk_strategy: h1-h2-h3
description: "Shipped-transition prevention and archive guard: the prevention/detection pairing, producer set, and locked-path authority."
doc_type: design-doc
schema_version: "1.0"
source: docs/design-docs/2026-08-shipment-shipped-prevention-envelope.md
title: "Shipment shipped-transition prevention and archive stamping guard (144-F)"
---

## Problem

Feature 143-F / shipment 127-S shipped the DETECTION half: `shippedEventPresence`
and a report-only doctor audit flag any archived shipment with
`archived_status: shipped` that lacks a durable `shipment_status_changed` event.

This document covers the PREVENTION half (144-F): closing the two paths that
could produce that residue in the first place.

## The Two Guards

### Guard 1 — generic shipped-transition prevention

Any call to `UpdateArtifactWithGate` / `updateArtifactUngated` /
`moveShipmentStatusWithHeadGuard` (topLevel=true) that attempts to move a
shipment to `shipped` is refused **unconditionally** with
`ErrShipmentShippedRequiresEnvelope`.

`ShipShipment` is exempt by construction: it ships via
`moveShipmentStatusWithHeadGuard` with `topLevel=false`, never through
`UpdateArtifactWithGate`. No exemption flag is threaded through `updates`
or `opts` — that would be a forgeable bypass lever.

The guard is implemented at two seams:

* **Unlocked peek** (`UpdateArtifactWithGate` in `gate_transition.go`): cheap
  fast-path refusal before any lock is acquired.
* **Locked write path** (`updateArtifactUngated` in `artifacts.go`): authoritative
  post-lock revalidation (TOCTOU close). Fires after `lockArtifactMutations` and
  the authoritative `findArtifact` reload, before the pre-update hook.

`CreateArtifact` also refuses an initial `status: shipped` for `artifact_type ==
"shipment"` (reusing the same sentinel). Shipments are born queued and transition
to shipped only via `ShipShipment`.

### Guard 2 — archive stamping prevention

`ArchiveItem` (`archive.go`) checks whether the artifact being archived is a
shipment sitting at `shipped` status. If so, it calls `shippedEventPresence`
(same `core` package, `doctor.go`) to verify a durable event exists before
allowing the stamp. Missing or unreadable event → refuse with
`ErrArchiveShippedRequiresEvent`.

The check runs after `lockArtifactMutations` and after the frontmatter is read
(providing `oldStatus` from the `fm` map), but before pre-archive hooks and
before any cascade or write. A refusal is therefore clean with no side effects.

The guard is scoped strictly to `artifactType == "shipment" && oldStatus == "shipped"`.
`done`, `abandoned`, and P-015 safe-close (`artifact_type != "shipment"`) are never
blocked.

## Detection Reuse

`shippedEventPresence` in `doctor.go` is the single source of truth for both
prevention (guard 2) and detection (doctor audit). Prevention and detection scan
the identical JSONL contract — no predicate drift.

## Full Producer Set

All known non-`ShipShipment` paths that could produce a shipped shipment:

| Path | Guard | Exit code |
|---|---|---|
| `UpdateArtifactWithGate` (move_item / update_item) | Guard 1 unlocked peek | 9 |
| `updateArtifactUngated` (locked write) | Guard 1 locked path | 9 |
| `MoveShipmentStatus` / `moveShipmentStatusWithHeadGuard` topLevel=true | Guard 1 move seam | 9 |
| `CreateArtifact` with initial `status: shipped` (create_item / harvest_stash) | Guard 1 create seam | 9 |
| `ArchiveItem` stamping `archived_status: shipped` without durable event | Guard 2 archive seam | 9 |

## Error Sentinel Mapping

| Sentinel | Location | MCP error_type | CLI exit code |
|---|---|---|---|
| `ErrShipmentShippedRequiresEnvelope` | `internal/errors` | `shipment_shipped_requires_envelope` | 9 |
| `ErrArchiveShippedRequiresEvent` | `internal/errors` | `archive_shipped_requires_event` | 9 |

Surface mapping is in `internal/mcp/errors.go` (`domainError` switch) and
`internal/cli/gate_exit.go` (`shipmentGovernanceExitError`).

## Fail-Closed Policy

Guards fail closed on missing or unreadable evidence:

* Guard 1: unconditional refusal — no evidence needed.
* Guard 2: if the JSONL is unreadable (`readable == false`) or no event
  is found (`present == false`), refuse. An unreadable log is NOT treated
  as "event absent" — it is treated as "evidence unknown → fail closed."

## Residual and Out-of-Band Exceptions

* Out-of-band edits (direct Markdown/git) remain out of scope. The doctor
  audit remains the after-the-fact net.
* Guard 2 also fires on the governed `ShipShipment` archival path. Normally the
  event is present (ShipShipment appended it). A transient JSONL read failure
  during governed archival fails closed and leaves the shipment shipped-but-unarchived
  as acceptable residue for the doctor audit.

## Related

* Deliberation: `docs/decisions/2026-08-18-shipment-shipped-prevention-hardening-deliberation.md`
* Plan: `docs/exec-plans/2026-08-18-shipment-shipped-prevention-hardening-plan.md`
* Detection design: see `doctor.go:shippedEventPresence` and `DoctorOptions.CheckShippedEventCompleteness`
