---
chunk_strategy: h1-h2-h3
description: 'Implementation plan to provision the markdownlint tooling P-008 assumes: a scoped .markdownlint.json + .markdownlintignore enabling exactly MD001/MD025/MD041, a Makefile md-lint target via markdownlint-cli2, an always-reporting SHA-pinned md-lint CI job gated by a md_touched paths-filter, a characterization test, and a P-008 wording reconciliation — using a curated ignore set rather than a 248-file retroactive cleanup.'
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
  (chosen direction: Option B — scoped rollout with a curated ignore set).
- Origin: PR #297 Copilot review of the 125-F docs-consistency plan (U6
  markdown-structure lint thread).
- Prior art (high-confidence compound matches):
  `docs/compound/github-actions/F013-workflow-sha-pinning.md` (SHA pinning +
  characterization-first YAML tests) and
  `docs/compound/github-actions/dorny-paths-filter-every-quantifier-semantics-2026-07-04.md`
  (single-pattern filter is quantifier-invariant; fail-safe `!= 'false'` step
  gating + `!cancelled()` job so required checks always report).

## Problem Frame

P-008 in `.github/policies/workflow-policies.md` (lines ~159–182) states as a
**precondition** that the workspace already has a `.markdownlint.json` enabling
MD001/MD025/MD041, and a **postcondition** that `markdownlint "**/*.md"` exits 0.
Neither is true: there is no config, no Makefile `md-lint` target, and no CI job
(`Makefile` has only `docs` and `docs-lint`; `.github/workflows/ci.yml` has jobs
`changes`, `test`, `docs-lint`, `cli-reference-drift`). The P-008 gate is thus
unreproducible and unenforced; agents fall back to markdownlint defaults.

A repo-wide `**/*.md` exit-0 gate cannot land today: a tracked-only inventory
(markdownlint-cli2 v0.23.1, MD001/MD025/MD041 only) found **249 violations across
248 files** — MD025=228, MD041=20, MD001=1 — concentrated in historical/archival
(`docs/closure` 74, `docs/compound` 38, `docs/archive` 20), machine-generated
(`.backlogit` 49), and template-generated (`.github/skills/*` under `.github`=14)
content. The fix provisions a **scoped, forward-looking** gate: enable exactly the
three rules, ignore the currently-dirty and ephemeral trees so the gate is green on
Day 1 and fail-closed for new/edited files elsewhere, wire it into CI following the
repo's pinned-action + always-reporting conventions, and reconcile the P-008 text.

## Requirements Trace

| Requirement (from stash / deliberation) | Implementation action | Unit |
|---|---|---|
| Config enabling **exactly** MD001/MD025/MD041 | Add `.markdownlint.json` (`default:false` + 3 rules) and `.markdownlintignore` (curated scope) | U1 |
| Makefile target invoking markdownlint-cli2 | Add `md-lint` target + `.PHONY` entry | U2 |
| Characterization coverage for the new CI job | Extend `tests/integration/ci_compliance_test.go` (RED→GREEN) | U3 |
| CI job so Ship + CI apply the same rules | Add always-reporting `md-lint` job + `md_touched` classifier to `ci.yml`, SHA-pinned `actions/setup-node` | U4 |
| P-008 wording no longer overstates config exists; scope reflects reality | Edit P-008 precondition + postcondition, keep MD001/MD025/MD041 named | U5 |

## Implementation Units

Each unit obeys the 2-hour rule (< 3 files, < 5 functions, < 4 test scenarios),
width isolation (single domain), and an atomic verifiable milestone.

### U1 — markdownlint config + ignore set (domain: config)

- **Changes**: Create `.markdownlint.json` with `{ "default": false, "MD001":
  true, "MD025": true, "MD041": true }`. Create `.markdownlintignore` listing:
  `.backlogit/`, `docs/archive/`, `docs/closure/`, `docs/compound/`,
  `.github/skills/`, `.copilot/`, `.copilot-tracking/`, `logs/` (and `node_modules/`
  defensively).
- **Files**: `.markdownlint.json`, `.markdownlintignore` (2 new files).
- **Load-bearing precheck (plan-review Architecture P2)**: markdownlint-cli2's
  first-class scoping is the `ignores` glob array inside its own config; honoring a
  separate `.markdownlintignore` is primarily a markdownlint-cli (v1) convention.
  **Empirically confirm** with the pinned cli2 version that the ignore file is
  consumed (run the linter with the file present vs absent and diff the linted file
  set). If cli2 does NOT honor `.markdownlintignore`, move the scope into an
  `ignores` array inside the config (or adopt the single `.markdownlint-cli2.jsonc`
  the deliberation considered). Do not let the scoping design rest on unverified
  auto-discovery.
- **Verification**: `npx markdownlint-cli2 "**/*.md"` (or the Makefile target from
  U2) exits 0 over the repo with the ignore set applied; a temporary scratch file
  with two H1s in a non-ignored dir would fail (spot-check, not committed). Only the
  three rules are active (no other rule IDs reported).
- **Posture**: config-first (the artifact is the config; validation is running the
  linter).

### U2 — Makefile `md-lint` target (domain: build/config)

- **Changes**: Add `md-lint: ## Run markdownlint-cli2 (MD001/MD025/MD041) over the
  scoped corpus` invoking `npx --yes markdownlint-cli2 "**/*.md"` (cli2 auto-reads
  `.markdownlint.json` + `.markdownlintignore`); add `md-lint` to the `.PHONY` line.
  do **not** touch `make.ps1`: it enumerates only Go-oriented targets
  (all/build/test/lint/vet/fmt/cover/clean/install/verify-plugin) and already omits
  the Makefile's `docs`/`docs-lint` targets, so there is no per-target docs parity to
  mirror (plan-review Scope P3 — resolved to Makefile-only).
- **Files**: `Makefile` (1 file).
- **Verification**: `make md-lint` runs markdownlint-cli2 and exits 0 on the current
  tree; the target is `.PHONY`.
- **Posture**: config-first. Depends on U1 (target invokes the U1 config).

### U3 — CI characterization test for the md-lint job (domain: tests)

- **Changes**: Extend `tests/integration/ci_compliance_test.go` (as `t.Run`
  subtests within one Test function, per file convention) with assertions that
  `ci.yml` defines a `md-lint` job with `name` set, `if: ${{ always() &&
  !cancelled() }}`, and that the `changes` job exposes a `md_touched` output consumed
  by the md-lint job's step gating (`needs.changes.outputs.md_touched != 'false'`).
  Add a **dedicated** assertion that resolves `wf.Jobs["md-lint"]`, locates its
  `actions/setup-node` step, and asserts the 40-char SHA — this is genuinely RED
  today (job absent) and guarantees the step's *presence*, since the existing
  job-iteration `TestAllActionsUseSHAPins` only validates SHA *format* on steps that
  already exist (plan-review Go P2). Assert **no trigger-level `paths:`/`paths-ignore:`**
  is introduced (Architecture P3). Reuse `readCIWorkflow`; add a small
  `findSetupNodeStep`-style helper (no drop-in exists — Go P3). Avoid asserting an
  unquoted `node-version` scalar (YAML int-vs-string trap, F013) — if asserted at
  all, require U4 to quote it. Do **not** assert `md-lint` as a required
  branch-protection context (it ships advisory-first — Learnings advisory).
- **Files**: `tests/integration/ci_compliance_test.go` (1 file).
- **Verification (test-first / RED→GREEN)**: the new assertions FAIL against the
  current `ci.yml` (no md-lint job) — RED — and PASS after U4 lands — GREEN.
  Scenario count ≤ 3 (job-exists+always-reporting+no-trigger-paths;
  setup-node presence+SHA; `md_touched` step-gating wiring). Optionally also assert
  the `.markdownlint.json` rule set (exactly MD001/MD025/MD041, `default:false`) as a
  standing guard for the plan's headline invariant (Constitution P2) — keep total
  scenarios ≤ 3 by folding this into the config subtest or tracking it as U1's own
  verification if it would exceed the granularity budget.
- **Posture**: characterization-first. Precedes U4 (write failing test before the
  workflow edit).

### U4 — md-lint CI job + classifier in ci.yml (domain: CI/config)

- **Changes**: In `.github/workflows/ci.yml`: (a) add a `md_touched` output to the
  `changes` job via a paths-filter step with a single pattern `md_touched:
  ['**/*.md']` (quantifier-invariant under the existing `predicate-quantifier:
  'every'`); (b) add a `md-lint` job (`needs: changes`, `if: ${{ always() &&
  !cancelled() }}`, `permissions: contents: read`) that checks out, sets up Node via
  **SHA-pinned** `actions/setup-node` (resolve the v-tag→SHA at implementation per
  F013), and runs `make md-lint` — every step gated on `needs.changes.outputs.md_touched
  != 'false'`, with a "skip" echo step for the `== 'false'` case. Do **not** add any
  trigger-level `paths:`/`paths-ignore:`.
- **Files**: `.github/workflows/ci.yml` (1 file).
- **Verification**: U3 characterization tests go GREEN; `go test
  ./tests/integration/ -run TestCI... ` passes; a docs-only PR runs the job's lint
  step, a code-only PR skips the step but the job still reports.
- **Posture**: characterization-first (U3 is its RED gate). Depends on U1, U2, U3.

### U5 — P-008 wording reconciliation (domain: docs/policy)

- **Changes**: In `.github/policies/workflow-policies.md` P-008: rewrite the
  **precondition** so it no longer claims the config already exists — instead state
  that the workspace **provides** `.markdownlint.json` (rules MD001/MD025/MD041,
  `default:false`) and `.markdownlintignore` (scoped corpus), reproducible via `make
  md-lint`. Adjust the **postcondition** to reflect **config-driven scoped
  enforcement** (markdownlint-cli2 over the configured globs honoring the ignore set)
  rather than a bare literal `markdownlint "**/*.md"`. Keep MD001/MD025/MD041 and the
  single-H1 / no-skip / first-line-H1 semantics named. Note the gate is advisory for
  its first green cycle, then promoted to required.
- **Files**: `.github/policies/workflow-policies.md` (1 file).
- **Verification**: precondition/postcondition match provisioned reality; `make
  docs-lint` (docline gate) still passes for the edited policy file. Depends on U1
  (references the actual provisioned filenames/scope).

## Dependency Graph

```text
U1 (config) ──┬─▶ U2 (Makefile)
              ├─▶ U4 (CI job)
              └─▶ U5 (P-008 wording)
U2 (Makefile) ─▶ U4 (CI job)   # job runs `make md-lint`
U3 (test, RED) ─▶ U4 (CI job)  # test-first: failing test precedes the workflow edit
```

No cycles. Suggested execution order: **U1 → U2 → U3 → U4 → U5** (U5 may run any
time after U1). U3 is authored RED before U4 and turns GREEN when U4 lands.

## Decisions and Rationale

- **Scoped ignore set over repo-wide fix or baseline** (deliberation Option B):
  Day-1 green + fail-closed for new files, without churning archival/template-generated
  content (Principle IV) or exceeding task granularity. markdownlint-cli2 has no
  native baseline, so Option C would add bespoke tooling and stale-baseline risk.
- **`.markdownlint.json` + `.markdownlintignore` filename pair**: honors P-008's
  existing `.markdownlint.json` name (minimal policy churn) and cleanly separates
  rule config from path scope; cli2 auto-discovers both. `.markdownlint-cli2.jsonc`
  (single file) was the considered alternative.
- **markdownlint-cli2 via `npx`**: current maintained tool; no committed
  `node_modules`, no `go.mod`/runtime impact (build/CI-time only) — keeps the CGo-free
  Go runtime unaffected (Principle VI).
- **Job inside `ci.yml`, not a standalone workflow**: reuses the `changes`
  classifier and matches the 089-S consolidation convention; a standalone workflow
  would duplicate checkout/setup and fragment the required-check surface.
- **`md_touched: ['**/*.md']` single pattern**: quantifier-invariant, so it is
  correct under the file's existing `predicate-quantifier: 'every'` (dorny compound).
- **Advisory-then-required rollout**: avoids blocking unrelated PRs before the gate
  is observed green once.

## Risks and Caveats

- **Fail-open CI gate**: a gate that never fires is a silent defect. Mitigation:
  `!= 'false'` step gating + `!cancelled()` job `if` + the U3 characterization test
  asserting the wiring.
- **New required check blocks PRs**: mitigation — ship advisory first; the ignore
  set guarantees Day-1 green; promote to required after one green cycle.
- **Node/npm dependency in a Go repo (Principle VI)**: justified as build/CI-time
  only; SHA-pinned action; no runtime footprint. Documented, not a silent add.
- **`npx` network fetch flakiness in CI**: mitigation — pin a markdownlint-cli2
  version in the `npx` invocation (e.g. `markdownlint-cli2@<ver>`) so the CI run is
  deterministic; `actions/setup-node` caching optional.
- **Ignore set drift vs inventory**: if a currently-dirty dir is omitted from the
  ignore set the gate goes red on Day 1. Mitigation — U1 verification runs the linter
  over the whole tree and must exit 0 before U4.
- **P-008/config divergence**: mitigation — U5 reconciles the policy text in the
  same shipment.

## Constitution Check

- **I. Safety-First Go** — N/A to production Go (no Go source changes); the only Go
  edit is a test (U3), which follows standard error/style conventions. `go vet` /
  `golangci-lint` / `gofmt` gates still apply to U3.
- **II. Test-First Development (NON-NEGOTIABLE)** — pass. U3 authors a failing
  characterization test (RED) before the U4 workflow edit (GREEN). Config/policy
  units (U1/U2/U5) are non-code; their verifiable milestone is the linter/docline
  gate exit-0.
- **III. Workspace Isolation & Security Boundaries** — pass. All files are created
  within the workspace root; no secrets; the CI job uses least-privilege
  `permissions: contents: read`.
- **IV. CLI Workspace Containment (NON-NEGOTIABLE)** — pass. All writes are in-tree.
  (Note: the decision to *ignore* template-generated `.github/skills/*` rather than
  edit them is driven by **durability/regeneration** — autoharness regeneration would
  overwrite hand edits — not by IV itself, which governs out-of-cwd writes. IV here
  attests only that every write lands in-tree; plan-review Constitution P2.)
- **VIII. Explicit Safety Modes for Elevated Risk** — pass (careful-mode posture).
  The elevated risk (a new required-check contract; fail-open/fail-closed CI gate) is
  handled explicitly via the `## Plan Hardening` ProposedAction/ActionRisk table and
  the advisory→required rollout, satisfying careful-mode enumeration and approval
  gating.
- **X. Agent Context Efficiency** — N/A. No agent tool surface or data-access pattern
  changes; the work is CI/config/docs only.
- **V. Structured Observability** — pass. The CI job reports as an always-run
  context; commits use conventional messages.
- **VI. Single Responsibility (NON-NEGOTIABLE-adjacent MUST/SHOULD)** — pass with
  documented justification. Adds a Node/CI-time dependency (markdownlint-cli2) —
  justified by a concrete requirement (P-008 enforcement), no runtime/`go.mod`
  impact, SHA-pinned action.
- **VII. Destructive Command Approval (NON-NEGOTIABLE)** — N/A. No destructive
  commands; all changes are additive files + a policy-text edit.
- **IX. Git-Friendly Persistence** — pass. All artifacts are human-readable
  JSON/YAML/Markdown.
- **XI. Merge Commit History Preservation (NON-NEGOTIABLE)** — pass. Ships via a
  merge commit like all work.

Constitution Check: pass

## Plan Hardening Signals

- Public API / schema / contract change: **present (minor)** — introduces a new CI
  status check that will become a **required** merge-gate contract. Justifies
  rollout care (advisory→required).
- Security / auth / permission / compliance-sensitive: **absent** — CI job is
  least-privilege read-only; no auth surfaces.
- Migration / backfill / destructive / irreversible: **absent** — all additive;
  fully reversible (remove job/target/config).
- External integration / operator checkpoint / external dependency: **present** —
  new Node/npm (markdownlint-cli2) build/CI-time dependency and `actions/setup-node`
  external action; operator decision on required-vs-advisory timing.
- High runtime / rollout / rollback risk: **present (moderate)** — a
  mis-scoped/fail-open gate could either block unrelated PRs or silently pass;
  paths-filter fail-safe semantics are load-bearing.

Requires plan hardening: yes

## Runtime Verification and Closure

- **U1/U2 (config, Makefile)** — runtime surface: developer/CI command. Verify
  `make md-lint` exits 0 locally; verify a deliberately-malformed scratch file in a
  non-ignored dir fails (then discard). Closure: none beyond CI green.
- **U3/U4 (CI)** — runtime surface: GitHub Actions. Verify on the PR that: (a) the
  `md-lint` context reports; (b) a docs touch actually runs the lint step; (c) the
  job is green. Closure artifact: note in the PR / closure record that the check is
  **advisory for one cycle**, with the rollback trigger = "remove the `md-lint` job
  and `md_touched` output; delete `.markdownlint.json`/`.markdownlintignore`/target"
  and owner = Ship. Promotion to a required check is a follow-up operator action.
- **U5 (policy)** — runtime surface: none. Verify `make docs-lint` still passes for
  the edited policy file.

## Out of Scope / Future Widening (sized, not harvested here)

The following bring currently-ignored directories into the enforced scope after
remediation. They are **deliberately deferred** (each is a separate future backlog
item) because they either churn archival/regenerated content or exceed the 2-hour /
< 3-files granularity; they are **not** part of this shipment. Honest sizing from the
inventory:

| Future bucket | Violations | Note |
|---|---|---|
| `docs/decisions` | 3 | Small live dir; realistic first widening candidate |
| `docs/memory` | 3 | Small; live |
| `docs/research` | 1 | Trivial |
| `docs/design-docs` | 4 | Small; live |
| `docs/reviews` | 5 | Small; live |
| `docs/exec-plans` | 23 | Larger; multi-file — split before widening |
| `plugin` | 8 | Product docs; separate domain |
| `.autoharness` | 3 | Config-adjacent |
| `docs/closure` / `docs/compound` / `docs/archive` | 74 / 38 / 20 | **Permanently ignored** (shipped history) |
| `.github/skills/*` | (subset of 14) | **Permanently ignored** (template-generated; Principle IV) |
| `.backlogit` | 49 | **Permanently ignored** (machine-generated; already excluded in CI) |

Widening a live bucket = "remediate MD025/MD041 in <dir>" + "remove <dir> from
`.markdownlintignore`", each scoped to that directory.

## Plan Hardening

**Hardening required?** Yes. Two signals are present: a new **CI gate that becomes
a required merge-contract** (rollout risk) and a **new external Node/CI dependency**
(`markdownlint-cli2` + `actions/setup-node`).

**Learnings and instruction files consulted:**

- `docs/compound/github-actions/F013-workflow-sha-pinning.md` — pin `actions/setup-node`
  to a full 40-char SHA (resolve tag→SHA at implementation); characterization-first
  YAML testing (RED before the workflow edit).
- `docs/compound/github-actions/dorny-paths-filter-every-quantifier-semantics-2026-07-04.md`
  — single-pattern `md_touched: ['**/*.md']` is quantifier-invariant; gate with
  `!= 'false'` (fail-safe) and keep the job at `${{ always() && !cancelled() }}` so a
  skipped step never drops a required context.
- `.github/instructions/ci-security.instructions.md` and
  `.github/instructions/workflows.instructions.md` — pinned actions, least-privilege
  `permissions`, no trigger-level `paths:`/`paths-ignore:` on required workflows.

**Protected invariants (must not regress):**

- The `changes`, `test`, `docs-lint`, and `cli-reference-drift` required contexts
  continue to report on every PR (existing 089-S contract). The new `md-lint` job is
  additive and must NOT alter their `if:` or trigger model.
- No workflow-level path filter is introduced; gating stays step-level.
- `ci.yml`'s `predicate-quantifier: 'every'` is unchanged; the new filter relies on
  single-pattern quantifier-invariance rather than flipping the quantifier.

**Risky actions (ProposedAction / ActionRisk / ActionResult):**

| ProposedAction | ActionRisk | Approval | Rollback | ActionResult |
|---|---|---|---|---|
| Add `md-lint` CI job + `md_touched` output to `ci.yml` | moderate (rollout/contract) | operator decides required-vs-advisory timing | Revert the `ci.yml` hunk (remove job + output) | planned |
| Introduce `markdownlint-cli2` via `npx` + SHA-pinned `actions/setup-node` | moderate (external dependency) | none for advisory; note in PR | Remove the target/job; delete config files | planned |
| Provision `.markdownlint.json` + `.markdownlintignore` | low (additive config) | none | Delete the two files | planned |
| Edit P-008 precondition/postcondition wording | low (docs) | none | Revert the policy hunk | planned |

**Reinforced verification:**

- CI environment precheck: confirm `actions/setup-node` SHA resolves and Node is
  available before the lint step; pin the `markdownlint-cli2` version in the `npx`
  call for deterministic runs.
- Must-RUN assertion: a docs-only PR actually executes the lint step (not silently
  skipped). Required-SKIP canary: a code-only PR skips the step yet the `md-lint`
  context still reports green.
- The U3 characterization test is the standing guard that the job exists, is
  always-reporting, and is SHA-pinned.

**Reinforced operational closure:**

- **Monitoring signal**: the `md-lint` check status on PRs.
- **Rollback trigger**: the gate blocks unrelated PRs or fails to fire on a docs
  change → remove the `md-lint` job/output and config in a follow-up revert.
- **Rollback procedure**: single-hunk revert of `ci.yml` + deletion of
  `.markdownlint.json`/`.markdownlintignore`/`md-lint` target.
- **Owner**: Ship agent during the rollout PR; operator for the required-check
  promotion decision.
- **Validation window**: one full green PR cycle in advisory mode before promoting
  the check to required.

**Unresolved operator decisions (carried forward):**

- Promote `md-lint` to a **required** status check now or after one green cycle
  (plan recommends: after one cycle).
- Final widening schedule for live directories (plan defers all widening).
- Exact `actions/setup-node` pin SHA (Ship resolves at implementation).

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved

**Gate rationale.** Five reviewer personas were dispatched as independent
sub-agents and all returned complete findings (`TOOL_OK: reviewer-subagent-dispatch`):
Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher
(always-on), and Architecture Strategist (cross-model, always-triggered).
Agent-Native Parity Reviewer and Security Lens Reviewer were **not triggered** — the
plan exposes no MCP tools/agent-facing actions and touches no auth/authz, API
surfaces, sensitive data stores, external trust-boundary integrations, or secrets
(it is CI/config/docs only). Merged result: **0 P0, 0 P1, several P2, several P3.**
Per the gate table, P2-only ⇒ **ADVISORY**. Structural gates pass: the plan carries a
`## Constitution Check` ending in `Constitution Check: pass`, hardening signals are
declared (`Requires plan hardening: yes`), and a `## Plan Hardening` section with a
`ProposedAction`/`ActionRisk` table is present.

**Operator authorization.** As Stage, I judge every advisory to be implementation-level
hardening for Ship (not plan-invalidating), and the highest-value P2s have already been
folded into the plan (see edits to U1, U2, U3, and the Constitution Check). Recorded
`operator_authorization: approved`; proceeding to harvest.

**Plan hardening:** required (yes) and satisfied — the `## Plan Hardening` section adds
verification canaries, rollback trigger/procedure, owner, and validation window, and
classifies risky actions.

### Findings by severity

**P2 (folded into the plan; Ship must action):**

- *Architecture (load-bearing)* — Verify markdownlint-cli2 actually honors
  `.markdownlintignore` (its native scoping is an `ignores` array); the whole
  248-violation scoping design depends on it. → Folded into **U1** as a load-bearing
  precheck with a fallback to an `ignores` array / `.markdownlint-cli2.jsonc`.
- *Go + Constitution* — U3 should add a **dedicated** `md-lint`→`actions/setup-node`
  presence+SHA assertion (the iteration test only checks SHA *format* on existing
  steps) and should guard the config's **exactly-three-rules** invariant. → Folded
  into **U3**.
- *Constitution* — Constitution Check omitted Principles VIII and X, and mis-labeled
  the `.github/skills` ignore rationale as IV. → **Constitution Check** updated (VIII
  pass, X N/A, IV wording corrected).
- *Architecture* — Advisory-then-required promotion is the true enforcement seam and
  is currently an untracked operator decision; until promoted the gate is
  observational. → Carried as an explicit unresolved operator decision + closure
  owner; recommend a follow-up backlog item for the branch-protection promotion.

**P3 (advisory, no plan change required):**

- *Scope* — `.markdownlintignore` includes dirs with no measured violations
  (`.copilot*`, `node_modules`) — mild YAGNI; acceptable as defensive.
- *Go* — YAML int-vs-string trap if asserting `node-version`; use `t.Run` subtests;
  add a `findSetupNodeStep` helper. → Noted in U3.
- *Architecture* — Prefer folding `md_touched` into the existing `classify`
  paths-filter step rather than a 4th dorny invocation (cohesion/cost); name-proximity
  of `docs-lint` vs `md-lint` — document the boundary in U5; keep `make md-lint`
  isolated from Go targets with a clear "Node/npx required" message.
- *Learnings* — Pin an exact `markdownlint-cli2@<ver>` in the `npx` call (already in
  the plan); quote `#` and the `**/*.md` glob when editing P-008 (U5) to avoid
  docline truncation; verify both dorny canaries (must-RUN + required-SKIP) at PR time.

**Runtime verification / closure:** present and adequate — the plan's Runtime
Verification and Closure section defines per-unit verification, a rollback trigger and
procedure, owner, and a one-cycle validation window; plan-review confirmed no gaps.

**Dispatch integrity:** `dispatch_mode: multi-agent-dispatch`; every selected persona
completed and returned findings — full-fidelity gate, no partial coverage.

