---
title: "Ship 211-S Session Checkpoint: Awaiting Decision on Envelope Revision Coherence Rules"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-02/ship-211-s-item2-coherence-halt.md
description: "Halt record for Shipment 211-S after Option B and round-2 rubber-duck fixes. Items 0, 1, 3, and 4 are committed. Item 2 (envelope revision coherence) conflicts with frozen fixtures. HEAD fails three RED subtests until the operator chooses an option."
shipment_id: 211-S
feature_id: 210-F
branch: feat/211-s-workflow-custody-handoff-210-f
current_head: 23e8d6ff
status: halted
---

## Status

* All seven member tasks and covering feature `210-F` are `done` and archived. Shipment `211-S` remains `active`.
* Option B (Decision 4b) committed as a test update (`c4abf402`). The Profile B docs-corroboration rule is withdrawn. No production code implemented it.
* Round-2 rubber-duck fixes committed:
  * Item 1, preview `resolved_entries` (`48ac19df`).
  * Item 3, verify sets `ActiveReceiptID` (`24f0e8ce`).
  * Item 4, approval-token claim binding to the selected receipt (`23e8d6ff`). Mismatch returns `ErrAuthorizationRejected`.
* Item 2, envelope revision coherence: NOT committed. Blocked (see below).
* HEAD fails three subtests of `TestRound2_ValidateCustodyEnvelopeAcceptedCoherence` (`98f1168d`): `accepted_receipt_above_custody_revision_is_refused`, `multiple_accepted_receipts_are_refused`, `active_accepted_receipt_below_custody_revision_is_refused`. The positive control passes.
* `go build ./cmd/backlogit`, `go vet ./...`, and repo-wide compile pass. Race selectors pass except the three Item 2 subtests.
* No PR is open. No merge has occurred. Closure docs are not written.

## Item 2: Conflict With Frozen Fixtures

Experiment (no commits; working tree restored to HEAD):

| Rule | Meaning | Frozen and existing tests |
|---|---|---|
| R1 | `accepted` receipt must have `Revision <= CustodyRevision` | Conflicts: `custody_exec_test.go:219` (TestU4 profile_a), `custody_review_fix_test.go:405` and `:493` |
| R2 | At most one `accepted` receipt | Compatible with every frozen test |
| R3 | `accepted` receipt must have `Revision == CustodyRevision` | Conflicts: same three lines |

The conflicting fixtures set an accepted receipt at revision 1 under custody revision 0 (`rcpt-adopt-1`).

## Why the Write Path Is Still Protected

The accept path already refuses any receipt whose revision is not `CustodyRevision + 1` (F1 remediation, `a67da9f5`). A stale offer therefore cannot regress the revision. R1 and R3 are defense-in-depth on envelope validation. R2 is the rule the rubber-duck needed, because it catches two accepted receipts.

## Options

* **A (recommended):** commit R2 only. Withdraw the R1 and R3 subtests of `TestRound2_ValidateCustodyEnvelopeAcceptedCoherence` in our own RED test, with a recorded commit explaining the conflict. The write path (F1) enforces the revision invariant. Record R1 and R3 as a follow-up. Needs your approval, because it reduces our own RED test's coverage.
* **B:** a recorded contract amendment revising the frozen fixtures (`custody_exec_test.go:219`, `custody_review_fix_test.go:405` and `:493`) so accepted receipts sit at `CustodyRevision`. Same authority as the earlier 4b decision.
* **C:** withdraw Item 2 entirely, including R2. Not recommended.

## Remaining Steps After Decision

1. Apply the chosen option, RED-first where a test changes.
2. Run the final local review on the final HEAD. The rubber-duck review was on `358e9956`, so the Option B and round-2 changes are not yet reviewed. The P-014 readiness record must be for the final HEAD.
3. Closure docs and PR, per the operator's directive.

## Readiness Statement For The PR (Draft, Pending Final HEAD)

* D1 (workspace `config.yaml` trust fallback): waived under Plan Section 5.2 #5 when `BACKLOGIT_CUSTODY_OPERATOR_PINS_FILE` is unset, deferred to GSD `OD-2`. With the pins file set, pins are enforced strictly with zero workspace fallback.
* F14 (caller-declared `contributor` and `source_kind` route Profile B): NOT closed by Option B. A caller can label operator-owned `docs/**` content as Stage-owned and obtain Profile B. An explicit operator waiver is required before P1 = 0 can be recorded.
* Live Profile A accept fails closed without a configured nonce store or injected provider (expected, Plan Section 10).
* `golangci-lint` is incompatible with the go1.27 toolchain. Waived. `TestU175_*_LintFix` is environmental.

## Resume Hint

Operator chooses A, B, or C for Item 2, and confirms the F14 waiver. Then final review on the final HEAD, closure docs, PR. Do not open the PR while HEAD has failing tests.
