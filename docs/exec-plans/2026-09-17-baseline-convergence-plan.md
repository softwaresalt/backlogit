---
chunk_strategy: h1-h2-h3
description: 'Restore repository-wide test/lint/format green by converging the complete unbounded golangci-lint v2.13.2 supported-platform baseline (497 unique findings across the windows and linux surfaces) through a bounded finding-remediation DAG, unblocking shipment 149-S.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-17-baseline-convergence-plan.md
title: 'Repository Baseline Convergence Release Unit'
---


# Repository Baseline Convergence Release Unit

> **PACKAGING SUPERSEDED (2026-09-19).** The task DAG below (98 members, 184 edges,
> 13 bounded waves) remains authoritative and unchanged. Its original single-shipment
> packaging in `156-S` was abandoned with PR #448 (closed without merge) and is
> **superseded** by 13 wave-aligned replacement shipments `157-S`..`169-S`
> (one bounded wave each, task-IDs-only, chained by `blocks` dependencies),
> preceded by the runner-bootstrap prerequisite shipment `176-S` (RS-W(-1)) that
> gates the sequence — strict order `176→157→…→169`, **14 shipments** total. The
> 98-member DAG below is the remediation sub-DAG; the feature additionally carries
> the three dependency-ordered prerequisite tasks `175.099-T`, `175.100-T`,
> `175.101-T` (see "## Runner-Bootstrap Prerequisite"),
> giving a feature executable-task total of **101**.
> The sole current execution recommendation is
> `docs/decisions/2026-09-19-baseline-convergence-decomposition.md`.
> References to `156-S` and to Ship-authored verifier scripts / `ci.yml` in this plan
> are retained as historical/design context only; that implementation is not landed
> on the current staging branch and is produced by Ship during execution.

## Problem Frame

The repository carries an enumerable golangci-lint v2.13.2 debt baseline plus a
shared line-ending root cause (`.gitattributes text=auto` -> CRLF on Windows
checkout) that keeps `golangci-lint run` and `gofmt -l .` red. The complete
**unbounded** golangci-lint v2.13.2 baseline spans the two supported-platform
surfaces `windows` and `linux`: the Windows-primary surface carries **489
findings across 160 files** (459 errcheck + 30 staticcheck), the Linux surface
adds **8 Linux-only errcheck findings** on `_unix.go` sources and excludes 4
Windows-only findings on `_windows.go` sources, and the **unique
supported-platform union is 497 findings across 166 files** (intersection 485).
It is captured with the same uncapped invocation semantics as
`scripts/verify-baseline-lint.ps1`. This complete union is authoritative and is
the exact set the release unit converges. `go vet ./...` is kept green throughout.

This is the covering release unit (feature `175-F`) that makes all four mandatory
quality gates — `go test ./...`, `go vet ./...`, `golangci-lint run`, and
`gofmt -l .` — green, fixes the shared line-ending root cause, converges the
complete lint baseline through bounded finding-remediation, and installs a
persistent CI line-ending guard so line endings cannot silently re-drift. It
unblocks shipment `149-S`.

## Requirements Trace

- Complete baseline convergence: every one of the 497 governed lint occurrences in
  the supported-platform union (237 stable groups under the
  `line-stable-group-multiset/v1` identity model) is owned by exactly one
  finding-remediation task as an exact disjoint occurrence-count partition of its
  stable group, and resolved before terminal convergence.
- Line-ending root cause fixed once (U1) and guarded persistently (U40 script +
  U14 always-running workflow).
- Task-scoped lint per member during intermediate waves; mandatory zero-warning
  full-repository `golangci-lint run` on both supported surfaces only at terminal
  convergence (U12).
- Bounded task granularity: each remediation task owns 1-16 exact findings across
  at most 2 files/packages and stays within the 2-hour effort bound.
- Executable acyclic DAG with a unique source (U1) and a unique terminal sink
  (U12), all ordering encoded as backlogit dependencies.

## Authoritative Decomposition

The release unit's **remediation sub-DAG** comprises **98 executable members**
(topology unchanged):

- **U1 (`175.001-T`) — baseline-control.** The sole homogeneous mechanical
  line-ending migration and the unique DAG source.
- **94 finding-remediation tasks** (`175.002-T` through `175.098-T`), each owning
  1-16 exact findings across 1-2 files/packages. 90 own Windows-primary findings
  and `175.095-T` through `175.098-T` own the 8 Linux-only findings.
- **U40 (`175.040-T`) — support.** The line-ending guard script task.
- **U14 (`175.014-T`) — support.** The dedicated always-running line-ending guard
  workflow task.
- **U12 (`175.012-T`) — terminal-convergence.** The unique terminal sink.

Roles map to the live baseline-convergence contract: exactly one
`baseline-control`, 94 `finding-remediation`, two `support` (U40, U14), and
exactly one `terminal-convergence`.

Beyond this 98-member remediation sub-DAG, the feature carries **three
dependency-ordered runner-bootstrap prerequisite members** — `175.099-T`
(task-lint runner), `175.100-T` (baseline-lint runner), and `175.101-T`
(terminal-lint runner) — that between them own creation of the three shared lint
runners and gate the sub-DAG. Counting them, the feature's executable-task total is
**101** (98 remediation sub-DAG + 3 prerequisite). The three are deliberately
outside the 98-member sub-DAG topology and are described in
"## Runner-Bootstrap Prerequisite" below.

## Runner-Bootstrap Prerequisite (RS-W(-1))

Every baseline-remediation member's task-lint contract invokes the canonical short
runner `scripts/verify-task-lint.ps1`, and the feature additionally requires the
intermediate-wave runner `scripts/verify-baseline-lint.ps1` and the terminal
runner `scripts/verify-terminal-lint.ps1`. None of those three runner scripts
exists on this branch, and no `175.*` remediation member owns creating them. Three
`runner-bootstrap` prerequisite tasks close that gap, one runner each (split from a
prior oversized single bootstrap task per PR #449 cycle 9 so each owns at most two
files and a bounded at-most-three-scenario matrix): `175.099-T` OWNS
`scripts/verify-task-lint.ps1`, `175.100-T` OWNS `scripts/verify-baseline-lint.ps1`,
and `175.101-T` OWNS `scripts/verify-terminal-lint.ps1`. All three ship together in
the prerequisite shipment `176-S` (RS-W(-1)). The prerequisite gates the replacement
sequence via `157-S depends_on 176-S` (shipment edge); the tasks are dependency
ordered `175.099-T -> 175.100-T -> 175.101-T` (baseline runner after task runner,
terminal runner after baseline runner), and U1 depends on the terminal sink
(`175.001-T depends_on 175.101-T`, task edge), making `175.099-T` the in-degree-zero
source of the FULL feature graph and `175.101-T` its bootstrap sink, while U1 remains
the source of the 98-member remediation sub-DAG. `176-S` owns zero baseline findings
and is not an intermediate-wave verifier target.

Because each of `175.099-T`, `175.100-T`, `175.101-T` CREATES the runner it verifies,
its task-lint GATE MUST NOT be a direct invocation of the runner it creates (a
circular self-dependency); it is that task's own owned Go harness
(`tests/runner_bootstrap_task_lint_175_099_test.go` /
`tests/runner_bootstrap_baseline_lint_175_100_test.go` /
`tests/runner_bootstrap_terminal_lint_175_101_test.go`, functions
`TestU175_099_TaskLintRunnerBootstrap` / `TestU175_100_BaselineLintRunnerBootstrap` /
`TestU175_101_TerminalLintRunnerBootstrap`), which COMPILES and whose targeted test
EXECUTES under existing main-branch tooling before its runner exists — RED (nonzero
exit) before that runner exists, GREEN after — statically proving the runner exists, is
tracked, is non-empty, parses cleanly under the built-in PowerShell AST parser,
and declares its required `param(...)` surface, and — after the runner exists —
behaviorally exercising the runner against harness-authored deterministic fixtures
for its success and failure/fail-closed contracts (non-vacuous). No source,
scripts, or tests are implemented in this planning PR; the three tasks declare the
ownership Ship executes later. Full contracts: `.backlogit/queue/175.099-T.md`,
`.backlogit/queue/175.100-T.md`, `.backlogit/queue/175.101-T.md`.

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

The 94 finding-remediation tasks partition the complete 497-identity
supported-platform union. Every governed identity has exactly one owner; every
remediation task owns at least one identity. Baseline-control, support, and
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
- Windows-owned files (4) own only `windows`-surface findings and are linted under
  the `windows` build surface: `internal/core/shipment_reconcile_append_windows.go`, `internal/core/shipment_reconcile_evidence_windows.go`, `internal/core/shipment_reconcile_fs_windows.go`, `internal/core/shipment_reconcile_snapshot_windows.go`.
- Linux-owned files (6) own only `linux`-surface findings and are linted under the
  `linux` build surface (cross-compiled via `GOOS=linux` on the same host): `internal/core/shipment_reconcile_append_unix.go`, `internal/core/shipment_reconcile_evidence_unix.go`, `internal/core/shipment_reconcile_fs_unix.go`, `internal/core/shipment_reconcile_lock_unix.go`, `internal/core/shipment_reconcile_snapshot_unix.go`, `internal/events/item_log_lock_unix.go`.

### Task lint contract (bounded, task-scoped)

Every remediation task carries a canonical `<!-- BEGIN:task-lint-contract -->`
block with a frozen `task_lint_cmd`, a closed structured `lint_scope`, and
`non_vacuity_evidence`, per the installed harness contract. The `task_lint_cmd`
invokes the canonical short runner
`pwsh -NoProfile -File scripts/verify-task-lint.ps1 -TaskId <task-id> -FeatureId 175-F`
(`lint_scope.kind: bounded-finding-set-go-lint`); task bodies never embed a
duplicated PowerShell runner. The canonical runner:

1. **Proves target participation before filtering (target-vacuity guard).** For
   each owned Go file it fails closed unless the file exists, is tracked
   (`git ls-files --error-unmatch`), is not ignored (`git check-ignore`), and
   participates in its analyzed package (`go list` `GoFiles`/`TestGoFiles`/
   `XTestGoFiles`). The runner cross-compiles `go list` and golangci-lint under
   both the `windows` and `linux` surfaces in-process (a `GOOS` override on a
   single host) and derives each owned file's required surface(s) from the
   inventory, so `_windows.go` files are proven under `windows` and `_unix.go`
   files under `linux` without depending on the analyzing host's own platform.
2. **Runs golangci-lint v2.13.2 in-memory** as a child process with in-memory
   stdout/stderr capture (no machine temp files, no `Remove-Item` cleanup),
   uncapped (`--max-issues-per-linter 0 --max-same-issues 0 --uniq-by-line=false`),
   and preserves native launch/exit failure distinctly.
3. **Validates schema before filtering:** empty stdout, malformed JSON, JSON
   `null`, a non-object root, missing `Issues`, or a non-collection `Issues` are
   hard failures; unknown/vacuous success shapes are rejected.
4. **Matches owned identities by line-stable stable-group multiplicity.** Ownership
   identity is the line-stable group-multiset model (`line-stable-group-multiset/v1`),
   NOT exact `path|line|column|linter|message`. Line and column are retained only as
   diagnostic baseline coordinates and are never part of durable ownership identity.
   Each owned finding belongs to a **stable group** keyed by the line-independent tuple
   `(path, linter, normalized_message)` and fingerprinted as
   `sha256(json([path,linter,normalized_message]))`; `normalized_message` is the linter
   message under Unicode NFC + trim + internal-whitespace collapse. Each remediation
   task owns, per governed stable group, an exact `owned_count` (occurrence multiplicity)
   drawn from a contiguous baseline-ordinal range, and declares its
   `expected_group_residual_after_task` (the count still owned by strictly-later sibling
   slices; `0` when the task is the sole or final owner of the group). The verifier
   re-derives each owned group's fingerprint from live golangci-lint output (path,
   `FromLinter`, normalized `Text` — all line-shift-invariant), counts the group's
   residual occurrences, and requires observed residual to **equal**
   `expected_group_residual_after_task` for every owned group. Observed **greater than**
   expected (an owned occurrence — including one shifted by a multiline edit — still
   present) fails; observed **less than** expected (the task resolved an occurrence a
   later slice owns) also fails, preserving task isolation even for interchangeable
   duplicates. Occurrences within one stable group are truly indistinguishable and
   semantically interchangeable, so the contract is enforced by required multiplicity
   decrease, not by matching a specific occurrence to a specific line. A group that
   appears in output with no baseline fingerprint (new finding) or a baseline group
   whose message/path/linter changed (reorder across distinguishable findings) fails.
   `Issues: []` counts as success only after target participation is proven.

Each member binds a stable `task_contract_sha256` computed over its canonical
task-lint-contract JSON, surviving the queue-to-archive lifecycle. The
`task_lint_cmd` is **task-scoped only**. The full repository `golangci-lint run`
is deferred to mandatory terminal convergence and is never used as a per-task
proxy. Same-wave sibling harnesses may remain red until wave convergence; no
later-wave harness or lint scaffolding is created early.

## Line-Stable Ownership Identity

The durable ownership identity is the **line-stable group-multiset model**
(`line-stable-group-multiset/v1`), recorded canonically in
`docs/decisions/baseline-lint-inventory.json` (`identity_model`,
`group_index`, and per-row `group_fingerprint`/`group_ordinal`/`group_multiplicity`).
Line and column survive only as diagnostic baseline coordinates (requirement:
they are never part of ownership identity).

- **Stable group key.** `(path, linter, normalized_message)` — every field is
  line-independent and reproducible at both baseline capture and verification.
  golangci-lint re-emits path, `FromLinter`, and message `Text` for each issue
  regardless of any line shift, so the fingerprint is stable across multiline edits.
  `normalized_message` = Unicode NFC + trim + collapse internal whitespace.
  `group_fingerprint = sha256(json([path, linter, normalized_message]))`.
- **Multiplicity and order.** `group_multiplicity` is the baseline occurrence count
  of the group; `group_ordinal` is a 1-based rank by ascending `(line, column)`, used
  only for the deterministic contiguous ownership partition and as a diagnostic — never
  as identity.
- **Ownership.** Every governed occurrence is owned by exactly one remediation task as
  an exact disjoint occurrence-count partition of its stable group. A group whose
  occupancy fits one task is owned wholly by that task; a group larger than a single
  slice is owned by an **ordered slice chain** (`slice k/n`, each depending on the
  prior slice) over contiguous ordinal ranges.
- **Interchangeability (explicit).** Occurrences within one stable group are truly
  indistinguishable and semantically interchangeable — identical path, linter, and
  normalized message; the identical mechanical fix at each call site. The ownership
  contract is therefore enforced by required **multiplicity decrease**, not by binding
  a specific occurrence to a specific line. This is the sound treatment of the
  same-file split groups (the only 6 split groups are
  `internal/cli/migrate.go` errcheck, `internal/cli/telemetry.go` errcheck, and
  `internal/telemetry/reporter.go` staticcheck QF1012).
- **Expected residual.** For a task `T` owning group `G`,
  `expected_group_residual_after_task = group_multiplicity(G) − Σ owned_count(s)` over
  `T` and every slice ordered at-or-before `T` — equivalently the summed `owned_count`
  of strictly-later slices. The intermediate-wave verifier's expected residual for `G`
  after a set of completed tasks equals the baseline multiplicity minus the summed
  `owned_count` of the completed owners of `G`. A task-scoped or wave verifier fails
  when observed residual is greater (a still-present owned occurrence, including a
  shifted one) OR less (an occurrence a later slice owns was resolved) than expected,
  and fails on any new-fingerprint or reordered-across-distinguishable-groups finding.

Simplest-correct-form rationale (requirement: "choose and document the simplest
correct form"): the model uses only attributes already present in the governed
inventory (`path`, `linter`, `message`) plus deterministically derived
multiplicity/ordinal data. It deliberately does **not** introduce enclosing-symbol or
normalized-AST/call fingerprints: those are absent from the inventory and deriving them
would require re-inspecting source at baseline coordinates (speculative mapping, which
is prohibited), and — because every actual split group's occurrences are already
genuinely interchangeable under `(path, linter, normalized_message)` — the extra
attributes are unnecessary for correctness. No symbol or fingerprint value is invented.

### Non-vacuity red scenarios (must fail)

These scenarios are the correctness contract the canonical runners
(`scripts/verify-task-lint.ps1`, `scripts/verify-baseline-lint.ps1`) implement, and
are mirrored by the runner-bootstrap tasks' behavioral fixtures (`175.099-T`,
`175.100-T`, `175.101-T`).

1. **Multiline edit before a remaining finding (shift must not hide it).** A completed
   task edits above a still-owned occurrence of group `G`, shifting it from L100 to
   L112. Because `G` is matched by fingerprint and counted by multiplicity, the shifted
   occurrence is still counted; observed residual for `G` stays above
   `expected_group_residual_after_task` → **FAIL**. (Under the retired exact-line model
   the shifted occurrence no longer matched L100 and silently vanished.)
2. **Repeated identical messages preserve multiplicity.** `internal/cli/telemetry.go`
   carries 16 interchangeable `errcheck` `fmt.Fprintf` occurrences. Slice `175.042-T`
   owns 7 with `expected_group_residual_after_task = 9`; resolving only 6 leaves observed
   10 > 9 → **FAIL**; resolving 7 leaves 9 → PASS; the group is only fully cleared after
   `175.044-T` (residual 0). Multiplicity, not line position, is enforced.
3. **Wrong sibling occurrence cannot satisfy a distinguishable owned fingerprint.** A
   task owning group `G1` (e.g. `ws.Close`) that instead resolves an occurrence of the
   distinguishable group `G2` (e.g. `fmt.Fprintln`) leaves `G1` residual above expected
   → **FAIL**; distinguishable findings are separate fingerprints with independent
   multiplicity accounting.
4. **New or reordered unmatched findings fail.** A golangci-lint output group whose
   fingerprint is absent from the baseline (new finding), or a baseline occurrence whose
   path/linter/message changed (reorder across distinguishable groups → new + missing),
   fails the wave verifier.
5. **Zero current findings only succeeds after target participation.** `Issues: []` is
   accepted only after the target-vacuity guard proves each owned file exists, is
   tracked, is not ignored, and participates in its analyzed package on its required
   surface; an empty result from a non-participating target fails closed.

### 175.042-T worked example

Under the split group `internal/cli/telemetry.go | errcheck | fmt.Fprintf`
(`group_multiplicity = 16`), the ordered chain is `175.042-T (slice 1/3)` →
`175.043-T (slice 2/3)` → `175.044-T (slice 3/3)`, owning 7, 6, 3 occurrences with
`expected_group_residual_after_task` 9, 3, 0 respectively; the parallel `fmt.Fprintln`
group (`group_multiplicity = 14`) owns 6, 6, 2 with residual 8, 2, 0. `175.042-T`
passes iff each of its owned groups drops to exactly its recorded residual — it cannot
pass by clearing a `175.043-T`/`175.044-T`-owned occurrence (residual would fall below
expected) and cannot pass while leaving one of its own (residual would stay above
expected), regardless of any line shifts introduced by its own multiline edits.

## U40 - Line-Ending Guard Script (support)

U40 (`175.040-T`) owns exactly `scripts/check-line-endings.ps1` plus its harness
`tests/lineendings_data_guard_175_test.go` (`TestU175_040_...`). It depends on U1.
The script performs fixed-set `.gitattributes` contract, worktree EOL, and
index/blob normalization checks. U40 owns zero lint findings and uses a canonical
`harness-file-scoped-go-lint` task-lint contract over its owned harness path with
the target-vacuity guard.

## U14 - Dedicated Line-Ending Guard Workflow (support)

U14 (`175.014-T`) owns only the new always-running
`.github/workflows/line-endings.yml` plus its harness
`tests/lineendings_workflow_175_test.go` (`TestU175_014_...`). It depends on U40;
the workflow invokes `scripts/check-line-endings.ps1`. Security criteria:
`permissions: contents: read`, full-SHA checkout pin, `persist-credentials: false`,
and a safe concurrency group/cancel. `.github/workflows/ci.yml` remains
byte-identical. U14 owns zero lint findings and uses a canonical
`harness-file-scoped-go-lint` task-lint contract.

## U12 - Terminal Verification Sink (terminal-convergence)

U12 (`175.012-T`) is the unique terminal sink and the sole harness-exempt unit
(P-002.1 `verification-only`, `harness-exempt` label, `harness_owner: none`,
`must-fail-before-deliverable` precondition). It runs the mandatory build/vet/format
gates — `go test ./...`, `go vet ./...`, and `gofmt -l .` — and delegates the mandatory
zero-warning terminal LINT gate to the canonical terminal runner
`scripts/verify-terminal-lint.ps1 -FeatureId 175-F`; it fails on any nonzero exit, a
missing `TERMINAL-LINT-OK:175-F` marker, or non-empty gofmt output, then validates its
evidence artifact and a machine-readable full-identity `//nolint` suppression inventory.

The canonical terminal runner `scripts/verify-terminal-lint.ps1 -FeatureId 175-F` is the
mandatory **zero-warning** full-repository gate: it screens both declared supported
surfaces (`windows`, `linux`) via an in-process `GOOS` override on a single host, fails
closed if either surface carries a residual warning or execution fails, and emits
`TERMINAL-LINT-OK:175-F` only after both surfaces are clean. The release unit is not
converged until both surfaces are clean; U12 is the task that invokes this runner.

U12's hardened verification protections are preserved exactly:

- **Cross-surface terminal lint gate** through the canonical terminal runner, proving
  zero residual warnings independently on the `windows` and `linux` surfaces (in-process
  `GOOS` override on a single host) before evidence work — covering both the
  windows-build-tagged U19-U22 files and the Linux-only `_unix.go` files without
  depending on the analyzing host's own platform.
- Docline closure frontmatter + docs-lint on the evidence artifact.
- Lexical-state-aware `//nolint` scanner, continuous NUL-delimited byte-stream
  tracked-file enumeration, strict throw-on-invalid UTF-8 path decoding,
  ordinal-identity comparison, capitalization-drift and U+00AD soft-hyphen
  rejection, and full-identity `//nolint` inventory (missing/extra/duplicate/
  empty-justification/stale/delimiter-tamper rejection).
- **Dual-mode content-binding gate** enforced by the canonical runner's
  `-Phase PreCommit` and `-Phase PostCommit` lifecycle screens. Pre-commit mode
  records the current `HEAD`, and the only working-tree/index delta is the exact
  evidence artifact under `docs/closure/`. Post-commit mode records `HEAD^`, and
  the current commit changes exactly the evidence artifact
  (`git diff --name-only HEAD^ HEAD`). The governed-claim union is the closed
  allowlist of the full `169-S` claim footprint (`core.ClaimShipment` persists
  the shipment and every queued member): `.backlogit/queue/169-S.md`,
  `.backlogit/queue/175.012-T.md`, and `.backlogit/hooks_queue.jsonl`;
  both modes fail closed on ambiguous mode, a malformed porcelain record, or any
  non-allowlisted dirty source, config, test, script, or workflow path.

U12 owns only its named evidence artifact under `docs/closure/` and owns zero
lint findings. It carries a canonical `task-lint-contract` block that proves its
owned delta carries no lintable Go surface (`no-go-lint-surface` prover),
fail-closed on any owned `.go` path.

## Dynamic DAG

The remediation sub-DAG is acyclic with **98 members and 184 edges** across **13
bounded waves** (11 remediation waves plus the source and terminal boundary waves;
the U40 and U14 support members fold into remediation waves):

- **U1 (`175.001-T`) is the unique source of the remediation sub-DAG**
  (in-degree 0 within the sub-DAG). Its direct dependents
  are `175.002-T`, `175.003-T`, `175.004-T`, `175.005-T`, `175.006-T`, `175.007-T`, `175.008-T`, `175.009-T`, `175.010-T`, `175.040-T`.
- **Full-feature source (outside the sub-DAG):** the runner-bootstrap prerequisite
  chain `175.099-T -> 175.100-T -> 175.101-T` (shipment `176-S`, RS-W(-1)) sits ahead
  of the sub-DAG. `175.099-T` is the in-degree-zero source of the full feature graph;
  its sink `175.101-T` gates the sub-DAG via `175.001-T depends_on 175.101-T` (task
  edge) and `157-S depends_on 176-S` (shipment edge). The 98-member sub-DAG's internal
  source remains U1; the bootstrap chain sits ahead of it as a pre-DAG bootstrap.
- **U40 depends on U1; U14 depends on U40** (support chain).
- The first remediation batch depends on U1. Each later remediation batch is
  gated by an explicit anchor dependency from the prior batch; per-file slices
  additionally depend on the preceding slice for that file. Per-wave remediation
  anchors (illustrative wave entry points; the authoritative execution ordering is
  encoded in the backlogit dependency edges, not this list): `175.002-T`, `175.011-T`, `175.022-T`, `175.031-T`, `175.041-T`, `175.043-T`, `175.044-T`, `175.068-T`, `175.077-T`, `175.086-T`, `175.095-T`.
- **U12 (`175.012-T`) is the unique terminal sink** and depends on exactly
  85 immediate predecessors — the final remediation frontier (84 tasks) plus U14
  (`175.014-T`) — with U1 and U40 transitive only (no direct edge).
- Every member is reachable from U1 and reaches U12; there is no disconnected or
  hidden member. All execution-blocking ordering is encoded as backlogit
  dependencies, not prose.

## Baseline-Convergence Contract

Feature `175-F` carries the canonical `baseline-lint-convergence-contract` block
authored after task content is final. It records: the mode/purpose; the
release-unit (`175-F`) and terminal task (`175.012-T`); the current packaging (the
prerequisite shipment `176-S` carrying the three bootstrap tasks `175.099-T`,
`175.100-T`, `175.101-T`, gating the replacement
sequence `157-S`..`169-S`, superseding the abandoned single shipment `156-S`); the
declared `supported_goos` (`windows`, `linux`); the `member_scope` of all 98
remediation sub-DAG members with their live role and the lowercase 64-hex
`task_contract_sha256` of each final committed canonical task-lint-contract JSON
(the runner-bootstrap prerequisites `175.099-T`, `175.100-T`, `175.101-T` are recorded
separately in the
`packaging.prerequisite_*` fields, outside this remediation `member_scope`); the
committed machine inventory (`docs/decisions/baseline-lint-inventory.json`, schema
`baseline-lint-platform-inventory/v3` carrying the `line-stable-group-multiset/v1`
`identity_model`, the `group_index`, and per-row
`group_fingerprint`/`group_ordinal`/`group_multiplicity`; SHA-256
of the canonical checked-in LF inventory bytes — equivalently the post-U1 LF
working-tree bytes, not the host CRLF checkout —
`bbb20803209fcf26af03e8dd13af1ad6236629fb2c7432caf14be5badddc5f4c`, 497 occurrences
across 237 stable groups);
the exact intermediate-wave verifier command; the canonical terminal command; and
the shipment-scoped operator authorization reference.

Intermediate-wave verifier (exact remaining-baseline monotonicity):

```
pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 -Inventory docs/decisions/baseline-lint-inventory.json -InventorySha256 bbb20803209fcf26af03e8dd13af1ad6236629fb2c7432caf14be5badddc5f4c -Shipment <replacement-shipment-id> -FeatureId 175-F -TerminalTask 175.012-T
```

> Packaging note (2026-09-19): under the accepted decomposition, the
> intermediate-wave verifier is invoked once per active replacement shipment in
> RS-W00..RS-W12 order (`157-S`..`169-S`), substituting `-Shipment` with that
> wave's replacement shipment id; the superseded `156-S` is never used as the
> `-Shipment` argument.
>
> The `-InventorySha256` value is the SHA-256 of the canonical checked-in LF
> inventory bytes (equivalently the post-U1 LF working-tree bytes, not the
> host CRLF checkout); U1 (`175.001-T`) normalizes the tracked working tree
> to LF before any intermediate-wave run, so the on-disk inventory hashes to
> this canonical digest.

Terminal command (zero-warning, mandatory at U12, both supported surfaces):

```
pwsh -NoProfile -File scripts/verify-terminal-lint.ps1 -FeatureId 175-F
```

## Harness-Exempt Set

Closed, single member: U12 (`175.012-T`). Every other member (U1 + 94
remediation + U40 + U14 + the three runner-bootstrap prerequisites `175.099-T`,
`175.100-T`, `175.101-T`) is a
normal harness-required task that exclusively owns exactly one deterministic
`*_test.go` harness path with a genuine targeted red-before-green
`TestU175_<NNN>_` selector. Each prerequisite is harness-required with
its own self-contained bootstrap harness
(`tests/runner_bootstrap_task_lint_175_099_test.go`,
`tests/runner_bootstrap_baseline_lint_175_100_test.go`,
`tests/runner_bootstrap_terminal_lint_175_101_test.go`) whose gate is that Go harness
itself — never the runner it creates — RED before that task's runner exists and GREEN
after. Ship's
harness phase scaffolds every non-exempt member of the admitted ready wave
together; same-wave sibling task-scoped red is expected and tolerated until wave
convergence. No later-wave harness is scaffolded early; harnesses are not
pre-committed at Stage.

## Operator Authorization

The operator authorization is recorded durably at
`docs/decisions/2026-09-17-baseline-convergence-authorization.md`: the exact
governed findings owned by unfinished remediation tasks may be retained until
terminal convergence; no new/unowned findings; mandatory zero-warning global lint
at U12; expiring at terminal convergence (U12 `175.012-T`) or on operator
revocation. It does not approve U1's destructive worktree refresh. Originally
scoped to shipment `156-S`, the authorization was extended — as a derived
application of existing authority over the operator-accepted decomposition — to
the wave-aligned replacement sequence `157-S`..`169-S` (see the authorization
record's scope-extension section).

## Stash Classification

Sources deferred-scope-expansion stash entries `92F79833` and `4DB1DFF1`, grouped
into this single baseline-convergence covering feature.

## Decisions and Rationale

- The complete unbounded supported-platform union (497 findings) is authoritative;
  the default display cap is truncation and is never used to scope the shipment.
  The 489-finding count is the Windows-primary surface only, not the all-platform
  total.
- Finding remediation is decomposed at bounded granularity (1-16 findings, 1-2
  files/packages) so Ship can scaffold and review bounded ready waves rather than
  one enormous wave.
- Line-ending remediation is centralized in U1 (root cause) and guarded by U40 +
  U14 so drift cannot silently return.
- Task-scoped lint per member with a proven-non-vacuous canonical task-lint runner
  avoids false green, while the mandatory terminal `golangci-lint run` guarantees
  repository-wide zero-warning convergence on both supported surfaces.

## Risks and Mitigations

- **Task-lint false green** (default caps suppress same-message findings, or a
  multiline edit shifts a still-unresolved owned occurrence off its baseline line):
  mitigated by uncapped flags, schema-before-filter validation, and the line-stable
  group-multiset identity model — matching by stable `(path, linter, normalized_message)`
  fingerprint and enforcing per-group multiplicity decrease, so a shifted occurrence is
  still counted and cannot silently satisfy the gate.
- **Target vacuity** (linting a file that does not participate in analysis):
  mitigated by the go-list/package-membership target-vacuity guard, with a host
  guard for Windows-only and Linux-only files.
- **Same-file concurrent mutation / shifted descendants**: mitigated by the ordered
  slice chain, the never-co-wave constraint, and line-independent multiplicity matching
  with a recorded `expected_group_residual_after_task` per split slice, so ownership and
  isolation hold regardless of line shifts.
- **Interchangeable-duplicate mis-attribution** (a slice passing by fixing a
  sibling-owned occurrence of the same stable group): mitigated by the cumulative
  ordinal partition plus the sequential slice dependency — a slice's expected residual
  fails both when it leaves an owned occurrence and when it resolves a later slice's.
- **Digest instability**: mitigated by the non-circular content/digest ceremony
  (finalize task bytes and timestamps, commit, digest, then author the feature
  member_scope and inventory bindings last).

## Constitution Check

- Test-first: every non-exempt member owns a genuine red-before-green harness.
- Safety-first Go: convergence drives all four gates green; terminal lint is
  zero-warning on both supported surfaces.
- Workspace isolation: all paths resolve within the workspace; U1's destructive
  refresh is operator-gated at execution.
- Git-friendly persistence: artifacts are Markdown + YAML, LF-normalized.
- Merge commit history preservation: this planning PR merges via merge commit.
- Documented deviation — I. Safety-First Go: intermediate waves retain only
  governed, owned golangci-lint findings until U12 per
  `docs/decisions/2026-09-17-baseline-convergence-authorization.md`; no new or
  unowned findings; zero-warning lint on both surfaces is mandatory at U12.
- Documented deviation — Task Granularity: U1 exceeds the fewer-than-3-files
  heuristic as one homogeneous byte-only line-ending migration; the 2-hour bound
  holds (see `## U1`).
- Documented deviation — VIII. Explicit Safety Modes: the `154-S` P-002.6
  active-residual halt waiver is a proposed high-risk action recorded
  UNAPPROVED / BLOCKED pending explicit operator confirmation (see
  `## Plan Hardening`); this planning cycle executes no waiver.

Constitution Check: documented-deviations

## Plan Review Status

> **SUPERSEDED (historical).** This PASS predates the 2026-09-20 shipment-claim /
> wave-scheduler convergence readiness rewrite and its 2026-09-20 corrective
> re-gate. The authoritative gate record is the final `## Plan Review` section at
> the end of this document (`decision: ADVISORY`, `operator_authorization:
> approved` scoped to the planning deliverable only). Do not consume this PASS.

dispatch_mode: multi-agent-dispatch
decision: PASS (superseded — see final `## Plan Review`)

Reviewed across Constitution, Correctness, Template
Integrity, Scope Boundary, Schema/CLI/Docs coupling, CI coupling, and
Maintainability personas, with the policy-sensitive harness/planning coupling
additionally escalated to the installed adversarial review and post-remediation
re-review. Residual P0=0, P1=0.

## Execution Readiness (BLOCKED) and Runtime Verification

**Execution readiness: BLOCKED.** This topology is structurally well-formed but
is **not executable** under the current shipment-claim and wave-scheduler
contracts. `core.ClaimShipment` (`internal/core/shipment_lifecycle.go`,
member-activation loop) transitions **every** queued manifest member to `active`
on claim, while Ship
wave admission (`.github/agents/_ship.agent.md`, Step 4.0 items 4 and 6) halts on **any** active
member and computes `ready_k` only from `queued` tasks. Claiming `176-S`
therefore makes all three bootstrap tasks `active` and immediately yields
`WAVE_NO_PROGRESS`; the same applies to every replacement shipment
`157-S`..`169-S`. No shipment in this feature may be treated as executable until
the shipment-claim / wave-scheduler convergence prerequisite lands.

**Convergence prerequisite (exact dependency).** Per
`docs/decisions/2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation.md`
(deliberation over stash `6434A4D7`), the chosen model preserves
`core.ClaimShipment` all-members-active semantics and realizes dependency-gated
wave admission additively. Baseline execution readiness depends on:

1. **`154-S` / `173-F`** — the in-repo scheduler-baseline **marker** enabling
   precondition (reviewed PASS). Encoded here as the advisory edge
   `154-S blocks 176-S` (the front of the replacement chain waits for the
   marker). This edge is advisory, not a hard shipped-only guard, until the
   `6434A4D7` claim-time guard lands.
2. **External autoharness P-002.6 scheduler marker-consumption** (out-of-workspace,
   P-017) — the scheduler must treat marked-active members as the wave-0
   admissible baseline and reserve the active-residual halt for unmarked
   residuals. Tracked as a cross-workspace follow-up; cannot be a backlog edge.
3. **`6434A4D7`** dependency-axis hardening (deferred Go-core release unit) —
   converts the advisory `156-S`/`149-S` edges into hard shipped-only guards and
   adds the governed non-claimable disposition. Governs, but does not gate, the
   baseline execution sequence.

**Bootstrap of the marker prerequisite itself.** The marker becomes usable only
after `154-S`/`173-F` ships **and** the external autoharness scheduler consumes
it, so every shipment on the marker's predecessor closure must run before the
marked-aware scheduler is available. That closure was recorded as the **complete
predecessor chain** `155-S (DAG root) → 154-S → 176-S → 157-S..169-S`
(`154-S depends_on 155-S` is a governed edge; DAG-root correction, commit
`69e900be`). **Refresh (2026-09-26, `main` `c57185c1`): `155-S` has SHIPPED**
(PR #450, merge `2c8759c3`; closure PR #451; `155-S` and all 45 manifest items
— `174-F` + 44 tasks — archived), so the `154-S depends_on 155-S` edge is satisfied and
retained as provenance, and the remaining chain is
`154-S (chain root) → 176-S → 157-S..169-S`. The bootstrap set was the
**bounded, non-circular two-shipment set {`155-S`, `154-S`}** — the marker's
predecessor closure; with `155-S` complete, **`154-S` is its only remaining
shipment**, and it does not extend to `176-S`..`169-S`, which use the
marked-aware scheduler with no exception. `154-S` is executed via an
**operator-supervised bootstrap**: Ship claims it, then drives the
claim-activated `173-F` members green in their declared dependency order
**without** the strict active-residual halt — the just-claimed members ARE the
intended working set. **The `154-S` active-residual halt bypass is a high-risk
action that remains UNAPPROVED / BLOCKED pending explicit operator confirmation
of that exact per-shipment waiver.** Ship's `155-S` records describe the original
authorization as scoped to {`155-S`, `154-S`} and exercised for `155-S` only; the
cited shipment comment exists only as a Ship-authored event in the local,
git-ignored `.backlogit/logs/155-S.jsonl`, not as an operator-authored
per-shipment approval on `154-S`, so it is not treated as the `154-S` approval.
**Routing hold:** with `155-S` shipped, `154-S` has no unshipped predecessor, so
the Orchestrator's status + dependency eligibility gate no longer holds it and
the current Orchestrator/Ship contracts do not read labels; `154-S` is held only
by its labels, body banner, and operator instruction and MUST NOT be routed by
`ship next`, an inferred `ship next`, or a dark-mode scope (label-aware refusal:
deferred stash `AF1E5075`; `173-F` contract re-validation against the
post-`174-F` `ClaimShipment` before the bootstrap: deferred stash `C29EBEE5`).
See the convergence deliberation's "Bootstrap Resolution"
(refresh status and waiver provenance note).

**Ready-state execution (once the prerequisite lands).** Ship executes the DAG
wave by wave: `176-S` (`175.099-T` -> `175.100-T` -> `175.101-T`) ships first —
creating the three shared runners — then `157-S`..`169-S` converges wave by wave.
Each member turns its own `^TestU175_<NNN>_` selector green and satisfies its
task-scoped lint verifier; the intermediate-wave verifier enforces exact
remaining-baseline monotonicity against the governed inventory; terminal
convergence at U12 (in `169-S`, RS-W12) requires all mandatory gates green
including the canonical terminal runner's zero-warning `golangci-lint` proof on
both supported surfaces (`scripts/verify-terminal-lint.ps1 -FeatureId 175-F`,
emitting `TERMINAL-LINT-OK:175-F`), the `//nolint` inventory validated, and the
Docline closure evidence artifact committed. The replacement sequence converges
and unblocks `149-S` by an advisory ordering edge (shipped-only readiness
governed by claim-routing policy, per the decomposition decision).

## Plan Hardening

Requires plan hardening: yes

**Hardening trigger (2026-09-20 convergence rewrite; 2026-09-20 corrective
re-hardening).** This amendment changes the plan's execution-readiness contract
(BLOCKED) and documents a shipment-lifecycle claim-semantics dependency and an
operator-supervised bootstrap execution exception. These are contract /
high-blast-radius signals on a shared core surface (`core.ClaimShipment`, the
P-002.6 wave scheduler), so hardening is required before the review re-gate. The
corrective re-hardening additionally (a) corrects the bootstrap to its complete
predecessor chain (earliest prerequisite `155-S`, not `154-S`; bounded two-shipment
set {`155-S`, `154-S`}) and (b) corrects the authorization state — the P-002.6
active-residual halt bypass is UNAPPROVED / BLOCKED, not operator-authorized.

**Refresh (2026-09-26).** The bootstrap / authorization narrative was
re-validated against `main` `c57185c1` after `155-S` shipped: the remaining
bootstrap set is {`154-S`} (chain root), and the `154-S` halt bypass remains
UNAPPROVED / BLOCKED pending explicit operator confirmation. The task
decomposition and manifests are unchanged (101 tasks, each exactly once across
`176-S` and `157-S`..`169-S`). The governed lint inventory and per-task owned
group counts were **not** re-captured in this planning-only refresh; `155-S`
modified `internal/core` Go sources, so drift is caught fail-closed at execution
and a pre-claim drift check is deferred stash `2E0CDF27`.

**Protected invariants.**

- `core.ClaimShipment` all-members-active semantics are UNCHANGED (backward
  compatibility; ~60 test call sites and two production callers depend on it).
  This plan proposes no in-place change to that verb.
- No Go source, script, workflow, or config file is modified by this planning
  cycle. The plan remains a design/decomposition contract only.
- The claim-time dependency guard, shipped-only readiness gate, and governed
  non-claimable disposition are DEFERRED core work (stash `6434A4D7`), not
  executed here.

**Risky actions (ProposedAction / ActionRisk).**

- **ProposedAction:** execute the remaining bootstrap shipment `154-S`/`173-F`
  (`155-S`, the other member of the bounded set {`155-S`, `154-S`}, has shipped)
  via an operator-supervised bootstrap that drives claim-activated members green
  without the strict P-002.6 active-residual halt.
  **ActionRisk:** high (bypasses a safety halt). **Approval:** operator-only,
  per-shipment; **UNAPPROVED — NOT authorized by this plan or by the durable
  lint-retention authorization; the bypass is BLOCKED until the operator
  explicitly confirms that exact waiver for `154-S`** (Ship's set-scoped
  attestation from the `155-S` bootstrap is not treated as approval).
  **Mitigation:** bounded, non-circular set (marker predecessor closure;
  terminated at DAG-root `155-S`, now shipped; `154-S` is the only remaining
  member); the halt being
  waived is a check against *orphaned prior-wave* claims, and the just-claimed
  members are the intended working set; every later shipment uses the normal
  marked-aware scheduler with no exception.
- **ProposedAction:** encode the advisory edge `154-S blocks 176-S`.
  **ActionRisk:** low (advisory backlog edge; queue filtering + direct claim
  bypass mean it is documentation of intent, not a hard runtime gate until
  `6434A4D7` lands). **ActionResult:** edge added and verified
  (`176-S → 154-S (blocks)`).

**Review-gate capability.** Sub-agent dispatch is available in this environment;
the re-gate MUST emit literal `dispatch_mode: multi-agent-dispatch` and a literal
`decision:` marker. If dispatch degrades mid-gate, fall back to a complete
single-agent rubric pass and emit `single-agent-declared-degradation` — never a
partial-coverage PASS.

**Learnings / context consulted.** Prior 2026-09-13 marker deliberation
(`docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md`);
checkpoint `ship-140-s-2026-09-10` (same active-residual mismatch); the
convergence deliberation
`docs/decisions/2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation.md`.

**Unresolved operator decisions still blocking safe execution.** (1) **explicit
operator confirmation of the P-002.6 active-residual halt waiver for `154-S`**
(`155-S` has shipped) — the primary blocker; UNAPPROVED, so the bootstrap is
BLOCKED; (2) external autoharness P-002.6 scheduler marker-consumption (P-017);
(3) whether to formalize the bootstrap exception as a Ship-contract clause
(Ship-owned).

## Plan Review

> **SUPERSEDED (historical).** This attempt-3 record predates the 2026-09-26
> refresh against `main` `c57185c1` (`155-S` shipped). Its references to the
> {`155-S`, `154-S`} bypass, to `155-S` labels, and to `174-F` as deferred are
> historical. The authoritative gate record is the final `## Plan Review`
> section at the end of this document.

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved
operator_authorization_scope: >-
  Authorizes ONLY (a) acceptance of the ADVISORY plan-review verdict on the
  planning / decision / backlog deliverable and (b) pursuit of the shipment-claim
  / wave-scheduler convergence recommendation. EXPLICITLY EXCLUDES the P-002.6
  active-residual halt bypass for the bootstrap set {155-S, 154-S}, which is a
  distinct execution-time Ship action that remains UNAPPROVED / BLOCKED pending
  explicit per-shipment operator approval of that exact waiver.

<!-- plan-review-attempt: 3 -->

**Corrective re-gate (2026-09-20).** This record supersedes the prior attempt-2
ADVISORY (which itself superseded the earlier `## Plan Review Status` PASS record
that predates the readiness rewrite). It re-gates the plan after a narrowly scoped
correction of two gate-blocking defects:

1. **Predecessor-chain correction.** The bootstrap was wrongly started at `154-S`,
   omitting its governed blocking predecessor `155-S`. The complete chain is
   `155-S (in-degree-0 DAG root) → 154-S → 176-S → 157-S..169-S`; the bootstrap set
   is the bounded, non-circular two-shipment set {`155-S`, `154-S`} terminating at
   the root `155-S`.
2. **Authorization correction.** The P-002.6 active-residual halt bypass is
   recorded UNAPPROVED / BLOCKED. The durable authorization
   (`docs/decisions/2026-09-17-baseline-convergence-authorization.md`) grants only
   governed lint-finding retention and confers no scheduler-halt waiver.

Dispatched relevant personas as independent sub-agents over the corrected
contract — Constitution Reviewer, Correctness Reviewer, Agent-Native Parity
Reviewer, Scope Boundary Auditor. Prior-cycle personas (Go Reviewer, Architecture
Strategist, Learnings Researcher) returned no new findings on the correction
surface. Plan hardening was required (`Requires plan hardening: yes`) and is
present with corrective re-hardening (`## Plan Hardening`).

**Prior cycle (historical).** Attempt 1 → FAIL (P0=0, P1=2): the BLOCKED readiness
and the bootstrap exception existed only as human-facing prose, giving no
machine-consumable signal. Remediation added machine signals and re-gated ADVISORY.

**Corrective re-gate findings and remediation (P0=0, P1=0 after fix):**

* Constitution P1 (residual overclaim): the prior review record still called the
  bootstrap procedure "operator-authorized." Corrected — every operative section
  and this record now record the halt bypass as UNAPPROVED / BLOCKED.
* Machine-signal parity: labels `bootstrap-exception`, `convergence-prerequisite`,
  `bootstrap-bypass-unapproved`, and the shared `do-not-claim-until-convergence`
  guard are now on BOTH `154-S` and `155-S` (previously `155-S` carried none),
  plus a "do NOT auto-claim" body comment on each. An autonomous claim-router has a
  symmetric hard-refuse signal for both bootstrap shipments.
* Go P2 (shipped-only gate must not mutate shared status taxonomy): decision item 3
  requires a NEW release-gating predicate that MUST NOT unify
  `IsNoLongerBlockingStatus`/`IsCascadeTerminalStatus` (deliberately divergent).
* Architecture/Scope P2: M2(ii) external admission algorithm reframed as a
  recommendation to the owning workspace with a single-sourced marker-consumption
  contract; `6434A4D7` flagged for module-boundary decomposition; the bootstrap
  procedure is documented as operator-**supervised** with its halt bypass
  UNAPPROVED/BLOCKED (NOT operator-authorized).

**Residual findings — accepted follow-ups (do not block this planning-only
deliverable):**

* [P2] External autoharness P-002.6 marker-consumption is a load-bearing readiness
  node that cannot be an in-repo edge (P-017); represented via labels + comment +
  docs + Orchestrator claim-routing governance rather than tool enforcement. The
  "bootstrap set does not extend past {155-S,154-S}" boundedness result is
  conditional on this external consumption landing and on pre-marker scheduling
  discipline (no unrelated multi-member shipment claimed in the pre-marker window).
* [P2] A first-class blocked/held lifecycle status (deferred `174-F`/`6434A4D7`
  work) would give status-level claim refusal independent of label parsing;
  currently the bootstrap shipments stay `queued` with labels as the enforcement
  surface. Deferred design guidance, not a defect in this deliverable.
* [P2] Ship-agent wave-admission contract (`.github/agents/_ship.agent.md:526-536`)
  lacks a forward-reference to the pending marked-aware convergence — Ship-owned
  agent-contract change, out of this freeze-scope cycle; current halt-on-any-active
  behavior is conservatively safe.
* [P2] `176-S` body prose still documents only the runner-bootstrap gate — mirrored
  via the readiness comment + labels; a full description rewrite is deferred to
  avoid clobbering the large governed manifest description.
* [P3] The machine go-signal (which mutation constitutes operator approval of the
  halt waiver) should be documented in the claim-routing policy; the refuse-signal
  is present, the approve-signal is prose-only.

**Gate rationale.** No P0/P1 remain after the corrective remediation. Remaining P2s
are inherent P-017 constraints, deferred core-cycle guidance, or Ship-owned
agent-contract work — none is a defect in the delivered planning/decision/backlog
artifacts. Per the operator's explicit approval to pursue the convergence
recommendation and proceed through the gates, the ADVISORY verdict on the
**planning deliverable** is authorized (`operator_authorization: approved`,
scoped as above). **This authorization does NOT extend to the P-002.6
active-residual halt bypass**, which remains UNAPPROVED / BLOCKED and requires a
separate, explicit, per-shipment operator approval for `155-S` and `154-S` before
any bootstrap execution. Learnings Researcher returned PASS (consistent with and
building on the 2026-09-13 marker decision; no contradiction).

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: pending
operator_authorization_scope: >-
  When approved, authorizes ONLY acceptance of this ADVISORY verdict on the
  refreshed planning / decision / backlog deliverable. It EXPLICITLY EXCLUDES the
  P-002.6 active-residual halt bypass for `154-S` (the only remaining bootstrap
  shipment; `155-S` shipped), which remains UNAPPROVED / BLOCKED pending explicit
  operator confirmation of that exact per-shipment waiver.

**Refresh re-gate (2026-09-26).** Re-gates this plan after PR #449 was refreshed
against `main` `c57185c1` (merge `63f8e7ea`; `155-S` / `174-F` shipped via PR #450
and closure PR #451). Scope: the refreshed bootstrap / authorization narrative
across this plan, the 2026-09-19 decomposition decision, the 2026-09-20
convergence deliberation, and `.backlogit/queue/154-S.md`, plus staleness caused
by `155-S` shipping. Task decomposition and manifests are unchanged: 101 tasks
(`175.001-T`..`175.101-T`) each appear exactly once across `176-S` and
`157-S`..`169-S`; no ID collides with `main`.

Dispatched six personas as independent sub-agents — Constitution Reviewer, Go
Reviewer, Scope Boundary Auditor, Learnings Researcher (always-on), Architecture
Strategist, and Agent-Native Parity Reviewer (triggered: agent-facing claim
routing). All six returned. Security Lens not triggered (no auth, API, secrets,
or trust-boundary surface). Plan hardening was required
(`Requires plan hardening: yes`) and is present (`## Plan Hardening`, with a
2026-09-26 refresh note).

**Findings and remediation (P0=0; P1=3 raised, all remediated in-scope):**

* [P1, fixed] Structural gate: `## Constitution Check` had no verdict line. Added
  the documented deviations (I lint retention to U12, Task Granularity for U1,
  VIII the UNAPPROVED `154-S` waiver) and `Constitution Check: documented-deviations`.
* [P1, fixed] Factual error in the refresh: the Ship waiver comment was recorded
  as "not found". It exists only as a Ship-authored `comment` event in the local,
  git-ignored `.backlogit/logs/155-S.jsonl`, and no tracked artifact carries it.
  All four files now say so. It is still not treated as operator approval for
  `154-S`, and no approval is claimed.
* [P1, fixed in-scope + deferred] Routing hold: with `155-S` shipped, `154-S` has
  no unshipped predecessor, and the Orchestrator/Ship contracts do not read
  labels. The plan, decision records, and the `154-S` banner now state that
  `154-S` is held only by labels, banner, and operator instruction, and MUST NOT
  be routed by `ship next`, an inferred `ship next`, or a dark-mode scope. The
  contract change (label-aware refusal plus a machine go-signal) is
  Orchestrator/Ship-owned: deferred stash `AF1E5075`.
* [P2, fixed] Stale code anchors (`shipment_lifecycle.go:46-98`,
  `_ship.agent.md:526-535`) now use symbol/section anchors in the live readiness
  sections. The deliberation's decision-time line refs are marked as dated.
* [P2, fixed] The refresh note overclaimed re-validation. It now states that the
  lint inventory and ownership counts were not re-captured.
* [P2, fixed] The attempt-3 record's `{155-S, 154-S}` scope and gate rationale
  were stale. That record is now marked historical and superseded here.
* [P3, fixed] Double-counted "`155-S`, `174-F` and all 45 manifest items" now
  reads "`155-S` and all 45 manifest items — `174-F` + 44 tasks". The
  deliberation's `155-S` waiver statement is attributed to Ship's reconcile
  record. The `176-S` pre-claim check accepts `archived` with
  `archived_status: shipped`. The deliberation's Constitution Check maps the
  safety-mode waiver.

**Copilot review findings on the refreshed heads (P-018, fixed in-scope):**

* [P1, fixed] U1 (`175.001-T`) and U12 (`175.012-T`) claim allow-sets modelled
  a single task transition and omitted the parent shipment manifest. Ship claims
  through `core.ClaimShipment`, which persists the shipment transition plus every
  queued member, so the normal claim footprint would have failed the closed
  clean-tree / `-Phase PreCommit` set-equality check. Both allow-sets now carry
  the full 3-path footprint (`157-S.md` / `169-S.md`, the member task, and
  `hooks_queue.jsonl`). `175-F` is excluded because the claim's parent-status
  cascade is bounded to the manifest member set. The U12 `task-lint-contract`
  JSON changed, so its `task_contract_sha256` pin in `175-F` was recomputed; U1's
  contract JSON is unchanged, so its pin still holds.
* [P2, fixed] The bootstrap contracts (`175.099-T`..`175.101-T`) over-modelled
  the `176-S` claim footprint with `175-F` and cited a stale line anchor. `175-F`
  is removed, so an unexpected feature mutation now fails closed, and the anchor
  is the `core.ClaimShipment` symbol. The change is prose-only, so contract pins
  are unchanged.

**Residual findings — accepted follow-ups (do not block this planning-only
deliverable):**

* [P2] `174-F` shipped a resumable shipment `blocked` status. It is reachable
  only from `active` (active→blocked; confirmed unblock), so it cannot hold a
  *queued* shipment. Do NOT use `shipment block` on `154-S`, because that
  requires a claim, which activates all members. Status-level pre-claim refusal
  remains `6434A4D7`'s governed non-claimable disposition. This corrects the
  attempt-3 residual that called `174-F` deferred.
* [P2] Before the `154-S` bootstrap, re-validate the `173-F` marker contracts
  against the post-`174-F` `ClaimShipment` (journaled rollback; blocked→active
  unblock restoration, which the claim-only marker seam would not mark). Also
  confirm the installed binary includes `2c8759c3` or later. Deferred stash
  `C29EBEE5`.
* [P2] Before `176-S`/`157-S` is claimed, drift-check the governed lint inventory
  (`bbb20803…`, 497 occurrences) and per-task ownership against the execution
  base. Drift is caught fail-closed by the verifiers. Deferred stash `2E0CDF27`.
* [P2] Label-aware Orchestrator/Ship claim refusal and a machine go-signal for
  operator waiver confirmation. Deferred stash `AF1E5075`.
* [P2] The `GOOS=linux go test` harness commands in `175.095-T`..`175.098-T`
  cannot execute on a Windows Ship host. The choice between a native Linux
  runner and a compile-only cross-GOOS split needs deliberation. Deferred stash
  `24D693E1` (Copilot, 2026-09-27).
* [Escalated; not a plan finding] Commit `46e17ee0` moved `168.001-T` to
  `.backlogit/archive/` with `status: done` and `commit: c976315f`, but `main`
  still has it `queued`. That commit exists only on
  `feat/149-s-trust-anchor-verification-key-lifecycle`; it is not on `main`.
  This is Ship-owned lifecycle state. Stage leaves it for an Orchestrator/operator
  decision rather than normalizing or reverting it.
* [P3] The attempt-3 note calls `IsNoLongerBlockingStatus` /
  `IsCascadeTerminalStatus` "deliberately divergent". On `main` they share one
  cascade set; the divergent pair is that set versus `IsReleasableStatus`. The
  intent (do not mutate the shared taxonomy) is unchanged.
* [P3] The merge renumbered the branch's 16 `hooks_queue.jsonl` events to seq
  `3151`..`3166` (after `main`'s `3150`); no tracked artifact cites the old
  values. The stash union added only `6434A4D7`, which is not in `main`'s stash
  archive.

**Gate rationale.** No P0 or P1 remain after in-scope remediation. The remaining
P2s are recorded as deferred follow-ups (`AF1E5075`, `C29EBEE5`, `2E0CDF27`,
`6434A4D7`) on contract surfaces outside this planning-only refresh (P-021 C1).
Decision: ADVISORY. `operator_authorization` stays `pending` until the operator
explicitly accepts this verdict. That acceptance never extends to the `154-S`
halt bypass, which remains UNAPPROVED / BLOCKED.
