---
title: "Ship 211-S Session Checkpoint: Awaiting Decision on Frozen Fixtures for Decision 4 Corroboration"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-02/ship-211-s-decision4b-fixture-halt.md
description: "Halt record for Shipment 211-S after Decisions 1-8, the rubber-duck logic remediation, and the external pins and nonce store wiring. The only failing test is the Decision 4 git-corroboration case, whose fixtures are frozen harnesses. Operator authority is needed before the PR."
shipment_id: 211-S
feature_id: 210-F
branch: feat/211-s-workflow-custody-handoff-210-f
current_head: ee99b441
status: halted
---

## Status

* All seven member tasks and covering feature `210-F` are `done` and archived. Shipment `211-S` remains `active`.
* HEAD `ee99b441`: `go build ./cmd/backlogit` exit 0. `go vet` exit 0 on core, db, mcp, and cli. Repo-wide compile exit 0.
* One test fails on the branch: `TestReview_ProfileBDocsCorroboratedAgainstBase/modified_docs_file_existing_at_base_requires_profile_a`. Everything else in the custody, review, rubber-duck, and external-config groups passes. `TestRegistryParity` passes under `-race`.
* No PR is open. No merge has occurred. Closure docs are not written.

## Completed Since Last Checkpoint

* Rubber-duck logic review (substituted by the `Correctness Reviewer` agent, because no `rubber-duck` subagent type exists in this environment). It found 17 items. Fixed with RED-then-GREEN: F1 revision monotonicity (P1), F2 adopt ID change, F3 byte-exact rollback, F4 and F5 shared error-code mapping and `trust_anchor_revoked` ordering, F6 workspace trust filter boundary, F7 savepoint in rehydration, F8 `~` path rule, F9 preview duplicate and order-independent digest, F11 accept-owned fields, F12 validation on read, F14 registry `--branch`, F16 deterministic rename error.
* Not changed, with reason: F10 (unreachable, snapshot runs under the global lock), F13 (kept `custodyAdoptedFunc` and `CustodyRequestIdentityDigest` because frozen tests use them; removed the unused callback and `ActiveReceiptDigest`), F15 (compensated journal cleanup, a recovery policy choice, follow-up), F17 (substring sites are outside scope, follow-up).
* Decision 1a: external operator pins file via `BACKLOGIT_CUSTODY_OPERATOR_PINS_FILE` or `--operator-pins-file`. Must resolve outside the workspace. Zero fallback to workspace anchors when set.
* Decision 1b (approved waiver): when the pins file is unset, the cooperative local trust model applies. Recorded below.
* Decision 2: external file-backed nonce store via `BACKLOGIT_CUSTODY_NONCE_STORE_PATH` or `--nonce-store-path`, outside the workspace. Redemption is also checked against it. Live Profile A accept fails closed when unconfigured (expected).
* Decision 5: unknown-origin paths route to Profile A.
* Decision 6: refusal list `main`, `master`, `develop`, `trunk`, plus trimmed resolved default.
* Decision 7: journal removal failure returns an error.
* Decision 8: per-receipt identity digest. The envelope-level digest equals the latest-revision receipt, so single-receipt proofs stay valid.
* Decision 4, part 1: Profile B limited to `<storage_root>/queue/**`, `<storage_root>/archive/**`, `docs/decisions/**`, `docs/plans/**`, `docs/exec-plans/**`, `docs/reviews/**`, `docs/memory/**`.

## Waiver Recorded (1b, Approved)

When no external pins file is configured, Profile A authority equals the cooperative local workspace trust model, per Plan Section 5.2 #5 (line 250 of the plan). Operator-approved. The waiver covers hostile same-user processes.

Scope note for the PR body: the plan at Section 5.2 #5 also says Profile A protects against agents "fabricating" approvals. An agent that edits `config.yaml` to add its own operator anchor is that case. The waiver as written does not clearly cover it, so the PR must state that the reviewers' D1 finding is operator-waived, not closed. Confirm this reading or narrow the waiver.

## Decision Required: Decision 4 Git Corroboration

The approved rule (Decision 4, 4b): a `docs/**` path in a Profile B receipt that already existed at `base_commit_sha` and was modified requires Profile A, unless already covered by an earlier valid Profile A receipt in the same workflow.

Frozen harnesses conflict with this rule. They modify committed `docs/plans` files and expect Profile B to succeed:

* `internal/core/custody_exec_test.go` lines 57, 111, 236
* `internal/core/custody_review_fix_test.go` lines 657, 729, and others in `TestReview_*`
* `internal/mcp/custody_handlers_test.go` lines 364, 393, 508
* `internal/cli/custody_parity_test.go` lines 475, 614

Implementing 4b requires editing those frozen harnesses. The operator pre-authorization covers only newly scaffolded Wave 5 and 6 fixtures, and Option B covered only specific Wave 4 changes. Neither covers this.

Options:

* **A (recommended):** approve a recorded contract amendment (a commit like `aefdf5d4`) that changes those modified-existing-doc fixtures to create the document as new, or to use a new `docs/plans/<new>.md`. The rule and the assertions stay as they are. Only the fixture setup changes.
* **B:** a written waiver that 4b is not enforced for this shipment. The failing case is then removed or marked skipped, with the reason recorded. Stage can then edit existing `docs/plans` files under Profile B.

Until you choose, HEAD cannot pass its own gate. The PR cannot be opened.

## Remaining Steps After Decision

1. Apply the chosen option, RED-first.
2. Run the final local review on the final HEAD. This is the multi-persona code review, the adversarial review, and the rubber-duck review. It is required before the PR, per the operator's standing instruction. The P-014 readiness record must be for that HEAD.
3. Write closure docs `docs/closure/2026-11-02-211-s-210-f-runtime-verification.md` and `docs/closure/2026-11-02-211-s-210-f-pre-merge-closure.md`. Validate with `backlogit_docs_lint` (0 violations).
4. Capture P-021 C2 stash entries for the deferred items (F15, F17, F10 reachability note, F13 `NonceLedger.Prune`, F6 not-yet-valid anchors, F8 `~1` acceptance, host-isolation and key-separation follow-up, storage-root prefix F16, docs drift F21).
5. Push, open the PR with the `## Local Review Readiness` block (`READY_WITH_FOLLOWUPS`, waivers listed explicitly), wait for CI and `autoharness gate copilot-review`, resolve threads, merge with `--merge --match-head-commit`, sync main, post-merge closure.

## Resume Hint

Operator chooses option A or B for the Decision 4 fixtures. Then continue from Remaining Steps. Do not re-claim 211-S.
