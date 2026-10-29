---
schema_version: "1.0"
doc_type: memory
title: Orchestrator 197-S PR-stage decisions after the confirming-review halt
description: Dark-mode decisions made while the operator was AFK so Ship can resume to PR - R7 partial ruling, P1-4 residual risk, one extra review-fix cycle, reviewer independence, and the AIC reading.
timestamp: "2026-10-11T00:10:00Z"
---

# Orchestrator 197-S PR-stage decisions

Context: Ship halted before the PR (`ship-197s-confirming-review-halt-memory.md`): five P1 findings, three open.
All decisions below are Orchestrator judgment under P-017 (operator AFK, sound-judgement authority) and are
**subject to operator veto**. Stage recorded them in plan Erratum E3.

## Decisions

1. **R7 is accepted as partial.** The Orchestrator text consumes the attested `workspace.queue_path` and
   `workspace.archive_path` (the R7 contract). The Go metadata catalog still hard-codes `storage/queue` and
   `storage/archive` for non-default `queue_layout.root_dir`. That can only cause a false fail-closed, never an unsafe
   pass, and it is a different contract surface (P-021 C1 fails). Named limitation: capture `00A9D01C`. This
   workspace uses the default layout and its catalog values are correct.
2. **P1-4 is accepted as documented residual risk.** The safe-close report does not list exact tracked side-effect
   paths (for example `.backlogit/hooks_queue.jsonl`), so the closure allowlist never stages them. It does not
   block closure: the closure commit stages them by explicit path. Ship captures it under P-021 C2 and cites it in
   the residual-risk record.
3. **One extra review-fix cycle, hard-capped.** The 3-cycle cap was reached. A single additional cycle is
   authorized, limited to in-scope P0/P1 findings of the confirming review. A P0/P1 that remains after it halts
   Ship; it must not iterate further. Everything else is a capture.
4. **P-007 step 3 policy conflict and other P2/P3 items** are different surfaces: capture only.
5. **The 10000 AIC rule never justifies stopping before the PR.** Past 10000, Ship finishes the shipment through
   PR, merge, and closure, then stops. Session usage was about 7.3k AIC when this was written (sum of
   `session.usage_record` `totalNanoAiu` across all subagent sessions in `events.jsonl`; an earlier 2.7k reading
   was stale).
6. **Reviewer independence.** Reviewers must not run on the author's model (`claude-haiku-5.5`). The first
   adversarial round had one haiku reviewer (not counted) and an unpinned review-skill model (not counted). The
   confirming pass must use `gpt-6.1-sol` (anchor) and `claude-sonnet-5.5`, with the model per reviewer recorded.

## Sequence for the next Ship invocation

Fix nothing the decisions above defer; apply in-scope fixes only through the single extra cycle; capture the
uncaptured P2/P3 items; re-run the AC3 drift refresh (`_ship.agent.md` changed); run the non-haiku confirming pass
on the final HEAD; then PR, Copilot loop, CI, merge, closure.
