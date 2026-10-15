---
title: "Stage 199-S P-002 red-deliverable and harness-exempt contract repair"
description: "Stage added canonical P-002.6 red-deliverable-contract blocks to 199.001-T, 199.003-T, 199.005-T, and 199.007-T, P-002.1 covered-by harness-exemption-contract blocks to 199.002-T, 199.004-T, 199.006-T, 199.008-T, and 199.009-T, and the closed harness-exempt-set block on 199-F to resolve WAVE_RED_MAPPING_UNRESOLVED."
doc_type: guide
source: docs/memory/2026-10-15/stage-199s-red-contract-amendment.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
timestamp: "2026-10-15T07:03:00Z"
---

# Stage 199-S P-002 red-deliverable and harness-exempt contract repair

## Session status and scope

- **Mode**: `DARK_MODE_ACTIVE` (P-017), activated `2026-10-15T06:25Z`, scope `[199-S]` only (Wave CY-A: `199-F` and `199.001-T`..`199.009-T`).
- **Route**: `gemini-4-argon` / `google` / `high` / `long_context` (resolved from `.autoharness/config.yaml`).
- **Trigger**: Ship halted `199-S` before claim at `P-002.2 WAVE_RED_MAPPING_UNRESOLVED` (`checkpoint-20261015-062809.json`, owned by Ship and left untouched) because the four test-first red deliverables (`199.001-T`, `199.003-T`, `199.005-T`, `199.007-T`) lacked canonical `red-deliverable-contract` blocks and the five code units (`199.002-T`, `199.004-T`, `199.006-T`, `199.008-T`, `199.009-T`) lacked `harness-exempt` / `harness-ready` classifications.
- **Scope invariance**: No task scope, acceptance criteria meaning, dependency edges, task statuses, or shipment membership (`S` = `199-F` + `199.001-T`..`199.009-T`, `M` = 9 tasks) changed.

## Validated wave schedule and red-to-green mapping (P-002.6)

The topological wave partition over `M` (9 task members, verified against `item_deps` and `docs/exec-plans/2026-10-08-cy-concurrency-lifecycle-hardening-plan.md` `CY-A: A1->A2->A4; A3->A4; A5->A6; A7->A8->A9`) is:

- **Wave 1**: `199.001-T` (A1), `199.003-T` (A3), `199.005-T` (A5), `199.007-T` (A7)
- **Wave 2**: `199.002-T` (A2), `199.006-T` (A6), `199.008-T` (A8)
- **Wave 3**: `199.004-T` (A4), `199.009-T` (A9)

| Red deliverable | Unit | Selector (`red_selector_command`) | Green-maker(s) | Closing wave |
|---|---|---|---|---|
| `199.001-T` | A1 | `go test -count=1 -v -run '^TestUCYA1_' ./internal/mcp` | `199.002-T` (W2) | `2` |
| `199.003-T` | A3 | `go test -count=1 -v -run '^TestUCYA3_' ./internal/core` | `199.004-T` (W3) | `3` |
| `199.005-T` | A5 | `go test -count=1 -v -run '^TestUCYA5_' ./internal/events` | `199.006-T` (W2) | `2` |
| `199.007-T` | A7 | `go test -count=1 -v -run '^TestUCYA7_' ./internal/core` | `199.008-T` (W2), `199.009-T` (W3) | `3` |

All four Wave 1 tasks remain harness-required (no `harness-exempt` label, and no pre-applied `harness-ready` label per P-004; `harness-architect` scaffolds their RED harnesses and applies `harness-ready` at Wave 1 admission).

## Harness-exempt classification and closed exempt set (P-002.1 / P-002.3 / P-002.4)

`199-F` now carries the governing `<!-- BEGIN:harness-exempt-set -->` block enumerating the closed exempt set `{199.002-T, 199.004-T, 199.006-T, 199.008-T, 199.009-T}`. Each member carries the `harness-exempt` label and a canonical 6-key `harness-exemption-contract` (`covered-by`) block:

| Task | Unit | Wave | Owner | `harness_owner_command` | Allowed delta surface (P-002.4) |
|---|---|---|---|---|---|
| `199.002-T` | A2 | 2 | `199.001-T` (direct) | `go test -count=1 -v -run '^TestUCYA1_' ./internal/mcp` | `internal/mcp/errors.go`, `internal/core/shipment.go` |
| `199.004-T` | A4 | 3 | `199.003-T` (direct) | `go test -count=1 -v -run '^TestUCYA3_' ./internal/core` | `internal/core/shipment.go` |
| `199.006-T` | A6 | 2 | `199.005-T` (direct) | `go test -count=1 -v -run '^TestUCYA5_' ./internal/events` | `internal/events/stream.go` |
| `199.008-T` | A8 | 2 | `199.007-T` (direct) | `go test -count=1 -v -run '^TestUCYA7_CanonicalBlockedEnvelopeKeys$' ./internal/core` | `internal/core/shipment_blocked_envelope.go`, `internal/core/artifacts.go` |
| `199.009-T` | A9 | 3 | `199.007-T` (transitive via `199.008-T`) | `go test -count=1 -v -run '^TestUCYA7_' ./internal/core` | `internal/core/artifacts.go` |

### Key technical notes recorded on the items

1. **U20C3 / U20C5 `%w` wrapping (`199.001-T`, `199.002-T`)**: In `internal/core/shipment.go`, `finishShipmentLifecycleCompensation` (`compensationStepError`, U20C3) replaces `step error: %v` with `step error: %w` while keeping `cause: %v` so `TestU20C3_*` in `internal/core/shipment_compensation_order_u20c3_harness_test.go` continues to isolate the pre-compensation cause sentinel; `BlockShipment` and `UnblockShipment` commit-journal errors (U20C5) replace `persist block/unblock shipment commit: %v: %w` with `%w: %w`.
2. **Cross-wave split for `199.007-T` (`199.008-T` in Wave 2, `199.009-T` in Wave 3)**: `199.007-T` scaffolds two top-level functions in `internal/core/shipment_blocked_envelope_keys_test.go` — `TestUCYA7_CanonicalBlockedEnvelopeKeys` (turned green by `199.008-T` in Wave 2) and `TestUCYA7_RefuseBlockedShipmentArtifactTypeFlip` (turned green by `199.009-T` in Wave 3). `199.008-T`'s `harness_owner_command` is scoped to `^TestUCYA7_CanonicalBlockedEnvelopeKeys$` so its Wave 2 loop and Step 4.3 gate pass while `199.007-T`'s `^TestUCYA7_` selector remains RED at Wave 2 convergence (Step 4.6 item 4) and turns completely GREEN at Wave 3 convergence (Step 4.6 item 5).
3. **Wave 2 `./internal/core` open reds**: Because `TestUCYA3_` and `TestUCYA7_RefuseBlockedShipmentArtifactTypeFlip` in `./internal/core` stay legitimately RED until Wave 3, the Wave 2 `exempt_verification_command` scripts for `199.002-T`, `199.006-T`, and `199.008-T` scope `./internal/core` checks to their own selectors (`^TestU20C(3|5)_` and `^TestUCYA7_CanonicalBlockedEnvelopeKeys$`), leaving the unfiltered `./internal/core/...` and `./...` runs to Wave 3 convergence.

## Validation and review

- **Adversarial persona review** (`multi-agent-dispatch`):
  - `Constitution Reviewer`: `APPROVE_WITH_NOTES` (0 P0, 0 P1, 0 P2, 1 P3 advisory confirming that `199.007-T` and `199.008-T` constrain A8 to its declared two-file scope `internal/core/shipment_blocked_envelope.go` and `internal/core/artifacts.go`).
  - `Scope Boundary Auditor`: `APPROVE` (0 findings).
  - `Correctness Reviewer`: `APPROVE` (0 findings).
- **Wave scheduler simulation**: `pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue` passed (`WAVE_SIM_OK: 200/200 assertions PASS across 26 scenario(s)`).
- **199-S contract and intake audit**: `CONTRACT_CHECK_199_S_OK: red=4 exempt=5 total=9`; `199-S` manifest (`199-F` + `199.001-T`..`199.009-T`) verified `record-consistent` (`PROCEED`); `backlogit_doctor` reports 0 findings on `199-S` / `199-F` / `199.*-T`; `backlogit_sync_index` indexed `2033` artifacts; concrete hook events acknowledged through `seq: 3760`.

## Role boundary and P-010 record

- Stage created branch `chore/stage-199-s-red-contracts` from `main` (`f3b3ca45`) and committed the Stage-owned backlog work-item files, this session memory file, and `.backlogit/hooks_queue.jsonl` locally by explicit path, leaving `.autoharness/config.yaml`, `.backlogit/stash.jsonl`, and `.backlogit/checkpoints/*.json` untouched.
- **P-010 boundary enforcement**: Per `.github/agents/_stage.agent.md` (`## Role Boundary (NON-NEGOTIABLE)`, `PR` row: `Forbidden: Create, push, or merge pull requests`), `.github/policies/workflow-policies.md` § P-010, and `.github/instructions/role-enforcement.instructions.md`, Stage is forbidden from pushing branches or creating/merging pull requests:
  `P-010 VIOLATION: _Stage attempted forbidden operation [Create, push, or merge pull requests] in category [PR].` (Refused before execution; no `git push`, `gh pr create`, or `gh pr merge` was executed by Stage.)
- **Handoff**: Redirected to the **Orchestrator** (Step 1.5 Staging Artifact Merge Gate / Continuity Carry-Forward Carve-Out) to push `chore/stage-199-s-red-contracts`, open the staging PR to `main`, run the Copilot review and CI gates, merge with `--merge`, and fast-forward local `main` before re-invoking Ship for `199-S`.
