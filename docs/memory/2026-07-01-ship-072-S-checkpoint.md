# Ship checkpoint — 072-S doctor --target nil-HeaderDef hardening

- **Date**: 2026-07-01
- **Agent**: Ship
- **Shipment**: `072-S` — "doctor --target: fail closed on nil header-def (072-F)"
- **Feature**: `072-F` · **Task**: `072.001-T` (single)
- **Branch**: `feat/072-doctor-target-nil-headerdef` (off local `main` @ `14776b6` Stage commit)
- **Plan**: `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md`

## Items completed

- `072.001-T` — **done**, commit `cedfe94`.
  - `internal/core/doctor_target.go`: inverted the `if ws.HeaderDef != nil` guard in
    `ValidateDoctorTargetResolved` → explicit `ws.HeaderDef == nil` early return with
    `kind=DoctorTargetIO` (exit 3), `OK=false`, message
    "header definition not loaded; cannot perform required-field validation".
    Broadened the `DoctorTargetIO` constant doc comment.
  - `internal/core/doctor_target_test.go`: added `TestDoctorTarget_NilHeaderDefFailsClosed`
    (TDD red→green) with a loaded-vs-nil precedence pair (2 scenarios).
- Shipment `072-S`: claimed → **active**.

## Decisions (with rationale)

- **io (exit 3), not validation (exit 1)**: a nil workspace `HeaderDef` is a system/config
  precondition fault, not a user-correctable artifact defect. Reuses the shipped 071-S
  exit-code contract (0/1/2/3/4) with **no new kind, no exit-code table change, no schema-version
  bump**. Matches the plan Decision (Option B) and Ship's own 071-S thread ack
  (`PRRT_kwDORzozKM6Ngr6a`).
- CLI + MCP consistency is structural — both route through the single shared
  `ValidateDoctorTargetResolved`; the one-function fix covers both surfaces.

## Quality gates (full mandated set, in order)

- `go test ./...` → **PASS** (all packages incl. cli/mcp/contract/integration)
- `go vet ./...` → **PASS** (exit 0)
- `golangci-lint run` → **PASS** (exit 0)
- `gofmt -l .` → **PASS for change** (my 2 files LF-normalized clean; whole-tree CRLF flags are a
  pre-existing Windows working-tree artifact affecting all 361 tracked files; git stores LF via
  autocrlf, so CI gofmt on LF is clean)

## Review gate

- code-review (report-only): **No P0/P1**. One P3 (informational, out-of-scope): the create/update
  write paths in `internal/core/artifacts.go:224,514` share the same fail-open *shape*
  (`ValidateArtifactFields` behind `if ws.HeaderDef != nil`). Captured as a follow-up stash for Stage.

## Branch / PR state

- **PR #158**: https://github.com/softwaresalt/backlogit/pull/158 (`feat/072-doctor-target-nil-headerdef` → `main`), HEAD `30e0a35`.
- Commits: `14776b6` (Stage, rides along — origin/main lacked it), `cedfe94` (fix), `30e0a35` (backlog state).
- **CI**: all 4 checks green (`test 1.23`, `test 1.24`, Docline gate, CLI Reference Drift).
- **Copilot review**: `COMMENTED`, 11/11 files, zero comments. **§1.9 gate PASS** (covers HEAD, 0 unresolved threads).
- **P-009**: satisfied (`allow_merge_commit=true`, squash/rebase=false).
- `reviewDecision=REVIEW_REQUIRED` — branch protection needs a formal approving review (operator/P-014 concern).
- Runtime verification: **PASS** (pass/io/scope exit codes intact via source build; new branch covered by unit test).
- Closure: `docs/closure/2026-07-01-072-S-doctor-nil-headerdef-{runtime-verification,closure}.md` — **READY**.
- Follow-up stashed for Stage: `266816CE` (artifacts.go write-path fail-open shape).
- **HALTED at P-014**: PR is merge-ready, awaiting explicit operator merge approval. Do NOT self-merge.
  Post-merge closure (shipment-reconcile → ship_shipment → knowledge graduation) runs in a later
  Orchestrator-routed session after operator approval.

## Circuit-breaker counters

- build-feature attempts: 1/5 · review-fix cycles: 0/3 · fix-ci cycles: 0/5 · consecutive failures: 0/3
