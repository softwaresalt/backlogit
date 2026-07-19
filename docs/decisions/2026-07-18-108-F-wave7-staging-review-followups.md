---
chunk_strategy: h1-h2-h3
title: "108-F wave-7 staging review follow-ups (Cluster B) — deferred to next staging round"
source: docs/decisions/2026-07-18-108-F-wave7-staging-review-followups.md
doc_type: decision
description: "Three verified Copilot wave-7 findings on the 099-S/108-F staging PR #259 (F5 backlog consistency, F6 create->size failure boundary, F7 env-expansion test scope), deferred by operator decision to the next staging round; captured with code evidence and recommended fixes."
docline:
    date: 2026-07-18
    origin: 099-S / PR #259 wave-7 review
    status: accepted
    linked_parent_work_item: 108-F
    tags:
        - size-estimation
        - staging
        - review-followup
schema_version: "1.0"
---

## Context

During staging of shipment `099-S` (feature `108-F`, size estimation for
feature and shipment artifacts) on PR #259, Copilot's wave-7 review (HEAD
`c79ba4b`, triggered by operator manual push `411182d`) raised 7 threads.
Four were scope-hygiene comments on operator-pushed files and were kept by
operator decision (declined with rationale, threads resolved). The remaining
three ("Cluster B") are genuine planning-content findings, each verified
against the current codebase.

## Decision

Keep PR #259 scope-pure and merge it first. Defer F5/F6/F7 to a dedicated
follow-up staging round. Queue them as backlogit stash `B062B90A`
(kind=feature, priority=medium). This doc and the stash entry are git-stashed
off PR #259 and will be `git stash pop`-ed onto `main` and committed after
PR #259 merges. Next round, Stage harvests bounded ≤2h width-isolated tasks
under `108-F`.

## Findings

### F5 — 108-F ↔ 099-S shipment-home consistency (backlog, low risk)

Thread: `.backlogit/queue/099-S.md:6`.

- Evidence: `.backlogit/queue/108-F.md` title (line 24) still reads
  "…impl-plan promotion pending"; the body states "the feature still has no
  shipment home" (line 29) and "It has NO shipment home: it is a member of no
  shipment manifest … until a later Stage restaging" (line 32). But 108-F is
  now a member of shipment 099-S.
- Recommended fix: add an authoritative reconciliation block to 108-F
  recording that it is now a member of 099-S and superseding the "no shipment
  home / promotion pending" statements; soften the title. Keep implementation
  deferred (staged, not in progress); retain the historical narrative for
  traceability, marked superseded. Single-file backlog edit.

### F6 — 108.002-T generic-create → size failure boundary (core, real robustness gap)

Thread: `.backlogit/queue/108.002-T.md:21`.

- Claim: the "MIGRATION-SAFE" generic-create-with-provenance path is
  under-specified across its failure boundary.
- Evidence (verified): `core.CreateArtifact`
  (`internal/core/artifacts.go:344-369`) writes the file (rename, :348) AND
  upserts the SQLite index (:363) before returning, and the create path has a
  destination-collision guard (:338-339, `blerrors.ErrIDCollision`). The
  migration caller (`cli/migrate.go` `WithFields -> CreateArtifact`) routes
  through the size seam AFTER the artifact already exists. If the subsequent
  estimate-history event append or size write fails, migration reports failure
  but leaves a persisted+indexed UNSIZED artifact; a retry re-runs
  `CreateArtifact` with the same ID and hits the destination-collision guard,
  so the import can never succeed on retry and the imported size/provenance is
  lost. The task's existing "event-before-write, fail-closed" note covers the
  size seam's own two-step, NOT the create-then-size two-step across
  `CreateArtifact`'s boundary.
- Recommended fix (tighten — do not silently narrow): either (a) integrate the
  size/estimate-history write into the create so an initial size is durable
  atomically (event-before-write BEFORE the artifact is durably
  persisted/indexed, or as one atomic step), OR (b) define explicit
  rollback/cleanup on partial failure (remove the just-created file AND the DB
  index row AND the `canonicalCache` record so a retry does not collide). ADD a
  fail-then-retry migration test (inject a size/event failure after create;
  assert no orphaned unsized artifact remains and the retry succeeds). If
  atomic integration is too wide for a ≤2h width-isolated unit, split the
  rollback/cleanup + fail-then-retry test into a NEW dependent task under
  108-F rather than dropping the requirement. Note: this is the same
  failure-boundary class as the earlier crash-window descope (Option B2) but
  on the in-process migration path — evaluate whether it should ride with or
  supersede the deferred crash-window stash `9D5BB492`.

### F7 — 108.009-T env-expansion test targets nonexistent behavior (config, test-scope correction)

Thread: `.backlogit/queue/108.009-T.md:17`.

- Claim: the env-expansion escape test has no current behavior to exercise;
  only webhook header values are env-expanded.
- Evidence (verified): in `internal/config/loader.go`, only notification
  endpoint HEADER values are validated for `$`/env-expansion
  (`loader.go:113-121`); search roots (`reg.Directories`) and
  `QueueLayout.RootDir` are NEVER passed through `os.ExpandEnv` (the sole
  `os.ExpandEnv` reference is a comment at `loader.go:114` describing header
  resolution). So a `$VAR` root is a literal in-workspace name, not an
  expansion escape. Testing "env-expansion escape in a search root rejected"
  exercises nonexistent behavior and would force introducing+rejecting path
  expansion solely to satisfy the test — which the impl-plan's alternative
  explicitly avoided.
- The LEXICAL gap is real and should stay: `loader.go:77-85` (LoadRegistry)
  validates only `reg.Directories` for `../`/absolute escape;
  `QueueLayout.RootDir` (`schema.go:99-104`) has no containment validation.
- Recommended fix: in 108.009-T (SE-7a) and impl-plan SE-7a, REMOVE the
  env-var-expansion rejection requirement and its test case; KEEP only the
  lexical `../`/absolute containment guard for `QueueLayout.RootDir` and every
  configured search root. Optionally add a test asserting roots are treated
  literally (NOT env-expanded).

## Staging outcome (completed)

Completed in the next staging round (shipment `100-S`, PR #260):

- Stash `B062B90A` triaged and **archived** (consumed).
- Shipment **`100-S`** created (`queued`) — "108-F wave-7 staging review follow-ups (F5/F6/F7)".
- **F5** → task `108.012-T` — reconcile 108-F backlog artifact for 099-S membership.
- **F6** → task `108.013-T` — add rollback/cleanup to `CreateArtifact` post-create failure boundary; **depends on `108.002-T`** (099-S), which introduces the post-create size/estimate-event operation whose failure boundary this task hardens.
- **F7** → task `108.014-T` — reconciliation record. The correction was applied **in place** to `108.009-T` (SE-7a, shipment 099-S) during staging (env-var-expansion rejection requirement + test dropped; lexical `../`/absolute containment guard retained). The containment implementation remains in `108.009-T` so the SE-7a/SE-7b two-layer invariant ships as one unit with `108.010-T` in 099-S; `108.014-T` carries no separate implementation.
- F6 / `9D5BB492` crash-window stash kept separate (different failure class).
