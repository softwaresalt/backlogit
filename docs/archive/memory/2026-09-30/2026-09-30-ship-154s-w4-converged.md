# Ship checkpoint: 154-S wave 4 converged

## State

- Shipment: `154-S`, active. Branch
  `feat/154-s-shipment-claim-scheduler-baseline-marker` (single worktree).
- Resumed from `.backlogit/checkpoints/checkpoint-20260930-060618.json` with
  explicit operator confirmation.
- W1-W4 converged. W5 (`173.005-T`, docs) is next.

## W4 evidence (`173.003-T`)

- Deliverable commit `ceda9a79`
  (`internal/core/shipment_claim_marker_rollback_test.go`); no production file
  changed.
- Scoped harness `go test -count=1 -run '^TestU3_' ./internal/core`: PASS.
- Amended frozen green-regression command (Stage `f8885c11`), first and only
  run: `go test -count=1 -race -timeout=30m ./internal/core/...
  ./internal/cli/... ./internal/mcp/... ./internal/db/...` exited 0 in
  13m47s (`internal/core` 813.4s).
- Pinned `golangci-lint@v1.64.8 run --timeout=10m`: exit 0.
  LF-normalized gofmt: clean on all branch-changed Go files.
- Task moved to `done` with commit tracked.

## W4 convergence gate (Step 4.6)

- Compile-only `go test -run='^$' -count=1 ./...`: exit 0.
- `go vet ./...`: exit 0.
- Open-red set empty, so the unfiltered `go test -timeout=30m ./...` ran:
  exit 0 in 27m48s.

## Next

Admit W5 (`173.005-T`), which carries neither `harness-ready` nor
`harness-exempt`; scaffold a content-probe harness before writing the doc.
