---
chunk_strategy: h1-h2-h3
description: 'Deliberation for the release-and-docs hygiene grouping (stash 9140F65C npm-publish workflow hygiene + B55985DD docs-lint --path wording cleanup). Confirms the covering feature scope, the npm-publish token-presence gating topology, the archive-file edit mechanism, and the external-provisioning scope carve-out; defers EED25928 and 21E17BFC with rationale.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-04-release-docs-hygiene-deliberation.md
title: 'Release pipeline and documentation hygiene: covering-feature scope and gating decisions'
stash_id: 9140F65C
decision_status: decided
promoted_to: plan
---

## Source

- Stash: `9140F65C` (kind=task, priority=low, age 3d) — "Fix/enable npm package publishing in the Release workflow." Actionable in-repo slice = npm-publish token-presence gate + retired package-template validation. External steps (provision the legacy scoped package, add `NPM_TOKEN` secret) require human-only npm-org + repo-secret provisioning.
- Stash: `B55985DD` (kind=task, priority=low, age 1d) — "Fix misleading `make docs-lint --path` wording in 076-S ride-along artifacts." Reword two docs so a fixed repo-wide `make docs-lint` is not implied to accept `--path`.
- Session: operator "Stage next" with a bias toward promotion (forward progress on remaining backlog).

## Grouping rationale (Step 1.5)

Two task-shaped, actionable, low-priority entries were eligible. Both are post-release / post-review **hygiene cleanup** born of the release pipeline and PR-review process. They are independent (no dependency edge), occupy **distinct skill domains** (CI/config, shell/test, docs prose), and each is comfortably under the 2-hour rule.

**Grouping options considered:**

- **Option A (CHOSEN) — single covering feature "Release pipeline & documentation hygiene"** carrying both entries as width-isolated tasks. One shipment clears two stale entries and produces one PR, maximizing forward progress with negligible added cost. Coherence: "release & docs hygiene" is a legitimate covering abstraction for janitorial work surfaced by the release/review process.
- Option B — promote only `9140F65C` (higher value); leave `B55985DD` deferred or as a ride-along. Rejected: `B55985DD` is tiny and independent; bundling it costs almost nothing and clears a second stale low-priority entry, which better serves the operator's forward-progress goal.

There is **no `chore` artifact type** in this backlogit workspace (valid types: feature, epic, task, subtask, bug, spike, review). Consistent with repo convention (e.g. `076-F` "Harden Stage harvest"), the maintenance covering unit is typed **`feature`**.

## Problem frame

1. **npm-publish red X (`9140F65C`).** `.github/workflows/release.yml` job `npm-publish` (`continue-on-error: true`) fails at "Publish platform packages" on every release because the legacy package scope is unprovisioned and/or `NPM_TOKEN` is absent (ENEEDAUTH / E404 / 403). `continue-on-error` keeps the GitHub Release unblocked, but the job still surfaces a red X on every release. The **in-repo** remedy is to make the publish steps *not run* (hence not fail) when the token is intentionally absent, and to validate that the retired packaging script emits publishable `package.json` files for the platform packages and wrapper.
2. **Misleading docs-lint wording (`B55985DD`).** `make docs-lint` is a fixed repo-wide invocation (`go run ./cmd/backlogit docs lint`, no args); scoping requires the direct `go run ./cmd/backlogit docs lint --path <file>` form. Two ride-along docs imply `make docs-lint` is `--path`-narrowable: `docs/exec-plans/2026-07-02-stage-harvest-docline-frontmatter-hardening-plan.md` (~L104, ~L124) and `.backlogit/archive/076.002-T.md` (~L22).

## Options and decisions

### Decision 1 — npm-publish gating topology (real open question)

GitHub Actions does **not** expose the `secrets` context in a job-level `if:`, so `if: secrets.NPM_TOKEN != ''` at the job level is invalid. Env-indirection is required.

- **Option 1 (CHOSEN) — step-level token-presence guard inside `npm-publish`.** A preflight step reads `secrets.NPM_TOKEN` into a step output (`has_token`); the two publish steps are gated `if: steps.preflight.outputs.has_token == 'true'`. Keep `continue-on-error: true` as defense-in-depth. Lowest blast radius (1 file, no new job, no new `needs` edges); eliminates the red X in the intended absent-token state; preserves the invariant "npm publish failure must never block the GitHub Release."
- **Option 2 — job-level gate via a separate `npm-preflight` job** whose output drives `npm-publish`'s `if:`. UI shows the job as *skipped* (marginally cleaner) but adds a job and `needs`-graph wiring (higher blast radius). Rejected for scope.

**Chosen: Option 1.** Rationale: minimal, reversible, single-file change that directly removes the recurring red X while keeping the intentional "never block release" behavior.

### Decision 2 — `.backlogit/archive/076.002-T.md` edit mechanism (backlogit-managed terminal state)

- **Option 1 (CHOSEN) — edit the prose body in place + `backlogit sync`.** The misleading wording is in the body paragraph (L22), not schema-critical frontmatter; `.backlogit/archive/` is outside the docline lint scope, so this is low risk and keeps the historical record accurate.
- **Option 2 (documented fallback) — plan-only fix.** If in-place archive edit + sync proves problematic for Ship, fixing only the live `docs/exec-plans/...` doc and leaving the archived record annotated is acceptable.

**Chosen: Option 1, with Option 2 as an explicit Ship-latitude fallback.**

### Decision 3 — external-provisioning scope carve-out

npm-publish steps (1) provision the `@backlogit` npm org/scope and (2) create + add the `NPM_TOKEN` repo secret require **human-only** npm-org and repo-secret provisioning that no agent (Stage or Ship) can perform. These are **out of scope** for this shipment. To avoid losing the work when `9140F65C` is archived, a narrowly-scoped follow-up entry is re-stashed capturing exactly the external provisioning steps. Once the scope + secret exist, the token-presence guard from Decision 1 will let the publish steps run automatically.

## Deferrals (not promoted this session)

- **`EED25928`** (task, 1d) — **DEFERRED.** Part (a) changes how the Stage harvest commit reaches the Ship feature branch (branch/push topology, larger blast radius, partly Ship-owned) — an evaluation/design item, not a quick fix. Part (b) targets the generating `.tmpl` sources in the **external autoharness repo**, outside this workspace tree; Constitution **Principle IV (CLI Workspace Containment, NON-NEGOTIABLE)** forbids out-of-tree writes, so part (b) is **flagged NOT ACTIONABLE here** by design. Remains in stash.
- **`21E17BFC`** (feature, 81d) — **DEFERRED.** Contingency item (singleton MCP server w/ multiplexed transport). Triage 2026-04-23: all four SQLite-contention fixes shipped in `031-F`; the contingency trigger (recurring multi-process contention under real workloads) is **not met**. No evidence found this session that the trigger fired. Remains in stash.

## Prior learnings (Step 1.8)

Compound library has no npm-publish / CI-secret-gating prior art. The one directly relevant entry, `docs/compound/2026-06-26-docline-frontmatter-contract.md`, is already honored in the downstream plan's frontmatter authoring. Confidence: low. Proceed without additional prior-art constraints.

## Outcome

Promote both entries under one covering **feature** "Release pipeline & documentation hygiene" with three width-isolated tasks (npm-publish token guard [config]; retired packaging-script output validation [shell/test]; docs-lint `--path` wording cleanup [docs]). Proceed to implementation planning.
