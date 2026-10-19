---
title: "Orchestrator 196-S lint-gate tool-version correction"
description: "Dark-mode Orchestrator decision resolving the 196-S LINT_GATE_FAILED halt as local golangci-lint version drift."
doc_type: memory
schema_version: "1.0"
---

# Orchestrator 196-S lint-gate tool-version correction

## Context

Ship halted 196-S at build-feature Step 0.5d with `LINT_GATE_FAILED`
(`docs/memory/2026-10-04/ship-196-S-u1-lint-gate-halt.md`, checkpoint
`checkpoint-20261004-205741.json`, HEAD `24733ac8`). The local PATH linter is
golangci-lint v2.13.2, which reported 57 diagnostics (50 errcheck, 7
staticcheck), all in files that 196-S does not touch.

## Evidence

- No `.golangci*` config exists; CI pins golangci-lint `v1.64.8` with
  `--timeout=5m` (`.github/workflows/ci.yml:112-115`).
- `go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --timeout=5m`
  at HEAD `24733ac8`: exit 0, zero findings.
- LF-canonical gofmt (`git -c core.autocrlf=false -c core.eol=lf archive HEAD`,
  extracted, `gofmt -l .`): zero findings. The format gate is clean.
- The v2.13.2 findings are governed baseline debt owned by the queued
  baseline-convergence feature `175-F` (shipments `157-S`..`169-S`, terminal
  `175.012-T`), outside the 196-S scope.
- Precedent: 155-S reached the same diagnosis
  (`docs/memory/2026-09-21/155-s-quality-gate-reassessment.md`) and its final
  review recorded pinned v1.64.8 lint plus LF-normalized gofmt as passing
  gates (`docs/closure/2026-09-25-155-S-final-review.md:368-369`).

## Decision

The halt is local tool-version drift, not a branch lint failure. For every
196-S lint gate (Step 0.5d, post-loop, Step 4.3 and P-002.6), Ship uses this
evidence pair:

1. Authoritative lint gate: CI-pinned golangci-lint `v1.64.8 run --timeout=5m`
   MUST exit 0 with zero findings.
2. No-new-debt guard: golangci-lint v2.13.2 findings in any Go file changed by
   the 196-S branch (diff against the merge-base with `origin/main`) MUST be
   zero new findings relative to the merge-base.

Format gate evidence uses the LF-canonical `gofmt -l .` check, which MUST be
empty. This corrects the tooling only. It does not relax any gate, tolerate
any finding introduced by 196-S, or change Step 0.5d's rule against fix
iterations on red deliverables.

## Next step

Ship resumes from `checkpoint-20261004-205741.json`, re-runs Step 0.5d with
the corrected tooling, records the evidence, and continues waves 1-4.
