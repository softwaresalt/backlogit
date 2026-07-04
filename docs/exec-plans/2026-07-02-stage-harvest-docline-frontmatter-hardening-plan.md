---
chunk_strategy: h1-h2-h3
description: 'Harden the Stage harvest pipeline so generated exec-plans are born-compliant with the docline frontmatter contract (doc_type plan plus top-level title and source) and a pre-harvest docline lint gate blocks invalid frontmatter before it can ride into a Ship feature PR. Root cause: 075-S PR #164 Docline gate blocker. Scope is harness/skill only (impl-plan and harvest SKILL.md); no Go code change; direction 3 (harvest delivery mechanism) deferred.'
doc_type: plan
ingested_at: "2026-07-02T20:45:44-07:00"
schema_version: "1.0"
source: docs/exec-plans/2026-07-02-stage-harvest-docline-frontmatter-hardening-plan.md
title: 'Harden Stage harvest: born-compliant plan frontmatter + pre-harvest docline lint gate'
---

## Source

- Stash: `A9D74372` (kind=task, priority=medium) — "Harden Stage harvest so an un-pushed
  local-main harvest commit with invalid docline frontmatter cannot ride into a Ship feature
  PR and break CI."
- Origin: process defect observed during the 075-S ship cycle. In PR #164 the Stage harvest
  commit `f316dfd` rode into the Ship feature branch (feature branch was cut off un-pushed local
  `main`); its plan file used `doc_type: exec-plan` (outside the closed vocabulary) and omitted
  top-level `title` + `source`, failing the "Docline frontmatter gate" CI check (3 violations) and
  blocking the PR until an operator-approved P-010 override let Ship fix the frontmatter. Captured
  by the Orchestrator process note 2026-07-02. See memory checkpoint
  `docs/memory/2026-07-02-075-S-HALT-inherited-stage-commit.md`.
- Prior art / compound learning (canonical prior, HIGH-confidence match):
  `docs/compound/2026-06-26-docline-frontmatter-contract.md` — the four-part sustainable-contract
  pattern. Two parts map directly onto this chore: **(3) born-compliant generation** ("teach the
  generator to emit compliant frontmatter at generation time so generated docs never break the
  gate") and **(4) CI lint gate** (`make docs-lint` -> `backlogit docs lint`). This plan applies
  the same pattern to the *plan generator* (the impl-plan skill) and shifts the lint gate left into
  the Stage harvest step. The same learning's YAML pitfall (unquoted `#` / `:` silently truncates a
  scalar) is folded into the frontmatter guidance because plan titles/descriptions routinely cite PR
  numbers.
- Contract source of truth: `internal/docline/policy.go` (closed `doc_type` vocabulary + authoring
  required-field set), `internal/docline/classify.go` (path -> doc_type), the authoring guide
  `docs/docline-frontmatter-authoring-guide.md`, and the green reference exec-plan
  `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md`.
- No separate deliberation artifact was created. Investigation confirmed the stash's root-cause
  hypothesis exactly (missing frontmatter guidance in the impl-plan skill + no pre-commit lint gate
  in harvest); there was no material divergence to deliberate. The option analysis over remediation
  directions 1/2/3 is captured in Decisions and Rationale below, per the lean-plan directive for a
  well-understood single-domain harness change. This mirrors the green reference, which likewise
  folded its decision into the plan.

## Problem Frame

The docline contract (authoring profile) requires three top-level frontmatter fields on in-scope
docs — `title`, `source`, `doc_type` — and `doc_type` must be a member of the closed vocabulary
`{reference, decision, spike, plan, closure, research, review, learning, spec, design, guide}`
(`internal/docline/policy.go`). Path classification maps `docs/exec-plans/**` to `doc_type: plan`
(`pathRules` in `policy.go`; confirmed by `internal/docline/classify.go`). The CI job `docs-lint`
("Docline frontmatter gate", `.github/workflows/ci.yml`) runs `make docs-lint` ->
`go run ./cmd/backlogit docs lint` (authoring profile) on every PR.

Two harness gaps let an invalid plan reach CI:

1. **No frontmatter contract in the plan generator.** `.github/skills/impl-plan/SKILL.md`
   specifies the plan's *body* sections (Problem Frame, Requirements Trace, Implementation Units,
   Plan Hardening Signals, etc.) but says **nothing** about the required docline frontmatter block.
   An author following the skill improvises the frontmatter — the observed failure mode was
   `doc_type: exec-plan` (a natural but invalid guess) with no `title`/`source`.

2. **No pre-harvest / pre-commit lint gate.** `.github/skills/harvest/SKILL.md` reads the plan and
   creates the backlog hierarchy, but never validates the plan's frontmatter against the docline
   contract. Combined with Stage committing harvest artifacts to local `main` (and Ship cutting the
   feature branch off that un-pushed commit), an invalid plan rides into the Ship PR and only
   surfaces as a red CI gate there — the most expensive place to catch it.

Note on scope safety: `internal/docline/policy.go` `scopeExcludeDirs` excludes `.github/` and
`docs/memory/` from linting, so editing the SKILL.md files does not itself create docline
obligations, and the fix targets the exact surface (`docs/exec-plans/**`) that *is* linted.

## Requirements Trace

| Stash remediation direction | Disposition | Implementation action |
|---|---|---|
| (1) Fix plan generation so it emits `doc_type: plan` + top-level `title` + `source` | **IN SCOPE** | Unit A: add a Plan Frontmatter Contract section + mandatory self-lint step to `impl-plan/SKILL.md`. |
| (2) Add a `backlogit docs lint` gate to the Stage harvest step before commit | **IN SCOPE** | Unit B: add a pre-decomposition docline lint gate phase to `harvest/SKILL.md` that HALTs on violations. |
| (3) Change how the harvest commit reaches the feature branch (Stage push / dedicated delivery) | **DEFERRED** | Out of scope: larger blast radius, alters branch/push topology owned partly by Ship. Recorded as follow-up stash `EED25928` (which also tracks the upstream `.tmpl` drift). Directions (1)+(2) fully close the observed frontmatter defect. |

## Implementation Units

### Unit A — Born-compliant plan frontmatter in the impl-plan skill

- **Domain**: docs / harness (single skill file).
- **File(s)**: `.github/skills/impl-plan/SKILL.md` (1 file).
- **Change**: Add a new **"Plan Frontmatter Contract (REQUIRED)"** subsection to the Phase 3
  ("Structure the Plan") output spec. It must instruct the author to emit, as the first block of the
  plan file, a docline frontmatter block that:
  - sets `doc_type: plan` (never `exec-plan`; the closed vocabulary value for `docs/exec-plans/**`);
  - sets a top-level `title` (single-quoted) and a top-level `source` equal to the plan's own
    repo-relative POSIX path;
  - includes `description`, `schema_version`, and `chunk_strategy` to match the green reference
    `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md` — these three are
    **recommended for green-reference parity**; only `title`, `source`, and `doc_type` are
    **strictly required** by the authoring-profile gate that failed in 075-S;
  - single-quotes any scalar containing `#`, `:`, or a leading special character (YAML truncation
    pitfall from the compound learning), since plan titles/descriptions cite PR numbers;
  - optionally derives `source`/`doc_type` deterministically via `backlogit docs migrate` — run the
    dry-run/plan first to review the diff, then `--apply --yes --path docs/exec-plans/<file>` to
    write — instead of hand-setting. Prefer the diff-first flow: `--apply --yes` is an in-place
    overwrite and, although the target is git-tracked (revertible), standing guidance should show the
    operator the diff before writing (Principle VII).
  Add a **mandatory verification step**: after writing the plan, run the docline linter **via the
  same entrypoint CI uses**. The repo-wide `make docs-lint` target runs `go run ./cmd/backlogit
  docs lint` with no arguments over all authored docs; to check a single file, use the scoped
  direct form `go run ./cmd/backlogit docs lint --path docs/exec-plans/<file>` (the `--path` flag
  belongs to `backlogit docs lint`, not to `make docs-lint`). Confirm `valid` / `0 violations`
  before the plan is considered complete; treat any violation as a blocker to fix in place. Using
  the source entrypoint (not a possibly-stale installed `backlogit` binary) guarantees the
  self-lint agrees with the CI Docline gate and cannot pass locally while CI fails.
- **Execution posture**: documentation change (no test framework); verified behaviorally.
- **Verification / acceptance**:
  - The skill contains a Plan Frontmatter Contract subsection naming `doc_type: plan`, top-level
    `title`, and top-level `source`, and referencing the green reference shape.
  - The skill mandates a post-authoring `backlogit docs lint --path` check with a 0-violation bar.
  - **Behavioral proof**: a plan authored per the corrected guidance passes
    `backlogit docs lint` with 0 violations. (This very plan file is the first instance and MUST
    pass — see Runtime Verification and Closure.)

### Unit B — Pre-harvest docline lint gate in the harvest skill

- **Domain**: docs / harness (single skill file).
- **File(s)**: `.github/skills/harvest/SKILL.md` (1 file).
- **Change**: Insert a **"Phase 1.5: Docline frontmatter gate (pre-decomposition)"** between the
  existing Phase 1 (Validate the reviewed plan) and Phase 2 (Parse the plan structure). The gate:
  - runs the docline linter (authoring profile) against the incoming plan **via the same entrypoint
    CI uses** — the repo-wide `make docs-lint` (`go run ./cmd/backlogit docs lint`, no arguments)
    or, to scope a single plan, the direct `go run ./cmd/backlogit docs lint --path <plan_path>`
    (the `--path` flag belongs to `backlogit docs lint`, not `make docs-lint`) — before any backlog
    mutation or Stage harvest commit, so the gate cannot
    pass on a stale installed binary while CI (source) fails;
  - on any violation, **HALTS** decomposition, reports the violations, and directs the author to fix
    the plan frontmatter (or run `backlogit docs migrate --apply --yes --path <plan_path>`) and
    re-run the gate — explicitly citing the 075-S root cause so the reason is legible;
  - is documented as the shift-left guard that prevents an invalid Stage-authored doc from riding
    into a Ship feature PR. Add it to the Guardrails list ("Do not decompose or commit a plan that
    fails `backlogit docs lint`").
  - **Commit-ordering (defect closure):** both gates sit upstream of every path by which the plan
    reaches a commit. Unit A's mandatory self-lint runs at authoring time, before the Stage harvest
    commit is created; Unit B's Phase 1.5 gate runs before decomposition and therefore before the
    harvest commit. No commit path bypasses both — this is what closes the 075-S vector where the
    harvest commit `f316dfd` carried an unvalidated plan into the feature branch. (The harvest skill
    itself does not commit — it creates backlog items; the commit is the enclosing Stage step, which
    both gates precede.)
- **Execution posture**: documentation change; verified behaviorally against a crafted bad plan.
- **Verification / acceptance**:
  - The harvest skill has a Phase 1.5 gate that runs `backlogit docs lint` on the plan and halts on
    violations before Phase 2 / any create call.
  - **Behavioral proof**: running `backlogit docs lint` against a plan whose frontmatter uses
    `doc_type: exec-plan` (or omits `title`/`source`) reports violations (non-zero), i.e. the gate
    would reject it; running it against a compliant plan reports 0 violations (gate passes).
  - The Guardrails section forbids decomposing/committing a plan that fails the lint gate.

## Dependency Graph

- Unit A and Unit B are **independent** (different files, different concerns) and may be executed in
  either order or in parallel. No hard dependency edge is wired.
- Soft narrative order (A then B) is preferred only for coherence: A prevents the violation at the
  source; B is the defense-in-depth net that catches any future regression. Ship may order them
  freely.

## Decisions and Rationale

- **D1 — Scope to directions (1)+(2); defer (3).** Directions (1) born-compliant generation and
  (2) shift-left lint gate together fully close the observed defect (invalid plan frontmatter
  reaching CI) at the two cheapest points: authoring and pre-harvest. Direction (3) — changing how
  the harvest commit reaches the feature branch (Stage pushes its harvest commit, or a dedicated
  harvest-delivery mechanism) — is a branch/push-topology change with a larger blast radius that is
  partly Ship's concern and not required to fix the frontmatter defect. Deferring it honors YAGNI
  and keeps the shipment tight. It is recorded as follow-up stash `EED25928` (created during this
  Stage session), which also carries the upstream `.tmpl`-drift remainder from D3, rather than
  bloating this unit.
- **D2 — No Go code change.** The `backlogit docs lint` (authoring profile, `--path` filter),
  `docs migrate`, and `docs classify` commands already exist and behave correctly (verified against
  binary v1.2.0). The gap is purely instructional (skill guidance) — the tooling is ready. This is a
  harness/docs-only change; no `internal/**` or `cmd/**` code is touched, so no TDD-first Go work is
  triggered.
- **D3 — Fix lives in the SKILL.md files, not the agent files.** No `.tmpl` template files exist in
  this repository; the agent/skill generator templates live in the external autoharness source. The
  authoritative, in-repo, tracked artifacts that the running Stage agent loads at runtime are
  `.github/skills/impl-plan/SKILL.md` and `.github/skills/harvest/SKILL.md`. Editing them changes
  live behavior without touching the operator's in-flux agent files
  (`.github/agents/.stage.agent.md` etc.). The upstream `.tmpl` should also be updated to prevent
  regeneration drift — recorded as a caveat, out of this repo's tree.
- **D4 — Covering release unit is a `feature`.** The backlogit type system has no `chore` artifact
  type (types: deliberation, feature, shipment, review, task, bug, subtask). `feature` (level 1)
  allows `task` children; `task` (level 2) allows `subtask`/`bug` children. So the "chore" is
  modeled as a covering **feature** with two task children — satisfying the parent-first covering
  invariant. No subtasks are created: each task is a single-file, sub-2-hour edit; further
  decomposition would be over-engineering (YAGNI).
- **D5 — Belt-and-suspenders, not either/or.** The stash offered (1) and/or (2). Both are adopted:
  (1) makes correct output the default (prevention), (2) makes incorrect output impossible to
  harvest/commit (enforcement). The compound learning shows a contract is only trustworthy when the
  generator is born-compliant *and* a gate enforces it.

## Risks and Caveats

- **Upstream template drift**: because the fix lives in generated SKILL.md files while the
  `.tmpl` sources live outside this repo, a future `Auto-Tune`/`Auto-MergeInstall` regeneration
  could overwrite the SKILL.md edits. Mitigation: note in each edited skill that the frontmatter
  contract / lint gate is load-bearing; flag the upstream `.tmpl` update as a follow-up. (Low
  likelihood in the near term; the in-repo skills are the live contract.)
- **Gate depends on a working `backlogit` binary**: the harvest lint gate assumes `backlogit docs
  lint` is available in the Stage environment. This is already a Stage prerequisite (Step 0.0 tool
  gate) and the CI gate uses the same command, so no new dependency is introduced.
- **Deferred direction (3)**: until (3) is addressed, an *unrelated* Stage artifact defect (one the
  docline gate does not check, e.g. scope of committed files) could still ride into a Ship PR. This
  plan intentionally scopes to the frontmatter defect only; the follow-up stash tracks (3).

## Plan Hardening Signals (REQUIRED)

- public API, schema, or contract change — **ABSENT**. No code, schema, or public API changes; the
  docline contract itself is unchanged (this plan aligns authoring *to* the existing contract).
- security, auth, permission, or compliance-sensitive behavior — **ABSENT**. No auth/security
  surface touched.
- migration, backfill, destructive data/config action, or irreversible step — **ABSENT**. Edits are
  additive documentation changes to two skill files; fully reversible via git.
- external integration, operator checkpoint, or external dependency — **ABSENT**. No new external
  dependency; `backlogit docs lint` is an existing in-repo command.
- high runtime, rollout, or rollback risk — **ABSENT**. Doc-only change; rollback is a revert.
  (Advisory: Unit B adds a HALT enforcement gate to the Stage harvest *workflow* control flow — a
  behavior change, but revert-reversible and not a product/public contract, so it does not trip the
  hardening threshold.)

Requires plan hardening: no

## Runtime Verification and Closure

- **Changed runtime surface**: none in the product (no CLI/API/MCP/UI/job behavior changes). The
  changed surface is the *agent authoring workflow* (how Stage generates and gates plans).
- **Runtime verification that should pass before absorption**:
  - Unit A: `backlogit docs lint --path docs/exec-plans/<new-plan>` returns `valid` / 0 violations
    for a plan authored per the corrected skill. **This plan file is the first proof instance and is
    validated as part of this Stage session (see Step 6 report).**
  - Unit B: `backlogit docs lint` reports violations for a deliberately non-compliant plan
    (`doc_type: exec-plan` or missing `title`/`source`) — demonstrating the gate would halt — and 0
    violations for a compliant plan.
  - Repo-wide: `make docs-lint` stays green (no regression to the existing corpus).
- **Operational closure**: no monitoring/rollback trigger needed for a doc-only change. Closure
  artifact = the merged skill edits + this plan passing the docline gate (the self-demonstrating
  proof that the fix works).

## Standards Check

- **2-Hour Rule**: each unit edits a single skill file (< 3 files, no functions, behavioral
  verification) — well under 2 hours.
- **Width Isolation**: both units are single-domain (docs/harness). No mixing of code + docs +
  tests.
- **Atomic Milestone**: each unit has a verifiable exit state (skill contains the section; a plan
  authored/checked against the guidance lints clean; the gate rejects a bad plan).
- **P-003 (decomposition integrity)**: source (stash `A9D74372`) -> this plan -> covering feature ->
  two tasks, each with >= 1 acceptance criterion. No orphan tasks.
- **P-005 / P-006 (gates)**: plan-review gate runs before harvest; hardening not required
  (all signals absent) and declared explicitly.
- **P-010 (role boundary)**: Stage authors decision/plan/backlog artifacts only; no code, branches,
  builds, or PRs. Implementation of Units A/B is Ship's scope.
- **P-012 (tool availability)**: fix relies on the already-verified `backlogit docs` command surface.
- **Principle II (test-first) — documented deviation**: the covering unit is typed `feature`, and
  Principle II normally requires a compiling-but-failing test harness before implementation. This is
  a docs/harness-only change (two SKILL.md edits) with no compilable product code, so no Go test
  harness is possible. Substituted verification: behavioral `backlogit docs lint` assertions (a
  compliant plan lints clean; a `doc_type: exec-plan` plan is rejected) plus the repo-wide
  `make docs-lint` gate. Recorded here as a justified, governance-visible deviation rather than an
  unstated skip. Any future Go change under this feature (none planned) reverts to test-first.
- **Principle VII (destructive-command approval)**: `backlogit docs migrate --apply --yes` is an
  in-place overwrite; guidance prefers a diff-first flow and relies on git-tracked reversibility as
  the safety net. No product data is mutated.
- **Principle IV (workspace containment)**: the deferred upstream `.tmpl` update (follow-up stash
  `EED25928`) MUST be performed in the external autoharness repository, never via out-of-tree writes
  from this workspace.
- **Ship role-boundary (P-010) handoff**: implementation edits target `.github/skills/**` harness
  files, which fall under Ship's "Source code / harness" Allowed column; the covering feature is
  therefore assignable to Ship for execution.

## Plan Review

**Gate decision: ADVISORY (PASS with advisories) — cleared for harvest.**

Reviewed via the plan-review skill with two always-on personas run in parallel (Scope Boundary
Auditor, Constitution Reviewer). Cross-model Architecture Strategist / Security Lens / Agent-Native
Parity personas were not triggered: the plan changes no product architecture, no auth/security/data
surface, and exposes no MCP tools — it edits two harness SKILL.md files only.

- **Plan hardening required?** No. All five hardening signals are ABSENT and the plan declares
  `Requires plan hardening: no` explicitly (docs-only, additive, git-reversible). The P-006 gate is
  satisfied without invoking plan-harden.
- **Findings:** 0 P0, 0 P1, 4 P2, 5 P3. No blocking findings; verdict is ADVISORY, not FAIL.

### P2 (addressed in this revision before harvest)

1. **Gate/self-lint version-skew (Constitution).** The lint gate and Unit A self-lint must invoke
   the *same entrypoint CI uses* (`make docs-lint` / `go run ./cmd/backlogit docs lint`), not a
   possibly-stale installed `backlogit` binary — otherwise the gate could pass locally while CI
   fails, negating the plan's purpose. *Resolved:* Units A and B now pin the CI entrypoint.
2. **Deferred direction (3) had no durable trace (Scope).** *Resolved:* follow-up stash `EED25928`
   created this session and cited in the Requirements Trace and D1.
3. **Commit-ordering not explicitly traced (Scope).** *Resolved:* Unit B now states both gates sit
   upstream of every commit path (Unit A self-lint at authoring; Unit B Phase 1.5 pre-decomposition).
4. **Principle II (test-first) not mapped (Constitution).** *Resolved:* Standards Check now records
   the docs-only test-first deviation as a justified, governance-visible entry.

### P3 (advisory — folded in)

- Required-vs-recommended frontmatter fields labeled in Unit A (only `title`/`source`/`doc_type` are
  gate-required).
- `docs migrate --apply --yes` reframed to a diff-first flow with git-reversibility noted (Principle
  VII).
- Ship role-boundary confirmed to cover `.github/skills/**` harness edits (P-010 handoff).
- Upstream `.tmpl` follow-up constrained to the autoharness repo, never out-of-tree (Principle IV);
  folded into stash `EED25928`.
- Unit B HALT-gate control-flow note added to the hardening signals block (advisory).

### Runtime verification / closure readiness

Adequate for a docs-only change: the plan is self-demonstrating — this file was authored under the
proposed Unit A guidance and passes `backlogit docs lint` with 0 violations; the repo-wide corpus
remains green. No monitoring/rollback artifact required (rollback = git revert).

**Decomposition cleared.** Source (stash `A9D74372`) → this plan → covering feature → two
acceptance-bearing tasks. Proceed to harvest.
