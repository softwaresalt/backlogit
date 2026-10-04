# Orchestrator dark run 196-S — halted (SERVED_ROOT_ATTESTATION_FAILED)

## Outcome

- DARK_MODE_ACTIVE scope `[196-S]`; merge pre-authorized; admin fallback not authorized.
- DARK_MODE_HALTED at Ship dispatch: `SERVED_ROOT_ATTESTATION_FAILED`.
- `196-S` remains `queued`; no claim, branch, task, PR, or closure mutation by Ship.
- `main` == `origin/main` == `2e89bfb6` (PR #475 merge).

## Completed this run

- Staging PR #475 (196-S `dag-root` waiver + `docs/memory/2026-10-03/stage-196s-sequencing-edge.md`): adversarial review (4 reviewers, F1–F8 remediated), 2 Copilot rounds (1 thread fixed in `ef820639`, replied, resolved via GraphQL), CI green, `autoharness gate copilot-review` PASS, merged with merge commit `2e89bfb6`, post-merge ff-only sync verified.
- Topology `pre_claim` gate PASS on clean `main` (`predecessor_source: declared_root`).
- Served-Root Handoff Procedure steps a–f PASS; R8 record: `docs/memory/2026-10-04/orchestrator-196s-served-root-handoff.md`.

## Halt cause (plan defect)

- Plan `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md` R14 (bootstrap) and U9 require Ship to call MCP `backlogit_get_metadata_catalog` (MCP-only; "there is no CLI attestation").
- `.github/agents/_ship.agent.md` frontmatter `tools:` does not grant `backlogit/backlogit_get_metadata_catalog` (nor `backlogit_get_version`). U9 is scoped to Step 4.0 item 4 / Step 4.1b text and never adds the grant.
- Ship correctly refused to substitute the CLI catalog output and halted before claim.

## Why the Orchestrator did not continue autonomously

- Fixing requires changing Ship's tool grants (authority surface) outside U9's declared file scope (P-021), as a prerequisite change merged before dispatch — new work outside dark scope `[196-S]` (P-017 no scope expansion).
- Runtime agent definitions may only reload on a new CLI session, so a same-session fix is not verifiable.
- Waiving the MCP-only attestation would downgrade a plan safety requirement without operator approval.
- P-017 stop condition: required tool unavailable with no permitted fallback.

## Follow-ups

- Stash `C24BC82F` (bug, critical): Stage re-plan Amendment 2 — add the tool grant(s) to Ship's allowlist with a contract-test assertion, decide bootstrap sequencing (grant merged and loaded before dispatch), and consider a plan-review check that agent-specific MCP tools exist in the target agent's allowlist.
- Uncommitted continuity changes left on `main` for the next run's Step 1.5 carry-forward: `.backlogit/stash.jsonl` (C24BC82F) and `docs/memory/2026-10-04/**`.

## Next steps (operator)

1. Decide the remediation path for `C24BC82F` (grant tool to Ship, or approve an alternative attestation).
2. Restart the CLI session after any allowlist change merges, then re-run `Run pipeline in dark mode` for `196-S`.
