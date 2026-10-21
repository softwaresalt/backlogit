---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: degraded
description: "Post-merge closure for 193-S (Ship role-boundary disposition ban, B4)."
doc_type: closure
docline:
  date: 2026-10-21T03:20:00Z
  status: final
  tags:
    - closure
    - 193-S
schema_version: "1.0"
shipment_id: 193-S
source: docs/closure/193-S-193-F-post-merge-closure.md
title: "193-S closure"
---

# 193-S closure

Context: feature 193-F (task 193.001-T), PR #559, merge `d81585ef96232c0fd879fd138719ea021b72f95d`. One-line agent-contract change: `.github/agents/_ship.agent.md` Role Boundary now forbids Ship from running the queued-shipment disposition (`backlogit_dispose_shipment`, `backlogit shipment dispose`) against any workspace; disposition is Stage-only. No runtime surface, so runtime-verification is not applicable.

## Releasability

`closure_status: READY`. Monitoring: none (agent-contract text). Rollback: revert PR #559. Owner: operator. Validation window: next Ship session. No conditions.

## Evidence

* Local review of HEAD `033f4969` before PR: correctness, schema/CLI/docs coupling and a shell-capable reviewer; P0=0, P1=0, P2=0, P3=1; READY.
* CI green for the PR head; Copilot review completed on the current HEAD with zero comments; `autoharness gate copilot-review` SATISFIED before and immediately before merge (merge commit strategy).
* `backlogit shipment ship 193-S --sha d81585ef...` returned `shipped` and archived `193.001-T`, `193-F`, `193-S`. The archive records (`status: archived`, `archived_status: shipped` for the shipment) were committed after the governed ship rewrote them.

* Closure-evidence gate: `autoharness gate closure-evidence --path <this file> --shipment 193-S` passes the filename, frontmatter predicate and discoverability checks and fails only at `close_path` (got None); the identical failure occurs on the already-merged 186-S and 192-S closures. The installed harness does not produce `close_path` or `close_evidence` (this closure used `backlogit shipment ship`, neither cascade nor an evidence JSON), so they are not fabricated here. Tracked by existing stash `3FF72EB8`.

## Compaction status (P-020)

`compaction_status: degraded`. compact-context was not allowed to move files: retained docs and checkpoints still reference existing `docs/memory/` paths, so no files were moved. `docs/memory/` is unchanged by this closure.

## Follow-ups

* P3: `plugin/agents/ship.agent.md` is a separate Ship copy without the Role Boundary wording; revisit under plan B-U22/B-U20 (6434A4D7). Not stashed by Ship beyond this record.

## Source artifact provenance (untouched)

Source stash `6434A4D7` and its deliberation are provenance only and were not mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated. Observation: backlogit CLI claim and ship calls took several minutes under server load (3 to 14 minutes); they converged, so poll state rather than re-issuing.
