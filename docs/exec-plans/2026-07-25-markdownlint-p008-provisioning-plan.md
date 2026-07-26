---
chunk_strategy: h1-h2-h3
description: 'Implementation plan to provision the markdownlint tooling P-008 assumes: .markdownlint.json enabling exactly MD001/MD025/MD041 (default:false, MD025 front_matter_title retargeted to a _title key) + .markdownlint-cli2.jsonc {gitignore:true}, paired scripts/md-lint.{sh,ps1} via a version-pinned markdownlint-cli2, a standalone repo-wide BLOCKING SHA-pinned md-lint CI job, characterization tests, and a P-008 reconciliation — repo-wide doctor-to-compliance (config dissolves 229/250 violations with zero edits; ~21 structural fixes bring the repo to 0).'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-25-markdownlint-p008-provisioning-plan.md
title: 'Provision markdownlint tooling to make P-008 reproducible and CI-enforced'
---

## Source

- Stash: `B3D30415` (kind=bug, priority=medium) — "Provision the markdownlint
  tooling that policy P-008 already assumes."
- Deliberation: `054-DL` and
  `docs/decisions/2026-07-25-markdownlint-p008-provisioning-deliberation.md`
  (chosen direction: refined Option A — repo-wide doctor-to-compliance,
  superseding the original Option B recommendation).
- Origin: PR #297 Copilot review of the 125-F docs-consistency plan (U6
  markdown-structure lint thread).
- Prior art (high-confidence compound match):
  `docs/compound/github-actions/F013-workflow-sha-pinning.md` (SHA pinning +
  characterization-first YAML tests).

## Problem Frame

P-008 in `.github/policies/workflow-policies.md` (lines ~159-182) states as a
**precondition** that the workspace already has a `.markdownlint.json` enabling
MD001/MD025/MD041, and a **postcondition** that `markdownlint "**/*.md"` exits 0.
Neither was true: there was no config, no Makefile `md-lint` target, and no CI job
(`Makefile` had only `docs` and `docs-lint`; `.github/workflows/ci.yml` had jobs
`changes`, `test`, `docs-lint`, `cli-reference-drift`). The P-008 gate was thus
unreproducible and unenforced; agents fell back to markdownlint defaults.

A repo-wide inventory (markdownlint-cli2 v0.23.1, MD001/MD025/MD041 with default
options) found **250 violations** — MD025=229, MD041=20, MD001=1. However, 229 of
those were MD025 false positives caused by the default `front_matter_title` regex
counting frontmatter `title:` as an H1. Retargeting MD025's `front_matter_title` to
a non-existent `_title` key eliminates all 229 with zero file edits, leaving 21
real structural violations (20 SKILL.md missing leading `# H1` + 1 heading-increment
skip) — a tractable in-place remediation.

> **Supersedes original plan design.** The original plan recommended Option B
> (a scoped rollout with a curated ignore set). A feasibility read showing the
> `_title` configuration dissolves 229/250 violations with zero file edits led
> the operator to supersede Option B with refined Option A — repo-wide
> doctor-to-compliance. The plan body below describes the as-built design.

## Requirements Trace

| Requirement (from stash / deliberation) | Implementation action | Unit |
|---|---|---|
| Config enabling **exactly** MD001/MD025/MD041 with the MD025 `_title` `front_matter_title` | Create `.markdownlint.json` (rules) and `.markdownlint-cli2.jsonc` (`gitignore: true` for local-equals-CI parity) | U1 |
| Remediate the 21 real structural violations so the repo is clean | Fix 20 SKILL.md leading `# H1` (MD041) + 1 heading-increment (MD001) | U2 |
| Reproducible local invocation (`make md-lint`) | Add paired `scripts/md-lint.{sh,ps1}` + Makefile/`make.ps1` targets | U3 |
| CI job so Ship + CI apply the same rules | Add standalone repo-wide `md-lint` job to `ci.yml`, SHA-pinned, blocking | U4 |
| P-008 wording reflects provisioned reality; guard tests lock the config | Reconcile P-008 + create `markdownlint_gate_test.go` | U5 |

## Implementation Units

Each unit obeys the 2-hour rule (< 3 files, < 5 functions, < 4 test scenarios),
width isolation (single domain), and an atomic verifiable milestone.

### U1 — markdownlint config (domain: config)

- **Changes**: Create `.markdownlint.json`:

  ```json
  {
    "default": false,
    "MD001": true,
    "MD025": { "front_matter_title": "^\\s*_title\\s*[:=]" },
    "MD041": true
  }
  ```

  Create `.markdownlint-cli2.jsonc` with `{ "gitignore": true }` (plus comments).
  `gitignore: true` makes cli2 skip gitignored scratch (`.copilot/`, `.autoharness/`,
  `logs/`) and lint the non-gitignored Markdown corpus. In a clean checkout (CI) that
  equals exactly the tracked set (1,839 files); locally it also covers new/untracked
  non-ignored Markdown (intentional pre-commit checking; scratch must be gitignored).
  This is a runner option for local-equals-CI parity. cli2 auto-discovers
  `.markdownlint.json` for the rules.
- **MD025 `_title` crux**: the default `front_matter_title` regex matches
  frontmatter `title:` and counts it as an H1, so the body `# H1` becomes a second
  top-level heading — MD025 fires on every frontmatter-plus-H1 file (all 229
  violations). Retargeting to `_title` (a non-existent key) stops the match:
  frontmatter `title:` is no longer counted, so the single body `# H1` passes.
  Eliminates 229/250 violations with zero file edits.
- **MD041 guardrail (HARD)**: MD041 stays `true` with default options. MD041's
  default `front_matter_title` regex still matches `title:`, crediting frontmatter
  `title:` toward MD041. If MD041 were retargeted to `_title` or `""`, every
  frontmatter file would fail MD041 (~1,262 files). **Only MD025 is retargeted;
  MD041 must never be retargeted.**
- **Files**: `.markdownlint.json`, `.markdownlint-cli2.jsonc` (2 new files).
- **Verification**: `npx --yes markdownlint-cli2@0.23.1 "**/*.md"` over the repo
  with the config applied reports exactly the 21 structural violations (20 MD041 +
  1 MD001), not 250.
- **Posture**: config-first (the artifact is the config; validation is running the
  linter).

### U2 — structural violation remediation (domain: docs/config)

- **Changes**: Fix the 21 real structural violations in place:
  - **20 SKILL.md files** missing a leading `# <name>` heading (MD041): 13 under
    `.github/skills/*/SKILL.md` and 7 under `plugin/skills/*/SKILL.md`.
  - **1 MD001** heading-increment skip (H3 to H4) at
    `docs/exec-plans/2026-07-06-pre-task-completion-gate-broker-plan.md` line ~83.
- **SKILL.md template note**: these files are generated from external autoharness
  templates not present in this repo (footer: "Generated by autoharness"). The
  upstream template is out of scope; files are fixed in place. The repo-wide
  required CI gate catches future regeneration drift; an upstream template fix is
  tracked as a separate follow-up.
- **Files**: 20 SKILL.md files + 1 exec-plan doc (21 files).
- **Verification**: after remediation, `npx --yes markdownlint-cli2@0.23.1
  "**/*.md"` exits 0 (zero violations across 1,839 tracked files).
- **Posture**: remediation. Depends on U1 (config must be in place to validate).

### U3 — invocation scripts + Makefile/make.ps1 targets (domain: build/config)

- **Changes**: Create paired scripts:
  - `scripts/md-lint.sh` — bash, `set -euo pipefail`,
    `npx --yes markdownlint-cli2@0.23.1 "**/*.md"`.
  - `scripts/md-lint.ps1` — PowerShell wrapper, same pinned npx invocation.

  Add Makefile `md-lint` target calling `bash scripts/md-lint.sh` (plus `.PHONY`).
  Add `make.ps1` `md-lint` case calling `scripts/md-lint.ps1`.
- **Why scripts, not inline Makefile**: an existing guard test
  (`tests/integration/plugin_manifest_test.go::TestActivePluginDocsDoNotReferenceRetiredNPMWrapper`)
  forbids the substring `npx ` in a fixed `activePaths` list that includes the
  Makefile. The Node/npx invocation lives in `scripts/md-lint.{sh,ps1}` (the
  repo's established paired-script convention, cf. `search.{sh,ps1}`), and the
  Makefile just calls bash.
- **Version pinning**: markdownlint-cli2 is pinned to `@0.23.1` in the npx
  invocations for deterministic runs.
- **Files**: `scripts/md-lint.sh`, `scripts/md-lint.ps1`, `Makefile`, `make.ps1`
  (4 files).
- **Verification**: `make md-lint` and `make.ps1 md-lint` both exit 0.
- **Posture**: config-first. Depends on U1 + U2 (target must exit 0).

### U4 — CI gate (domain: CI/config)

- **Changes**: Add a standalone repo-wide `md-lint` job in
  `.github/workflows/ci.yml`. The job:
  - Has no `needs`, no `if`, no path-filter — always runs on every PR.
  - Uses `permissions: contents: read`.
  - Uses SHA-pinned `actions/checkout`
    (`11bd71901bbe58b213ffa02c9e9f1d69`, v4.2.2) and `actions/setup-node`
    (`49933ea5288caeca8642d1e84afbd3f7d6820020`, v4.4.0) with
    `node-version: "22"` (Node 22+ required by markdownlint-cli2@0.23.1 `engines.node: ">=22"`).
  - Runs `make md-lint`.
  - Is **blocking/required from the start** — the repo is clean (0 violations),
    so the gate is blocking from the start.
- **Branch-protection registration**: registering `md-lint` as a required
  branch-protection check is an external admin action tracked in follow-up stash
  `918BCDAF`.
- **Files**: `.github/workflows/ci.yml` (1 file).
- **Verification**: the `md-lint` job runs on the PR, executes the lint step, and
  reports green. `go test ./tests/integration/ -run TestCI` passes.
- **Posture**: characterization-first (U5 guard tests written before or alongside).
  Depends on U1 + U2 + U3.

### U5 — P-008 reconciliation + guard tests (domain: docs/tests)

- **Changes**:
  - **Guard tests**: create `tests/integration/markdownlint_gate_test.go` asserting:
    (a) `.markdownlint.json` enables exactly MD001/MD025/MD041 with `default: false`
    and the MD025 `front_matter_title` value targeting `_title`; (b) the `md-lint`
    CI job is repo-wide (no path-filter, no `needs` on a classifier) and SHA-pinned.
    The existing `tests/integration/ci_compliance_test.go` already asserts SHA pins
    across all jobs. The empirical 0-violation check is captured as a verifiable
    assertion.
  - **P-008 reconciliation**: update P-008 in
    `.github/policies/workflow-policies.md` to name the active rule set
    (MD001/MD025/MD041) and document the MD025 `_title` `front_matter_title`
    configuration. P-008's universal "all markdown" statement is now genuinely true
    repo-wide — no scope/subset compromise.
- **Files**: `tests/integration/markdownlint_gate_test.go` (1 new file),
  `.github/policies/workflow-policies.md` (1 edit).
- **Verification**: `go test ./tests/integration/ -run TestMarkdownlint` passes;
  `make md-lint` exits 0.
- **Posture**: test-first (RED then GREEN for the guard test). May run in parallel
  with U4.

## Dependency Graph

```text
U1 (config) ──┬──> U2 (remediation)
              |──> U3 (invocation)
              └──> U5 (P-008 + tests)
U2 (remediation) ──> U3 (invocation)  # scripts must exit 0
U3 (invocation) ───> U4 (CI gate)     # CI job runs make md-lint
U5 (tests) ────────> U4 (CI gate)     # characterization tests precede/accompany the job
```

No cycles. Suggested execution order: **U1 then U2 then U3 then U5 then U4** (U5
may run any time after U1). U5 guard tests are authored RED before U4 and turn
GREEN when U4 lands.

## Decisions and Rationale

- **Repo-wide doctor-to-compliance over scoped rollout or baseline**
  (supersedes deliberation Option B): the `_title` MD025 `front_matter_title`
  configuration dissolves 229/250 violations with zero file edits, making repo-wide
  compliance tractable within task granularity. P-008's "all markdown" statement is
  now genuinely true, with no subset compromise or ignore-list maintenance burden.
- **MD025 `_title` `front_matter_title` + MD041 default**: only MD025 is
  retargeted; MD041 stays default so frontmatter `title:` still credits MD041. This
  is load-bearing — retargeting MD041 would fail ~1,262 files.
- **`.markdownlint.json` (rules) + `.markdownlint-cli2.jsonc` (`gitignore: true`)**:
  honors P-008's existing `.markdownlint.json` name (minimal policy churn) and
  cleanly separates rule config from runner config. `gitignore: true` ensures
  local-equals-CI parity. A bare pinned `markdownlint-cli2` auto-discovers both.
- **markdownlint-cli2 via `npx`**: current maintained tool; no committed
  `node_modules`, no `go.mod`/runtime impact (build/CI-time only) — keeps the
  CGo-free Go runtime unaffected (Principle VI).
- **Paired scripts (`scripts/md-lint.{sh,ps1}`) instead of inline Makefile**: an
  existing guard test forbids `npx ` in the Makefile's `activePaths` list. The
  paired-script convention (`search.{sh,ps1}`) is already established.
- **Standalone repo-wide CI job, no path-filter**: the `md-lint` job always runs on
  every PR — no `needs` on a classifier job, no `if` condition, no path-filter.
  This is simpler and avoids the fail-open risk of path-filter misconfiguration.
  SHA-pinned `actions/checkout` + `actions/setup-node`.
- **Blocking from start**: the repo is clean (0 violations), so
  there is no risk of blocking unrelated PRs on Day 1. Branch-protection
  registration is an external admin action tracked as follow-up stash `918BCDAF`.
- **SKILL.md fixed in place, upstream template tracked as follow-up**: the 20
  SKILL.md files are generated from external autoharness templates. Fixing in place
  is immediate; the repo-wide gate catches regeneration drift. An upstream template
  fix is a separate concern.

## Risks and Caveats

- **SKILL.md regeneration drift**: autoharness regeneration may reintroduce missing
  leading `# H1` in SKILL.md files. Mitigation: the repo-wide required CI gate
  catches this immediately; upstream template fix tracked as follow-up.
- **New Node/npm dependency in a Go repo (Principle VI)**: justified as
  build/CI-time only; SHA-pinned action; no runtime footprint. Documented, not a
  silent add.
- **`npx` network fetch flakiness in CI**: mitigated by pinning
  `markdownlint-cli2@0.23.1` in the `npx` invocation for deterministic runs.
- **MD025 `_title` config fragility**: if a future contributor changes the
  `front_matter_title` value or retargets MD041, the gate fires on hundreds of
  files. Mitigation: guard test in `markdownlint_gate_test.go` asserts the exact
  config values; CI breaks on drift.
- **P-008/config divergence**: mitigated by reconciling the policy text in the same
  shipment (U5).

## Constitution Check

- **I. Safety-First Go** — N/A to production Go (no Go source changes); the only Go
  edit is a test (U5), which follows standard error/style conventions. `go vet` /
  `golangci-lint` / `gofmt` gates still apply.
- **II. Test-First Development (NON-NEGOTIABLE)** — pass. U5 authors guard tests
  (RED) before/alongside the U4 workflow edit (GREEN). Config/remediation/invocation
  units (U1/U2/U3) are non-code; their verifiable milestone is the linter exit 0.
- **III. Workspace Isolation & Security Boundaries** — pass. All files are created
  within the workspace root; no secrets; the CI job uses least-privilege
  `permissions: contents: read`.
- **IV. CLI Workspace Containment (NON-NEGOTIABLE)** — pass. All writes are in-tree.
- **V. Structured Observability** — pass. The CI job reports as an always-run
  context; commits use conventional messages.
- **VI. Single Responsibility** — pass with documented justification. Adds a
  Node/CI-time dependency (markdownlint-cli2) — justified by a concrete requirement
  (P-008 enforcement), no runtime/`go.mod` impact, SHA-pinned action.
- **VII. Destructive Command Approval (NON-NEGOTIABLE)** — N/A. No destructive
  commands; all changes are additive files + in-place remediation edits.
- **VIII. Explicit Safety Modes for Elevated Risk** — pass (careful-mode posture).
  The repo-wide gate is safe because the repo starts clean (0 violations).
- **IX. Git-Friendly Persistence** — pass. All artifacts are human-readable
  JSON/JSONC/Markdown.
- **X. Agent Context Efficiency** — N/A. No agent tool surface or data-access
  pattern changes.
- **XI. Merge Commit History Preservation (NON-NEGOTIABLE)** — pass. Ships via a
  merge commit like all work.

Constitution Check: pass

## Plan Hardening Signals

- Public API / schema / contract change: **present (minor)** — introduces a new CI
  status check that is a **blocking** merge-gate contract.
- Security / auth / permission / compliance-sensitive: **absent** — CI job is
  least-privilege read-only; no auth surfaces.
- Migration / backfill / destructive / irreversible: **absent** — all additive or
  in-place remediation; fully reversible (remove job/target/config; revert fixes).
- External integration / operator checkpoint / external dependency: **present** —
  new Node/npm (markdownlint-cli2) build/CI-time dependency and `actions/setup-node`
  external action; operator action required for branch-protection registration.
- High runtime / rollout / rollback risk: **low** — the repo is clean on Day 1; no
  risk of blocking unrelated PRs.

Requires plan hardening: yes

## Runtime Verification and Closure

- **U1/U2 (config, remediation)** — runtime surface: developer/CI command. Verify
  `make md-lint` exits 0 locally after config and remediation are complete. Closure:
  none beyond CI green.
- **U3 (invocation)** — runtime surface: developer command. Verify `make md-lint`
  and `make.ps1 md-lint` both exit 0. Verify a deliberately malformed scratch file
  fails (then discard).
- **U4 (CI)** — runtime surface: GitHub Actions. Verify on the PR that the
  `md-lint` context reports and is green. Closure artifact: note the check is
  **blocking from start**. Rollback trigger: gate blocks unrelated PRs — remove
  the `md-lint` job, scripts, and config. Owner: Ship. Branch-protection
  registration tracked as follow-up stash `918BCDAF`.
- **U5 (tests + policy)** — runtime surface: `go test`. Verify
  `go test ./tests/integration/ -run TestMarkdownlint` passes.

## Out of Scope

The following are **deliberately deferred** and are not part of this shipment:

- **Upstream autoharness SKILL.md template fix**: the 20 SKILL.md files are
  generated from external templates not present in this repo. The in-place fix
  ensures current compliance; the repo-wide gate catches future regeneration drift.
  An upstream template fix is a separate follow-up.
- **Branch-protection registration**: registering `md-lint` as a required
  branch-protection status check is an external admin action, tracked as follow-up
  stash `918BCDAF`.
- **Additional markdownlint rules**: the gate enforces exactly MD001/MD025/MD041
  (the P-008 rule set). Expanding the rule set is a separate decision.

## Plan Hardening

**Hardening required?** Yes. Two signals are present: a new **blocking CI gate**
and a **new external Node/CI dependency** (`markdownlint-cli2` +
`actions/setup-node`).

**Learnings and instruction files consulted:**

- `docs/compound/github-actions/F013-workflow-sha-pinning.md` — pin
  `actions/setup-node` to a full 40-char SHA (resolve tag to SHA at implementation);
  characterization-first YAML testing (RED before the workflow edit).
- `.github/instructions/ci-security.instructions.md` and
  `.github/instructions/workflows.instructions.md` — pinned actions, least-privilege
  `permissions`, no trigger-level `paths:`/`paths-ignore:` on required workflows.

**Protected invariants (must not regress):**

- The `changes`, `test`, `docs-lint`, and `cli-reference-drift` required contexts
  continue to report on every PR. The new `md-lint` job is additive and must NOT
  alter their `if:` or trigger model.
- No workflow-level path filter is introduced.

**Risky actions (ProposedAction / ActionRisk / ActionResult):**

| ProposedAction | ActionRisk | Approval | Rollback | ActionResult |
|---|---|---|---|---|
| Add `md-lint` CI job to `ci.yml` | moderate (rollout/contract) | operator approves PR | Revert the `ci.yml` hunk (remove job) | planned |
| Introduce `markdownlint-cli2` via `npx` + SHA-pinned `actions/setup-node` | moderate (external dependency) | none for config | Remove scripts/job/config | planned |
| Create `.markdownlint.json` with `_title` MD025 config | low (additive config) | none | Delete the file | planned |
| Remediate 21 structural violations in place | low (in-place edits) | none | Revert the 21 file edits | planned |
| Edit P-008 precondition/postcondition wording | low (docs) | none | Revert the policy hunk | planned |

**Reinforced verification:**

- CI environment precheck: confirm `actions/setup-node` SHA resolves and Node is
  available before the lint step; pin the `markdownlint-cli2` version in the `npx`
  call for deterministic runs.
- The `markdownlint_gate_test.go` guard test is the standing guard that the config
  enables exactly the three rules with the `_title` value and that the CI job is
  repo-wide and SHA-pinned.

**Reinforced operational closure:**

- **Monitoring signal**: the `md-lint` check status on PRs.
- **Rollback trigger**: the gate blocks unrelated PRs or fails to fire on a docs
  change — remove the `md-lint` job, scripts, and config in a follow-up revert.
- **Rollback procedure**: revert `ci.yml` hunk + delete scripts + delete config
  files + revert Makefile/make.ps1 changes.
- **Owner**: Ship agent during the rollout PR; operator for the branch-protection
  registration.
- **Validation window**: the repo starts clean; the gate is blocking from the first
  PR. Branch-protection registration is tracked as stash `918BCDAF`.

**Unresolved operator decisions (carried forward):**

- Register `md-lint` as a **required** branch-protection status check — tracked as
  follow-up stash `918BCDAF`.
- Upstream autoharness SKILL.md template fix — tracked as a separate follow-up.

## Plan Review

> **Historical note (SUPERSEDED).** The plan review below was conducted against the
> original Option B design (a scoped rollout with a curated ignore set). The
> operator subsequently superseded Option B with refined Option A (repo-wide
> doctor-to-compliance) after a feasibility read showed the MD025 `_title`
> `front_matter_title` configuration dissolves 229/250 violations with zero file
> edits. Reviewer findings are preserved as audit trail; the plan body above
> reflects the as-built design.

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved

Five reviewer personas were dispatched as independent sub-agents: Constitution
Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, and
Architecture Strategist. Merged result: **0 P0, 0 P1, several P2, several P3.**
The plan was approved with advisory-level findings folded into the implementation.

The highest-value P2 findings (verified ignore mechanism, dedicated SHA assertion,
exactly-three-rules guard test, Constitution Check completeness) were incorporated
before the operator authorized harvest. All P2 and P3 findings have been
re-evaluated against the as-built repo-wide design and remain addressed or
rendered moot by the simpler architecture (e.g., the ignore-mechanism verification
became irrelevant because there is no ignore set in the final design).
