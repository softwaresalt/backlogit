---
title: "Copilot review-loop convergence under dismiss-stale-on-push rulesets"
source: docs/compound/2026-07-13-copilot-review-loop-convergence.md
doc_type: learning
description: "On repos where copilot_code_review runs on every push and dismiss_stale_reviews_on_push is true, each fix-push re-triggers a fresh review and dismisses the prior one — a risk of repeated review resets. Batch fixes; per §1.8 the cycle cap stops automated pushing but never clears the merge gate: only fixed or objectively invalid/informational threads may be resolved, while valid unresolved findings stay merge-blocking until fixed or explicitly operator-overridden."
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

# Copilot Review-Loop Convergence Under Dismiss-Stale-On-Push Rulesets

## Context

Surfaced during shipment 091-S (PR #231) post-merge closure. The `softwaresalt/backlogit`
`main` ruleset (14767379) combines two settings that interact non-obviously:

* `copilot_code_review` runs on **every push** to a PR branch.
* `dismiss_stale_reviews_on_push: true`.

Required checks: `Detect code changes`, `test`, `Docline frontmatter gate`.
`required_approving_review_count: 0`, `required_review_thread_resolution: true`,
`allowed_merge_methods: ["merge"]`.

## Problem

Because every push re-triggers a fresh Copilot review **and** dismisses the prior
review, a naive "fix one comment → push → re-review → fix → push" loop risks
**repeatedly resetting** the review: each remediation push spawns a new review that
may raise new threads, and `required_review_thread_resolution: true` keeps the PR
merge-blocked until every thread is resolved. This does **not** make convergence
impossible — a remediation push followed by a clean review is a natural fixed point
(the 091-S feature PR #231 reached exactly that on its only review; see Evidence).
The real failure modes are *slow* or *oscillating* convergence: unbatched
per-comment pushes multiply review resets, and without a bound automated fixing can
run longer than is useful. Hence a cycle cap — but the cap must never be used to
clear the merge gate on findings that are still valid and unfixed.

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
   gate. Batch fixes into a single push (each push is a review reset, per §1.7),
   and after the final push run the §1.9 pre-merge readiness gate against the live
   HEAD SHA.

## Evidence and honest scope

In 091-S itself the loop was **not** triggered: the single Copilot review was clean
(COMMENTED, "6/6 files, no comments", 0 threads) on the first and only HEAD, so 0
review-fix cycles ran. This entry therefore documents the *dynamic and the
convergence rule derived from the verifiable ruleset configuration and §1.8*, not a
loop this session had to break. The guidance stands for any future PR on this
ruleset that does draw Copilot threads.

## Applicability

Applies to any GitHub repo whose branch protection combines review-on-every-push
automation with `dismiss_stale_reviews_on_push` and thread-resolution requirements.
Key operational reflexes: (a) treat every push as a review reset, so batch fixes
rather than pushing per-comment; (b) never push purely to "re-trigger" a review;
(c) at the cycle cap, resolve **only** threads that were fixed or objectively
invalid/informational — valid unresolved findings stay merge-blocking until fixed or
explicitly operator-overridden, so **halt and escalate** rather than blanket-resolving
to clear the gate; (d) verify readiness from scratch via a fresh GraphQL query
against the live HEAD SHA.
