---
title: "Copilot review-loop convergence under auto-review-on-push rulesets"
source: docs/compound/2026-07-13-copilot-review-loop-convergence.md
doc_type: learning
description: "On repos where copilot_code_review auto-triggers on every push, each push spawns a fresh Copilot review that may raise new threads while prior unresolved threads persist (dismiss_stale_reviews_on_push affects stale approvals, not COMMENTED reviews or their threads). Batch fixes to minimize review passes; per §1.8 the cycle cap stops automated pushing but never clears the merge gate: only fixed or objectively invalid/informational threads may be resolved, while valid unresolved findings stay merge-blocking until fixed or explicitly operator-overridden."
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
    date: 2026-07-13T00:00:00Z
    severity: medium
    tags:
        - github
        - pull-request
        - copilot-review
        - branch-protection
        - ci
        - circuit-breaker
        - ship
---

# Copilot Review-Loop Convergence Under Auto-Review-On-Push Rulesets

## Context

Surfaced during shipment 091-S (PR #231) post-merge closure and reinforced on the
closure PR #232. The `softwaresalt/backlogit` `main` ruleset (14767379) combines two
settings whose interaction is easy to misread:

* `copilot_code_review` runs on **every push** to a PR branch — each push
  auto-triggers a fresh Copilot review.
* `dismiss_stale_reviews_on_push: true` — this dismisses stale **approvals**
  (`APPROVED` reviews) only. `CHANGES_REQUESTED` and `COMMENTED` reviews are **not**
  dismissed by this setting, and Copilot posts `COMMENTED` reviews — so it does not
  dismiss Copilot's review or remove its threads: prior unresolved threads
  **persist** across pushes (they may render as `isOutdated` when the lines they
  anchored to shift, but they still count toward
  `required_review_thread_resolution`).

Required checks: `Detect code changes`, `test`, `Docline frontmatter gate`.
`required_approving_review_count: 0`, `required_review_thread_resolution: true`,
`allowed_merge_methods: ["merge"]`.

## Problem

The convergence risk is **not** that each push "resets" the review. It is that every
push auto-triggers a *new* Copilot review that can raise *new* threads — including on
the very text added to remediate earlier findings — while the earlier unresolved
threads **persist**, and `required_review_thread_resolution: true` keeps the PR
merge-blocked until every thread is resolved. A naive "fix one comment → push →
re-review → fix → push" loop can therefore accumulate threads across passes.

This does **not** make convergence impossible: a push whose review raises no new
threads, with all prior threads resolved, is a natural fixed point (the 091-S feature
PR #231 reached a clean review on its only pass; see Evidence). The real failure
modes are *slow* or *oscillating* convergence — unbatched per-comment pushes multiply
review passes and give the bot more chances to raise fresh threads. Hence a cycle cap,
which must never be used to clear the merge gate on findings that are still valid and
unfixed.

## Solution

Two rules converge it **without bypassing the merge gate**:

1. **Bound the loop.** `.github/instructions/github-pr-automation.instructions.md`
   §1.8 caps review-fix-push cycles at 3. The cap stops additional automated
   *pushing* — it does **not** clear the merge gate (§1.8 is explicit: unresolved
   Copilot threads stay merge-blocking until resolved or **explicitly
   operator-overridden**).
2. **Converge by fixing, not by blanket-resolving.** Handle each remaining thread
   on its merits (§1.3): **fix** valid findings (reply with the fix commit, then
   resolve the thread), or **decline** threads that are objectively
   invalid/informational (reply with the rationale, then resolve). Only
   *fixed-or-objectively-invalid* threads may be auto-resolved. A **valid** finding
   must never be converted into a "reasoned decline" merely to satisfy branch
   protection — it stays merge-blocking until fixed or explicitly
   operator-overridden. If valid findings remain unfixed when the cycle cap is hit,
   **halt and escalate to the operator** rather than resolving them to clear the
   gate. Batch fixes into a single push (each push auto-triggers another Copilot
   review that may raise new threads, per §1.7), and after the final push run the
   §1.9 pre-merge readiness gate against the live HEAD SHA.

## Evidence and honest scope

In 091-S itself the loop was **not** triggered: the single Copilot review was clean
(COMMENTED, "6/6 files, no comments", 0 threads) on the first and only HEAD, so 0
review-fix cycles ran. This entry therefore documents the *dynamic and the
convergence rule derived from the verifiable ruleset configuration and §1.8*, not a
loop this session had to break. The guidance stands for any future PR on this
ruleset that does draw Copilot threads.

## Reinforcement — 092-S (PR #235, merge `4a90bf4`)

The feature PR for shipment 092-S (item-writer UTC timestamp normalization — 36
changed files across `internal/models`, `internal/core`, `internal/core/templates`,
and `internal/cli`) reached the **same clean fixed point on its first and only
HEAD**: Copilot posted a single `COMMENTED` review ("36/36 files, no comments"),
0 threads, so **0 review-fix cycles** ran and the §1.9 readiness gate passed
directly. This is a second, larger data point (36 files vs 091-S's 6) that a
substantive change can converge in one pass — and the operative variable was
**upstream hardening, not luck**: the implementation plan carried an exhaustive
writer-site inventory and an explicit parallel-test-safe RED-phase design (the
`t.Parallel()` / hermetic-`TZ`-subprocess caveat), so the code that landed left
little for the bot to flag. Contrast the *staging* PR in the same lineage, which
ran a multi-cycle loop before its plan was hardened. The durable inference: the
cheapest way to keep the Copilot loop at zero cycles is to eliminate findings
**before** the first push (thorough plan + local `review` gate + green
constitution gates), not to get better at resolving threads after the fact. The
cycle-cap discipline (§1.8) remains the safety net for PRs that *do* draw
threads, and must never be used to clear the merge gate on still-valid findings.

## Applicability

Applies to any GitHub repo whose branch protection auto-triggers a bot review on
every push alongside `required_review_thread_resolution`. Key operational reflexes:
(a) treat every push as a trigger for another Copilot review that may raise new
threads while prior threads persist, so batch fixes rather than pushing per-comment;
(b) never push purely to "re-trigger" a review; (c) at the cycle cap, resolve **only**
threads that were fixed or objectively invalid/informational — valid unresolved
findings stay merge-blocking until fixed or explicitly operator-overridden, so **halt
and escalate** rather than blanket-resolving to clear the gate; (d) verify readiness
from scratch via a fresh GraphQL query against the live HEAD SHA.
