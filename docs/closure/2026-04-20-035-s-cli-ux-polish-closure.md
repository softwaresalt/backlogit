---
title: "035-S CLI UX Polish Follow-ups — Post-Merge Closure"
description: "Release-readiness, monitoring, and rollback record for Shipment 035-S. PR #45 merged to main on 2026-04-20."
ms.date: 2026-04-20
---

## Closure Summary

* **Shipment**: 035-S — CLI UX Polish Follow-ups
* **Feature**: 033-F (deferred tasks from 034-S review)
* **PR**: [#45](https://github.com/softwaresalt/backlogit/pull/45) merged to `main` on 2026-04-20T21:32:36Z
* **Merge SHA**: `133077f4743274551b1e48866c89aab8ac0af36c`
* **Branch**: `shipment/035-s-cli-ux-polish`
* **Mode**: post-merge

## Change Summary

Two deferred P3 review findings from Shipment 034-S, promoted to implementation:

* **033.011-T — Workflow permissions layering**: Added job-level `permissions: contents: read` to `.github/workflows/cli-reference-drift.yml`. Satisfies the CI security convention requiring both workflow-level default and explicit job-level declarations.

* **033.012-T — TTY detection wiring**: Extracted `isTerminal(w io.Writer) bool` helper in `internal/cli/list.go` using `golang.org/x/term`. Updated `newRenderer(f string, w io.Writer)` to accept the writer and pass it through to `isTerminal`, eliminating the implicit `os.Stdout` assumption. Updated all 4 call sites (`list.go`, `queue_cmd.go`, `shipment.go`, `stash.go`) to pass `cmd.OutOrStdout()`. Added `internal/cli/tty_test.go` with two coverage paths (non-file writer returns false, pipe fd returns false).

Dependency addition: `golang.org/x/term v0.38.0` (direct), `golang.org/x/sys v0.39.0` (indirect). Both pinned explicitly to avoid a go directive bump (see compound learning).

## CI Status at Merge

| Check | Result |
|---|---|
| CI/test (1.23) | ✅ PASS |
| CI/test (1.24) | ✅ PASS |
| CLI Reference Drift | ✅ PASS |

All 5 Copilot review threads resolved before merge.

## Invariants to Preserve

* `go.mod` directive stays at `go 1.24.0`. `TestWorkflowGoVersionMatchesMod` in `tests/integration/ci_compliance_test.go` enforces this automatically.
* `isTerminal` returns `false` for any `io.Writer` that is not an `*os.File` or for a pipe fd — ensures JSON/table rendering is not TTY-gated in tests or piped CLI usage.
* Job-level `permissions: contents: read` is present in `cli-reference-drift.yml` in addition to the workflow-level default.
* `newRenderer` must never fall back to `os.Stdout` directly — always accepts the writer from the caller.

## Pre-Deploy Checks

Not applicable — this change is already merged to `main`. Pre-merge CI gates were satisfied.

## Deployment / Rollout Path

Merge-only to `main`. No binary release, no deployment, no feature flags. The `backlogit` binary is installed via `go install` by developers; no automated distribution pipeline is triggered by this merge.

## Post-Deploy Smoke Checks

These can be verified by any developer after pulling `main`:

1. `go test ./internal/cli/... -run TestIsTerminal` — both TTY tests pass.
2. `backlogit list 2>&1 | cat` — no TTY escape codes appear in piped output.
3. `backlogit list --format json` — valid JSON returned to stdout.
4. `go vet ./...` — zero findings.
5. `golangci-lint run` — zero findings.

## Risky Action Record

| Action | Risk | Approval | Result |
|---|---|---|---|
| Add `golang.org/x/term` dependency | moderate | reviewed in build session | applied — pinned to v0.38.0 |
| Accidental `x/term@v0.42.0` pull (go directive bump to 1.25) | high | caught by `TestWorkflowGoVersionMatchesMod` | rolled back — pinned v0.38.0 + x/sys@v0.39.0; go directive manually reset to 1.24.0 |
| Workflow permissions restructure | low | reviewed by Copilot (comment fix) | applied — both layers present |

## Healthy Signals

* CI green on all Go version matrix builds (1.23, 1.24).
* `go.mod` go directive remains `1.24.0`.
* `isTerminal` returns expected values for the two tested cases.
* No regression in TTY-sensitive rendering output.

## Failure Signals / Rollback Triggers

| Signal | Action |
|---|---|
| `go.mod` directive bumped above `1.24.0` after a future dependency update | Run `TestWorkflowGoVersionMatchesMod`; pin the offending dependency; manually reset directive |
| `backlogit list` emits ANSI codes when piped | Regression in `isTerminal` — check `newRenderer` call sites pass `cmd.OutOrStdout()` |
| CI matrix test failure after Go toolchain update | Verify `go.mod` directive and `x/term` / `x/sys` pin versions are still compatible |

## Rollback Procedure

This change is internal tooling only (no user data, no config schema change). Rollback is `git revert` of the merge commit `133077f`:

```bash
git revert -m 1 133077f4743274551b1e48866c89aab8ac0af36c
git push origin main
```

This restores `isTerminal` removal (rendering falls back to default TTY check) and removes the `x/term` dependency. No data or schema migration is involved.

## Monitoring Plan

No production runtime surface was changed. Monitoring consists of CI pass/fail on the next PR that touches `internal/cli/`. No alerting, dashboard, or observation window is required.

## Validation Window

* **Duration**: Until the next CI run on `main` (automated — no human observation window required).
* **Owner**: CI pipeline.

## Follow-up Items

| Item | ID | Priority |
|---|---|---|
| Add typed `format.Format` param to `newRenderer` (deferred F-003 from review) | 033.013-T | low |

## Closure Status

**READY** — merge occurred successfully. All CI gates passed. All review threads resolved. No runtime surface, data model, or deployment path was affected. No further post-deploy action required.
