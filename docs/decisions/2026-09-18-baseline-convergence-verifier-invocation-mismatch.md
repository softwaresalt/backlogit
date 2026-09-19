---
chunk_strategy: h1-h2-h3
description: Fail-closed STOP record — the baseline-convergence verifier's unbounded golangci-lint invocation cannot reproduce the governed 56-finding baseline (observes 489)
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-18-baseline-convergence-verifier-invocation-mismatch.md
title: "Baseline-Convergence Verifier Invocation Mismatch (Fail-Closed STOP)"
docline:
    feature_id: 175-F
    shipment_id: 156-S
    linter: golangci-lint
    linter_version: 2.13.2
---

# Baseline-Convergence Verifier Invocation Mismatch (Fail-Closed STOP)

## Resolution

**Resolved and superseded.** The operator subsequently authorized the full-baseline
rescope that this STOP record required. The verifier's unbounded invocation is the
authoritative truth: shipment 156-S / feature 175-F binds the complete 489-finding
golangci-lint v2.13.2 baseline across 160 files (459 errcheck + 30 staticcheck). The
former 56-finding figure was golangci-lint display-cap truncation and is no longer
authoritative. The current binding and its session-scoped authorization live in
`docs/decisions/baseline-lint-inventory-489.json`, the `175-F` canonical
baseline-convergence contract, and
`docs/decisions/2026-09-17-baseline-convergence-authorization.md`. The fail-closed STOP
below is retained as historical decision evidence.

## Decision

Stage **stops and reports** rather than binding the `baseline-lint-convergence-contract`
for shipment 156-S / feature 175-F. The harness-owned verifier
`scripts/verify-baseline-lint.ps1` (installed by harness commit `5b5d0324`) cannot
reproduce the governed 56-finding baseline: with its own hard-coded invocation it
observes **489** findings. Binding the contract to the governed 56-finding inventory
would make the verifier fail-closed on its first run; rescoping the baseline to 489 is
explicitly forbidden (it silently changes scope and shatters the frozen
40-task / 41-member / 75-edge, 36-file-owned DAG). The resolution requires either a
harness-owner change to the verifier or an explicit operator scope decision — both
outside Stage's role boundary.

## Evidence

All runs used the same `golangci-lint v2.13.2` (go1.26.5) in the repository root at
HEAD `5b5d03240ae240270367d7b5bacd845c4c163bdd`, branch `stage/baseline-convergence-main`.
There is **no `.golangci` config file** in the repository, so golangci-lint uses its
built-in defaults unless flags override them.

### Verifier invocation (unbounded) — what the contract will actually run

`scripts/verify-baseline-lint.ps1` L183–196 launches:

```text
golangci-lint run --output.json.path stdout --output.text.path stderr \
  --show-stats=false --max-issues-per-linter 0 --max-same-issues 0 \
  --uniq-by-line=false --issues-exit-code 0
```

Fresh result: **489 findings** — errcheck 459 + staticcheck 30, across 160 files.

### Governed baseline capture (default caps) — the durable 56-finding inventory

`docs/decisions/2026-09-17-baseline-lint-inventory.md` captured:

```text
golangci-lint run --issues-exit-code 0 --output.text.path=NUL --output.json.path=<file>
```

Fresh result reproduced exactly: **56 findings** — errcheck 50 + staticcheck 6, across
37 files. This matches the durable inventory's headline count (errcheck 50 + staticcheck 6).

### Root cause — issue caps, not config or version drift

- The 56 default-cap findings are a **strict subset** of the 489 unbounded findings
  (0 of 56 are absent from the unbounded set). This proves pure cap-truncation, not a
  config/version difference.
- errcheck: **50** under default caps vs **459** unbounded. 50 is exactly golangci-lint's
  default `--max-issues-per-linter 50`; the verifier sets it to `0` (unbounded).
- staticcheck: **6** under default caps vs **30** unbounded, driven by the default
  `--max-same-issues 3` (verifier sets `0`) and default `--uniq-by-line true`
  (verifier sets `false`).
- Same linter version, same working directory, no config file. The **sole** difference
  is the three cap flags the verifier hard-codes.

## Why this is unresolvable within Stage's role

1. **Cannot edit the verifier.** `scripts/verify-baseline-lint.ps1` is harness-owned
   (last touched by `5b5d0324`) and out of scope for Stage edits.
2. **Cannot rescope 56 → 489.** The mandate forbids silently changing the governed
   scope, and 489 findings span 160 files — incompatible with the frozen
   40-task / 41-member / 75-edge DAG built on the 36 file-owned lint tasks.
3. **Cannot bind 56 to a 489-observing verifier.** The verifier compares observed
   findings against the inventory; at the initial all-queued state it expects the full
   set of owned residual findings and requires observed == expected. With a 56-row
   inventory it would exit `85` (`BASELINE-RESIDUAL-MISMATCH:new-or-changed`) on the
   433 uncapped errcheck + 24 uncapped staticcheck findings that are not in the
   inventory — a guaranteed, known-broken contract.

## Options for the operator / harness owner (not actioned by Stage)

- **A — Harness fix (recommended):** the harness owner changes the verifier's
  invocation to golangci-lint default caps (`--max-issues-per-linter 50
  --max-same-issues 3`, default uniq-by-line) so the runtime observation matches the
  governed 56-finding, default-cap baseline the shipment was decomposed against.
- **B — Operator rescope:** explicitly accept a new, larger baseline (e.g., 489) and
  re-run Stage decomposition; this rebuilds the DAG (≈160 files) and is a scope change,
  not a correction.

Stage takes neither action here. This record is the precise fail-closed evidence.

## Scope of this session

No `baseline-lint-convergence-contract` block was written, no task/feature/plan bodies
were rewritten, no checkpoint successor was created, and no topology, member, dependency,
or status was changed. The 14-item executable-contract mandate is blocked at its item-2
STOP-gate. The worktree remained clean at `5b5d0324` prior to committing this record.
