---
title: "Documentation consistency and README brochure rework for backlogit v1.7.0"
description: "Decision to run a broad drift audit across user-facing docs and fully restructure the top of README.md with a brochure-style value proposition, aligned to the shipped v1.7.0 CLI/MCP surface (stash ECC3C116)."
source: docs/decisions/2026-07-24-docs-consistency-readme-brochure-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "Bring user-facing docs into consistency with v1.7.0 and rework README top as a product brochure"
depth: standard
decision_status: decided
promoted_to: plan
stash_id: ECC3C116
linked_artifacts:
  - "docs/exec-plans/2026-07-24-docs-consistency-readme-brochure-plan.md"
tags:
  - "docs"
  - "readme"
  - "documentation-consistency"
  - "cli-mcp-surface"
  - "v1.7.0"
---

## Source

- Stash: `ECC3C116` (kind=feature, priority=medium, age 0d) — "Update documentation,
  quickstart, and install/setup instructions to be consistent with the current release
  version of backlogit (v1.7.0). Audit README, quickstart, and install/setup docs for stale
  version numbers, commands, flags, and MCP tool names that drifted from the shipped CLI/MCP
  surface. Additionally: rework the top of the README so it opens with a brochure-style
  explanation of product value (what backlogit is, who it's for, the core value proposition)
  before diving into install/usage."
- Session: operator "Stage next" — advance the next actionable stash entry through staging.
- Confirmed release surface: `backlogit_get_version` → `current: 1.7.0`, `latest: v1.7.0`,
  `go1.24.13`, commit `7daf8c3`, `update_available: false`.

## Grouping and routing (Step 1 / 1.5)

`ECC3C116` is **feature-shaped**: it declares a cohesive body of work (audit + fix drift
across multiple docs, plus a README restructure) with multiple implied tasks. Feature-shaped
entries skip contextual grouping (Step 1.5) and proceed directly to deliberation.

Other active stash entries were triaged and **not** selected this session:

- `7F0A6E89` (low, task) — BLOCKED on the external autoharness repo (spike SKILL template);
  forbidden by Constitution Principle IV (CLI Workspace Containment). Deferred, stays active.
- `6FA0829B` (low, task) — BLOCKED on the external autoharness repo (plan-review SKILL
  template); forbidden by Principle IV. Deferred, stays active.
- `0F2E5BA9` (medium, task) — already fully processed: deliberation `053-DL` (archived) →
  feature `122-F` (archived/shipped). This is a stale leftover; it will be archived as
  consumed-stash hygiene (Step 5.6), not re-planned.

There is **no `chore` artifact type** in this workspace (valid types: feature, epic, task,
subtask, deliberation, shipment, spike, review). Consistent with repo convention for
maintenance covering units (e.g. `080-S` release-docs hygiene, `076-F`), this documentation
work is typed **`feature`**.

## Problem Frame

The shipped release is **v1.7.0**, but user-facing documentation has drifted from the actual
CLI/MCP surface across three axes:

1. **Version strings** — hard-coded version numbers, dependency versions, and Go-version
   badges may lag the shipped release. Example signals: `README.md` badge `go-1.24`,
   Technology Stack table pins `modernc.org/sqlite v1.34.0` and `spf13/cobra v1.10.2`
   (`README.md:249-250`) that must be reconciled against `go.mod`; `docs/installation.md`
   references release-version wording (`:243-247`).
2. **Commands and flags** — prose examples of `backlogit` commands and flags in README Quick
   Start, `installation.md`, `configuration.md`, `plugin-guide.md`, `workflow.md`, and
   `migration-guide.md` can name commands/flags that were renamed, added, or removed.
3. **MCP tool names** — prose references to `backlogit_*` MCP tools can be stale relative to
   the shipped registration set.

Separately, the **README opening does not read as a product brochure**. The current top jumps
from a one-line tagline (`README.md:23`) straight into a technical CQRS storage explanation
(`## Overview`, `:29-35`). A new reader cannot quickly answer "what is this, who is it for,
and why would I use it" before being asked to install it.

**Who cares:** new evaluators and operators reading the README/install/quickstart; agents that
consume tool/command names from prose; maintainers who want docs that don't lie about the
shipped surface.

**Success criteria:** every user-facing doc in scope reflects the v1.7.0 CLI/MCP surface
(no stale versions, commands, flags, or tool names); the README opens with a brochure-style
value proposition; changed `docs/**` files pass `backlogit docs lint`; generated CLI reference
remains drift-free (`make docs` produces no diff).

**Out of scope (explicit):** implementing a new automated prose-vs-surface drift test;
changing any CLI/MCP behavior or code; editing generated `docs/cli-reference/*.md` by hand;
architecture/reference-only docs not named below (`ARCHITECTURE.md`, `rationale.md`,
`telemetry-fields.md`, `pre-task-completion-gate.md`, `docline-frontmatter-authoring-guide.md`).

## Research Findings

**Authoritative sources of truth for the audit** (from the learnings-researcher sweep of
`docs/compound/`; confidence medium overall, HIGH on the doc-vs-surface dimension):

- CLI command/flag surface: Cobra `Short`/`Long`/`Flags()` in `internal/cli/*.go`.
  `docs/cli-reference/*.md` is **generated** via `go run ./cmd/gen-docs docs/cli-reference`
  (a.k.a. `make docs`) and guarded by a CI "CLI Reference Drift Check" (since 089-S).
  **Never hand-edit `docs/cli-reference/`.**
  Ref: `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`.
- Shipped MCP tool names: `s.addTool(mcplib.NewTool("backlogit_...", ...))` registrations in
  `internal/mcp/tools.go`; live introspection via `srv.ToolDefs()`.
  Ref: `docs/compound/workflow-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md`,
  `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md`.
- Honest MCP→CLI map: `.autoharness/backlog-registry.yaml`, locked by
  `internal/cli/registry_parity_test.go`; discoverability guide at
  `docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md`.
- Dependency/version truth: `go.mod` (module versions) and `backlogit --version` / build
  metadata (release version).

**Tooling scope caveats:**

- `backlogit docs lint` validates `docs/**` **frontmatter** only, and its scope excludes
  `docs/memory/**` and `docs/archive/**`. It does **not** catch stale versions, commands,
  flags, or tool names in prose, and `README.md` (repo root) is **outside `docs/**`** — so the
  README is not subject to docline lint at all.
  Ref: `docs/decisions/2026-07-04-release-docs-hygiene-deliberation.md`,
  `docs/docline-frontmatter-authoring-guide.md`.
- Docline frontmatter authoring: quote any scalar containing `#` or `:`; edits must be
  body-preserving.

**No prior art** exists for README brochure/value-proposition authoring conventions or for
cross-doc version-string management (learnings gap, confidence LOW). Reusable drift-test
patterns exist if a future guard is desired, but building one is out of scope here.

## Options Evaluated

### Option A: Broad audit + fuller README restructure (CHOSEN)

Audit and fix drift across the full user-facing doc set (README + installation, configuration,
plugin-guide, workflow, migration-guide, backlogit-vs-backlog-md), and restructure the top of
README.md with a hero value proposition (what it is / who it's for / why), an "at a glance"
table, and the technical CQRS explanation moved lower.

- **Pros:** clears drift everywhere a reader/agent looks; README reads as a real product front
  door; single coordinated release unit. Directly matches the operator's explicit selections
  (broad audit, fuller restructure).
- **Cons:** largest scope; more tasks; higher review surface. Mitigated by width-isolated,
  per-document tasks under the 2-hour rule.
- **Effort:** medium (docs-only, but breadth adds task count).
- **Fit:** best matches the stash intent and the operator's confirmed choices.

### Option B: Focused audit (README + installation only) + light README touch-up

Fix drift only in README and installation.md; add a small value-prop paragraph without
restructuring.

- **Pros:** smallest, fastest; low review surface.
- **Cons:** leaves configuration/plugin/workflow/migration docs drifted; README stays
  technical-first. **Rejected** — operator explicitly chose broad audit and fuller restructure.

### Option C: Audit + fix + build an automated prose-vs-surface drift test

Option A plus a new CI test that asserts README/quickstart command and MCP-tool-name mentions
match the live surface.

- **Pros:** prevents future drift structurally; reusable patterns exist.
- **Cons:** scope creep / YAGNI for a docs-consistency pass; the test design (prose parsing,
  false-positive tuning) is its own feature with real risk. **Rejected for now** — recorded as
  a deferred follow-up, not part of this shipment.

## Trade-off Comparison

| Criterion | Option A (chosen) | Option B | Option C |
|---|---|---|---|
| Coverage of drift | Full user-facing set | README + install only | Full + future-proofed |
| README brochure depth | Fuller restructure | Light touch-up | Fuller restructure |
| Scope / task count | Medium | Small | Large |
| Risk / blast radius | Low (docs-only) | Low | Medium (new test) |
| Matches operator choice | Yes | No | Partially |
| Regression prevention | Existing gates only | Existing gates only | New guard added |

## Decision

Adopt **Option A**. Type the covering work item as a **`feature`**: "Documentation consistency
and README brochure rework for v1.7.0."

**In-scope documents (7):**

1. `README.md` — brochure restructure (hero value-prop: what it is / who it's for / why; an
   "at a glance" table; move the CQRS technical explanation lower) **and** drift fix (Quick
   Start commands/flags, MCP tool names, Technology Stack versions, Go badge/version strings).
2. `docs/installation.md` — drift fix (versions, install commands, flags).
3. `docs/configuration.md` — drift fix (config keys/flags, commands).
4. `docs/plugin-guide.md` — drift fix (install paths, commands, tool names).
5. `docs/workflow.md` — drift fix (commands, MCP tool names, flags).
6. `docs/migration-guide.md` — drift fix (versions, commands).
7. `docs/backlogit-vs-backlog-md.md` — drift fix (feature/command claims, tool names).

**Audit method:** reconcile prose against the authoritative sources above — `go.mod` for
dependency/Go versions, the Cobra command tree / generated `docs/cli-reference/` for
commands+flags, and `internal/mcp/tools.go` (`s.addTool`) / `srv.ToolDefs()` for MCP tool
names. Do not hand-edit `docs/cli-reference/`.

**Verification (executed by Ship during build):** changed `docs/**` files pass
`backlogit docs lint`; `make docs` (regenerate `docs/cli-reference/`) yields no git diff; a
spot cross-check of README/quickstart MCP tool names against the shipped registration set;
`gofmt`/`go vet`/`go test ./...`/`golangci-lint run` remain green (docs-only changes should not
affect them, but the quality gates still run).

## Rejected Alternatives

- **Option B** — too narrow; contradicts the operator's confirmed broad + fuller selections.
- **Option C** — building an automated prose-vs-surface drift test is out of scope for a
  consistency pass; deferred as a follow-up idea.
- **Editing `docs/cli-reference/*.md` directly** — forbidden; those are generated and
  CI-guarded. Command doc fixes belong in Cobra `Long`/`Short`/`Flags`, then regenerate.

## Unresolved Questions

- Exact magnitude of drift per document is unknown until the audit runs; task effort estimates
  assume ≤2h per width-isolated document task and will be validated during execution.
- Whether the "at a glance" table should live above or below the Features list — a presentation
  detail deferred to the README task's implementation, guided by the writing-style instructions.

## Risks and Mitigations

- **Risk: over-correcting a "stale" reference that is actually intentional.** Mitigation: fix
  only against authoritative sources (go.mod, Cobra tree, tools.go/ToolDefs); when ambiguous,
  leave as-is and note it.
- **Risk: README restructure loses accurate technical content.** Mitigation: relocate, do not
  delete, the CQRS/Overview material; keep it lower in the document.
- **Risk: docline lint failure from frontmatter edits.** Mitigation: body-preserving edits;
  quote scalars containing `#`/`:`; run `backlogit docs lint` on each changed `docs/**` file.
- **Risk: same-file thrash on README (restructure + drift fix).** Mitigation: sequence the
  README drift fix after the restructure (dependency edge) or combine them into one README task.
