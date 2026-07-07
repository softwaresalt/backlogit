---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 084-S — ancestor-aware shipment-gate member-evidence staleness (084-F, task 084.001-T + 3 subtasks). Feature PR #182 merged into main via a true 2-parent merge commit f49ce3c37b460afce81591ca6e354b8de3a14a17 (merge strategy preserved per P-009; admin-bypassed the human-approval ruleset under explicit operator P-014 approval). Shipment 084-S closed via backlogit shipment ship using a backlogit.exe REBUILT from merged main (the bootstrapping step — the security fix closes its own shipment): shipment_status=shipped, 6 archived_ids (084-F + 084.001-T + 3 subtasks + 084-S), merge SHA recorded, 0 returned/blocked. The completion gate PASSED because member feature-branch heads are now ancestors of the merge commit; strict-equality would have falsely rejected them (the exact bug this shipment fixes). shipment-reconcile pre (4 members pre-archived + covering feature auto-completed by ShipShipment, PROCEED) and post (6/6 in archive, 0 deletions, PROCEED) both passed; P-007 archive integrity clean. Knowledge graduation: 2 durable compound learnings added (ancestor-aware staleness + fail-closed merge-base exit-code handling; bounded-helper-timeout hard cap DoS), 082-S timeout-before-probe entry updated with a forward cross-ref; compound-refresh report written. Deployment path merge-only (Go core; no migration/service/rollout). Rollback = git revert of the merge commit. Readiness READY (feature merged). Closure work isolated on post-merge/084-S branch; per explicit operator directive the closure PR (#183) is AUTHORIZED for autonomous merge (admin bypass, merge-commit) with no separate approval gate — the feature PR #182 was the individually operator-approved P-014 gate.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-084-S-post-merge-closure.md
title: 084-S ancestor-aware shipment-gate staleness — Post-Merge Operational Closure
---

# Post-Merge Operational Closure — 084-S ancestor-aware shipment-gate staleness

**Mode**: `post-merge`
**Context**: PR #182 **MERGED** · shipment 084-S / feature 084-F (task 084.001-T + 3 subtasks) · closure work on branch `post-merge/084-S`.
**Feature merge commit**: `f49ce3c37b460afce81591ca6e354b8de3a14a17` (2-parent merge commit — parents `57722ba` prior-main + `1441c6d` feature HEAD; merge strategy preserved per P-009).
**Merge authorization**: explicit operator **P-014** approval + authorized admin bypass of the human-approval ruleset (`--admin`). `--delete-branch` removed the feature branch.
**Verification report**: `docs/closure/2026-07-06-084-S-feature-pr-runtime-verification.md` (PASS, real `git merge-base --is-ancestor` subprocess, 5 scenarios).
**Adversarial review**: `docs/closure/2026-07-06-084-S-feature-pr-adversarial-review.md` (3 models, 0 P0/P1).
**Pre-merge operational closure**: `docs/closure/2026-07-06-084-S-feature-pr-operational-closure.md`.

## Summary of change

The shipment completion gate (`internal/core/shipment_gate.go`
`validateMemberGateEvidence`) previously judged a member's recorded `head_sha` stale
by strict inequality against the current shipment head. After a normal merge-commit
merge, a member's feature-branch head becomes an *ancestor* of the merge commit
(never equal), so strict equality falsely rejected the shipment's own members and
blocked closure. The fix makes the non-empty-member-head path **ancestor-aware** via
a bounded, fail-closed `git merge-base --is-ancestor`, keeping every
timeout/cancel/exec-error/non-{0,1}-exit/head-drift path refusing. Merged and live on
`main`.

## Merge confirmation

- `gh pr view 182 --json state,mergedAt,mergeCommit` → `state: MERGED`,
  `mergedAt: 2026-07-07T06:53:22Z`, `mergeCommit.oid: f49ce3c…`.
- `git rev-list --parents -n 1 f49ce3c` → **2 parents** (`57722ba` + `1441c6d`) →
  true merge commit, **not** squash/rebase (P-009 preserved).
- `git merge-base --is-ancestor f49ce3c origin/main` → exit 0: merge is in
  `origin/main` history. **MERGE_CONFIRMED.**

## Shipment closure (bootstrapping — the fix closed its own shipment)

- `backlogit.exe` was **rebuilt from merged `main`** (`go build -o backlogit.exe
  ./cmd/backlogit`) before closure, so 084-S was closed with the NEW ancestor-aware
  binary. This is load-bearing: 084-S's own members recorded feature-branch heads
  (e.g. `7f609e0`) that are now ancestors of `f49ce3c`; the old strict-equality
  binary would have refused to close 084-S.
- `backlogit shipment ship 084-S --sha f49ce3c… --message "Merge pull request #182…"
  --author "Derek Williams <…>"` → `shipment_status: shipped`, **6 archived_ids**
  (084.001.001-ST, 084.001.002-ST, 084.001.003-ST, 084.001-T, 084-F, 084-S),
  `returned_ids: []`, `commit_sha: f49ce3c…`. **Completion gate PASSED (exit 0).**
- The covering feature 084-F was `active` at ship time and auto-completed to `done`
  by `ShipShipment` (`shipment_lifecycle.go:192`).

## Reconciliation (shipment-reconcile)

- **Pre** (`.backlogit/reconcile/084-S-pre-2026-07-06-235630.md`): 4 member work
  items pre-archived (done from build phase); covering feature 084-F active by
  design (auto-completed by ship); 0 missing / 0 orphan → **PROCEED**.
- **Post** (`.backlogit/reconcile/084-S-post-2026-07-06-235810.md`): 6/6 items
  present in archive; **P-007 deleted-file guard: 0 deletions** → **PROCEED**.

## Item disposition

| Item | Type | Final status | Location |
|---|---|---|---|
| 084-F | feature | done (released) | archive |
| 084.001-T | task | done | archive |
| 084.001.001-ST | subtask | done | archive |
| 084.001.002-ST | subtask | done | archive |
| 084.001.003-ST | subtask | done | archive |
| 084-S | shipment | shipped | archive |

## Knowledge graduation

- **New compound learnings** (docline-clean):
  - `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md` — the
    ancestor-vs-equality pattern + fail-closed `git merge-base --is-ancestor`
    exit-code handling (incl. the Windows ctxErr-before-ExitError gotcha).
  - `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md` — a bounded helper
    still needs a hard cap; do not adopt a 600s command timeout for near-instant
    metadata reads on a lock-holding path (the Copilot-remediated DoS).
- **compound-refresh** (`docs/closure/2026-07-06-084-S-compound-refresh.md`):
  082-S `external-process-timeout-before-probe` entry classified **update**
  (forward cross-ref added to the hard-cap entry); no supersessions/deletions.
- No `ARCHITECTURE.md` / `AGENTS.md` structural change required (localized core fix).

## Security posture

- **Non-weakening confirmed** (3-model adversarial review, 0 P0/P1): ancestor-inclusion
  is a reachability guarantee; divergent heads still rejected (exit 1); residual
  post-gate edits covered by the unchanged aggregate diff check. All
  git-exec/timeout/cancel/malformed/head-drift paths fail closed.
- Scope untouched (verified): empty-member-head bypass, empty-shipment-head fail-open,
  malformed-JSONL path — all separately reasoned, none modified.

## Deployment / monitoring / rollback

- **Deployment**: merge-only. Go core change; no migration, service, or rollout. Any
  environment closing multi-commit shipments must run a `backlogit` built at/after
  `f49ce3c`.
- **Monitoring**: watch for `shipment ship` completion-gate refusals citing member
  staleness — post-fix these should occur only for genuinely divergent heads or
  fail-closed (timeout/malformed/drift) conditions, never for ancestor heads.
- **Rollback**: `git revert f49ce3c` (revert the merge commit). Reverting restores
  strict-equality staleness and will re-block multi-commit shipment closures; only
  do so if the ancestor-aware path proves defective.

## Deferred follow-ups (stashed for Stage triage)

Documented in the pre-merge operational closure §6 and carried forward here:

- **ADV-2** — ABA / there-and-back HEAD race (not a regression of this change).
- **ADV-3** — ambient-HEAD vs `--sha` target-anchor semantics (pre-existing).
- **ADV-5** — scope interaction with the empty-member-head bypass (`B85DAEE8`) and
  malformed-JSONL (`F3844849`) next-phase items.

## Source artifact cleanup

- Plan `docs/exec-plans/2026-07-06-shipment-gate-ancestor-aware-staleness-plan.md`
  and deliberation `docs/decisions/2026-07-06-shipment-gate-ancestor-aware-staleness-deliberation.md`
  were delivered atomically in PR #182 (via the Stage harvest) and remain as the
  durable planning record. Source stash `885A7F65` was consumed by Stage during
  harvest (pre-Ship); no additional stash removal performed by Ship (scope-excluded
  stash items `B85DAEE8`/`F3844849`/`1AEA2B0E` untouched).

## Readiness

**READY (feature merged).** Feature PR #182 merged under operator P-014 approval. Closure work
isolated on `post-merge/084-S`. Per explicit operator directive for this run, the
closure PR (#183) is **authorized for autonomous merge** (admin bypass, merge-commit, delete branch) —
the closure is docs/administrative only and the feature merge was the individually
operator-gated decision.
