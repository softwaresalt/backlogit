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
> the prerequisite task `175.099-T` (see "## Runner-Bootstrap Prerequisite"),
> giving a feature executable-task total of **99**.
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

- Complete baseline convergence: every one of the 497 governed lint identities in
  the supported-platform union is owned by exactly one finding-remediation task
  and resolved before terminal convergence.
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

Beyond this 98-member remediation sub-DAG, the feature carries **one
runner-bootstrap prerequisite member**, `175.099-T`, that owns creation of the
three shared lint runners and gates the sub-DAG. Counting it, the feature's
executable-task total is **99** (98 remediation sub-DAG + 1 prerequisite).
`175.099-T` is deliberately outside the 98-member sub-DAG topology and is
described in "## Runner-Bootstrap Prerequisite" below.

## Runner-Bootstrap Prerequisite (RS-W(-1))

Every baseline-remediation member's task-lint contract invokes the canonical short
runner `scripts/verify-task-lint.ps1`, and the feature additionally requires the
intermediate-wave runner `scripts/verify-baseline-lint.ps1` and the terminal
runner `scripts/verify-terminal-lint.ps1`. None of those three runner scripts
exists on this branch, and no `175.*` remediation member owns creating them.
`175.099-T` (`runner-bootstrap`) closes that gap: it OWNS creation of the three
`scripts/verify-*.ps1` runners and ships alone in the prerequisite shipment
`176-S` (RS-W(-1)). The prerequisite gates the replacement sequence via
`157-S depends_on 176-S` (shipment edge) and `175.001-T depends_on 175.099-T`
(task edge), making `175.099-T` the in-degree-zero source of the FULL feature
graph while U1 remains the source of the 98-member remediation sub-DAG. `176-S`
owns zero baseline findings and is not an intermediate-wave verifier target.

Because `175.099-T` CREATES the shared runners, its task-lint GATE MUST NOT be a
direct invocation of `scripts/verify-task-lint.ps1` (a circular self-dependency);
it is the owned Go harness `tests/runner_bootstrap_175_099_test.go`
(`TestU175_099_RunnerBootstrap`), which COMPILES and whose targeted test EXECUTES
under existing main-branch tooling before any runner exists — RED (nonzero exit)
before the runners exist, GREEN after — statically proving each runner exists, is
tracked, is non-empty, parses cleanly under the built-in PowerShell AST parser,
and declares its required `param(...)` surface, and — after the runners exist —
behaviorally exercising each runner against harness-authored deterministic fixtures
for its success and failure/fail-closed contracts (non-vacuous). No source,
scripts, or tests are implemented in this planning PR; `175.099-T` declares the
ownership Ship executes later. Full contract: `.backlogit/queue/175.099-T.md`.

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
4. **Matches owned identities by ordinal exact identity.** Each remediation task
   matches its exact owned `path|line|column|linter|message` finding identity set
   (1-16 identities over 1-2 owned files/packages) by ordinal set equality. The
   verifier fails if any owned finding remains, and emits its success marker only
   when zero owned findings are observed. `Issues: []` counts as success only
   after target participation is proven.

Each member binds a stable `task_contract_sha256` computed over its canonical
task-lint-contract JSON, surviving the queue-to-archive lifecycle. The
`task_lint_cmd` is **task-scoped only**. The full repository `golangci-lint run`
is deferred to mandatory terminal convergence and is never used as a per-task
proxy. Same-wave sibling harnesses may remain red until wave convergence; no
later-wave harness or lint scaffolding is created early.

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
  allowlist `.backlogit/queue/175.012-T.md` and `.backlogit/hooks_queue.jsonl`;
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
  `175.099-T` (shipment `176-S`, RS-W(-1)) is the in-degree-zero source of the
  full feature graph via `175.001-T depends_on 175.099-T` (task edge) and
  `157-S depends_on 176-S` (shipment edge). The 98-member sub-DAG's internal
  source remains U1; `175.099-T` sits ahead of it as a pre-DAG bootstrap.
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
prerequisite shipment `176-S` carrying `175.099-T`, gating the replacement
sequence `157-S`..`169-S`, superseding the abandoned single shipment `156-S`); the
declared `supported_goos` (`windows`, `linux`); the `member_scope` of all 98
remediation sub-DAG members with their live role and the lowercase 64-hex
`task_contract_sha256` of each final committed canonical task-lint-contract JSON
(the runner-bootstrap prerequisite `175.099-T` is recorded separately in the
`packaging.prerequisite_*` fields, outside this remediation `member_scope`); the
committed machine inventory (`docs/decisions/baseline-lint-inventory.json`, SHA-256
`6cd1d3468d6fe334b165763b33dc03aacdcfad04d369dda3c88057459b998f3a`, 497 identities);
the exact intermediate-wave verifier command; the canonical terminal command; and
the shipment-scoped operator authorization reference.

Intermediate-wave verifier (exact remaining-baseline monotonicity):

```
pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 -Inventory docs/decisions/baseline-lint-inventory.json -InventorySha256 6cd1d3468d6fe334b165763b33dc03aacdcfad04d369dda3c88057459b998f3a -Shipment <replacement-shipment-id> -FeatureId 175-F -TerminalTask 175.012-T
```

> Packaging note (2026-09-19): under the accepted decomposition, the
> intermediate-wave verifier is invoked once per active replacement shipment in
> RS-W00..RS-W12 order (`157-S`..`169-S`), substituting `-Shipment` with that
> wave's replacement shipment id; the superseded `156-S` is never used as the
> `-Shipment` argument.

Terminal command (zero-warning, mandatory at U12, both supported surfaces):

```
pwsh -NoProfile -File scripts/verify-terminal-lint.ps1 -FeatureId 175-F
```

## Harness-Exempt Set

Closed, single member: U12 (`175.012-T`). Every other member (U1 + 94
remediation + U40 + U14 + the runner-bootstrap prerequisite `175.099-T`) is a
normal harness-required task that exclusively owns exactly one deterministic
`*_test.go` harness path with a genuine targeted red-before-green
`TestU175_<NNN>_` selector. The prerequisite `175.099-T` is harness-required with
a self-contained bootstrap harness (`tests/runner_bootstrap_175_099_test.go`,
`TestU175_099_RunnerBootstrap`) whose gate is the Go harness itself — never the
runner it creates — RED before the three runners exist and GREEN after. Ship's
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

- **Task-lint false green** (default caps suppress same-message findings):
  mitigated by uncapped flags and schema-before-filter validation.
- **Target vacuity** (linting a file that does not participate in analysis):
  mitigated by the go-list/package-membership target-vacuity guard, with a host
  guard for Windows-only and Linux-only files.
- **Same-file concurrent mutation**: mitigated by slice ordering and the
  never-co-wave constraint.
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

## Plan Review Status

dispatch_mode: multi-agent-dispatch
decision: PASS

Reviewed across Constitution, Correctness, Template
Integrity, Scope Boundary, Schema/CLI/Docs coupling, CI coupling, and
Maintainability personas, with the policy-sensitive harness/planning coupling
additionally escalated to the installed adversarial review and post-remediation
re-review. Residual P0=0, P1=0.

## Runtime Verification and Closure

Ship executes the DAG wave by wave: the prerequisite shipment `176-S`
(`175.099-T`) ships first — creating the three shared runners — then the
replacement sequence `157-S`..`169-S` converges wave by wave. Each member turns
its own `^TestU175_<NNN>_` selector green and satisfies its task-scoped lint
verifier; the intermediate-wave verifier enforces exact remaining-baseline
monotonicity against the governed inventory; terminal convergence at U12 (in
`169-S`, RS-W12) requires all mandatory gates green including the canonical terminal
runner's zero-warning `golangci-lint` proof on both supported surfaces
(`scripts/verify-terminal-lint.ps1 -FeatureId 175-F`, emitting `TERMINAL-LINT-OK:175-F`),
the `//nolint` inventory
validated, and the Docline closure evidence artifact committed. The replacement
sequence converges and unblocks `149-S` by an advisory ordering edge (shipped-only
readiness governed by claim-routing policy, per the decomposition decision).
