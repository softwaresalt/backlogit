# 195-S Wave-3 Convergence

## Scope and state

- Scope remained limited to shipment `195-S`.
- Branch: `feat/claimed-versus-started-bootstrap-repair-for-ship-wave-admission-2a355f83`.
- HEAD before the closure commit: `78a88df2a7b3afe0f1df2d229ff0ba6b7bef7c62`.
- Tasks `195.001-T` through `195.007-T` are all `done`; shipment `195-S` and feature `195-F` remain `active`.
- Every task dependency in the frozen seven-task manifest is `done`; the shipment's `154-S` predecessor is archived with shipped status.
- The open-red set is empty: the declared UCS1 and UCS3 selectors both pass.
- `ROUTING_DEGRADED` remains non-blocking and must be reported. E1 remains passed under the Orchestrator-supplied served-server evidence.
- No pull request has been created, and no merge was performed.

## Wave-3 convergence evidence

- `go test -run=^$ -count=1 ./...` passed.
- `go vet ./...` passed.
- CI-pinned `golangci-lint` v1.64.8 passed.
- LF-blob `gofmt` checks passed for `claim_start_admission_contract_test.go` and `claim_start_wave_sim_contract_test.go`.
- The 195.006-T and 195.007-T scoped verification commands printed their exact `EXEMPT_VERIFY_OK` markers.
- `go test -count=1 -run '^TestUCS1_' ./tests/integration` passed.
- `go test -count=1 -run '^TestUCS3_' ./tests/integration` passed.
- The unfiltered `go test -timeout=30m ./...` suite passed.

## Persistence and next steps

- The authorized carry-forward files were restored byte-for-byte: `ship-195s-startup-halt.md` SHA-256 `0363C3042D84B7EC57BB00F2EFA4E8273AAC984DDCAD4488357E724D5F4F626F`, and `circuit-break-195-006-proof.md` SHA-256 `B749CAE0687A9CF3A6CC256DF358BEC20F12954E7BBD761BE09F1B65AEEC5495`.
- The 195.007-T `done` transition and authorized carry-forward memory restoration are included in closure commit `1b736922217b768fc9dd86dc0a71e85f4221e875`; neither remains uncommitted.
- The earlier 195.006-T proof halt is superseded by the granted reset and successful proof completion. The historical circuit-breaker record remains unchanged.

## Review remediation checkpoint

- The report-only review initially blocked readiness. In-scope fixes now strengthen UCS1 admission assertions, compare all UCS3 scenario IDs, and constrain raw item-log reads and the CLI fallback to verified paths and argv-safe invocation.
- Policy and simulation metadata now agree on P-002.6 version `1.32.0`; the simulator coverage comment names claim-assigned and indeterminate claim-start scenarios.
- Two pre-existing out-of-scope findings were captured for Stage deliberation: `FBD6E6F8` from `195.003-T` and `8B9BD74A` from `195.005-T`. Both record the ambiguous discovery candidates `BFACAE09`, `B3701713`, and `4A990AF9`; task comments point to the captures.
- Validation after these changes: `go test ./...` under Go 1.26.5, `go vet ./...`, the queue-backed scheduler simulation (`WAVE_SIM_OK: 196/196`), CI-pinned golangci-lint v1.64.8 under Go 1.24.0, changed-file LF-only `gofmt`, and the Go 1.24.0 CLI build passed.
- Review fixes are committed at `d5426b5b`. The report-only review at that HEAD returned `READY_WITH_FOLLOWUPS` with zero P0/P1/P2 findings and one P3 for this stale status line. No PR exists; complete the bootstrap dispatch and VMR checks before PR creation. Stop at merge-ready; do not merge.
