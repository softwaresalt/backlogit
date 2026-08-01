---
chunk_strategy: h1-h2-h3
description: 'One durable operational hazard graduated from 114-S — backlogit shipment ship (core.ShipShipment) archives the ENTIRE feature ancestry of every manifest item unconditionally. featureScopeRoots walks up parent_id collecting all ancestor features, and collectArchiveCandidateIDs appends each non-archived feature as an archive candidate with NO "children terminal" gating. Shipping the first sub-shipment of a multi-cycle feature therefore prematurely archives the parent feature even when later units (harvested or not-yet-harvested) are still intended. Mitigation: after ship, if a parent feature should stay open, git restore its queue file and re-sync the index; consider filing a bug to gate feature archival on remaining non-terminal descendants.'
doc_type: learning
docline:
    date: 2026-07-31T00:00:00Z
    severity: high
    tags:
        - core
        - shipment
        - lifecycle
        - archive
        - cascade
        - ship_shipment
        - backlog-state
        - closure
schema_version: "1.0"
source: docs/compound/2026-07-31-ship-shipment-unconditionally-archives-ancestor-features.md
title: 'backlogit shipment ship archives ancestor features unconditionally — restore multi-cycle parents after ship (114-S)'
---

# Ship Archives Ancestor Features Unconditionally

One durable operational hazard graduated from shipment 114-S (feature 106-F,
"Formal-gate foundations" F2+F3, PR #324, merge
`f8870f864d596a1f3593405e54396d8129aa8871`).

## The Hazard

`core.ShipShipment` does not limit archival to the shipment's manifest items.
It computes a **feature scope** by walking up `parent_id` from every released
item and archives every ancestor feature it finds — with **no check that the
feature's remaining work is complete**.

Concretely:

* `featureScopeRoots` starts at each manifest/released item and follows
  `currentID = item.ParentID` up the chain, collecting every artifact whose
  `artifact_type == "feature"`.
* `collectArchiveCandidateIDs` then appends each such feature **unconditionally**
  as an archive candidate: `if feature.Status != models.StatusArchived {
  candidates = append(candidates, feature.ID) }`. There is no
  "all children terminal" / `CheckChildrenTerminal` gate on the feature append,
  unlike the descendant items which are gated by `isTerminalReleaseStatus`.

So for a feature designed to span **multiple shipments** (e.g. 106-F spans
F1–F6 harvested across separate Stage cycles), shipping the first sub-shipment
(114-S = F2 + F3) archives the parent 106-F even though F1/F4/F5/F6 remain
intended future work. The parent's `status` becomes `archived`
(`archived_status: done`) and its queue file is relocated to
`.backlogit/archive/`. The archived shipment manifest never listed the feature
as a member — the feature is pulled in purely as an ancestor.

## Evidence

* `internal/core/shipment_lifecycle.go` `collectArchiveCandidateIDs` — the
  unconditional `candidates = append(candidates, feature.ID)` inside the
  `for _, featureID := range featureIDs` loop (no terminal-children gate).
* `internal/core/shipment_lifecycle.go` `featureScopeRoots` — the
  `currentID = item.ParentID` upward walk that seeds `featureIDs`.
* Observed on 114-S: `backlogit shipment ship 114-S` returned
  `archived_ids: [106.001-T, 106.002-T, 106-F, 114-S]` — `106-F` was archived
  though the manifest `items` were only `[106.001-T, 106.002-T]`.

## Mitigation (post-ship closure)

When a shipped sub-shipment belongs to a feature that must remain open for
later cycles:

1. Inspect the `archived_ids` in the `ship_shipment` result. If an
   ancestor feature you intend to keep active appears there, correct it.
2. `git restore .backlogit/queue/<feature>.md` — restores the committed
   `status: active` queue file (the ship command leaves it as a tracked
   deletion `D`, so restore recovers it cleanly).
3. Remove the tool-generated `.backlogit/archive/<feature>.md` (untracked `??`).
4. `backlogit sync` — the SQLite index is a disposable cache; re-sync rebuilds
   it from the markdown source of truth so queries see the feature active again.
5. Verify the archived shipment manifest never listed the feature as a member
   (it will not) to confirm the archival was ancestor-cascade, not manifest-driven.

## Root-Cause Follow-Up

The unconditional feature append is arguably a defect: feature archival should
be gated on the feature having no remaining non-terminal descendants (and,
ideally, an explicit "spans multiple shipments" guard for features whose future
units are planned-but-unharvested). Consider filing a bug so Stage can triage a
`CheckChildrenTerminal`-style gate on the `featureIDs` archival branch of
`collectArchiveCandidateIDs`. Until then, the post-ship restore above is the
authoritative workaround.
