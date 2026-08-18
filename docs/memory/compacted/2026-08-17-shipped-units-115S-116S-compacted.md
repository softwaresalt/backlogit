---
title: "Compacted memory: shipped units 115-S and 116-S (features 133-F, 134-F)"
description: "Consolidated session memory for the ShipShipment partial-feature cascade fix (133-F / 115-S) and the shipment sequencing primitives (134-F / 116-S), replacing three verbose session-memory files archived under docs/archive/memory"
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Compaction record

| Field | Value |
|---|---|
| Compacted on | 2026-08-17 |
| Trigger | `docs/memory/` exceeded the 40-file threshold (49 files, 224 KB) |
| Files consolidated | 3 |
| Size before | 19.8 KB |
| Work items | all shipped and archived; no active or queued item references these files |
| Verbose originals | `docs/archive/memory/` (paths listed below) |

| Original | Archived to |
|---|---|
| `docs/memory/2026-07-31/stage-shipshipment-partial-feature-cascade-memory.md` | `docs/archive/memory/2026-07-31-stage-shipshipment-partial-feature-cascade-memory.md` |
| `docs/memory/2026-08-01/ship-115-S-local-review-readiness-memory.md` | `docs/archive/memory/2026-08-01-ship-115-S-local-review-readiness-memory.md` |
| `docs/memory/2026-08-02/ship-116-S-memory.md` | `docs/archive/memory/2026-08-02-ship-116-S-memory.md` |

## Unit 133-F / shipment 115-S - ShipShipment partial-feature archive cascade

**Origin**: stash `C0909DB5` (high, bug). Staged 2026-07-31; shipped via
`feat/133-shipshipment-cascade-fix`, reviewed HEAD `1dfa8a49`.

**Problem**: `core.ShipShipment` over-archived and over-closed the covering feature on
partial-feature (children-only) shipments. Three mutation seams in
`internal/core/shipment_lifecycle.go` were responsible: the `featureIDs` loop marking the
feature done unconditionally, `collectArchiveCandidateIDs` appending each covering feature
plus its linked deliberation with no membership gate, and the transitive parent-status
rollup through `cascadePersistedParentStatuses` / `ComputeParentStatus`.

**Chosen fix**: gate the two direct seams on explicit manifest membership and neutralize
the transitive rollup for non-member covering features with a narrow pre-ship
snapshot-and-restore. No new terminal-status predicate.

**Decomposition**: `133.002-T` and `133.003-T` (tests, no deps), `133.004-T` (code, depends
on both test tasks), `133.005-T` (check-only doctor audit, depends on `133.004-T`),
`133.006-T` (retire the P-015 manual workaround in `.ship.agent.md` 6.1.b, depends on
`133.004-T`). Four `blocks` edges.

**Review outcome**: seven personas, zero P0. The one confirmed P1 was derived independently
by the Go Reviewer and the Architecture Strategist from different angles: a feature nested
under an explicit-member root (reachable via `AdoptItem` re-parenting, which unlike
`CreateArtifact` is not gated by `AllowedChildren`) is captured as "non-member" by
`featureScopeRoots`' upward walk but is legitimately swept into the archive scope as a
genuine descendant. The deferred `restoreRolledUpNonMemberFeatures` then reverted that
legitimate archival, corrupting `archived_from` and `archived_status`.

**Fix and proof**: `TestShipShipment_LegitimatelyArchivesNestedFeatureDescendantOfMember`
was written, confirmed RED, then made green by hoisting `archivedIDs` above the defer and
threading it into `restoreRolledUpNonMemberFeatures` as an exclusion set. Committed as
`1dfa8a49`.

**Durable lessons**:

* Independent derivation of the same defect by two reviewers on different angles is the
  strongest possible signal; treat it as a confirmed P1 without further debate.
* `gofmt -l` flagging touched core files in this repository is a pre-existing repo-wide CRLF
  artifact, not new drift. Confirm with `git diff --stat` before treating it as a finding.
* Deferred restore logic must know what the same call legitimately archived, or it will
  revert its own correct work.

## Unit 134-F / shipment 116-S - shipment sequencing primitives

**Scope**: shipment sequencing primitives for dark-factory queue ordering. PR #330 merged as
`f3c6f76a`; all ten items (seven tasks, the feature, deliberation `055-DL`, and the shipment)
archived by `backlogit_ship_shipment`.

**Surfaces changed**: `internal/core/shipment.go` (variadic options, priority threading),
`internal/core/dependencies.go` (`AddShipmentBlock`), `internal/cli/shipment.go`
(`--priority`), `internal/cli/dep.go` (routing plus a `depCWD` helper),
`internal/mcp/tools.go` (`priority` param, `AddShipmentBlock` routing),
`.autoharness/backlog-registry.yaml`, and two regenerated `docs/cli-reference/` pages.

**Durable lessons**:

* Merge strategy was a merge commit, per P-009. No squash or rebase.
* The operator's "keep working autonomously until the task is truly finished" was treated as
  approval for that specific pre-built PR only. The session memory explicitly recorded that
  this must **not** be treated as precedent for bypassing local review in normal Ship cycles.
  Preserve that caveat.
* A variadic-options signature is the backward-compatible way to extend shipment creation;
  the pattern was graduated to
  `docs/compound/2026-08-02-variadic-options-backward-compatible-shipment-creation.md`.

## Cross-unit note for later sessions

Both units touched `internal/core/shipment_lifecycle.go` around the same rollback and
covering-feature-restore machinery that feature `143-F` (shipment `127-S`) now plans to
modify again. Anyone working `143.004-T` should read the 133-F lesson above before changing
`restoreRolledUpNonMemberFeatures` or the surrounding defer ordering.
