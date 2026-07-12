---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-05-08T00:00:00Z
    origin: .backlogit/queue/044-DL.md
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-05-08-release-binary-readiness-plan.md
title: Release Binary Readiness
---

## Problem Frame

The linked deliberation `044-DL` frames the next release as blocked operational work, not new feature development. The CLI and documentation audit is already complete, but the release path is still red because `gofmt -l .` reports widespread pre-existing formatting drift across source files outside the recent audit change set. Releasing before that gate is green would cut a public binary from a repository state that does not satisfy the documented quality bar.

The work therefore needs to stage as a bounded release-readiness feature with two phases: first, clear the repository-wide formatting debt in reviewable slices; second, perform the version bump and tag-driven release flow only after the mandatory gates are green. This keeps the public release operation small, auditable, and reversible.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | `gofmt -l .` must produce zero source-file output before the release proceeds | `044-DL` Problem Frame |
| R2 | The standard gates (`go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`) must remain green after the formatting cleanup | User request, `044-DL` Notes |
| R3 | The source version and released tag must match the intended new binary version | `internal/version/version.go`, prior release closure |
| R4 | The existing tag-triggered release workflow must remain the canonical binary publication path | `.github/workflows/release.yml` |
| R5 | Release verification must include binary assets, checksums, and install-surface validation | `.github/workflows/release.yml`, `tests/integration/shipment_040_release_install_harness_test.go`, prior release closure |
| R6 | Work must be decomposed into reviewable units that stay within the Stage 2-hour rule | Stage agent instructions |

## Scope Boundaries

### In Scope

* Clearing repository source formatting drift currently reported by `gofmt -l .`
* Re-running the full Go quality gates after the formatting sweep
* Bumping the canonical source version for the next binary release
* Executing the existing tag-driven GitHub release workflow after the repo is green
* Verifying published binaries, checksums, and install/release surfaces

### Non-Goals

* Shipping any new product feature beyond release readiness
* Reworking the release workflow design or adding new release tooling
* Reformatting session artifacts under `.copilot/`, `.autoharness/`, or other tool-managed state outside the source release surface
* Expanding npm publishing behavior beyond the current workflow contract

### Deferred to Implementation

* The exact new semantic version number to cut after the repo is green
* Whether the formatting sweep should be executed package-by-package or with a top-level `gofmt -w` plus package-scoped review commits
* Whether any generated docs need regeneration after the version bump

## Implementation Units

### Unit 1: Format entrypoint and version surfaces

**Files:** `cmd/backlogit/main.go`, `cmd/gen-docs/main.go`, `cmd/gen-docs/main_test.go`, `internal/version/version.go`, `internal/version/version_test.go`, `scripts/migrate-ids.go`, `scripts/migrate_ids_test.go`
**Test files:** `cmd/gen-docs/main_test.go`, `internal/version/version_test.go`, `scripts/migrate_ids_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** prior formatting-first release-prep pattern in `docs/exec-plans/2026-04-11-build-docs-cli-parity-plan.md`; quality gates in `AGENTS.md`
**Dependencies:** none

**Approach:**
Format the command-entrypoint, generator, version, and script surfaces first so the release-specific parts of the repo start from a clean baseline. Keep this slice isolated from the larger package sweeps so later version-bump work only lands on already-formatted files.

**Verification:**
`gofmt -l` returns zero output for the listed files, and `go test ./cmd/gen-docs ./internal/version ./scripts` passes.

### Unit 2: Format CLI package surfaces

**Files:** `internal/cli/*.go`, `internal/cli/format/*.go`
**Test files:** `internal/cli/*_test.go`, `internal/cli/format/*_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** existing CLI package structure and focused package verification via `go test ./internal/cli/...`
**Dependencies:** Unit 1

**Approach:**
Apply formatter-only cleanup across the CLI package family as a single bounded slice. Review the resulting diff for any files whose formatting change reveals latent test or lint issues before moving to the next package family.

**Verification:**
`gofmt -l internal/cli/...` produces zero output, and `go test ./internal/cli/...` passes.

### Unit 3: Format config, model, parser, stash, and error packages

**Files:** `internal/config/*.go`, `internal/errors/*.go`, `internal/models/*.go`, `internal/parser/*.go`, `internal/stash/*.go`
**Test files:** `internal/config/*_test.go`, `internal/errors/*_test.go`, `internal/models/*_test.go`, `internal/parser/*_test.go`, `internal/stash/*_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** package-scoped format-and-test loops already used in the repository
**Dependencies:** Unit 1

**Approach:**
Clear the smaller supporting packages in one grouped slice. These packages are structurally related and have focused test suites, so they fit a single reviewable formatting task without mixing in release operations.

**Verification:**
`gofmt -l` produces zero output for the listed package paths, and `go test ./internal/config ./internal/errors ./internal/models ./internal/parser ./internal/stash` passes.

### Unit 4: Format core package production surfaces

**Files:** `internal/core/*.go`, `internal/core/templates/*.go`
**Test files:** none
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** formatting-only package slices from prior parity plans; preserve existing CQRS and workspace-boundary code paths without behavioral edits
**Dependencies:** Unit 1

**Approach:**
Handle production-side `internal/core` formatting separately from the core test and harness sweep so the most central package remains reviewable. Limit the unit strictly to formatter output and reject opportunistic behavior changes.

**Verification:**
`gofmt -l` produces zero output for the listed production files, and the package still builds cleanly through the later full-gate run.

### Unit 5: Format core test and harness surfaces

**Files:** `internal/core/*_test.go`, `internal/core/templates/*_test.go`
**Test files:** `internal/core/*_test.go`, `internal/core/templates/*_test.go`
**Effort size:** medium
**Skill domain:** tests
**Execution note:** characterization-first
**Patterns to follow:** existing harness-style tests in `internal/core/`
**Dependencies:** Unit 4

**Approach:**
Sweep the core test and harness files after the production-side core formatting so reviewers can separate product-surface changes from test-surface churn. This keeps the most numerous formatting diffs in an isolated verification slice.

**Verification:**
`gofmt -l` produces zero output for the listed test files, and `go test ./internal/core/...` passes.

### Unit 6: Format database and MCP packages

**Files:** `internal/db/*.go`, `internal/mcp/*.go`
**Test files:** `internal/db/*_test.go`, `internal/mcp/*_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** package-level verification through `go test` and existing MCP contract coverage
**Dependencies:** Unit 1

**Approach:**
Apply formatter cleanup to the data and MCP layers together because they are adjacent release surfaces and share downstream contract coverage. Keep the work formatter-only so schema and tool behavior remain untouched.

**Verification:**
`gofmt -l` produces zero output for the listed package paths, `go test ./internal/db ./internal/mcp` passes, and later contract tests remain green.

### Unit 7: Format events, hooks, and telemetry packages

**Files:** `internal/events/*.go`, `internal/hooks/*.go`, `internal/telemetry/*.go`
**Test files:** `internal/events/*_test.go`, `internal/hooks/*_test.go`, `internal/telemetry/*_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** package-level verification through `go test` and existing telemetry durability tests
**Dependencies:** Unit 1

**Approach:**
Treat event-stream, hook, and telemetry formatting as one infrastructure slice. These packages share operational concerns, but this unit remains formatting-only and should not alter logging, checkpoint, or telemetry semantics.

**Verification:**
`gofmt -l` produces zero output for the listed package paths, and `go test ./internal/events ./internal/hooks ./internal/telemetry` passes.

### Unit 8: Format contract and integration test packages

**Files:** `tests/contract/*.go`, `tests/integration/*.go`
**Test files:** `tests/contract/*.go`, `tests/integration/*.go`
**Effort size:** medium
**Skill domain:** tests
**Execution note:** characterization-first
**Patterns to follow:** existing release/install contract coverage in `tests/integration/shipment_040_release_install_harness_test.go`
**Dependencies:** Units 2, 3, 5, 6, 7

**Approach:**
Finish the formatting sweep by cleaning the contract and integration suites after the package-level slices settle. This avoids test-only formatting noise obscuring earlier package reviews.

**Verification:**
`gofmt -l` produces zero output for the listed test paths, and `go test ./tests/contract ./tests/integration` passes.

### Unit 9: Bump the canonical source version

**Files:** `internal/version/version.go`
**Test files:** `internal/version/version_test.go`, `tests/contract/version_tool_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** prior release closure in `docs/closure/2026-04-24-v1.1.0-release-closure.md`; version surface in `internal/version/version.go`
**Dependencies:** Units 1 through 8

**Approach:**
Once the repository is green, update the canonical source version to the intended next release value. Keep the version bump isolated from the formatting work so the release commit is small, obvious, and easy to tag.

**Verification:**
`go test ./internal/version ./tests/contract` passes, and a freshly built binary reports the expected version through `backlogit version` and `backlogit --version`.

### Unit 10: Execute the tag-driven release and validate assets

**Files:** `.github/workflows/release.yml`, `retired packaging script`, `retired-wrapper/package.json`, `retired-platform-packages/darwin-arm64/package.json`, `retired-platform-packages/darwin-x64/package.json`, `retired-platform-packages/linux-arm64/package.json`, `retired-platform-packages/linux-x64/package.json`, `retired-platform-packages/win32-x64/package.json`, `tests/integration/shipment_040_release_install_harness_test.go`
**Test files:** `tests/integration/shipment_040_release_install_harness_test.go`
**Effort size:** medium
**Skill domain:** config
**Execution note:** characterization-first
**Patterns to follow:** tag-triggered release path in `.github/workflows/release.yml`; prior operational closure in `docs/closure/2026-04-24-v1.1.0-release-closure.md`
**Dependencies:** Unit 9

**Approach:**
Use the existing tag-triggered release workflow rather than introducing a new publication path. After the version bump lands and the full gates are green, create the release tag, let GitHub Actions build and publish the binaries, then verify the GitHub release assets, checksums, and install surfaces. Treat npm publish as a best-effort sub-surface because the workflow already marks it `continue-on-error`.

**Verification:**
The release workflow completes successfully for the mandatory jobs, the GitHub release contains the expected binaries plus `SHA256SUMS`, and the install/release validation checks implied by `tests/integration/shipment_040_release_install_harness_test.go` remain satisfied.

## Dependency Graph

```text
Units 1, 2, 3, 4, 6, 7 can start after initial baseline capture.
Unit 5 depends on Unit 4 because it slices core tests away from core production files.
Unit 8 depends on the package-family formatting units so test-only churn lands last.
Unit 9 depends on all formatting units and a fully green repository.
Unit 10 depends on Unit 9 and the full mandatory gate run.
```

This sequence keeps the public release operation narrow: formatting first, version bump second, release execution last.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Split the repo-wide formatting debt by package family before harvest | Keeps each task reviewable and closer to the Stage 2-hour rule, even though the underlying change is mechanical | One monolithic `gofmt` cleanup task, rejected because it would create an oversized diff and poor reviewability |
| D2 | Keep the version bump in its own unit after the formatting sweep | Makes the release commit small and auditable, and avoids mixing semantic version changes with mechanical formatting churn | Bumping the version during the formatting work, rejected because it obscures the release boundary |
| D3 | Use the existing tag-triggered release workflow and packaging script as the only release path | The workflow already builds binaries, packages npm artifacts, generates checksums, and publishes the GitHub release | Adding a bespoke release script or manual binary upload flow, rejected because it would create an unreviewed parallel process |
| D4 | Treat npm publication as advisory rather than a release blocker | The current workflow explicitly allows npm publish to fail without blocking GitHub Release publication | Making npm publish mandatory for release readiness, rejected because it conflicts with the existing workflow contract |
| D5 | Use freshly built source binaries for version validation | Prior audit work proved the installed PATH binary can be stale relative to the checked-out source | Validating against the installed binary, rejected because it can misreport release readiness |

## Risks and Caveats

1. Repo-wide formatting will create broad mechanical diffs across active packages and could conflict with unrelated in-flight branches.
2. Formatting-only changes can still expose latent test expectations or tooling assumptions, especially in harness-style tests.
3. The release workflow touches external systems: GitHub Actions, GitHub Releases, and npm. Those surfaces increase blast radius even when source changes are small.
4. The workflow publishes npm packages on a best-effort basis only. GitHub release success is the hard release criterion; npm results still need review and documentation in closure.
5. A stale local binary can mislead version verification. Ship should always build from the checked-out source before validating the release tag.

## Plan Hardening Signals

* public API, schema, or contract change: **present** — the work produces a new public binary release and updates the version surface seen by CLI and MCP consumers
* security, auth, permission, or compliance-sensitive behavior: **absent** — no auth or permission model changes are expected
* migration, backfill, destructive data/config action, or irreversible step: **absent** — no data migration is planned, and release rollback remains tag and release deletion
* external integration, operator checkpoint, or external dependency: **present** — the work depends on GitHub Actions, GitHub Releases, and npm publication behavior
* high runtime, rollout, or rollback risk: **present** — a bad public binary release requires rollback through tag and release cleanup

Requires plan hardening: yes

## Runtime Verification and Closure

* Units 1 through 8 do not intentionally change runtime behavior. Runtime verification for those units is limited to proving the formatting gate is green and package or test-suite checks still pass.
* Unit 9 changes the public version surface. Verification must prove a freshly built binary reports the intended new version through both CLI version commands and any exposed version tool contracts.
* Unit 10 changes the public release surface. Verification must prove:
  * the release workflow finished green for the required jobs
  * the GitHub release contains the expected binaries and `SHA256SUMS`
  * release assets are attributable to the intended tag and commit
  * install surfaces remain coherent with the binary-first release model documented by the repository
* Operational closure should produce a new artifact at `docs/closure/{date}-v{version}-release-closure.md` that records healthy signals, failure signals, rollback procedure, validation window, owner, and the final release URL.

## Learnings Applied

* `docs/compound/runtime-errors/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md` — validate release readiness with freshly built source binaries, not the installed PATH binary
* `docs/compound/github-actions/F013-workflow-sha-pinning.md` — if the release workflow needs edits during execution, keep action references pinned instead of introducing floating tags

## Standards Check

* **Type-safe Go (I):** formatting and version-bump work stays within the existing Go codebase and does not weaken typing.
* **MCP protocol fidelity (II):** release verification explicitly includes public version surfaces and published binaries used by MCP consumers.
* **Test-first development (III):** the version-bump unit names the exact regression tests that must stay green, and the full gate run remains mandatory before release.
* **Workspace containment and security (IV):** no backlog files are edited directly outside backlogit CLI mutations; release rollback remains tag and release deletion, not destructive workspace surgery.
* **Structured observability (V):** the release workflow, prior release closure pattern, and new closure artifact provide the required release traceability.
* **Single-binary simplicity (VI):** the plan preserves the existing single-binary build and tag-triggered publication path.
* **CQRS data architecture (VII):** this work is release-prep and operational only; it does not alter backlogit storage contracts.
* **Git-friendly persistence (VIII):** formatting and version changes remain plain-text diffs that are easy to review.

## Plan Hardening

Hardening was required because Units 9 and 10 change a public version surface and then publish release artifacts through external systems. The plan now carries forward explicit approval boundaries, rollback triggers, and post-release observation detail so review can evaluate the release operation as a controlled rollout rather than a vague follow-up step.

### Reinforcing Context Consulted

* `.github/instructions/strict-safety.instructions.md`
* `.github/instructions/release-observability.instructions.md`
* `docs/compound/runtime-errors/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md`
* `docs/compound/github-actions/F013-workflow-sha-pinning.md`
* `docs/closure/2026-04-24-v1.1.0-release-closure.md`

### Protected Invariants

* Do not create or push the release tag until `gofmt -l .`, `go test ./...`, `go vet ./...`, and `golangci-lint run` are all green.
* The intended release tag, `internal/version/version.go`, and the reported CLI version must match exactly.
* Release validation must use a freshly built binary from the current checkout, not a stale installed binary.
* The GitHub release is not complete until all expected binaries and `SHA256SUMS` are present.
* Rollback steps must be written before the tag push, not improvised afterward.

### Risky Actions

| ProposedAction | Targets | change_kind | rollback | approval_required | ActionRisk | ActionResult |
|---|---|---|---|---|---|---|
| Clear the remaining source formatting drift in staged package slices | `cmd/`, `internal/`, `scripts/`, `tests/` source files currently reported by `gofmt -l .` | local edit | Revert the formatting commit or restore the affected slice before merge | no | moderate | planned |
| Bump the canonical source version for the next release | `internal/version/version.go`, fresh local build outputs, version contract checks | local edit / public contract update | Revert the version-bump commit before tagging | yes, before tag push | high | planned |
| Push the release tag that triggers GitHub Actions publication | git tag `vX.Y.Z`, `.github/workflows/release.yml` runtime, GitHub Release assets, npm publication surfaces | external call / rollout | Delete the GitHub release, delete the tag, revert the version bump, then rerun after correction | yes | high | planned |

### Added Verification Detail

#### Pre-release audit checklist

* Confirm the working tree contains only intended release-prep changes.
* Confirm `gofmt -l .` produces zero source-file output.
* Confirm `go test ./...`, `go vet ./...`, and `golangci-lint run` all pass on the release commit.
* Confirm the chosen version is not already tagged upstream.
* Confirm `.github/workflows/release.yml` still uses pinned action SHAs if it was edited during release prep.
* Confirm a freshly built binary from the current checkout reports the intended new version before the tag push.

#### Monitoring and observation plan

| Signal | Baseline | Alert threshold | Observation path | Owner |
|---|---|---|---|---|
| Quality gates | All four gates pass locally on the release commit | Any gate failure or non-zero `gofmt -l .` output | Local gate run before tagging | Ship / operator |
| Release workflow health | Mandatory jobs complete green | Any failed mandatory job or canceled build matrix entry | GitHub Actions run for `.github/workflows/release.yml` | Ship / operator |
| Release asset completeness | GitHub release contains expected binaries plus `SHA256SUMS` | Missing asset, missing checksum file, or mismatched tag target | GitHub Release page and workflow artifacts | Ship / operator |
| Version fidelity | Source version, tag name, and built-binary version all match | Any mismatch between `internal/version/version.go`, tag, or CLI version output | Fresh local build plus published asset spot-check | Ship / operator |
| npm publication | Wrapper and platform package publication behave as expected for the current workflow contract | Unexpected wrapper-package failure or any publish anomaly that changes install guidance | `npm-publish` job logs and release closure notes | Ship / operator |

#### Observation window

* Validation window: 24 hours after release publication
* Owner: repository operator with Ship support
* Success condition: no missing assets, checksum mismatches, version mismatches, or install-surface regressions are reported during the window

### Rollback Triggers and Procedure

**Rollback triggers**

* Any published binary reports the wrong version
* `SHA256SUMS` is missing or does not match the uploaded binaries
* A required release asset is missing from the GitHub release
* A fresh install smoke path fails because the release points at the wrong commit or broken artifact set

**Rollback procedure**

1. Delete the GitHub release and its tag.
2. Revert the version-bump commit if the source version is wrong.
3. Correct the release-prep issue on a normal reviewable branch.
4. Re-run the full gates.
5. Re-tag only after the corrected release commit is verified.

### Human Checkpoints

* Operator approval is required before any tag push or other command that will publish public release artifacts.
* If release prep reveals a need to change `.github/workflows/release.yml` or npm packaging metadata, route that change back through plan review before execution rather than editing during the release run.
* npm publish anomalies that do not block GitHub Release publication still require explicit closure notes so the install guidance stays accurate.

### Unresolved Operator Decisions

* Choose the exact next semantic version once the formatting sweep is complete.
* Confirm whether any release-note or packaging prerequisites beyond the current workflow contract should be treated as mandatory for this cut.

## Plan Review

### Gate Decision: PASS

### Summary

The plan passes review with no P0, P1, or P2 findings. The release plan needed hardening because it culminates in a public binary release, and that requirement is satisfied by the appended `## Plan Hardening` section with explicit risky actions, approval boundaries, monitoring signals, rollback triggers, and operator checkpoints.

### Findings

#### P0 -- Critical (must fix before proceeding)

None.

#### P1 -- High (should fix before proceeding)

None.

#### P2 -- Moderate (user discretion)

None.

#### P3 -- Low (advisory)

* Freeze each formatting task to the `gofmt -l` output captured when that task is claimed so later unrelated branch activity does not silently expand a reviewed task boundary.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| No constitutional conflicts found | Constitution Reviewer | GPT-5.4 |
| Verification and gate sequence are sufficient for release prep | Go Quality Reviewer | GPT-5.4 |
| Formatting slices are broad but still reviewable because the work is formatter-only and release execution is isolated | Scope Boundary Auditor | GPT-5.4 |
| Fresh-binary validation and SHA-pinning learnings were applied | Learnings Researcher | GPT-5.4 |
| Dependency ordering is realistic: formatting first, version bump second, release execution last | Architecture Strategist | GPT-5.4 |
| Public version and release workflow surfaces are explicitly covered | Agent-Native Parity Reviewer | GPT-5.4 |

### Next Steps

Proceed to `harvest`. Create a parent feature for release binary readiness, decompose the approved implementation units into child tasks, and assemble a shipment so Ship receives a shipment ID rather than only a feature ID.
