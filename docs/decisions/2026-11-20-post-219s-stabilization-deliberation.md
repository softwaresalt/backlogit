---
chunk_strategy: h1-h2-h3
description: 'P-021 C6 deliberations for post-219-S stabilization stash entries 39F868B1 (105-DL), EF372865 (106-DL), and post-210-S architectural resolution of DF533572 (079-DL), sequenced after 220-S as Shipments 221-S and 222-S.'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - .backlogit/archive/079-DL.md
    - .backlogit/archive/105-DL.md
    - .backlogit/archive/106-DL.md
    - .backlogit/queue/220-F.md
    - .backlogit/queue/221-S.md
    - .backlogit/queue/221-F.md
    - .backlogit/queue/222-S.md
    - docs/decisions/2026-11-01-repository-format-gate-session-cache-containment-deliberation.md
    - docs/plans/2026-11-20-mcp-session-path-and-tools-dir-docs-plan.md
    - docs/plans/2026-11-20-repository-format-gate-containment-plan.md
  promoted_to: both
  tags:
    - session-mux
    - validateSessionPath
    - tools-dir
    - gofmt
    - 39F868B1
    - EF372865
    - DF533572
    - 105-DL
    - 106-DL
    - 079-DL
  topic: 'Post-219-S stabilization: Windows validateSessionPath separator normalization (39F868B1 / 105-DL), tools_dir documentation (EF372865 / 106-DL), and repository format-gate containment (DF533572 / 079-DL)'
schema_version: "1.0"
source: docs/decisions/2026-11-20-post-219s-stabilization-deliberation.md
title: 'Deliberation: Post-219-S Stabilization — MCP Session Path Validation, Toolchain Documentation, and Repository Format-Gate Containment (39F868B1, EF372865, DF533572)'
---

## Problem Frame

Three post-`219-S` stabilization items are ready for staging following Group 1A (`219-F` / `220-S`):

1. **`39F868B1` (`high`, `bug`, `DEFERRED SCOPE EXPANSION` -> `105-DL`)**: In `internal/mcp/session_mux.go:384`, `(*Server).validateSessionPath` normalizes only backslashes (`strings.ReplaceAll(strings.TrimSpace(path), "\\", string(filepath.Separator))`). On Windows (`filepath.Separator == '\\'`), forward slashes `'/'` remain unnormalized when `strings.Split(path, string(filepath.Separator))` splits segments:
   * `"sub/../existing.md"` and `"junction/../existing.md"` pass `filepath.IsLocal` (`true` on Go 1.24), stay unsplit as a single element (bypassing `if component == ".."`), and collapse lexically inside `filepath.Join(".", path)` to `"existing.md"` without rejecting parent traversal or inspecting `"sub"` / `"junction"` for symlink/reparse status!
   * `"junction/missing.md"` (where `junction` is a Windows NTFS directory junction pointing outside the workspace) stays unsplit, skips explicit per-ancestor `root.Lstat("junction")` + `core.IsSymlinkOrReparsePoint`, and relies on `root.Lstat("junction\\missing.md")` returning an unwrapped `errPathEscapes` error rather than `corerrors.ErrValidation` (`"MCP tool path traverses a symlink or reparse point"`).
2. **`EF372865` (`high`, `task`, `DEFERRED SCOPE EXPANSION` -> `106-DL`)**: Shipments `218-S` (`217-F`) and `219-S` (`218-F`, PR `#665`) added optional `tools_dir` in `.backlogit/config.yaml` and `.backlogit/toolchain.yaml`, `.bin` default discovery (with `bin` fallback when `bin/` exists and `.bin/` does not), `VerifyProposedLauncherScripts`, and `backlogit init` / `backlogit install` `[--tools-dir <dir>]` binary + `.mcp.json` + `toolchain.yaml` provisioning, but `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` still document only manual `./bin/*` setup.
3. **`DF533572` (`medium`, `task`, already linked to decided `079-DL`)**: As framed in [`079-DL`](2026-11-01-repository-format-gate-session-cache-containment-deliberation.md), running literal `gofmt -l .` in the workspace root traverses gitignored `.copilot/session-state/.../gomodcache/.../testdata/issue*.go` compiler syntax error fixtures and `logs/` proof trees, emitting 25,144 lines and exiting 2 even when all repository Go packages (`cmd`, `internal`, `tests`, `scripts`) are formatting-clean. `079-DL` deferred the permanent post-`210-S` remedy between scoping the shared formatting gate to repository Go source directories vs relocating `.copilot` session caches.

## P-021 C5/C6 Provenance and Triage Outcomes

| Stash ID | Deliberation | Kind / Priority | Reconciled Source Refs | Duplicate Scan (Obligation A) | Late-Identifier Reconciliation (Obligation B) |
| --- | --- | --- | --- | --- | --- |
| `39F868B1` | `105-DL` | `bug` / `high` | `task_id=215.003-T`; `feature_id=215-F`; `shipment_id=216-S`; `PR=#654`; `review_thread_id=N/A` | `CLEAN DUPLICATE SCAN` (`0` duplicates across active & archived stash) | Reconciled `PR: N/A` to `PR #654` in place from `docs/closure/216-S-215-F-post-merge-closure.md`; `review_thread_id: N/A` stands ("no late review-thread identifier found") |
| `EF372865` | `106-DL` | `task` / `high` | `task_id=218.005-T`; `feature_id=218-F`; `shipment_id=219-S`; `PR=#665`; `review_thread=N/A` | `CLEAN DUPLICATE SCAN` (`0` duplicates across active & archived stash) | Reconciled `PR: N/A` to `PR #665` in place from `docs/closure/219-S-218-F-post-merge-closure.md`; `review_thread: N/A` stands ("no late review-thread identifier found") |
| `DF533572` | `079-DL` | `task` / `medium` | `task_id=209.001-T`; `feature_id=209-F`; `shipment_id=210-S`; `PR=#638`; `review-thread ID=N/A` | `CLEAN DUPLICATE SCAN` (recorded in `079-DL`) | Reconciled to `PR #638` in `079-DL`; `review-thread ID: N/A` stands |

## Research Findings

### Prior Institutional Learnings (`docs/compound/`)

* `docs/compound/security-issues/mcp-validator-sdk-parser-differentials-2026-11-08.md` and `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`: Path separator differentials between `validateSessionPath` and OS/handler consumers on Windows (`/` vs `\`) allow traversal or junction-ancestor bypasses unless both separators are normalized identically before splitting and `os.Root.Lstat` inspection; Windows junctions (`cmd /c mklink /J`) test reparse containment without requiring elevated privileges.
* `docs/compound/2026-06-26-docline-frontmatter-contract.md` and `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`: Hand-authored updates to `docs/configuration.md` and `docs/design-docs/**` must keep Docline v1 frontmatter valid (`chunk_strategy: h1-h2-h3`, `schema_version: "1.0"`) and leave generated `docs/cli-reference/*.md` untouched.
* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`: Unscoped `gofmt -l .` walks non-product workspace caches; furthermore, Level 1 toolchain pinning (`214-S`, `VerifyLauncherScripts` in `internal/toolchain/launcher.go`) **requires** `start.ps1` and `start.sh` to bind `COPILOT_HOME` inside the workspace (`.copilot`). Relocating `.copilot` outside the workspace would violate Level 1 toolchain containment! Meanwhile, `git ls-files '*.go'` shows 100% of tracked Go files (`1,420` files) reside in the four top-level Go source directories `cmd`, `internal`, `tests`, and `scripts`.

## Options Evaluated

### Sub-Decision 1 (`39F868B1` / `105-DL`): Windows Separator Normalization in `validateSessionPath`

* **Option 1A**: Rely on `filepath.IsLocal` without normalizing `'/'` on Windows — **Rejected** (leaves `"sub/../existing.md"` and `"junction/missing.md"` unsplit on Windows).
* **Option 1B (Selected)**: Normalize both `'/'` and `'\\'` to `string(filepath.Separator)` via `strings.NewReplacer("/", string(filepath.Separator), "\\", string(filepath.Separator))` before `filepath.IsLocal`, `strings.Split`, `component == ".."`, and `root.Lstat` + `core.IsSymlinkOrReparsePoint` inspection (skipping empty `""` segments from consecutive/trailing slashes only after rejecting leading-slash/absolute paths via `filepath.IsLocal`, while keeping `"."` inspected), tested in `internal/mcp/session_mux_test.go` with forward-slash, backslash, mixed-separator, trailing-slash, real Windows junction fixtures, and end-to-end `ServeSessionStream` `"path"`-bearing tool calls (`backlogit_export_command_map` and `backlogit_docs_lint`).

### Sub-Decision 2 (`EF372865` / `106-DL`): `tools_dir` & `init`/`install --tools-dir` Documentation

* **Option 2A**: Pair `docs/configuration.md` and `docs/design-docs/...` with a new Go test — **Rejected** (mixes Markdown docs and Go test files across two doc surfaces, pushing past `< 3 files` if bundled, whereas P-002.1 `docs-only` with a content + `go run ./cmd/backlogit docs lint` probe is the canonical width-isolated pattern from `214-F`).
* **Option 2B (Selected)**: Deliver `EF372865` as a width-isolated 2-file `P-002.1` `docs-only` task updating `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` with a non-vacuous `P-002.3` `exempt_verification_command` probing required content anchors and running `go run ./cmd/backlogit docs lint --format json`.

### Sub-Decision 3 (`DF533572` / `079-DL`): Repository Format Gate vs Session-Cache Relocation & Shipment Packaging

* **Option 3A**: Build a `.copilot` session-cache relocation utility — **Rejected** (`VerifyLauncherScripts` in `internal/toolchain/launcher.go` mandates workspace-contained `COPILOT_HOME=.copilot`; relocating `.copilot` outside the workspace violates Level 1 toolchain pinning, and deleting/mutating `.copilot/session-state/` violates Constitution Principles IV/VII).
* **Option 3B**: Scope repository formatting gates to `gofmt -l cmd internal tests scripts` (and `gofmt -w cmd internal tests scripts` in `build-feature/SKILL.md`, `fix-ci/SKILL.md`, and `workspace-profile.yaml`), packaged inside a single Group 1B shipment alongside `39F868B1` and `EF372865` — **Suboptimal** (combines product MCP/docs stabilization with a cross-cutting 15-file harness/build/manifest format-gate update in one PR).
* **Option 3C (Selected)**: Scope the repository formatting gate to `gofmt -l cmd internal tests scripts` and `gofmt -w cmd internal tests scripts` (covering 100% of tracked and untracked new Go source trees `cmd`, `internal`, `tests`, `scripts` while intrinsically excluding `.copilot/`, `.bin/`, `.engram/`, `.kbdb/`, `.gsd/`, `.backlogit/`, and `logs/`, and leaving external-consumer `plugin/skills/*` generic), and split Group 1B cleanly by artifact class into **two sequential shipments**:
  1. **Shipment `221-S` (`220-F`, `blocks` on `220-S`)**: Product MCP & Toolchain Docs Stabilization (`39F868B1` -> `220.001-T` + `EF372865` -> `220.002-T`; 2 tasks, zero `.github/` or `.gsd/` harness overlap!).
  2. **Shipment `222-S` (`221-F`, `blocks` on `221-S`)**: Repository Formatting-Gate Source-Tree Scoping & Manifest Reconciliation (`DF533572` / `079-DL` -> `221.001-T`..`221.010-T`; updating `Makefile` [Wave 1], `make.ps1` [Wave 2], `.gsd/workspace-profile.yaml` [Wave 3], and in Wave 4 `AGENTS.md`, `.github/copilot-instructions.md`, `constitution.instructions.md` [with `1.1.1 (PATCH)` amendment record], `go.instructions.md` [including `goimports -l cmd internal tests scripts`], `_ship.agent.md`, `go-engineer.agent.md`, `build-feature/SKILL.md`, `fix-ci/SKILL.md`, `workflow-policies.md` [with `1.33.1` amendment log], `copilot-code-review.instructions.md`, `github-pr-automation.instructions.md`, and `.gsd/harness-manifest.yaml` with `profile_hash`, `FORMAT_COMMAND`, `HARNESS_ENFORCED_SUMMARY`, `drift_allowed` on `constitution.instructions.md` and `go.instructions.md`, and LF-normalized SHA-256 `checksum`s).

## Trade-off Comparison

| Criterion | Option 3A (Relocate `.copilot`) | Option 3B (Single Group 1B Shipment) | Option 3C (Split `221-S -> 222-S`) |
| --- | --- | --- | --- |
| Preserves Level 1 `COPILOT_HOME` pinning | Fail | Pass | Pass |
| Covers untracked new `.go` files in `build-feature` | N/A | Pass (`cmd internal tests scripts`) | Pass (`cmd internal tests scripts`) |
| Blast-radius isolation & `.gsd/harness-manifest.yaml` serialization | N/A | Moderate (mixes MCP code + harness manifest) | Optimal (`220-S -> 221-S -> 222-S` clean artifact separation) |

## Decision

Adopt **Option 1B**, **Option 2B**, and **Option 3C**:

1. **Feature `220-F` / Shipment `221-S`** (`blocks` on `220-S`):
   * `220.001-T` (`U1`, `harness-ready`): Normalize `'/'` and `'\\'` in `(*Server).validateSessionPath` (`internal/mcp/session_mux.go` + `internal/mcp/session_mux_test.go`).
   * `220.002-T` (`U2`, `harness-exempt: docs-only`): Document `tools_dir`, `.bin` default (`bin` fallback), and `init`/`install --tools-dir` in `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`.
2. **Feature `221-F` / Shipment `222-S`** (`blocks` on `221-S`):
   * `221.001-T`..`221.010-T`: Scope `gofmt -l .` / `gofmt -w .` to `gofmt -l cmd internal tests scripts` / `gofmt -w cmd internal tests scripts` across `Makefile` (Wave 1), `make.ps1` (Wave 2), `.gsd/workspace-profile.yaml` (Wave 3), and shared harness docs/instructions/skills/agents/policies + `.gsd/harness-manifest.yaml` (co-scheduled in Wave 4 so each wave has at most one `harness-ready` task on `format_gate_containment_test.go` and `TestU180006_ManifestChecksumsCurrent`, `TestU12_HarnessManifestNamespaceReconciliation`, and `TestU12_GSDModelRoutingManifestReconciliation` stay green at every wave boundary). While `220-S` and `221-S` execute ahead of `222-S`, `079-DL`'s bounded `gofmt -l cmd internal tests scripts` + diff check remains authorized if `.copilot/session-state/` Go caches are populated on the host.

## Unresolved Questions

None.

## Risks and Mitigations

* **Wave-boundary checksum coupling in `222-S`**: `TestU180006_ManifestChecksumsCurrent` and `220-S`'s `TestU12_GSDModelRoutingManifestReconciliation` check manifest-tracked Markdown files against `.gsd/harness-manifest.yaml` at every wave's full-suite gate. Scheduling `.gsd/workspace-profile.yaml` (`U9`) alone in Wave 3 and co-scheduling all Markdown edits (`U3`..`U8`) with `.gsd/harness-manifest.yaml` (`U10`, last in Wave 4 manifest order) keeps Waves 1, 2, 3, and 4 100% green and ensures at most one `harness-ready` task touches `format_gate_containment_test.go` per wave.
* **Future top-level Go directory**: `TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo` in `tests/integration/format_gate_containment_test.go` dynamically verifies (both in `.git` checkouts via `git ls-files '*.go'` and inside `redhonesty`'s `.git`-less `git archive` export via `filepath.WalkDir`) that zero root-level `*.go` files exist and every `.go` file's top-level directory belongs to `{cmd, internal, tests, scripts}`, failing closed if a new top-level Go directory is ever introduced without updating the format gate!

## Decision Review (`gsd-dark-mode` Step 2.1 Multi-Persona + `rubber-duck`)

* **Reviewers applied**: `Constitution Reviewer`, `Security Lens Reviewer`, `Go Reviewer`, `Architecture Strategist`, `Scope Boundary Auditor`, and `rubber-duck` adversarial critique.
* **`rubber-duck` challenge & resolution**: Verified that (1) adding a dynamic `git ls-files '*.go'` top-level directory guard in `format_gate_containment_test.go` prevents silent omission if a future package is added outside `cmd`/`internal`/`tests`/`scripts`; (2) `constitution.instructions.md` and `go.instructions.md` require `drift_allowed: true` + `drift_reason` in `.gsd/harness-manifest.yaml` when updated in `222-S` (`go.instructions.md`'s upstream template hardcodes `gofmt -l .`, and `constitution.instructions.md` records a v1.1.1 amendment log); and (3) splitting `221-S` (MCP + product docs) from `222-S` (shared format-gate + harness manifest) eliminates blast-radius coupling and strictly orders `.gsd/harness-manifest.yaml` mutations (`220-S` -> `222-S`). Verdict: **PASS (`0 P0, 0 P1`)**.
