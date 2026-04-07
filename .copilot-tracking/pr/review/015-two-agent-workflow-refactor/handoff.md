<!-- markdownlint-disable-file -->
# PR Review Handoff: 015-two-agent-workflow-refactor

## PR Overview

This branch introduces the shipment-backed two-agent workflow refactor, expands the harness and workflow guidance around the new agent model, migrates stash handling to JSONL-backed indexing, repairs hierarchical F015 artifacts, and closes the follow-up review findings that surfaced during branch hardening.

* Branch: `015-two-agent-workflow-refactor`
* Base Branch: `main`
* Total Files Changed: 289
* Total Review Comments: 0

## PR Comments Ready for Submission

No outstanding review comments remain. The merged P1, P2, and P3 findings were fixed on branch before PR creation.

## Review Summary by Category

* Security Issues: 0 unresolved
* Code Quality: shipment persistence, MCP startup, and stash rehydration hardened
* Convention Violations: durable review and memory artifacts moved under `docs/closure/` and `docs/memory/`
* Documentation: workflow, agent, and memory conventions updated

## Review Artifacts

* Review closure: `docs/closure/2026-04-06/015-two-agent-workflow-refactor-review-closure.md`
* Memory artifact: `docs/memory/[20260406-195954]-015-shipment-validation-review-fixes-memory.md`
* Scratch tracker: `.copilot-tracking/pr/review/015-two-agent-workflow-refactor/in-progress-review.md`
