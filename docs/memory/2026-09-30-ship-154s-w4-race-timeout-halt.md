# Ship checkpoint: 154-S W4 race-regression blocker

## State

- Shipment: `154-S`, active.
- Branch: `feat/154-s-shipment-claim-scheduler-baseline-marker`
- HEAD: `86ad6daf451155f72ae79d1a4d7e3250f1fd619e`
- W1–W3 are converged. W3's unfiltered `go test -timeout=30m ./...` passed; the open-red set is empty.
- W4 task `173.003-T` remains active. Harness and rollback-test deliverable are uncommitted in `internal/core/shipment_claim_marker_rollback_test.go`; no task implementation commit was made.
- The task metadata has uncommitted backlogit harness/commit/comment updates. Preserve all current changes; do not clean, stash, or reset.

## W4 evidence

- Exact-M snapshot admitted only `173.003-T` to W4. Its dependencies `.001` and `.009` are done; `181.001-T` is archived terminal-success. `.005` remains a later wave because it depends on `.003`.
- Harness commit: `86ad6daf` (`test: scaffold 173.003-T rollback coverage`).
- Initial source-shape harness RED was named and assertion-based: both required scenario declarations were absent.
- The build-feature agent added the two specified rollback tests; `go test -count=1 -run '^TestU3_' ./internal/core` passed. Repo compile-only, pinned lint, and LF-normalized gofmt passed.
- Frozen `green_regression_cmds` is exactly `go test -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/... ./internal/db/...`.
- It was run once and failed: `internal/core` timed out after 10 minutes in the pre-existing `TestShipShipment_RestoresNonMemberFeatureBeforePostShipHooksObserveIt` (`internal/core/shipment_test.go`); the other listed packages passed. Do not rerun it under the task's no-known-flake-rerun policy. No completion commit was made, and W4 convergence has not passed.

## P-021 disposition

- The observed finding is outside the exact 173.003-T test-file change surface: its cause has not been isolated, and the timed-out function is pre-existing. No code fix was applied.
- A distinct capture-only deferred entry was created before any closure action: `6E37FD63`.
- Discovery covered active and archived stash plus task/run residual records. It found multiple unconfirmed candidates, none positively matching the same expansion. The stash entry records `DISCOVERY-STATUS: AMBIGUOUS` and all candidate IDs: `7AA35A39`, `A592FC1C`, `5A1C4D3F`, `6434A4D7`, `AF1E5075`, `24D693E1`, `FE440C62`, `513E62AB`, `BDA56ED8`, `C29EBEE5`.
- The task comment and this run-level checkpoint cite `6E37FD63`. This was genuinely pre-PR and has no review thread; when a PR/closure residual-risk record is later created, cite the same entry and preserve the discovery candidate list. Do not edit the captured stash entry.
- Existing risk entries `D116AF58` (per-member ship validation performance) and `67F17B6B` (harness/backlog lock-sidecar collision) were inspected and do not positively match this test timeout. Neither was edited.
- Structured Ship checkpoint: `.backlogit/checkpoints/checkpoint-20260930-060618.json`.
- The blocker-record commit is `e9d00076`; it contains only the queue/stash/checkpoint/memory records, not the uncommitted rollback-test source. The structured checkpoint's `head_sha` is its creation-time code HEAD (`86ad6daf`); this later metadata-only commit did not change the code baseline.

## Next action

Halt and surface the exact race-regression timeout and preserved working-tree state to the operator/Stage. Resume only with governed direction for the failed required gate or a Stage amendment. Do not move `173.003-T` to done, commit its test deliverable, advance to W5, or create a PR until the gate is resolved under the authorized contract. No PR, CI run, Copilot review, merge, or approval exists.
