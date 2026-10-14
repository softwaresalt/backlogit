---
title: "Compacted memory: 2026-09-24 155-S corrective tasks, full-suite timeouts, and 073-DL test-budget decision"
date_range: "2026-09-24 to 2026-09-24"
doc_type: compacted-memory
source_count: 13
---

# Compacted memory: 2026-09-24

Compacts 13 source files covering shipment 155-S (feature 174-F): corrective tasks, repeated governed full-suite runs, Stage decisions on the test-budget policy (073-DL, 11BE840F, 62C6A469), and halts. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

- Corrective tasks ran through ordinary lifecycles. 174.069-T isolated ordinary ShipShipment fixtures from executable gate brokers (test-only). 174.070-T made durability helper return discards explicit. Both were archived (commits 14084d9b, 46145ed5, 1895ef9c, 53a2bd2f).
- Wave 17 correctives: 174.071-T added a test-only no-recovery workspace seam (commits cfe37339, 8a347ba3). 174.072-T set an explicit Ship governed tool allowlist with manifest parity (commits 94ddde9d, 9868af4a). Upstream tune-preservation follow-up stash: EC43AB70.
- Each authorized go test ./... execution was single-shot. No failed run was retried without new authorization.
- Stage 073-DL (rev22.3): the governed full-suite command is go test -timeout=30m ./... for P-002.6, P-004, and Ship Steps 4.6 and 5. Evidence: internal/core passed in 578.4 s, then four runs timed out at 600.8 to 602.7 s. Projected need was 760 to 1017 s, giving 1.8 to 2.4 times headroom.
- Policy O11 (in 11BE840F): budget formula max(10m, ceil_5m(N_max x 900ms x 2)), capped at 45m. N_max is the static count of top-level tests in the largest package. Today internal/core has 885 tests, so the budget is 30m0s. The single source of truth is internal/testbudget, with a governed entry at cmd/test-budget.
- The 18-task O11 harvest was rejected as over-engineering after four review FAILs. The minimal ensemble was chosen: 174.074-T (governed full-suite budget contract test, RED deliverable), 174.075-T (pin the 30m budget in Ship and P-002.6; depends on 174.074-T), and 174.076-T (reconcile harness-manifest checksums; depends on 174.075-T). 155-S grew to 39 items.
- 62C6A469 (DEFERRED SCOPE EXPANSION) was split into one test-only task, 174.073-T: route size and complexity core_test fixtures through the recovery-free seam. Option A2 was chosen over converting all fixtures, raising the timeout, partitioning, and template caching (deliberation 072-DL).
- Stash: 11BE840F archived as consumed. D8EF5443, 5F1A1873, 5A1C4D3F, and 95DF7CE9 stayed active. CB8887AF and 7CE12EE6 were created.

## Files modified or created

- internal/core/gate_transition_test.go, internal/core/shipment_test.go, internal/core/025_archive_harness_test.go (174.069-T)
- internal/core/shipment_shipped_event_durability_test.go (174.070-T one-line correction)
- internal/core/workspace_test_seam_test.go, workspace_no_recovery_regression_test.go, artifacts_expansion_test.go (174.071-T)
- .github/agents/_ship.agent.md, .autoharness/harness-manifest.yaml, tests/integration/shipment_155_harness_contract_test.go (174.072-T)
- tests/integration/governed_full_suite_budget_test.go (planned 174.074-T harness)
- docs/exec-plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md (Wave 18 and 19 sections, plan review records)
- .backlogit/queue/072-DL.md, 073-DL and 174.073-T (deliberation and task)
- logs/diagnostics captures for the failed full-suite runs (workspace-contained, not tracked)

## Key learnings

- Full repository test runs on this Windows host exceed the default 10-minute per-package timeout in internal/core. Timeouts landed in unrelated tests (gate version probes, workspace construction, template file reads), so the cause is package-wide runtime cost rather than one hung fixture.
- Ordinary shipment fixtures must not run executable gate brokers. Fake runners such as *taskAwareRunner and *headChangingRunner must stay assignable, while real exec-backed types are rejected structurally.
- Repository-established lint gate: CI-pinned golangci-lint v1.64.8 with Go 1.24.0, run with --new-from-rev against base 37a5cba4.
- On Windows, raw gofmt -l is distorted by CRLF materialization. Scoped LF-normalized verification is the valid check.
- backlogit MCP tools were not surfaced in these sessions, so the registry CLI fallback was used. backlogit add --section silently dropped acceptance-criteria and implementation-notes sections, while update --section name=value applied them. Rendered task bodies must be verified after add.
- A governed done transition archived the queue record of 174.074-T. That was outside the rev2 waiver, so Ship failed closed.
- Two early Ship checkpoint creates produced unparseable files, which were addressed later (see the 2026-09-25 record).

## Failed or rejected approaches

- Full-suite runs: each execution failed or timed out. Outcomes were: a wave-14 timeout in TestShipShipment_RestoresNonMemberFeatureEvenWhenShipFailsAfterRollup (blocked in gate ExecVersionRunner.Version); a post-174.070 failure with an internal/core timeout (lock path in recoverPendingShipmentOperations) plus an integration failure because backlogit_block_shipment was missing from the Ship tool declaration; and a post-wave-17 timeout in TestSetArtifactSize_BusyLockReturnsErrTaskBusy while loading templates.
- Narrowing injectBroker to *fakeGateRunner: a compile failure that rejected existing specialized fakes. The remedy kept structural rejection of exec-backed types while restoring compatibility.
- A pinned lint failure (errcheck on requireShippedAppendPartial in shipment_shipped_event_durability_test.go line 359) from earlier durability work. It was outside the 174.069-T contract, so Stage had to disposition it.
- A scoped format command that PowerShell failed to parse (variable interpolation). No retry.
- The 18-task O11 plan: plan-review attempts 1 to 3 failed, and attempt 4 failed with a P0 in a source-shape harness that pinned transition-only scaffold state.

## Outcomes and open items

- Completed and archived: 174.069-T, 174.070-T, 174.071-T, 174.072-T.
- Planned: 174.073-T (ready for Ship). Ship was to implement it, then stop before any further go test ./... run.
- 174.074-T halted after its governed done transition archived its queue record. Ship did not proceed to W2 or W3.
- The attempt-4 review escalation resolved to gpt-6-sol, but no Engram handoff was available, so it was ESCALATION_DEGRADED and the session halted to the operator.
- 155-S stayed active throughout. 154-S stayed queued and untouched. PR 449 was untouched. No PR or merge for 155-S occurred.
- Any new full-suite run required explicit operator authorization.

## Sources

- docs/memory/2026-09-24/ship-155s-174069-174070-complete.md
- docs/memory/2026-09-24/ship-155s-174069-green-compile-blocked.md
- docs/memory/2026-09-24/ship-155s-174069-pinned-lint-blocked.md
- docs/memory/2026-09-24/ship-155s-174070-format-command-blocked.md
- docs/memory/2026-09-24/ship-155s-post-174070-full-suite-failed.md
- docs/memory/2026-09-24/ship-155s-post-wave17-full-suite-timeout.md
- docs/memory/2026-09-24/ship-155s-w1-w3-blocked-174074-autoarchive.md
- docs/memory/2026-09-24/ship-155s-wave14-full-suite-timeout.md
- docs/memory/2026-09-24/ship-155s-wave17-correctives-complete.md
- docs/memory/2026-09-24/stage-073dl-redeliberation-minimal-155s.md
- docs/memory/2026-09-24/stage-11be840f-attempt4-review-fail.md
- docs/memory/2026-09-24/stage-11be840f-governed-test-budget.md
- docs/memory/2026-09-24/stage-62c6a469-wave18-corrective.md
