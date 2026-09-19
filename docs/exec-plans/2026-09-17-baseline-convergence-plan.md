---
chunk_strategy: h1-h2-h3
description: 'Restore repository-wide test/lint/format green by converging the complete unbounded golangci-lint v2.13.2 baseline (489 findings across 160 files) through a bounded finding-remediation DAG, unblocking shipment 149-S.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-17-baseline-convergence-plan.md
title: 'Repository Baseline Convergence Release Unit'
---


# Repository Baseline Convergence Release Unit

## Problem Frame

The repository carries an enumerable golangci-lint v2.13.2 debt baseline plus a
shared line-ending root cause (`.gitattributes text=auto` -> CRLF on Windows
checkout) that keeps `golangci-lint run` and `gofmt -l .` red. The complete
**unbounded** golangci-lint v2.13.2 baseline is **489 findings across
160 files** (459 errcheck + 30
staticcheck), captured with the same uncapped invocation semantics as
`scripts/verify-baseline-lint.ps1`. This complete count is authoritative and is
the exact set the release unit converges. `go vet ./...` is kept green throughout.

This is the covering release unit (feature `175-F`, shipment `156-S`) that makes
all four mandatory quality gates — `go test ./...`, `go vet ./...`,
`golangci-lint run`, and `gofmt -l .` — green, fixes the shared line-ending root
cause, converges the complete lint baseline through bounded finding-remediation,
and installs a persistent CI line-ending guard so line endings cannot silently
re-drift. It unblocks shipment `149-S`.

## Requirements Trace

- Complete baseline convergence: every one of the 489 governed lint
  identities is owned by exactly one finding-remediation task and resolved before
  terminal convergence.
- Line-ending root cause fixed once (U1) and guarded persistently (U40 script +
  U14 always-running workflow).
- Task-scoped lint per member during intermediate waves; mandatory zero-warning
  full-repository `golangci-lint run` only at terminal convergence (U12).
- Bounded task granularity: each remediation task owns 1-16 exact findings across
  at most 2 files/packages and stays within the 2-hour effort bound.
- Executable acyclic DAG with a unique source (U1) and a unique terminal sink
  (U12), all ordering encoded as backlogit dependencies.

## Authoritative Decomposition

The release unit decomposes into **94 executable members**:

- **U1 (`175.001-T`) — baseline-control.** The sole homogeneous mechanical
  line-ending migration and the unique DAG source.
- **90 finding-remediation tasks** (`175.002-T` through the
  highest remediation ID), each owning 1-16 exact findings across 1-2
  files/packages.
- **U40 (`175.040-T`) — support.** The line-ending guard script task.
- **U14 (`175.014-T`) — support.** The dedicated always-running line-ending guard
  workflow task.
- **U12 (`175.012-T`) — terminal-convergence.** The unique terminal sink.

Roles map to the live baseline-convergence contract: exactly one
`baseline-control`, 90 `finding-remediation`, two `support`
(U40, U14), and exactly one `terminal-convergence`.

## U1 - Line-Ending Migration (baseline-control)

U1 (`175.001-T`) is the unique DAG source and the single mechanical line-ending
migration: `.gitattributes` hardening plus a path-scoped worktree
refresh/verification of the tracked CRLF working-tree paths (confirmed by
read-only `git ls-files --eol`). The `<3 files` heuristic is documented as
exceeded — a byte-only EOL migration touches every CRLF working-tree file with
zero semantic change — and the 2-hour effort bound holds. The expected
implementation diff is `.gitattributes` only. U1 owns exactly one harness
(`TestU175_001_...`) and owns zero lint findings.

U1's actual destructive worktree renormalization/refresh requires immediate
operator-only approval at the moment of execution during Ship; the durable
shipment authorization record does not pre-approve it.

## Finding-Remediation Tasks

The 90 finding-remediation tasks partition the complete
489-identity baseline. Every governed identity has exactly one owner;
every remediation task owns at least one identity. Baseline-control, support, and
terminal-convergence members own zero findings.

### Granularity bounds

- Each remediation task owns **1-16 exact findings** and **at most 2
  files/packages**.
- When a single file carries more than 16 findings it is split into sequential
  slices; two tasks that mutate the same file never share a wave, and each later
  slice depends on the preceding slice for that file. The same-file slice groups
  are: [`175.011-T`, `175.041-T`]; [`175.042-T`, `175.043-T`, `175.044-T`]; [`175.038-T`, `175.045-T`].
- Cohesive same-package pairs are used only when total findings <= 16 and the work
  stays within the 2-hour bound; otherwise one file per task or sliced file tasks.
- Windows-owned files (4) retain a host guard so their lint
  verification runs only on Windows: `internal/core/shipment_reconcile_append_windows.go`, `internal/core/shipment_reconcile_evidence_windows.go`, `internal/core/shipment_reconcile_fs_windows.go`, `internal/core/shipment_reconcile_snapshot_windows.go`.

### Task lint contract (bounded, task-scoped)

Every remediation task carries a canonical
`<!-- BEGIN:task-lint-contract -->` block with a frozen `task_lint_cmd`, a closed
`lint_scope`, and `non_vacuity_evidence`, per the installed harness contract. The
`task_lint_cmd` is a bounded file-scoped lint verifier (FSLV) that:

1. **Proves target participation before filtering (target-vacuity guard).** For
   each owned Go file it fails closed unless the file exists, is tracked
   (`git ls-files --error-unmatch`), is not ignored (`git check-ignore`), and
   participates in its analyzed package (`go list` `GoFiles`/`TestGoFiles`/
   `XTestGoFiles`). Windows-only files are gated to the Windows host; off-host
   they short-circuit as a satisfied host guard rather than a false green.
2. **Runs golangci-lint v2.13.2 in-memory** as a child process
   (`System.Diagnostics.ProcessStartInfo`, in-memory stdout/stderr capture — no
   machine temp files, no `Remove-Item` cleanup), uncapped
   (`--max-issues-per-linter 0 --max-same-issues 0 --uniq-by-line=false`,
   `--output.json.path stdout --show-stats=false --issues-exit-code 0`), and
   preserves native launch/exit failure distinctly.
3. **Validates schema before filtering:** empty stdout, malformed JSON, JSON
   `null`, a non-object root, missing `Issues`, or a non-collection `Issues` are
   hard failures; unknown/vacuous success shapes are rejected.
4. **Matches owned identities by ordinal exact identity.** Non-slice tasks match
   the owned path(s) with any-linter ordinal path identity; slice tasks match the
   exact `path|line|column|linter` identity set. The verifier fails if any owned
   finding remains, and emits its success marker only when zero owned findings are
   observed. `Issues: []` counts as success only after target participation is
   proven.

The `task_lint_cmd` is **task-scoped only**. The full repository
`golangci-lint run` is deferred to mandatory terminal convergence and is never
used as a per-task proxy. Same-wave sibling harnesses may remain red until wave
convergence; no later-wave harness or lint scaffolding is created early.

## U40 - Line-Ending Guard Script (support)

U40 (`175.040-T`) owns exactly `scripts/check-line-endings.ps1` plus its harness
`tests/lineendings_data_guard_175_test.go` (`TestU175_040_...`). It depends on U1.
The script performs fixed-set `.gitattributes` contract, worktree EOL, and
index/blob normalization checks. U40 owns zero lint findings and uses a
harness-path FSLV (any-linter over its owned `*_175_test.go` harness path with the
target-vacuity guard).

## U14 - Dedicated Line-Ending Guard Workflow (support)

U14 (`175.014-T`) owns only the new always-running
`.github/workflows/line-endings.yml` plus its harness
`tests/lineendings_workflow_175_test.go` (`TestU175_014_...`). It depends on U40;
the workflow invokes `scripts/check-line-endings.ps1`. Security criteria:
`permissions: contents: read`, full-SHA checkout pin, `persist-credentials: false`,
and a safe concurrency group/cancel. `.github/workflows/ci.yml` remains
byte-identical. U14 owns zero lint findings and uses a harness-path FSLV.

## U12 - Terminal Verification Sink (terminal-convergence)

U12 (`175.012-T`) is the unique terminal sink and the sole harness-exempt unit
(P-002.1 `verification-only`, `harness-exempt` label, `harness_owner: none`,
`must-fail-before-deliverable` precondition). Its `exempt_verification_command`
executes the four mandatory gates — `go test ./...`, `go vet ./...`,
`golangci-lint run`, and `gofmt -l .` — fails on any nonzero exit or non-empty
gofmt output, then validates its evidence artifact and a machine-readable
full-identity `//nolint` suppression inventory.

The terminal `golangci-lint run` is the mandatory **zero-warning** full-repository
gate: the release unit is not converged until it is clean.

U12's hardened verification protections are preserved exactly:

- **Windows-native host gate first** (exit 19 before any repository gate or
  evidence work), then repository gates and evidence work.
- Docline closure frontmatter + docs-lint on the evidence artifact (exit 20).
- Lexical-state-aware `//nolint` scanner, continuous NUL-delimited byte-stream
  tracked-file enumeration, strict throw-on-invalid UTF-8 path decoding,
  ordinal-identity comparison, capitalization-drift and U+00AD soft-hyphen
  rejection, and full-identity `//nolint` inventory (missing/extra/duplicate/
  empty-justification/stale/delimiter-tamper rejection).
- **Dual-mode content-binding gate** that succeeds under both build-feature's
  pre-commit exemption-completion check and Ship's post-commit Step 4.3 rerun.
  Pre-commit mode records the current `HEAD`, and the only working-tree/index
  delta is the exact evidence artifact under `docs/closure/`. Post-commit mode
  records `HEAD^`, and the current commit changes exactly the evidence artifact
  (`git diff --name-only HEAD^ HEAD`). The completion stage may include exactly
  the convergence evidence artifact plus the exact governed claim/task-state
  paths and nothing else; both modes fail closed on ambiguous mode, a malformed
  porcelain record, or any non-allowlisted dirty tracked/untracked path.

U12 owns only its named evidence artifact under `docs/closure/` and owns zero
lint findings. It carries a canonical `task-lint-contract` block that proves its
owned delta carries no lintable Go surface (`no-go-lint-surface` prover, exits
31-34, marker `NO_GO_SURFACE_OK:175.012-T`), fail-closed on any owned `.go` path.

## Dynamic DAG

The DAG is acyclic with **94 members and 177 edges** across
**10 bounded waves** (~9-9
remediation tasks per wave):

- **U1 (`175.001-T`) is the unique source** (in-degree 0). Its direct dependents
  are `175.002-T`, `175.003-T`, `175.004-T`, `175.005-T`, `175.006-T`, `175.007-T`, `175.008-T`, `175.009-T`, `175.010-T`, `175.040-T`.
- **U40 depends on U1; U14 depends on U40** (support chain).
- The first remediation batch depends on U1. Each later remediation batch is
  gated by an explicit anchor dependency from the prior batch; per-file slices
  additionally depend on the preceding slice for that file. Batch anchors:
  `175.002-T`, `175.011-T`, `175.022-T`, `175.031-T`, `175.041-T`, `175.043-T`, `175.044-T`, `175.068-T`, `175.077-T`, `175.086-T`.
- **U12 (`175.012-T`) is the unique terminal sink** and depends on exactly
  82 immediate predecessors — the final remediation frontier
  (81 tasks) plus U14 — with U1 and U40 transitive only
  (no direct edge).
- Every member is reachable from U1 and reaches U12; there is no disconnected or
  hidden member. All execution-blocking ordering is encoded as backlogit
  dependencies, not prose.

## Baseline-Convergence Contract

Feature `175-F` carries the canonical `baseline-lint-convergence-contract` block
authored after task content is final. It records: the mode/purpose, the
shipment (`156-S`), release-unit (`175-F`), and terminal task (`175.012-T`); the
`member_scope` of all 94 executable members with their live role and
the lowercase 64-hex SHA-256 of each final committed task artifact; the committed
machine inventory (`docs/decisions/baseline-lint-inventory-489.json`, SHA-256 `7330d6baecc93e9cac4f4f96438a21565994106984f50c239cd1e6d8f86d2b66`, 489
identities); the exact intermediate-wave verifier command; the terminal command
`golangci-lint run`; and the shipment-scoped operator authorization reference.

Intermediate-wave verifier (exact remaining-baseline monotonicity):

```
pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 -Inventory docs/decisions/baseline-lint-inventory-489.json -InventorySha256 7330d6baecc93e9cac4f4f96438a21565994106984f50c239cd1e6d8f86d2b66 -Shipment 156-S -TerminalTask 175.012-T
```

Terminal command: `golangci-lint run` (zero-warning, mandatory at U12).

## Harness-Exempt Set

Closed, single member: U12 (`175.012-T`). Every other member (U1 +
90 remediation + U40 + U14) is a normal harness-required task that
exclusively owns exactly one deterministic `*_test.go` harness path with a genuine
targeted red-before-green `TestU175_<NNN>_` selector. Ship's harness phase
scaffolds every non-exempt member of the admitted ready wave together; same-wave
sibling task-scoped red is expected and tolerated until wave convergence. No
later-wave harness is scaffolded early; harnesses are not pre-committed at Stage.

## Operator Authorization

The narrow shipment-scoped operator authorization is recorded durably at
`docs/decisions/2026-09-17-baseline-convergence-authorization.md`: shipment
`156-S` may retain the exact governed findings owned by unfinished remediation
tasks until terminal convergence; no new/unowned findings; mandatory zero-warning
global lint at U12; expiring when `156-S` closes or the authorization is revoked.
It does not approve U1's destructive worktree refresh.

## Stash Classification

Sources deferred-scope-expansion stash entries `92F79833` and `4DB1DFF1`, grouped
into this single baseline-convergence covering feature.

## Decisions and Rationale

- The complete unbounded 489-finding baseline is authoritative; the
  default display cap is truncation and is never used to scope the shipment.
- Finding remediation is decomposed at bounded granularity (1-16 findings, 1-2
  files/packages) so Ship can scaffold and review bounded ready waves rather than
  one enormous wave.
- Line-ending remediation is centralized in U1 (root cause) and guarded by U40 +
  U14 so drift cannot silently return.
- Task-scoped lint per member with a proven-non-vacuous FSLV avoids false green,
  while the mandatory terminal `golangci-lint run` guarantees repository-wide
  zero-warning convergence.

## Risks and Mitigations

- **FSLV false green** (default caps suppress same-message findings): mitigated by
  uncapped flags and schema-before-filter validation.
- **Target vacuity** (linting a file that does not participate in analysis):
  mitigated by the go-list/package-membership target-vacuity guard, with a Windows
  host guard for Windows-only files.
- **Same-file concurrent mutation**: mitigated by slice ordering and the
  never-co-wave constraint.
- **Digest instability**: mitigated by the non-circular content/digest ceremony
  (finalize task bytes and timestamps, commit, digest, then author the feature
  member_scope and inventory bindings last).

## Constitution Check

- Test-first: every non-exempt member owns a genuine red-before-green harness.
- Safety-first Go: convergence drives all four gates green; terminal lint is
  zero-warning.
- Workspace isolation: all paths resolve within the workspace; U1's destructive
  refresh is operator-gated at execution.
- Git-friendly persistence: artifacts are Markdown + YAML, LF-normalized.

## Plan Review Status

dispatch_mode: multi-agent-dispatch
decision: PASS

Reviewed at the exact current HEAD across Constitution, Correctness, Template
Integrity, Scope Boundary, and Schema/CLI/Docs coupling personas, with the
policy-sensitive harness/planning coupling additionally escalated to the installed
adversarial review and post-remediation re-review. Residual P0=0, P1=0.

## Runtime Verification and Closure

Ship executes the DAG wave by wave: each member turns its own
`^TestU175_<NNN>_` selector green and satisfies its task-scoped FSLV; the
intermediate-wave verifier enforces exact remaining-baseline monotonicity against
the governed inventory; terminal convergence at U12 requires all four mandatory
gates green including a zero-warning `golangci-lint run`, the `//nolint` inventory
validated, and the Docline closure evidence artifact committed. Shipment `156-S`
converges and unblocks `149-S`.
