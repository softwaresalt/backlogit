# Ship 196-S U8 Amendment 3 Completion Checkpoint

## Session state

- Shipment `196-S`, P-017 dark mode, scope `[196-S]` only. Branch
  `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`.
- Plan: `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md`, Amendment 3 A3.5
  Phase 1 (U8 in-task P-021 C3 remediation, not gated).
- Commits this session so far:
  - `2911d017`: carry the Orchestrator note.
  - `84b70a9f`: `test(harness)`, the A3.2.3 append. 18 lines added, 0 deleted.
  - `f18fcb11`: `chore(backlog)` traceability, restoring `harness-ready` and adding the note
    and manifest.
- Checkpoint `checkpoint-20261005-052350.json` was restored (operator Decision 1) and resolved
  after the Phase 0 preconditions passed.

## 196.008-T outcome

- Status `done` on the red path. The deliverable is the persistent RED test
  `TestUSR8_ShipServedRootAttestationContract`.
- `red_baseline_sha` = `f18fcb11` (A3.3 item 4).
- build-feature Step 0.5 results:
  - Compile passed.
  - `go test -count=1 -v -run '^TestUSR8_' ./tests/integration` exited 1: `DefinitionLiterals`
    FAIL, `CallSites` FAIL, `PreservedInvariants` PASS. All five A3.2.3 red strings appear
    exactly once, and no anchor-guard message appears.
  - The zero-delta set is empty.
- Step 4.3 gates:
  - `go vet ./tests/integration` passed.
  - The staged-blob gofmt check is clean, and gofmt is clean on an LF export of HEAD.
  - golangci-lint v1.64.8 (`GOTOOLCHAIN=local`, `--concurrency=1`) found no new issues vs
    `2741626a`.
- Step 4.4 review: three-persona adversarial review, report-only, at HEAD `f18fcb11`.
  - Counts: P0 0, P1 0, P2 2, P3 0. Outcome `READY_WITH_FOLLOWUPS`.
  - F1 is deferred to `1EF5BDC6` (sync the U9 Change 2 plan body with the A3.2.3 Green
    wording).
  - F2 is deferred to `33C4B816` (make the 196.009-T description cite A3.2.3 verbatim).
  - Both are out of scope under P-021 C1 and were captured on the C2 threadless path.
    Discovery found zero matching active or archived entries.
- Open-red entry added: selector `^TestUSR8_`, green-maker `196.009-T`, declared closing wave 3.
  The effective closing wave is subject to the G-A3 ruling.

## A3.6 stash citations (status-neutral comments)

- `CDBCB258` cited on 196.009-T (R03, R10) and on 196.002-T (R10).
- `F6F3AA0E` cited on 196.004-T (R05).
- `147BD825` (R09) and `3B25D37F` (R12) cited on 196.007-T.
- No comment on 196.008-T is a `WORK_STARTED` record.

## Next step

Run the wave-1 Step 4.6 convergence gate with open-red set {001, 003, 008}, then Gate G-A3.
Gate G-A3 applies: no ruling file `docs/memory/<date>/operator-196-S-a3-ruling.md` exists, so
Ship halts before wave 2. Do not run Step 4.0 for wave 2. Under ruling A, the operator runs
A3.4 Option A.
