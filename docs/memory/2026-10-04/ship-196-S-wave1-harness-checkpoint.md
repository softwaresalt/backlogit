# Ship 196-S Wave 1 Harness Checkpoint

## Session state

- Shipment: `196-S`, P-017 dark-mode scope; no other shipment is in scope.
- Branch: `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`
- Current HEAD: `b34ae1c9`.
- HEAD before wave-1 harness commit: `42fd2167`.
- Harness scaffold commit: `413dff42` (`test(harness): scaffold 196-S wave 1 red contracts`).
- Traceability commit: `b34ae1c9` (`chore(harness): link 196-S wave-1 harness evidence`).
- Served workspace root: `C:\Source\GitHub\backlogit`.
- Served storage root: `C:\Source\GitHub\backlogit\.backlogit`.
- R14 use-time attestation passed at wave admission: MCP metadata catalog roots matched the supplied roots; `pragma_database_list` returned exactly one `main` row pointing to `.backlogit\backlogit.db`. Shipment `196-S` was the sole active shipment, and two live shipment reads agreed on the explicit manifest.
- Wave schedule: `M` contains nine tasks; `196-F` is the only non-task manifest member. Wave 1 is `196.001-T`, `196.003-T`, `196.004-T`, `196.007-T`, `196.008-T`.
- Fresh exact-`M` status/dependency snapshot: all nine tasks are `active`; no blocked or unsupported items; wave-1 tasks have no unfinished dependencies.
- Step 4.0 claim-state census: 9 active, all 9 claim-assigned, 0 active residuals, 0 indeterminate. Every task marker was `196-S`, every task was explicitly listed, and each safely opened log contained a shipment-claim event but no current-epoch `WORK_STARTED: 196-S` comment.
- Scoped P-012 declaration recorded before raw reads: read-only no-follow access to `.backlogit/logs/{196.001-T,196.002-T,196.003-T,196.004-T,196.005-T,196.006-T,196.007-T,196.008-T,196.009-T}.jsonl` for Step 4.0 classification. No log was changed.

## Wave-1 harness admission

The Amendment 2 closed exempt set was checked against the task contracts. `196.004-T` and `196.007-T` are both `verification-only`, have complete ordered exemption contracts, are enumerated in the closed set, add only characterization tests, and have exact read-only commands that pass the P-002.5 screen. Their P-002.3 pre-work probes have not yet been run; run each exactly once at its Step 4.1a claim-time gate after the harness commit.

The harness-architect skill scaffolded only the three non-exempt wave-1 tasks:

- `196.001-T`: `tests/integration/orchestrator_served_root_handoff_contract_test.go`; `TestUSR1_OrchestratorServedRootHandoffContract`; selector `go test -count=1 -v -run '^TestUSR1_' ./tests/integration`; green-maker `196.002-T`, closing wave 2.
- `196.003-T`: `tests/integration/shipment_reconcile_feature_member_contract_test.go`; `TestUSR3_ShipmentReconcileExplicitFeatureMemberContract`; selector `go test -count=1 -v -run '^TestUSR3_' ./tests/integration`; green-maker `196.005-T`, closing wave 2.
- `196.008-T`: `tests/integration/ship_served_root_attestation_contract_test.go`; `TestUSR8_ShipServedRootAttestationContract`; selector `go test -count=1 -v -run '^TestUSR8_' ./tests/integration`; green-maker `196.009-T`, closing wave 3.

`go test -run=^$ -count=1 ./...` passed, so all harnesses compile. The three task-scoped selectors each exited 1 on the intended assertion failures, not build errors or vacuous passes:

- U1: `ProcedureLiterals` and `CallSites` failed; `CrossReferenceInvariant` passed.
- U3: expected explicit-feature literal, proceed/Ship Step 6, and superseded-wording checks failed; preserved-invariant assertions passed.
- U8: `DefinitionLiterals` and `CallSites` failed; `PreservedInvariants` passed.

All three tasks now carry `harness-ready`; their statuses remain `active`. The two valid exempt tasks were not scaffolded. `gofmt -l` on the three new harness files returned no paths; `git diff --check` passed. No production code was changed and no task build has been dispatched.

## Pending work

1. The harness files, labels, hooks, and checkpoint are committed; the three task records are associated with `413dff42`. Explicit P-004 harness manifest comments record `Compilation: PASS`, `Red Phase: CONFIRMED`, exact selectors, named results, and the scaffold commit SHA.
2. Start the wave-1 tasks sequentially. Before each task's first Step 4.1b log re-read, repeat the R14 `pragma_database_list` check. For U4/U7, run the exact screened pre-work probe once, require a non-zero precondition result, record the output, capture a clean-tree baseline, and halt on any real characterization defect without changing production Go.
3. Red-deliverable U1/U3/U8 must be dispatched through build-feature's red-deliverable branch with a post-scaffold/pre-task baseline SHA and must remain assertion-red with an empty delta.
4. Converge wave 1 before admitting wave 2. Continue all remaining work only within shipment `196-S` on this branch.
