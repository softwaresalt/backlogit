# Stage session — 6FDC4A49 → 171-F → 152-S (checkpoint create schema-reject)

**Date**: 2026-09-06
**Agent**: Stage
**Mode**: NOT dark mode — staging PR to merge-ready, do not merge without fresh operator approval.

## Outcome (handoff to Ship)

- **Shipment**: `152-S` (queued) — handoff token to Ship.
- **Feature**: `171-F` (queued).
- **Tasks**: `171.001-T` (S), `171.002-T` (M), `171.003-T` (S), `171.004-T` (M), `171.005-T` (S) — all high priority.
- **Deps**: 002←001; 003←002; 004←002; 005←003,004.
- **PR**: #428 (base `main`, head `stage/6fdc4a49-checkpoint-create-schema-reject`), HEAD `09756844`.
- **Stash 6FDC4A49**: archived (consumed).

## Artifacts

- Deliberation: `docs/decisions/2026-09-06-checkpoint-create-schema-reject-deliberation.md`
- Plan: `docs/exec-plans/2026-09-06-checkpoint-create-schema-reject-plan.md` (hardened; plan-review PASS, multi-agent-dispatch, attempt 2).

## Pipeline record

- Tool availability: backlogit MCP tools NOT connected → DEGRADED_MODE via CLI fallbacks (registry-declared). Index synced.
- Crash-resumption scan: 0 stage-active, 0 anomalous checkpoints → normal startup.
- Hooks: 212 historical events, no priority signals; acked seq 2704 (runtime, gitignored).
- Triage: single high-priority bug; solo group; synthesized covering feature 171-F.
- Plan review attempt 1 FAIL (2 P1: incomplete Constitution Check; legacy verbatim reintroduced quarantine-poisoning). Revised → attempt 2 PASS (4 blocking personas re-verified empty).

## Design decision (key)

Legacy import is **upgrade-or-reject, never verbatim**: coerce legacy shape (absent or integer-literal `0` schema_version) to V1, populate defaults, run full V1 validation, write only if valid; every successful import is readable without quarantine. Raw-token schema_version classification (reject wrong-typed/null/non-integral/overflow/duplicate). Variadic functional options keep the repo green at each unit boundary. Surface-neutral sentinel in internal/errors with two-%w wrap; mandatory bounded MCP error mapping.

## Git hygiene

- Branched off `origin/main` (excluded PR #427's commit). Preserved and never staged: `start.ps1`, `.autoharness/gates/`, `.backlogit/checkpoints/checkpoint-20260905-031054.json`, 5 prior `docs/memory/*.md`.
- stash.jsonl staged diff = exactly the 6FDC4A49 removal (no line-ending churn).
- Included quarantine audit evidence (`checkpoint-20260906-231751.json` + disposition) — checkpoint state is git-tracked per policy.
- This memory file intentionally left untracked (repo convention).

## Next steps

- Await CI/Copilot gate completion on #428, then fresh operator approval before merge.
- Ship claims `152-S` after merge.
