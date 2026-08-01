---
chunk_strategy: h1-h2-h3
doc_type: closure
docline:
    date: 2026-08-01T00:00:00Z
    tags:
        - compound-refresh
        - shipment
        - 115-S
        - 133-F
        - p-015
ingested_at: "2026-08-01T20:05:00Z"
schema_version: "1.0"
source: docs/closure/2026-08-01-115-S-compound-refresh.md
title: 'Compound Refresh — Shipment 115-S (ShipShipment partial-feature cascade fix)'
---

# Compound Refresh — Shipment 115-S

**Context:** Feature 133-F / Shipment 115-S — `core.ShipShipment` stops
over-archiving covering features on partial-feature shipments (PR #327, merge
`47dfcc93698a6b0b2c5420c701c365a538895580`).
**Mode:** apply
**Date:** 2026-08-01

## Entries Reviewed

| File | Classification | Action |
|---|---|---|
| `2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md` | update | Added a 2026-08-01 update section (body) and rewrote `description` (frontmatter) recording that the root-cause fix it called for has shipped in 133-F/PR #327; cross-referenced the updated P-015 policy text and the new version-skew caveat entry. Original 114-S incident record preserved verbatim below the new section — no citations lost. |
| All other compound entries | keep | No topical overlap with shipment archival cascade, map-iteration test design, or CLI version-skew discovered this session. |

## New Entries Created

| File | Reason |
|---|---|
| `2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md` | New reusable learning: a merged fix to a self-hosted tool's own source does not protect real operations performed via a separately pinned, already-installed release binary of that same tool. Discovered directly: the installed `C:\Tools\backlogit.exe` v1.7.0 (commit `7daf8c3`, 2026-07-23) predates the 133-F fix (merge `47dfcc93`, 2026-08-01) by 9 days; every real CLI call this session — including the actual `115-S` archival — ran under the pre-fix binary. Distinct from the existing `2026-07-13-post-merge-lifecycle-requires-fresh-binary.md` entry, which concerns a local dev build going stale relative to the *same* working copy's own uncommitted/just-merged changes (solved by "just rebuild"); here the operational convention pins a *separately versioned release artifact* with no local-rebuild step in its path at all. |
| `2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md` | New reusable learning: a single-pair regression test for a Go map-iteration-order bug has only ~50% probability of failing per run and can pass-by-luck even against unfixed code, or flake in CI long after merge. An N-independent-pair design (this session used N=8) drives the miss probability to `0.5^N` (~0.4% at N=8) and was decisive in this session's Thread C fix: 5/8 pairs failed pre-fix in one run, 3 consecutive runs passed post-fix. |

## Evidence Used

- `internal/core/shipment_lifecycle.go` diff between staging commit `4518b71d` (pre-133-F) and merge `47dfcc93` (post-133-F): `collectArchiveCandidateIDs` explicit-membership gate on both the feature-archival branch and its descendant/terminal-sibling sweep; `snapshotNonMemberFeatureStatuses` / `restoreRolledUpNonMemberFeatures` snapshot-and-revert mechanism for the independent `completeReleaseScope` → `cascadePersistedParentStatuses` rollup path; deepest-first restoration order via `depthSortedIDs`.
- `.github/policies/workflow-policies.md` P-015 section, already updated on `main` (commits `c444d5ae` "retire P-015 manual workaround for membership-gated ships" and `73301353` "clarify P-015 subtree archival exception", both ancestors of `47dfcc93`, both part of the 133-F implementation work merged in PR #327) to state the Code Enforcement rationale and that the Ship agent MAY call the native cascade for partial-feature shipments.
- `internal/core/shipment_test.go` — `TestRestoreRolledUpNonMemberFeatures_RestoresDeepestFirstRegardlessOfMapOrder` (8-independent-pair table-driven test; RED 5/8 pre-fix, GREEN ×3 post-fix).
- Runtime verification dogfood test (`docs/closure/2026-08-01-133-shipshipment-cascade-fix-runtime-verification.md`): 2-level nested covering-feature hierarchy against the compiled `main` HEAD binary confirmed the leaf manifest member archived and both non-member ancestors restored to their pre-ship `active` status, remaining in queue.
- `backlogit version` output for the installed `C:\Tools\backlogit.exe` (`1.7.0`, commit `7daf8c3`, built `2026-07-23T22:32:43Z`) and `git merge-base --is-ancestor 7daf8c3 47dfcc93` (confirmed ancestor, exit 0) establishing the 9-day version-skew gap.

## Follow-up Items

Per this release unit's explicit dark-mode scope constraint ("Do not create or
process new backlog/stash planning work"), the following are recorded here as
Ship's required closure output and reported to the Orchestrator, **not**
created as new `backlogit_stash`/backlog entries:

1. **(P2, non-blocking, doc-only)** The doc comment on the `restored` flag in
   `shipment_lifecycle.go` has wording that could be read as implying more
   aggressive retry semantics than the code implements (local review
   readiness finding on PR #327, outcome `READY_WITH_FOLLOWUPS`).
2. **(Deferred, systemic, pre-existing)** Copilot review Thread A on PR #327:
   `loadArtifact` can silently drop `Links` data on certain read paths.
   Confirmed pre-existing, not introduced or worsened by 133-F; deferred with
   documented rationale in the PR #327 review thread.
3. **(Operational risk, high-visibility)** The installed `C:\Tools\backlogit.exe`
   (`v1.7.0`, commit `7daf8c3`) predates the 133-F fix by 9 days. Any future
   partial-feature shipment closed via this pinned CLI before it is upgraded
   past `47dfcc93` remains exposed to the original over-archiving cascade bug.
   Recommend the Orchestrator/operator cut and install a new `backlogit`
   release incorporating `47dfcc93` (or later) before closing the next
   partial-feature-shaped shipment, and re-run `backlogit version` to confirm
   before relying on the cascade path.
