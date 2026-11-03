---
title: "Ship 211-S Session Checkpoint: Review Remediation Halt Awaiting Operator Decisions"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-02/ship-211-s-review-remediation-halt.md
description: "Halt record for Shipment 211-S after review remediation. Readiness is BLOCKED at 17b6de73 by an operator-trust design question (D1, P0) and related P1s. Operator decisions are required before PR or merge."
shipment_id: 211-S
feature_id: 210-F
branch: feat/211-s-workflow-custody-handoff-210-f
reviewed_head: 17b6de73
current_head: 6ad5456e
status: halted
---

## Status

* All seven member tasks and covering feature `210-F` are `done` and archived. Shipment `211-S` remains `active`.
* HEAD `6ad5456e` passes `go build ./cmd/backlogit`, `go vet` on the core, db, mcp, and cli packages, and repo-wide compile.
* No PR is open. No merge has occurred. Closure docs have not been written.
* Readiness at `17b6de73` (re-review, two independent reviews): BLOCKED. Re-review at `6ad5456e` has not been run. It would be wasted until the decisions below are made.

## Remediation Completed (Verified in Code)

* D3 redemption record: `consumed_at` recorded in the NonceLedger. The accept path records the redemption and the item-log acceptance event. Verify requires a matching redemption. A missing or unreadable ledger and log fail closed. Redemption `consumed_at` must lie inside the proof's `[not_before, not_after]` window (F2).
* Receipt expiry enforced on offer, accept, and verdict. Expired prior receipts are not carried into adoption (F13).
* Profile B only for `stage_to_ship_handoff`. Every other kind requires Profile A (F12).
* Preexisting-adoption actions require Profile A. The restored subtest `preexisting_adoption_action_requires_profile_a` passes (`5a0be9c2`).
* Nonce consumption moved after all validation checks. Post-consume failure reports that the token was consumed.
* Approval binds `receipt_id` and `accepting_role`. Digest checked on verify.
* Rollback terminal phase `compensated` on removal failure. Recovery skips non-intent journals.
* Windows path aliases rejected lexically: trailing `.` or space, `:`, `~`, `.git` case-insensitive, reserved device names. Every ancestor component is checked. The remaining TOCTOU window is documented, not closed.
* Idempotency digest compared. Conflicting reuse fails closed.
* Bounded reads: 64 MiB for worktree and git output. Locks released by `defer`.
* Adopt reads canonical Markdown and fails closed when it is missing. No SQLite fallback (F18).
* `CreateArtifact` refuses `workflow_custody` in `WithFields`. Custody writes preserve DB-only links.
* Default-branch refusal covers literal `main`, `master`, `develop`, `trunk`.
* Registry `accept_custody` includes `--auth-token` and `--idempotency-key`.
* Ledger and log reads refuse symlinks (F6). Item-log appends in `internal/events` are out of scope and unchanged.

## Blocked by Operator Decision

### Decision 1, P0 blocking: trust root for operator verdicts (D1)

Reviewers verified that the shipped trust root is `config.yaml` in the workspace, through `custodyWorkspaceOperatorAnchors`. The CLI and MCP paths never set pins. A workspace writer can add its own operator anchor, sign a proof, write a redemption line to the ledger or item log, and obtain `ADOPTED_OPERATOR_VERIFIED`, with no operator involvement. This is not an accepted residual. It defeats Profile A's purpose.

Options:

* **1a (recommended):** operator trust anchors and pins come from outside agent-writable paths. For example, a host-supervisor-managed file or an env-supplied fingerprint. Wire `ExternalPins` into the CLI and MCP custody paths. `operator_verified` never falls back to workspace anchors.
* **1b:** written operator waiver that accepts "Profile A authority equals workspace write access" as the documented model. Requires your explicit rationale. The P0 would then be recorded as accepted risk.

### Decision 2, P1 blocking: redemption authority (D3, F2)

The redemption evidence (`nonce-ledger.json`, `logs/<shipment_id>.jsonl`) is workspace-writable. Option 1a, with a redemption store outside the workspace, closes it. The window check added in F2 limits replay only.

### Decision 3, P1 blocking: production nonce provider (F3)

No production `NonceConsumeProvider` exists. The U6/U7 instruction to add a file-backed or in-process provider was not implemented. The tests inject an in-memory provider through context. Consequence: live Profile A accept fails closed in every shipped surface, so the operator-verified path cannot be used. A file-backed provider inside the workspace is not rollback-resistant and is workspace-writable, so it would not resolve Decision 2. Recommended: an external provider aligned with Decision 1a.

### Decision 4, P1 blocking: caller-declared labels drive Profile A (F14)

`contributor`, `source_kind`, and `staged_state` come from the caller. An agent can label operator-authored `docs/` content as `contributor: stage` and receive Profile B. Options: (a) require operator co-sign for operator-authored content, or (b) corroborate labels against Git evidence. Option (b) needs a trusted path-ownership source, which hits Decision 1 again.

### Decision 5: `ForeignPaths` source (D2 item, F13)

The `ForeignPaths` trigger is not implemented. Its baseline lives in `internal/gates/worktree.go`, which is out of scope. Current behaviour routes unknown-origin paths to Profile A, which is safe. Approve adding `internal/gates` to scope, or accept the current default.

### Decision 6: fixture conflict for default branch (F17 item 2)

`TestReview_ProfileEligibility` (8 subtests) and `TestReview_DefaultBranchOriginPrefix` fail because their fixtures lack `origin/HEAD`. The fail-closed rule for an unresolved default is therefore not applied. Options: (a) approve adding `origin/HEAD` to those fixtures, or (b) keep the current behaviour (literal refusal list only). Note these are our own remediation RED tests, not the frozen U1–U4 harness.

### Decision 7: contradictory assertion (F7)

`TestReview_CompensatedJournalIsTerminal` asserts that recovery returns no error when removing an intent journal fails. F7 requires recovery to fail loudly in that case. Approve changing that assertion to expect the error.

### Decision 8: prior-adoption digest coupling (C)

The request digest covers all receipts in the envelope. A later offer therefore invalidates every earlier operator verdict (`UNVERIFIED_OPERATOR_PROVENANCE`). Options: bind per-receipt digests, or accept the coupling and document it.

## Non-Blocking Follow-Ups (Recorded, Not Fixed in This Pass)

* F15: `.backlogit/**` Profile B scope includes `config.yaml`, the ledger, and item logs. Decide after Decision 1.
* F16: hardcoded `.backlogit/` prefix. `AGENTS.md` describes the default storage root as `.backlog/`.
* F19: DB-only link merge on custody writes can persist stale rows as canonical links.
* F20: Profile B `by` is an unauthenticated string. Document as a limitation.
* F21: `backlogit_custody_accept.md` describes a configured nonce provider that does not ship. `backlogit_custody_verify.md` does not disclose the workspace-anchor fallback. Docs are out of scope for this pass.
* Lint: `golangci-lint` is incompatible with the go1.27 toolchain (waived). `TestU175_060_LintFix` in `internal/cli` failed once under parallel load and passed on rerun.

## Merge Gate

`merge_approval_pre_authorized` requires a §1.9 PASS for the current HEAD and a P-018 PASS. Neither holds. Readiness is BLOCKED. Do not open a PR or merge until the operator resolves Decisions 1 to 4 and a re-review returns READY or READY_WITH_FOLLOWUPS with P0 = 0 and P1 = 0.

## Resume Hint

Operator answers Decisions 1 to 8. Then Ship dispatches the approved fixes (RED first), re-runs both reviews on the new HEAD, and only then writes the closure docs and opens the PR. Do not re-claim 211-S.
