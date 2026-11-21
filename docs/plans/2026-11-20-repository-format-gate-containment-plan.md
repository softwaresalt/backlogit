---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for Group 1B Part 2 (DF533572 / 079-DL -> 221-F / 222-S): scope repository formatting gates to Go source directories (cmd, internal, tests, scripts) to prevent .copilot session-cache and logs/ traversal while reconciling .gsd/workspace-profile.yaml and .gsd/harness-manifest.yaml.'
doc_type: plan
schema_version: "1.0"
source: docs/plans/2026-11-20-repository-format-gate-containment-plan.md
title: 'Plan: Repository Formatting-Gate Source-Tree Scoping and Session-Cache Containment (221-F / 222-S)'
---

## Problem Frame

As established in [`079-DL`](../decisions/2026-11-01-repository-format-gate-session-cache-containment-deliberation.md) and [the post-219-S stabilization deliberation](../decisions/2026-11-20-post-219s-stabilization-deliberation.md) (`DF533572`), running literal `gofmt -l .` in the workspace root walks gitignored `.copilot/session-state/.../gomodcache/.../src/cmd/compile/internal/syntax/testdata/issue*.go` (intentionally invalid Go compiler syntax error fixtures inside workspace-bound `COPILOT_HOME=.copilot`) and `logs/` proof trees, emitting 25,144 lines and exiting 2 even when 100% of repository Go files are formatting-clean.

Because Level 1 workspace toolchain pinning (`214-S`, `VerifyLauncherScripts` in `internal/toolchain/launcher.go`) requires `start.ps1` and `start.sh` to bind `COPILOT_HOME` to `<workspace>/.copilot`, `.copilot/` resides inside the workspace root by design and cannot be relocated outside the workspace. Meanwhile, 100% of tracked Go files (`1,420` files across `git ls-files '*.go'`) live inside the four top-level Go source directories `cmd`, `internal`, `tests`, and `scripts`. Scoping the repository formatting gate from `gofmt -l .` / `gofmt -w .` to `gofmt -l cmd internal tests scripts` / `gofmt -w cmd internal tests scripts` across `Makefile`, `make.ps1`, shared harness guidance/skills/agents/policies, `.gsd/workspace-profile.yaml`, and `.gsd/harness-manifest.yaml` (backed by a dynamic `git ls-files '*.go'` completeness guard) permanently resolves `DF533572`.

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Update `Makefile` `fmt` target to `gofmt -l cmd internal tests scripts` and guard with a `redhonesty`-safe tracked/exported Go directory completeness check failing closed on root-level `*.go` or ungrouped top-level Go directories | `DF533572` / `079-DL` | Unit U1 (`221.001-T`, Wave 1) | `TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo` |
| R2: Update `make.ps1` `all` and `fmt` targets from `gofmt -l .` to `gofmt -l cmd internal tests scripts` | `DF533572` / `079-DL` | Unit U2 (`221.002-T`, Wave 2) | `TestU2_MakePS1FormatGateScopesSourceDirectories` |
| R9: Update `.gsd/workspace-profile.yaml` `format.{command,check_command,fix_command}` to `gofmt -l cmd internal tests scripts` and `gofmt -w cmd internal tests scripts` | `DF533572` / `079-DL` | Unit U9 (`221.009-T`, Wave 3) | `TestU9_WorkspaceProfileFormatGateScopesSourceDirectories` |
| R3–R8: Update shared harness guidance, constitution (with `1.1.1 (PATCH)` amendment record), `go.instructions.md` (including `goimports -l cmd internal tests scripts`), skills (`build-feature` and `fix-ci` — both `gofmt -l` and `gofmt -w`), agents (`_ship`, `go-engineer`), and `workflow-policies.md` (`v1.33.1` amendment record) from `gofmt -l .` / `gofmt -w .` to `gofmt -l cmd internal tests scripts` / `gofmt -w cmd internal tests scripts` | `DF533572` / `079-DL` | Units U3–U8 (`221.003-T`..`221.008-T`, Wave 4) | P-002.3 `EXEMPT_VERIFY_OK:221.003-T`..`221.008-T` |
| R10: Reconcile `.gsd/harness-manifest.yaml` (`FORMAT_COMMAND`, `HARNESS_ENFORCED_SUMMARY`, `profile_hash`, `drift_allowed: true` + amended/inserted `drift_reason` on `constitution.instructions.md` and `go.instructions.md`, and LF-normalized SHA-256 `checksum`s for all 9 modified manifest-tracked artifacts while preserving `TestU180006`, `TestU12`, `TestU12_GSDModelRoutingManifestReconciliation`, `TestUSR6`, `TestUCXS17`, `TestU19R3`, and `TestGovernedFullSuiteBudget_*`) | `DF533572` / `079-DL` | Unit U10 (`221.010-T`, Wave 4 — LAST in Wave 4 manifest order) | `TestU10_HarnessManifestFormatGateAndChecksumsReconciled`, `TestU180006_ManifestChecksumsCurrent`, `gsd verify-workspace --workspace . --json` |

## Closed P-002.1 Harness-Exempt Set

The closed `harness-exempt` (`docs-only`) set for Feature `221-F` / Shipment `222-S` is:

* `221.003-T` (`U3`), `221.004-T` (`U4`), `221.005-T` (`U5`), `221.006-T` (`U6`), `221.007-T` (`U7`), `221.008-T` (`U8`).

`221.001-T` (`U1`), `221.002-T` (`U2`), `221.009-T` (`U9`), and `221.010-T` (`U10`) are `harness-ready` (`test-first`).

## Implementation Units

### Unit U1: Scope `Makefile` `fmt` target and add tracked-Go directory completeness guard (`221.001-T`, Wave 1)

* **Domain**: `build` / `tests` (`harness-ready`, `test-first`; 2 files, 1 test function, 2 scenarios)
* **Size / Complexity**: `S` / `low`
* **Execution posture**: `test-first`
* **Affected files (2)**:
  * `Makefile`
  * `tests/integration/format_gate_containment_test.go`
* **Changes needed**:
  1. In `Makefile` (`fmt:` target): replace `gofmt -l .` with `gofmt -l cmd internal tests scripts`.
  2. In `tests/integration/format_gate_containment_test.go` (`package integration_test`): add `TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo` asserting:
     * (a) `Makefile` `fmt` target invokes `gofmt -l cmd internal tests scripts` and contains zero unscoped `gofmt -l .` lines; and
     * (b) **`redhonesty`-safe Go source-directory completeness check**: inspect `_, statErr := os.Stat(filepath.Join(repoRoot, ".git"))` — when `statErr == nil` (normal `go test` and CI), run `git -C <repoRoot> ls-files -z -- '*.go'`; when `errors.Is(statErr, os.ErrNotExist)` (inside `redhonesty`'s `.git`-less `git archive` export directory `.backlogit/runtime/redhonesty/<subdir>`, which contains exclusively committed blobs), walk `repoRoot` via `filepath.WalkDir` skipping dot-prefixed directories (`d.IsDir() && d.Name() != "." && strings.HasPrefix(d.Name(), ".")` -> `filepath.SkipDir`) and collecting all `.go` files (failing closed via `require.NoError(t, statErr)` on any other stat error). Assert `require.NotEmpty(t, goFiles)`, fail closed if any `.go` path is at repository root (`filepath.Dir(rel) == "."`), and assert every top-level directory containing `.go` files belongs to `{cmd, internal, tests, scripts}`.
* **Scoped verification command (`P-002.6` `harness_cmd`)**: `go test -timeout=30m -count=1 -v -run '^TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo$' ./tests/integration`

### Unit U2: Scope `make.ps1` `all` and `fmt` targets to Go source directories (`221.002-T`, Wave 2)

* **Domain**: `build` / `tests` (`harness-ready`, `test-first`; 2 files, 1 test function, 2 scenarios)
* **Size / Complexity**: `S` / `low`
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on `221.001-T` (`U1`)
* **Affected files (2)**:
  * `make.ps1`
  * `tests/integration/format_gate_containment_test.go`
* **Changes needed**:
  1. In `make.ps1`: replace both `$bad = gofmt -l .` occurrences (`all` and `fmt` targets) with `$bad = gofmt -l cmd internal tests scripts`.
  2. In `tests/integration/format_gate_containment_test.go`: add `TestU2_MakePS1FormatGateScopesSourceDirectories` asserting both `all` and `fmt` targets in `make.ps1` run `gofmt -l cmd internal tests scripts` and contain zero `gofmt -l .` occurrences, and inside a short-named `t.TempDir()` probe seeding all four directories `cmd/`, `internal/`, `tests/`, and `scripts/` (with formatted BOM-free LF `.go` files) plus `.copilot/invalid.go` (intentionally invalid Go syntax), verify `gofmt -l cmd internal tests scripts` (and `pwsh -NoProfile -NonInteractive -File make.ps1 fmt` when `pwsh` is present, without top-level `t.Skip`) exits `0` with `0` lines.
* **Scoped verification command (`P-002.6` `harness_cmd`)**: `go test -timeout=30m -count=1 -v -run '^TestU2_MakePS1FormatGateScopesSourceDirectories$' ./tests/integration`
* **`green_regression_cmds` (`P-002.6` `green-regression-contract`)**: `go test -timeout=30m -count=1 -v -run '^TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo$' ./tests/integration`

### Unit U9: Update `.gsd/workspace-profile.yaml` format commands (`221.009-T`, Wave 3)

* **Domain**: `config` / `tests` (`harness-ready`, `test-first`; 2 files, 1 test function, 2 scenarios)
* **Size / Complexity**: `XS` / `low`
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on `221.002-T` (`U2`) (scheduled alone in **Wave 3** ahead of Wave 4 so each wave has at most one `harness-ready` writer on `tests/integration/format_gate_containment_test.go`, preventing `redhonesty` `RejectHeadNotGreen` between `U9` and `U10`, and giving `U10` an explicit `blocks` edge on `U9`'s committed `.gsd/workspace-profile.yaml`!)
* **Affected files (2)**:
  * `.gsd/workspace-profile.yaml`
  * `tests/integration/format_gate_containment_test.go`
* **Changes needed**:
  1. In `.gsd/workspace-profile.yaml`: update `format.command: "gofmt -l cmd internal tests scripts"`, `format.check_command: "gofmt -l cmd internal tests scripts"`, and `format.fix_command: "gofmt -w cmd internal tests scripts"` (with a brief inline comment noting `scripts/` contains tracked Go utility code).
  2. In `tests/integration/format_gate_containment_test.go`: add `TestU9_WorkspaceProfileFormatGateScopesSourceDirectories` asserting recursive `yaml.Node` key uniqueness on `.gsd/workspace-profile.yaml` and verifying `format.command`, `format.check_command`, and `format.fix_command` equal the scoped `cmd internal tests scripts` commands while preserving `test.command == "go test -timeout=30m ./..."` and `test.runner == "go test"`.
* **Scoped verification command (`P-002.6` `harness_cmd`)**: `go test -timeout=30m -count=1 -v -run '^TestU9_WorkspaceProfileFormatGateScopesSourceDirectories$' ./tests/integration`
* **`green_regression_cmds` (`P-002.6` `green-regression-contract`)**: `go test -timeout=30m -count=1 -v -run '^(TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo|TestU2_MakePS1FormatGateScopesSourceDirectories|TestGovernedFullSuiteBudget_RenderInputs)$' ./tests/integration`

### Units U3–U8: Update shared harness guidance, constitution, skills, agents, and policies (`221.003-T`..`221.008-T`, Wave 4)

All six `docs-only` tasks depend on `221.009-T` (`U9`) so they co-schedule in **Wave 4 (`ready_4`)** ahead of `U10` in manifest order (preserving `TestU180006_ManifestChecksumsCurrent` and `220-S`'s `TestU12_GSDModelRoutingManifestReconciliation` across Waves 1, 2, 3, and 4!):

* **Unit U3 (`221.003-T`, 2 files)**: `AGENTS.md`, `.github/copilot-instructions.md` — replace `gofmt -l .` with `gofmt -l cmd internal tests scripts` (preserving `073-DL` `go test -timeout=30m ./...` anchors pinned by `TestGovernedFullSuiteBudget_Guidance`). (`EXEMPT_VERIFY_OK:221.003-T`)
* **Unit U4 (`221.004-T`, 2 files)**: `.github/instructions/constitution.instructions.md`, `.github/instructions/go.instructions.md` — replace `gofmt -l .` with `gofmt -l cmd internal tests scripts` in Technical Constraints, Quality Gates, General Rules, and Commands, replace `goimports -l .` with `goimports -l cmd internal tests scripts` in `go.instructions.md`, and record **Constitution Amendment `1.1.1` (PATCH)** in `constitution.instructions.md` (`Amendment log` + `Sync impact report (1.1.1)` enumerating all 15 updated surfaces + `**Version**: 1.1.1`) documenting that scoping `gofmt -l` to repository Go source roots (`cmd internal tests scripts`) preserves 100% repository Go formatting enforcement while excluding workspace-bound `.copilot` session caches (`079-DL` / `DF533572`). (`EXEMPT_VERIFY_OK:221.004-T`)
* **Unit U5 (`221.005-T`, 2 files)**: `.github/agents/_ship.agent.md`, `.github/agents/subagents/go-engineer.agent.md` — replace `gofmt -l .` with `gofmt -l cmd internal tests scripts` (preserving all `TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery` and `TestWorkspaceAgentsGrantFullBacklogitToolSurface` contracts). (`EXEMPT_VERIFY_OK:221.005-T`)
* **Unit U6 (`221.006-T`, 2 files)**: `.github/skills/build-feature/SKILL.md`, `.github/skills/fix-ci/SKILL.md` — replace **every** `gofmt -l .` with `gofmt -l cmd internal tests scripts` AND replace **every** `gofmt -w .` in **both** `build-feature/SKILL.md` (including Post-Loop Quality Gates auto-fix) and `fix-ci/SKILL.md` with `gofmt -w cmd internal tests scripts` (preserving prose prohibitions on bare `gofmt -w`), so neither `build-feature` nor `fix-ci` ever traverses or mutates `.copilot/` session caches. (`EXEMPT_VERIFY_OK:221.006-T`)
* **Unit U7 (`221.007-T`, 2 files)**: `.github/policies/workflow-policies.md`, `.github/instructions/copilot-code-review.instructions.md` — replace `gofmt -l .` with `gofmt -l cmd internal tests scripts` (bumping `workflow-policies.md` `**Version**: 1.33.1` and appending a `1.33.1` row in its `## Amendment Log`, while preserving `go test -timeout=30m ./...` anchors pinned by `TestGovernedFullSuiteBudget_RenderInputs`). (`EXEMPT_VERIFY_OK:221.007-T`)
* **Unit U8 (`221.008-T`, 1 file)**: `.github/instructions/github-pr-automation.instructions.md` — replace `gofmt -l .` with `gofmt -l cmd internal tests scripts`. (`EXEMPT_VERIFY_OK:221.008-T`)

*(Note: Consumer-facing `plugin/skills/{build-feature,fix-ci}/SKILL.md` intentionally retains generic `gofmt -l .` / `gofmt -w .` because external Go repositories installing the Copilot plugin bundle do not necessarily use `cmd internal tests scripts` top-level directories; historical `note`/`drift_reason` strings inside `.gsd/harness-manifest.yaml` and `governed_budget_render_inputs_test.go` self-test strings are likewise preserved.)*

### Unit U10: Reconcile `.gsd/harness-manifest.yaml` format command, `profile_hash`, drift records, and LF checksums (`221.010-T`, Wave 4 — 7th / LAST in Wave 4 manifest order)

* **Domain**: `config` / `tests` (`harness-ready`, `test-first`; 2 files, 1 test function, 3 scenarios)
* **Size / Complexity**: `M` / `medium`
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on `221.009-T` (`U9`) (co-scheduled LAST in Wave 4 manifest order after `U3`–`U8`)
* **Affected files (2)**:
  * `.gsd/harness-manifest.yaml`
  * `tests/integration/format_gate_containment_test.go`
* **Changes needed**:
  1. In `.gsd/harness-manifest.yaml` (BOM-free UTF-8, LF `\n` line endings, zero duplicate YAML keys):
     * Confirm `221.003-T`..`221.008-T` are `done`, update `profile_hash` to match the LF-normalized SHA-256 of `.gsd/workspace-profile.yaml` (`U9`), and update `tuned_at`.
     * Update `variables_used.FORMAT_COMMAND` to `"gofmt -l cmd internal tests scripts"` and update `variables_used.HARNESS_ENFORCED_SUMMARY`'s first bullet from ``(`gofmt -l .`, `golangci-lint run`, `go vet ./...`)`` to ``(`gofmt -l cmd internal tests scripts`, `golangci-lint run`, `go vet ./...`)`` (keeping the `go test -timeout=30m ./...` bullet pinned by `TestGovernedFullSuiteBudget_RenderInputs` intact!).
     * On `.github/instructions/constitution.instructions.md` and `.github/instructions/go.instructions.md`, set `drift_allowed: true` and distinguish **amend vs insert** on `drift_reason` (merging into any existing `drift_reason` rather than inserting a duplicate YAML key): `"DF533572 / 079-DL: scoped gofmt -l cmd internal tests scripts (and Constitution v1.1.1 amendment record) to exclude workspace-bound .copilot session caches while enforcing 100% repository Go source formatting. Do not auto-revert."`.
     * Rotate LF-normalized SHA-256 `checksum`s on all 9 modified manifest-tracked artifacts (`AGENTS.md`, `.github/copilot-instructions.md`, `.github/instructions/constitution.instructions.md`, `.github/instructions/go.instructions.md`, `.github/agents/_ship.agent.md`, `.github/agents/subagents/go-engineer.agent.md`, `.github/policies/workflow-policies.md`, `.github/instructions/copilot-code-review.instructions.md`, `.github/instructions/github-pr-automation.instructions.md`), while **preserving** every existing `drift_allowed: true`, `drift_reason` (`"Do not auto-revert."`, `227A2930`, `B83081F5`, `75E02C17`..`F05661B1`, `073-DL rev22`, `PR #400`), and historical `note`!
  2. In `tests/integration/format_gate_containment_test.go`: add `TestU10_HarnessManifestFormatGateAndChecksumsReconciled` asserting recursive `yaml.Node` key uniqueness on `.gsd/harness-manifest.yaml`, `profile_hash == lfSHA256Hex(.gsd/workspace-profile.yaml)`, `FORMAT_COMMAND == "gofmt -l cmd internal tests scripts"`, zero live `gofmt -l .` AND zero live `gofmt -w .` commands across the exact **14** workspace-governed files (`Makefile`, `make.ps1`, `AGENTS.md`, `.github/copilot-instructions.md`, `constitution.instructions.md`, `go.instructions.md`, `_ship.agent.md`, `go-engineer.agent.md`, `.github/skills/build-feature/SKILL.md`, `.github/skills/fix-ci/SKILL.md`, `workflow-policies.md`, `copilot-code-review.instructions.md`, `github-pr-automation.instructions.md`, and `.gsd/workspace-profile.yaml`), and 100% LF SHA-256 checksum parity across `.gsd/harness-manifest.yaml`.
* **Scoped verification command (`P-002.6` `harness_cmd`)**: `go test -timeout=30m -count=1 -v -run '^TestU10_HarnessManifestFormatGateAndChecksumsReconciled$' ./tests/integration`
* **`green_regression_cmds` (`P-002.6` `green-regression-contract`)**: `go test -timeout=30m -count=1 -v -run '^(TestU180006_ManifestChecksumsCurrent|TestU12_HarnessManifestNamespaceReconciliation|TestU12_GSDModelRoutingManifestReconciliation|TestUSR6_HarnessManifestDriftRecords|TestUCXS17_RefreshedDriftRecords|TestU19R3_ManifestBudgetReconciliation|TestGovernedFullSuiteBudget_.*)$' ./tests/integration`

## Dependency Graph

* **Shipment sequencing**: Shipment `222-S` (`221-F`) `blocks` on Shipment `221-S` (`220-F`), which `blocks` on Shipment `220-S` (`219-F`).
* **Intra-shipment waves**:
  * **Wave 1**: `221.001-T` (`U1`: `Makefile` + `TestU1_*`)
  * **Wave 2**: `221.002-T` (`U2`: `make.ps1` + `TestU2_*`; `blocks` on `221.001-T`)
  * **Wave 3**: `221.009-T` (`U9`: `.gsd/workspace-profile.yaml` + `TestU9_*`; `blocks` on `221.002-T`)
  * **Wave 4**: `221.003-T`..`221.008-T` and `221.010-T` (`U3`..`U8` `docs-only` + `U10` `.gsd/harness-manifest.yaml` + `TestU10_*`; each `blocks` on `221.009-T`, ordered `221.003-T`..`221.008-T` -> `221.010-T` in `222-S` manifest so each wave has at most one `harness-ready` writer on `format_gate_containment_test.go` and all Markdown and profile edits complete before `221.010-T` reconciles `.gsd/harness-manifest.yaml` prior to Wave 4 Step 4.6 convergence).

## Decisions and Rationale

* **Source-tree scoping (`cmd internal tests scripts`) over `.copilot` relocation**: Preserves Level 1 toolchain `COPILOT_HOME=.copilot` workspace binding (`214-S`), formats/checks both tracked and untracked new `.go` files during `build-feature` pre-commit loops, and enforces future directory completeness in `TestU1_*` across both live `.git` checkouts and `redhonesty` `git archive` exports.
* **Constitution v1.1.1 PATCH and Workflow-Policies v1.33.1 amendments**: `constitution.instructions.md` Governance and `workflow-policies.md` require version bumps and amendment logs whenever updated; including Amendment `1.1.1` in `U4` and `1.33.1` in `U7` satisfies governance integrity cleanly.

## Risks and Caveats

* **Preserving `073-DL` governed budget anchors**: Edits to `AGENTS.md`, `.github/copilot-instructions.md`, `constitution.instructions.md`, `go.instructions.md`, `workflow-policies.md`, `copilot-code-review.instructions.md`, `workspace-profile.yaml`, and `harness-manifest.yaml` must touch only `gofmt`/`goimports` lines and keep `go test -timeout=30m ./...` lines untouched so all 6 `TestGovernedFullSuiteBudget_*` suites stay green.
* **Literal-pathspec staging & foreign worktree paths**: Because `U9` and `U10` modify `.gsd/workspace-profile.yaml` and `.gsd/harness-manifest.yaml`, use explicit literal pathspecs (never directory-wide `git add .gsd/`) and never stage, commit, or list `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, `.gsd/config.yaml`, or `.kbdb/` in any task `## File Budget`. Write all modified YAML/Markdown files as BOM-free UTF-8 with LF (`\n`) line endings.

## Constitution Check

* **I. Safety-First Go**: `pass` — Integration tests use Go 1.24 + `testify` + `gopkg.in/yaml.v3` with zero `unsafe`.
* **II. Test-First Development (NON-NEGOTIABLE)**: `pass` — `U1`, `U2`, `U9`, and `U10` carry compiling, failing Go integration harnesses (`TestU1_*`, `TestU2_*`, `TestU9_*`, `TestU10_*`), and `U3`–`U8` carry complete fail-before-deliverable `P-002.1`/`P-002.3` `docs-only` contracts.
* **III. Workspace Isolation and Security Boundaries**: `pass` — Prevents `gofmt -l` and `gofmt -w` from traversing or mutating `.copilot/` session caches while keeping `COPILOT_HOME` workspace-contained.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE)**: `pass` — All operations and temp test probes stay inside `t.TempDir()` and the workspace root.
* **V. Structured Observability**: `pass` — Named test scenarios, `EXEMPT_VERIFY_OK` markers, and Constitution v1.1.1 amendment log.
* **VI. Single Responsibility**: `pass` — Every unit modifies `1` or `2` files (`< 3 files`) in a single domain; zero new dependencies.
* **VII. Destructive Command Approval (NON-NEGOTIABLE)**: `pass` — Zero deletion or mutation of `.copilot/session-state/` or `logs/`; all `exempt_verification_command` probes are strictly read-only.
* **VIII. Explicit Safety Modes**: `pass` — Wave 3 (`U9`) + Wave 4 (`U3`–`U8`, `U10`) scheduling protects `redhonesty`, `TestU180006_ManifestChecksumsCurrent`, `TestU12_GSDModelRoutingManifestReconciliation`, and `gsd verify-workspace` across wave boundaries.
* **IX. Git-Friendly Persistence**: `pass` — LF-normalized edits and unique YAML keys.
* **X. Agent Context Efficiency**: `pass` — Eliminates 25,144-line `.copilot` `gofmt` output floods during Ship quality gates.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)**: `pass` — Merges via `--merge` commit (`P-009`).

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — Amends `constitution.instructions.md` (`v1.1.1`), `workflow-policies.md`, `Makefile`, `make.ps1`, `.gsd/workspace-profile.yaml`, and `.gsd/harness-manifest.yaml`.
* **security, auth, permission, or compliance-sensitive behavior**: `absent` — Preserves Level 1 `COPILOT_HOME` containment untouched.
* **migration, backfill, destructive data/config action, or irreversible step**: `present` — Rotates `.gsd/harness-manifest.yaml` `profile_hash`, `FORMAT_COMMAND`, `drift_allowed`, and 9 artifact checksums.
* **external integration, operator checkpoint, or external dependency**: `absent` — None.
* **high runtime, rollout, or rollback risk**: `present` — Wave-boundary checksum coupling with `TestU180006_ManifestChecksumsCurrent` and `gsd verify-workspace`.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime/gate surfaces**: `Makefile` (`make fmt`), `make.ps1` (`.\make.ps1 fmt`), Ship / `build-feature` / `fix-ci` formatting gates (`gofmt -l cmd internal tests scripts`), `.gsd/workspace-profile.yaml`, and `.gsd/harness-manifest.yaml`.
* **Required verification**:
  1. `gofmt -l cmd internal tests scripts` exits `0` with `0` lines even when `.copilot/` contains session caches.
  2. `gsd verify-workspace --workspace . --json` reports `0` checksum mismatches, `0` blockers, and `strict_schema_blockers: []`.
  3. Unfiltered `go test -timeout=30m ./...` (including `TestU1_MakefileFormatGate*`, `TestU2_MakePS1FormatGate*`, `TestU9_WorkspaceProfileFormatGate*`, `TestU10_HarnessManifestFormatGate*`, `TestU180006_ManifestChecksumsCurrent`, `TestU12_HarnessManifestNamespaceReconciliation`, and all 6 `TestGovernedFullSuiteBudget_*` suites), `scripts/md-lint.ps1`, and `go run ./cmd/backlogit docs lint` pass.
* **Operational closure**: `docs/closure/222-S-221-F-post-merge-closure.md` recording format-gate containment verification, `_Ship` owner, 7-day validation window, and scoped revert procedure.

## Plan Hardening

* **Hardening required**: Yes — P-006 triggered by Constitution v1.1.1 amendment, repository-wide format-gate command change, and `.gsd/harness-manifest.yaml` / `TestU180006` wave-boundary checksum coupling.
* **Protected invariants**:
  * **I1 (100% repository Go source coverage without `.copilot/` or `logs/` traversal)**: `TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo` dynamically verifies (both in `.git` checkouts and inside `redhonesty`'s `.git`-less `git archive` export) that zero root-level `*.go` files exist and `{cmd, internal, tests, scripts}` covers 100% of Go top-level directories, while `gofmt -l cmd internal tests scripts` and `gofmt -w cmd internal tests scripts` (in both `build-feature/SKILL.md` and `fix-ci/SKILL.md`) never touch `.copilot/`, `.bin/`, `.engram/`, `.kbdb/`, `.gsd/`, `.backlogit/`, or `logs/`.
  * **I2 (Wave 3 `U9` + Wave 4 `U3..U8, U10` scheduling for `redhonesty`, `TestU180006`, and `TestU12_GSDModelRoutingManifestReconciliation`)**: Units `U1` (`Makefile`), `U2` (`make.ps1`), and `U9` (`.gsd/workspace-profile.yaml`) are not in `.gsd/harness-manifest.yaml` `artifacts[]` and run one `harness-ready` task per wave in Waves 1, 2, and 3; all manifest-tracked Markdown edits (`U3`–`U8`) co-schedule in Wave 4 ahead of `U10` (`.gsd/harness-manifest.yaml`) so each wave has at most one `harness-ready` writer on `format_gate_containment_test.go` and `TestU180006_ManifestChecksumsCurrent` + `TestU12_GSDModelRoutingManifestReconciliation` pass at Waves 1, 2, 3, and 4.
  * **I3 (Preservation of `073-DL` governed full-suite budget, drift records, and foreign worktree paths)**: Every edit preserves `go test -timeout=30m ./...` lines and all existing `drift_reason` citations (`"Do not auto-revert."`, `227A2930`, `B83081F5`, `75E02C17`..`F05661B1`, `073-DL rev22`, `PR #400`), amends/inserts `drift_allowed: true` + `drift_reason` on `constitution.instructions.md` and `go.instructions.md` without duplicate YAML keys, and uses literal pathspecs excluding `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, `.gsd/config.yaml`, and `.kbdb/`.
* **Reinforced instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`, `.github/instructions/go.instructions.md`, `.github/instructions/markdown.instructions.md`, `.github/policies/workflow-policies.md`
  * `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
  * `docs/compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md`
  * `docs/compound/2026-10-16-gitattributes-eol-renormalization-worktree-refresh.md`
* **Risky actions classification**:

| ProposedAction | Target Surface | ActionRisk | Approval / Guardrail Path | Expected ActionResult |
| --- | --- | --- | --- | --- |
| Scope `Makefile` and `make.ps1` `fmt` targets (`U1`, `U2`) | `Makefile`, `make.ps1`, `tests/integration/format_gate_containment_test.go` | `low` (local build script targets) | Guard with dynamic `git ls-files '*.go'` directory check | `make fmt` and `.\make.ps1 fmt` check 100% of Go source dirs and ignore `.copilot/` |
| Amend Constitution (`1.1.1`) and update shared harness docs/skills/agents/policies (`U3`–`U8`) | `AGENTS.md`, `.github/**` (11 files) | `moderate` (constitution & shared workflow gates) | Include Constitution v1.1.1 amendment log + sync impact report; preserve `go test -timeout=30m ./...` anchors | Consistent `gofmt -l cmd internal tests scripts` across all agent/skill/policy surfaces |
| Reconcile `.gsd/workspace-profile.yaml` and `.gsd/harness-manifest.yaml` (`U9`, `U10`) | `.gsd/workspace-profile.yaml`, `.gsd/harness-manifest.yaml`, `tests/integration/format_gate_containment_test.go` | `moderate` (manifest `profile_hash`, `drift_allowed`, and 9 checksums) | Run `U9` in Wave 3 and `U10` in Wave 4 after `U3`–`U8`; verify `TestU180006`, `TestU12_GSDModelRoutingManifestReconciliation`, `TestGovernedFullSuiteBudget_*`, and `gsd verify-workspace` | `0` checksum mismatches, `0` schema blockers, all full-suite guards green |

* **Review-gate capability and marker requirements**:
  * `plan-review` MUST probe subagent dispatch capability, record literal `dispatch_mode:` and `decision:` markers, cover all 7 required personas plus `rubber-duck` critique, and achieve `decision: PASS` before harvest.
* **Unresolved operator decisions**: None.

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `TOOL_OK: reviewer-subagent-dispatch`
* `decision: FAIL`
* **Gate rationale (Attempt 1 — superseded by Attempt 2)**: Multi-persona + `rubber-duck` review (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`, and `rubber-duck`) identified `P1` blockers across (1) live mutating `gofmt -w .` in `.github/skills/build-feature/SKILL.md` omitted from `U6`/`TestU10` and `13` vs `14` file count, (2) `P-002.6`/`P-004` `harness_cmd` bundling on `U2`/`U10` and trailing `$` regex on `TestGovernedFullSuiteBudget_`, (3) `redhonesty` `RejectHeadNotGreen` when co-scheduling two `harness-ready` tasks (`U9` and `U10`) on `format_gate_containment_test.go` in Wave 3, and (4) `git ls-files` returning empty inside `redhonesty`'s `.git`-less `git archive` export in `TestU1_*`, plus `P2` improvements (`goimports -l`, `workflow-policies.md` `v1.33.1`, root-level `*.go` check, 4-dir `t.TempDir()` seeding, and `drift_reason` amend-vs-insert), all remediated in Attempt 2 above.

<!-- plan-review-attempt: 2 -->

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `TOOL_DEGRADED: reviewer-subagent-dispatch`
* `decision: PASS`
* **Gate rationale (Attempt 2)**: Dispatched all 7 required `plan-review` personas plus `rubber-duck` (`/gsd-dark-mode` Step 2.1); 7 subagents returned structured `PASS` verdicts (`0 P0, 0 P1, 0 P2`), and following an empty stream return from `Agent-Native Parity Reviewer` a complete sequential rubric pass was executed across all 7 personas + `rubber-duck` per `plan-review/SKILL.md` Terminal States. Plan hardening was required (`Requires plan hardening: yes`) and is satisfied by `## Plan Hardening` (`I1`–`I3` and `ProposedAction` / `ActionRisk` table). `## Constitution Check` concludes with literal `Constitution Check: pass`. Merged findings: `0 P0, 0 P1, 0 P2` (Attempt 2 P3 polish notes on `errors.Is(statErr, os.ErrNotExist)` + dot-dir skip in `TestU1_*` and Principle VIII Wave 4 wording incorporated above; `TestU10_*` live-command scan excludes historical `Amendment Log` / `Governance` prose sections).
* **Per-persona findings summary**:
  * **Constitution Reviewer**: `PASS` (`0 P0, 0 P1, 0 P2`; P3 Principle VIII Wave 4 wording incorporated).
  * **Go Reviewer**: `PASS` (`0 P0, 0 P1, 0 P2`; P3 `os.ErrNotExist` check and `Amendment Log` prose exclusion noted).
  * **Scope Boundary Auditor**: `PASS` (`0` findings).
  * **Learnings Researcher**: `PASS` (`0 P0, 0 P1, 0 P2, 1 P3` advisory on `cmd.Dir` binding in `TestU2_*`, `confidence: high`).
  * **Architecture Strategist**: `PASS` (`0 P0, 0 P1, 0 P2`; P3 dot-dir skip in `TestU1_*` and Wave 4 wording incorporated).
  * **Agent-Native Parity Reviewer** (complete rubric pass): `PASS` (`0` findings). Verified all 14 workspace-governed `gofmt -l` / `gofmt -w` surfaces, `goimports -l` in `go.instructions.md`, and documented external-consumer boundary for `plugin/skills/*`.
  * **Security Lens Reviewer**: `PASS` (`0` findings). Both `build-feature/SKILL.md` and `fix-ci/SKILL.md` `gofmt -w` auto-fixes scoped away from `.copilot/` caches.
  * **`rubber-duck`**: `PASS` (`0` findings).
