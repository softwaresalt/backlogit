---
title: "211-S 210-F pre-merge closure"
doc_type: closure
schema_version: "1.0"
source: docs/closure/2026-11-02-211-s-210-f-pre-merge-closure.md
description: "Pre-merge releasability, local review readiness, operator waivers, monitoring, rollback, and follow-ups for shipment 211-S (feature 210-F, workflow-custody/v1)."
chunk_strategy: h1-h2-h3
shipment_id: 211-S
feature_id: 210-F
mode: pre-merge
closure_status: READY_WITH_CONDITIONS
compaction_status: pending
---

## Outcome

Local review readiness is `READY_WITH_FOLLOWUPS`. There are zero unwaived
P0 findings. The explicitly waived P0 and P1 findings are listed under
Operator waivers. One P1 finding (D3) is not covered by an explicit waiver and
is held open until the operator confirms it. The merge is held on that
confirmation.

## Local Review Readiness

* Reviewed HEAD (code): `ebdbb8d9`. Documentation commits after this point do not change code. The PR head is the final documentation commit.
* Outcome: `READY_WITH_FOLLOWUPS`.
* Blocking findings: P0 = 0 unwaived (D1 waived explicitly). P1 = 1 open, pending confirmation (D3). P1 waived explicitly: F14.
* P2 captured as follow-ups, not fixed: F16, F17, F19, F20, F21.
* P3 captured as follow-ups: F6, F8, F13, F15, and the rest of the low-priority hardening.
* Full local build: `go build ./cmd/backlogit` exit 0 at `ebdbb8d9`. Repository-wide `go vet ./...` exit 0. Repository-wide compile exit 0.
* Independent review coverage: the multi-persona, adversarial, correctness, and rubber-duck reviews covered `358e9956` and earlier. Commits after `358e9956` were verified by build, vet, and tests only. This includes the rubber-duck remediation, the external pins and nonce store, the Option B docs change, round-2 items 1, 3, and 4, and the R2 change. The operator closed the review cycle at three cycles.
* Shadow review posture: hosted Copilot review is advisory until the operator elevates it. The P-018 gate is run before merge, as the operator instructed.

## Operator waivers

Each waiver below is recorded in
`docs/memory/2026-11-02/orchestrator-211-s-dark-mode-activation.md`.

* **D1, P0, waived not closed.** When `BACKLOGIT_CUSTODY_OPERATOR_PINS_FILE` (or `--operator-pins-file`) is unset, operator trust anchors come from the workspace `config.yaml`. An agent with workspace write access can register its own operator anchor. The waiver rests on the cooperative local trust model in Plan Section 5.2 #5, and the host key isolation is deferred to GSD OD-2. Plan Section 5.2 #5 also says Profile A protects against agent fabrication. The self-anchor case is the case being waived, not closed. When the pins file is set, pins are enforced strictly with no workspace fallback. Follow-up: `F31ED251`.
* **F14, P1, waived not closed.** Caller-declared `contributor` and `source_kind` can route operator-owned `docs/**` content to Profile B. The waiver rests on Plan Section 5.2 #5 and Plan Section 7 (the GSD `ClassifyWorktreeDeltas` `ForeignPaths` baseline boundary). Option B permits modified Stage-scoped docs under Profile B. Follow-up: `E5CF9048`.
* **D3, P1, open pending confirmation.** Redemption evidence in the NonceLedger projection and the shipment item log is workspace-writable when `BACKLOGIT_CUSTODY_NONCE_STORE_PATH` is unset. A forged redemption line plus an operator-signed proof yields `ADOPTED_OPERATOR_VERIFIED`. The operator's waiver text does not name D3. The orchestrator considers it within the same cooperative-model rationale, and asks the operator to confirm before merge. Follow-up: `7E6A0F58`.

## Expected behaviour, not a finding

* Live Profile A `accept` fails closed with `ErrNonceProviderUnavailable` or `ErrApprovalChannelUnavailable` unless `BACKLOGIT_CUSTODY_NONCE_STORE_PATH` or an injected provider is configured. Plan Section 10 requires this.

## Decisions applied in this release unit

* Profile B is limited to Stage-scoped paths (Decision 4). The git-corroboration rule was withdrawn under Option B.
* The default-branch refusal list is `main`, `master`, `develop`, and `trunk`, plus the trimmed resolved default (Decision 6).
* A journal removal failure returns an error and leaves the journal `compensated` (Decision 7).
* The approval proof is bound to a per-receipt digest. The envelope-level digest is retained for `TestU3` (Decision 8).
* External operator pins and an external nonce store are supported. Each resolves outside the workspace (Decisions 1a and 2).
* Only one custody receipt may be `accepted` at a time (round 2, Option A, R2). The R1 and R3 rules are withdrawn and captured as follow-up `67703B16`.

## Gates at code HEAD `ebdbb8d9`

* `go build ./cmd/backlogit`: exit 0.
* `go vet ./...`: exit 0.
* `go test -run=^$ ./...`: exit 0.
* Race selectors for `TestRound2_`, `TestU1_` through `TestU7_`, and `TestReview` in `internal/core`: `ok`.
* `TestRegistryParity` under race in `internal/cli`: `ok`. Full `internal/cli`: `ok`.
* `internal/mcp`: `ok`.
* `internal/db`: fails only `TestU175_075_LintFix`, `TestU175_076_LintFix`, `TestU175_077_LintFix`, and `TestU175_078_LintFix`. These are the golangci-lint environment failure covered by the lint waiver.

## Blocked prerequisites

* `golangci-lint` v1.64.8 cannot read go1.27 export data. The operator waived lint. CI must run the real lint on the PR head.
* Host operator approval channel and key isolation (GSD OD-2) are not implemented here. Plan Section 10 makes them a bootstrap prerequisite.

## Monitoring

* The CI matrix on the PR head, including the lint job, must pass or be explicitly accepted by the operator.
* After merge, the first `backlogit sync` on `main` must rebuild `custody_receipts` without error. Check its exit status.
* The read-only probe `backlogit custody verify 211-S --branch feat/211-s-workflow-custody-handoff-210-f --json` must return a non-error result on the live workspace.

## Rollback

* Revert the merge commit with `git revert -m 1 <merge-sha>`.
* Custody state is canonical in shipment Markdown and the item log. The `custody_receipts` table is derived, and `backlogit sync` rebuilds it. Reverting therefore does not lose operator data.
* Shipments that already carry a `workflow_custody` envelope keep that key after the revert. Older builds do not interpret it. Confirm this with the operator before reverting in a live workspace.

## Owner and validation window

* Owner: the repository owner, `softwaresalt`.
* Validation window: until the post-merge closure PR is merged and the first `backlogit sync` on `main` has run.

## Follow-ups

* `F31ED251`: host-supervisor operator key channel (D1, high).
* `7E6A0F58`: external redemption authority (D3, high). Requires operator confirmation before merge.
* `E5CF9048`: label-independent Profile B corroboration (F14, high).
* `731141DE`: compensated journal cleanup (F15, medium).
* `4287DCF8`: typed degraded-mode detection, storage-root prefix, and doc drift (F16, F17, F21, medium).
* `67703B16`: restore R1 and R3 envelope rules (medium).
* `E6224C01`: remaining low-priority hardening (F6, F8, F13, F19, F20, adopt rename links).

## Compaction

`compaction_status` is `pending`. Post-merge closure runs `compact-context` under P-020.
