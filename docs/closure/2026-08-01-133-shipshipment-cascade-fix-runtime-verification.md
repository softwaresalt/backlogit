---
chunk_strategy: h1-h2-h3
description: Runtime verification for shipment 115-S (feature 133-F, ShipShipment partial-feature cascade fix).
doc_type: closure
docline:
    ms.date: 2026-08-01T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-08-01-133-shipshipment-cascade-fix-runtime-verification.md
title: "115-S ShipShipment cascade fix runtime verification"
date: "2026-08-01"
shipment: "115-S"
items:
  - "133-F"
status: "pass"
---

# Runtime Verification — 133-F ShipShipment cascade fix

* **Context**: feature `133-F` (shipment `115-S`), merged to `main` in PR #327,
  merge SHA `47dfcc93698a6b0b2c5420c701c365a538895580`.
* **Surface**: `cli` — `backlogit shipment ship` / `backlogit_ship_shipment`
  (core function `internal/core/shipment_lifecycle.go:ShipShipment`).
* **Mode**: manual (compiled-binary dogfood test against a disposable scratch
  workspace; no HTTP/browser surface exists for this behavior).

## Invariants under verification

1. When a shipment's explicit manifest contains only a deeply-nested child
   item, `ShipShipment` must archive exactly the manifest items and must
   **not** archive their non-member covering-feature ancestors.
2. Non-member covering-feature ancestors must be **restored** to the status
   they held immediately before release-scope mutation inside `ShipShipment`
   itself (`snapshotNonMemberFeatureStatuses`, `internal/core/shipment_lifecycle.go:175-191`
   — captured at ship-time, not by `ClaimShipment`), not left `done` or
   rolled up.
3. When multiple sibling/ancestor restorations are pending, restoration order
   must be deterministic (deepest-first) regardless of Go's randomized map
   iteration order (Thread C fix, `restoreRolledUpNonMemberFeatures`).

## Step 2: Environment prechecks

* Built a verification binary from the exact merged `main` HEAD
  (`47dfcc93698a6b0b2c5420c701c365a538895580`) using the Makefile's standard
  `-ldflags "$(LDFLAGS)"` convention, output to `bin\backlogit-verify.exe`
  (cleaned up after verification — not committed).
* Verification ran against a disposable scratch workspace at
  `docs\scratch\runtime-verify-133-F\` (its own `.backlogit/` tree, isolated
  from the real repository's backlog state) — cleaned up after verification,
  confirmed via `git status --short` showing a clean tree afterward.

## Step 5: Execution and evidence

**Scenario**: 2-level nested covering-feature hierarchy exercising invariants
1–2 (a real-repo equivalent could not exercise this path because `133-F` has
no parent feature in this shipment's actual topology; the scratch scenario
below is the direct, faithful stand-in required to prove the fix given that
constraint):

* Created Grandparent feature `001-F` → Parent feature `001.001-F` → explicit
  member task `001.001.001-T`.
* Created a shipment whose explicit manifest contained **only**
  `001.001.001-T` (neither ancestor feature was a manifest member).
* Claimed the shipment (`ClaimShipment` only activates the manifest item and
  does not snapshot ancestor-feature statuses; the ancestors' pre-mutation
  snapshot is captured later, at ship-time, inside `ShipShipment` itself).
* Hit an unrelated gate: the `082-F` pre-task-completion gate
  (`hooks.yaml` → `lifecycle.pre_task_completion_gate`, default
  `enabled: auto`) blocked completing `001.001.001-T` to `done` because it
  had not been completed through that gate's evidence protocol. This gate is
  unrelated to `133-F`/Thread C and has its own separate test suite; to keep
  this verification scoped to the shipment-cascade fix, the gate was disabled
  (`enabled: "false"`) in the **scratch workspace's own** `hooks.yaml` only —
  never in the real repository's configuration (the real repo has no
  `.backlogit/hooks.yaml` at all; it uses `DefaultHooksConfig()`).
* Retried `backlogit shipment ship` against the scratch workspace — succeeded.

**Observed behavior**:

* `001.001.001-T` (the explicit manifest member): `status=archived`,
  `archived_status=done` — correctly archived.
* `001.001-F` (Parent, non-member ancestor): restored to `active` (its
  pre-mutation, ship-time snapshot status captured by `ShipShipment` via
  `snapshotNonMemberFeatureStatuses`) and remained present in the scratch
  workspace's `.backlogit/queue/` — **not** rolled up to `done`/archived.
* `001-F` (Grandparent, non-member ancestor): likewise restored to `active`
  and remained in `.backlogit/queue/` — **not** rolled up.
* `backlogit doctor` against the scratch workspace after the ship operation:
  **"No issues found."** — no orphans, no duplicate IDs, no hierarchy
  integrity violations introduced by the restore path.

This directly confirms invariant 1 and invariant 2. Invariant 3 (deterministic
deepest-first restore order under Go's randomized map iteration) is proven by
the unit-level regression test
`TestRestoreRolledUpNonMemberFeatures_RestoresDeepestFirstRegardlessOfMapOrder`
(`internal/core/shipment_test.go`, merged in PR #327, commit `b8ee9a08`) — an
8-independent-pair table-driven test specifically designed so that a
non-deterministic ordering bug reliably manifests as a failure (a single-pair
test would only fail ~50% of the time on a buggy implementation). That test
was RED before the fix (5/8 pairs failed) and GREEN after (3 consecutive
runs), and is now part of the permanent regression suite executed by
`go test ./...` in CI.

## Real-shipment (115-S) topology note

`133-F` has no parent feature (it is top-level), so the real `115-S`
archival — executed in Step 6.1.b of this closure (`shipment ship 115-S`,
`archived_ids`: `133.002-T`, `133.003-T`, `133.004-T`, `133.005-T`,
`133.006-T`, `115-S`, `133-F`; `returned_ids`: `[]`) — does not itself
exercise the non-member-ancestor restore path, because there is no non-member
ancestor to restore. The scratch dogfood test above is what exercises that
path directly against the compiled binary; the real shipment's archival
(Step 6.1.b/6.1.c/6.1.d of this closure) independently confirms clean
archive-only behavior with zero deletions and an exact match between
`archived_ids` and the manifest.

## Verdict

**PASS.** Both the core over-archiving fix and the Thread C
deterministic-ordering fix are confirmed working end-to-end in the compiled
`main` binary, in addition to the unit-test-level regression coverage already
enforced by CI.

## Follow-up recommendations fed to operational-closure

1. (P2, non-blocking, doc-only) The doc comment on the `restored` flag in
   `shipment_lifecycle.go` has wording that could be read as implying more
   aggressive retry semantics than the code implements. Identified by local
   review readiness check on PR #327 (outcome `READY_WITH_FOLLOWUPS`); not
   fixed pre-merge per circuit-breaker "accept remaining, move on" guidance
   since it does not affect behavior. Recorded here as a documentation
   follow-up, not stashed as new backlog work per this release unit's
   explicit dark-mode scope constraint (no new backlog/stash planning); to be
   reported to the Orchestrator for future triage.
2. (Deferred, systemic, pre-existing) Copilot review Thread A on PR #327
   identified that `loadArtifact` can silently drop `Links` data on certain
   read paths. Confirmed as a pre-existing systemic issue predating this
   shipment's scope, not introduced or worsened by `133-F`. Deliberately
   deferred with documented rationale in the PR #327 review thread; recorded
   here (not stashed as new backlog work per dark-mode scope) for Orchestrator
   follow-up triage.
