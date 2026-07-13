---
title: "Copilot review-loop convergence under dismiss-stale-on-push rulesets"
source: docs/compound/2026-07-13-copilot-review-loop-convergence.md
doc_type: learning
description: "On repos where copilot_code_review runs on every push and dismiss_stale_reviews_on_push is true, each fix-push re-triggers a fresh review; converge by resolving the final review's threads without pushing once the cycle limit is hit."
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
review, a naive "fix review comment → push → re-review → fix → push" loop has no
natural fixed point: each remediation push spawns a new review that may raise new
threads, and `required_review_thread_resolution: true` keeps the PR merge-blocked
until every thread is resolved. Left unbounded this is a non-terminating
review-fix loop that never reaches "clean".

## Solution

Two rules converge it:

1. **Bound the loop.** `.github/instructions/github-pr-automation.instructions.md`
   §1.8 caps review-fix-push cycles at 3. The cap stops additional automated
   *pushing* — it does not clear the merge gate (§1.8 is explicit: unresolved
   Copilot threads stay merge-blocking until resolved or operator-overridden).
2. **Converge without pushing at the cap.** When the cycle-3 limit is reached with
   threads still open, do **not** push again (a push would only dismiss the review
   and spawn another). Instead, reply to and **resolve** each remaining Copilot
   thread via `gh api graphql` (`resolveReviewThread`) as an accepted follow-up
   with a reasoned decline, then run the §1.9 pre-merge readiness gate against the
   current HEAD. Resolving-without-pushing is the terminating move: it satisfies
   `required_review_thread_resolution` without triggering a fresh review.

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
(c) at the cycle cap, resolve remaining bot threads (fix or reasoned decline)
without pushing, then verify readiness from scratch via a fresh GraphQL query
against the live HEAD SHA.
