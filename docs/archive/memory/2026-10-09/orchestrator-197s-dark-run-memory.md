---
schema_version: "1.0"
doc_type: memory
title: Orchestrator dark run 197-S staging and preflight
description: P-017 dark-mode record for scope [197-S]. Preflight decisions - parked operator config, stale 153-S checkpoint left alone, dag-root sequencing decision, resolved agent routes.
timestamp: "2026-10-10T02:25:00Z"
---

# Orchestrator dark run 197-S staging and preflight

## DARK_MODE_ACTIVE

- Scope: `[197-S]` only. Ordered sequence `[197-S]`; last completed: none; next: `197-S`.
- `merge_approval_pre_authorized: true` (operator granted PR open and merge approval).
- `admin_fallback_pre_authorized: false` (not granted).
- Required before PR: multi-persona adversarial review of code and decisions.
- Copilot loop: wait patiently for each review, fix, push, reply to every comment, then resolve each thread with `gh api graphql`; repeat until a review pass is clean.
- Budget: past 10000 AIC, finish the current shipment through merge, then stop. Scope is one shipment, so the stop is already implied.

## Preflight decisions

- **Stale checkpoint left alone.** `checkpoint-20261008-073934.json` (ship, 153-S, `active`) is out of scope. PR #485 and closure PR #486 are merged, so it is stale bookkeeping. I did not resolve it (owning-agent action) and did not route recovery. A later Ship session can resolve it.
- **Operator config parked (ProposedAction, risk moderate, reversible).** `.autoharness/config.yaml` carried the operator's uncommitted routing edit. The pre-claim topology gate (`WORKTREE_DIRTY`) and Ship's clean-tree gate both block on it. I saved a byte-identical backup to `logs/config.yaml.operator-edit-2026-10-09.bak` (sha256 `6D5E5614FCC6A82FD6B83BBE692595DD1667DB835C1B2D9160B6B14DDCC661F5`) and ran `git stash push -- .autoharness/config.yaml` (stash message names this run). Restore with `git stash pop` after the run; if the operator re-edited the file meanwhile, diff against the backup first. The prior orchestrator memory had already offered "commit it in its own PR or stash it" as the two options.
- **Routes resolved from the fresh config read, before the stash.** The edit passed `autoharness verify-workspace` (`strict_schema_blockers: []`; only route-variable-stale warnings). Stage: `claude-sonnet-5.5`/anthropic/xhigh. Ship: `claude-haiku-5.5`/anthropic/xhigh/`long_context`. Ship escalation: `claude-sonnet-5.5`/anthropic/medium. These are passed as per-invocation overrides.
- **Served-root handoff passed.** Workspace and storage roots attested through the MCP catalog and `pragma_database_list`; no reparse points; the `197-S` manifest in `queue` matched `backlogit_get_shipment`.
- **Pre-claim gate blocked 197-S as `UNSEQUENCED_SHIPMENT`.** I decided on a `dag-root` declaration (a `blocks` edge onto 196-S would fail `PREDECESSOR_CLOSURE_INCOMPLETE`, as for 196-S over 195-S). Stage executed it on `chore/stage-197-S-sequencing`; see `stage-197s-sequencing-edge.md`. A rubber-duck review of the decision returned `APPROVE_WITH_NOTES`.
- **Staging PR #490.** The Orchestrator owns it under the Step 1.5 carve-out.

## Next steps

1. Merge #490 after Copilot and CI are clean; fast-forward local `main`; re-run `pre_claim` for 197-S on clean `main` (expect `declared_root`).
2. Invoke Ship for 197-S with the Ship route above and the served-root payload.
3. After closure, `git stash pop` the operator config and verify it equals the backup.
