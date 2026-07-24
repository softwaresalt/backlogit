---
chunk_strategy: h1-h2-h3
description: 'Implementation plan to bring backlogit user-facing docs into v1.7.0 consistency and restructure the README top as a product brochure.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-24-docs-consistency-readme-brochure-plan.md
title: 'Documentation consistency and README brochure rework for v1.7.0'
---

Source deliberation: `docs/decisions/2026-07-24-docs-consistency-readme-brochure-deliberation.md`
(stash `ECC3C116`). Chosen direction: **Option A** — broad drift audit across the user-facing
doc set plus a fuller README top restructure.

## Problem Frame

The shipped release is **v1.7.0** (`backlogit_get_version` → `current: 1.7.0`, `go1.24.13`,
commit `7daf8c3`). User-facing documentation has drifted from the actual shipped surface along
three axes — version strings, command/flag examples, and `backlogit_*` MCP tool names — and the
`README.md` opening leads with a technical CQRS storage explanation (`README.md:29-35`) instead
of a product value proposition. This plan corrects both: a factual reconciliation of the docs
against authoritative in-repo sources, and a prose restructure of the README top.

Authoritative sources of truth (do not trust hand-written prose or hand-edit generated docs):

- Dependency versions + the `go-1.24` major.minor badge → `go.mod` (the `go 1.24.0` directive).
  The **patch-level** runtime string (`go1.24.13`) is NOT from `go.mod` — it comes from
  `runtime.Version()`/build metadata (`internal/mcp/version_tool.go` `go_version`); reconcile any
  `go1.24.x` string against `backlogit --version`.
- CLI commands + flags → Cobra `Use`/`Short`/`Long`/`Flags()` in `internal/cli/*.go` (or
  `backlogit <cmd> --help`); the generated mirror `docs/cli-reference/*.md` (produced by `make
  docs` / `go run ./cmd/gen-docs docs/cli-reference`, CI-guarded — never hand-edited).
- MCP tool names → **primary authority is live introspection via `srv.ToolDefs()`/`ListTools()`**,
  which captures every registered tool. Registrations are spread across the whole `internal/mcp/`
  package (`tools.go`, `dynamic.go`, `hook_tools.go`, `docs_tools.go`, `version_tool.go`, …), so a
  static grep for `mcplib.NewTool("backlogit_...")` MUST scope the whole `internal/mcp/` package —
  never `tools.go` alone (grepping one file misses shipped tools).
- Release/version wording → `backlogit --version` build metadata.

## Requirements Trace

| # | Requirement (from deliberation) | Implementation action | Unit |
|---|---|---|---|
| R1 | README opens with brochure value-prop (what / who / why) + "at a glance" table; CQRS explanation moved lower | Restructure README top | U1 |
| R2 | README/Quick Start commands, flags, MCP tool names, versions, Technology Stack match v1.7.0 | Reconcile README facts against sources | U2 |
| R3 | `installation.md` + `configuration.md` free of stale versions/commands/flags | Drift fix (setup/config cluster) | U3 |
| R4 | `plugin-guide.md` + `migration-guide.md` free of stale install paths/commands/tool names/versions | Drift fix (install-path/migration cluster) | U4 |
| R5 | `workflow.md` + `backlogit-vs-backlog-md.md` free of stale commands/tool names/feature claims | Drift fix (usage/positioning cluster) | U5 |
| R6 | Whole change verified against generated CLI reference, docline lint, and shipped tool set | Verification & reconciliation | U6 |

## Implementation Units

Every unit is a **single skill domain: documentation**. Each touches ≤2 files, has no code
functions, and its verification is doc-render/lint rather than unit tests. Execution posture for
all authoring units is **characterization-first**: enumerate the authoritative facts from the
sources above, then edit prose to match.

### U1 — README brochure restructure

- **Preparation (one-time, up front — U1 is the first runnable unit, no dependencies):** Run
  `make docs` once to confirm the generated `docs/cli-reference/` mirror is drift-free before the
  drift-fix units (U3–U5) reconcile prose against it. If the mirror is stale, treat Cobra
  `--help`/source as the primary authority and record the generated-reference drift as a
  **separate follow-up** (do not hand-edit `docs/cli-reference/`).
- **Changes:** Add a hero value-proposition block after the H1 (what backlogit is, who it's for,
  the core value proposition), add an "at a glance" summary table, and relocate the existing
  CQRS/`## Overview` technical explanation lower in the document (below the brochure + Features).
  Do **not** delete the technical content — move it. Follow
  `.github/instructions/writing-style.instructions.md` and `markdown.instructions.md`.
- **Files:** `README.md`.
- **Keep version-pinned facts out of the new brochure/table** (leave those to U2) so U1 stays a
  pure layout/prose unit and does not introduce facts U2 must re-verify.
- **Verify:** README renders with value-prop + at-a-glance table before install/usage; CQRS
  section present lower in the doc; no broken intra-README anchors. **Prefer preserving existing
  heading slugs when relocating sections; if a heading is renamed or moved, sweep `docs/**` (and
  the repo root) for inbound links to the old anchor (e.g. `README.md#overview`) and update them.**
  (README is repo-root, outside `docs/**`, so `backlogit docs lint` does not apply to it.)
- **Posture:** docs-authoring (writing-style guided).

### U2 — README + Quick Start drift fix

- **Changes:** Reconcile every factual reference in `README.md` against the authoritative
  sources: Quick Start command/flag examples, `backlogit_*` MCP tool names, the Technology Stack
  table (`README.md:243-256` — e.g. `modernc.org/sqlite`, `spf13/cobra` versions vs `go.mod`),
  the Go badge (`README.md:25`), and any embedded version strings. Correct anything that appears
  in the U1-added brochure/table too.
- **Files:** `README.md`.
- **Verify:** every command/flag/tool-name/version in README matches the v1.7.0 shipped surface;
  Technology Stack rows equal `go.mod` entries.
- **Posture:** characterization-first (enumerate sources, then fix).

### U3 — installation.md + configuration.md drift fix

- **Changes:** Fix stale versions, install/config commands, and flags in `docs/installation.md`
  and `docs/configuration.md` against `go.mod`, the Cobra command tree / generated
  `docs/cli-reference/`, and the MCP tool set (`srv.ToolDefs()` / whole `internal/mcp/` package,
  not `tools.go` alone).
- **Files:** `docs/installation.md`, `docs/configuration.md`.
- **Verify:** no stale versions/commands/flags; each file passes `backlogit docs lint`
  (body-preserving edits; quote frontmatter scalars containing `#`/`:`).
- **Posture:** characterization-first.

### U4 — plugin-guide.md + migration-guide.md drift fix

- **Changes:** Fix stale install paths, commands, MCP tool names, and version references in
  `docs/plugin-guide.md` and `docs/migration-guide.md` against the authoritative sources.
- **Files:** `docs/plugin-guide.md`, `docs/migration-guide.md`.
- **Verify:** matches shipped surface; each file passes `backlogit docs lint`.
- **Posture:** characterization-first.

### U5 — workflow.md + backlogit-vs-backlog-md.md drift fix

- **Changes:** Fix stale commands, `backlogit_*` MCP tool names, flags, and feature/positioning
  claims in `docs/workflow.md` and `docs/backlogit-vs-backlog-md.md` against the sources.
- **Files:** `docs/workflow.md`, `docs/backlogit-vs-backlog-md.md`.
- **Verify:** matches shipped surface; each file passes `backlogit docs lint`.
- **Posture:** characterization-first.

### U6 — Consistency verification and reconciliation

- **Changes:** No new prose. Run the source-entrypoint verification (the one-time `make docs`
  baseline moved to U1, the first runnable unit, so this terminal gate has no "do first" step that
  the dependency-aware queue would hide):
  - `make docs` (regenerate `docs/cli-reference/`) MUST yield no git diff.
  - `backlogit docs lint` (repo-wide, `make docs-lint`) MUST report 0 violations.
  - **Enumerate the canonical fact set once** — the shipped MCP tool names (`srv.ToolDefs()`), the
    CLI command/flag surface (`backlogit <cmd> --help`), and the version strings
    (`backlogit --version`, `go.mod`) — then cross-check those names/versions across **every edited
    doc** (README + all of U3–U5's files), not just README. This closes the cross-unit
    fact-consistency gap: no single authoring unit sees the whole surface.
  - **Mechanical command check:** run each documented command with `--help` (or in a scratch
    workspace) against the v1.7.0 binary to confirm command/flag existence, rather than relying
    only on a subjective read-through.
  - **Markdown-structure lint (P-008):** run the **repo-wide** P-008 gate — `markdownlint "**/*.md"`
    exits 0 for **every staged/committed Markdown file** (not just the seven edited docs) — under the
    repo's declared P-008 rule set **MD001, MD025, MD041** (`.github/policies/workflow-policies.md`),
    so a heading-hierarchy regression (skipped heading level, multiple H1, missing top-level heading)
    in any changed Markdown (including backlog lifecycle artifacts) is caught here rather than in CI.
    The edited-file list (README + U3–U5 docs) scopes only the factual prose cross-check above, not
    this structure gate. **Prerequisite:** the repo does not yet ship a `.markdownlint.json`; U6 MUST
    provision one enabling exactly MD001/MD025/MD041 per P-008 before running the gate — pinning the
    rule set makes the check deterministic instead of relying on markdownlint's environment defaults.
    If provisioning is deferred, treat it as a blocking follow-up and apply the P-008 heading-hierarchy
    rules manually.
  - Run the quality gates (`gofmt -l .`, `go vet ./...`, `make test` / `go test ./...`,
    `golangci-lint run`) to confirm docs-only edits kept the build green.
- **Accepted residual risk:** No mechanical gate proves README/installation/etc. **prose** matches
  the shipped surface — `make docs` validates only the *generated* reference vs Cobra source,
  `backlogit docs lint` validates *frontmatter only*, and README is outside docline scope. The
  prose-vs-surface guarantee therefore rests on characterization-first authoring + the enumerated
  fact cross-check above. This residual manual dependency is accepted (an automated prose-drift
  test is a deferred follow-up, per Decisions).
- **Files:** none expected. If `make docs` reveals generated-reference drift, that indicates a
  Cobra `Long`/`Short` doc bug — record it as a **separate follow-up** (out of this feature's
  docs scope); do not hand-edit `docs/cli-reference/`.
- **Verify:** clean `make docs` diff; 0 docline violations; enumerated tool-name/version set
  matches across all edited docs; markdownlint clean **repo-wide** under the pinned P-008 rule set
  (MD001/MD025/MD041 — 0 violations across all staged/committed Markdown); quality gates green.
- **Posture:** verification.

## Dependency Graph

```text
U1 (README restructure)
  └── U2 (README drift fix)   # same file — restructure before fact reconciliation
U3 (install + config)         # independent
U4 (plugin + migration)       # independent
U5 (workflow + vs-backlog-md) # independent
U6 (verification) depends on: U1, U2, U3, U4, U5
```

- U2 → depends on U1 (both edit `README.md`; do the layout move first, then reconcile facts).
- U3, U4, U5 are mutually independent and independent of U1/U2 (disjoint files) — parallelizable.
- U6 is the terminal gate; it depends on all authoring units. The one-time `make docs`
  mirror-refresh **baseline lives in U1** (the first runnable unit, no dependencies) — not in
  terminal U6 — because the dependency-aware queue hides U6 until U1–U5 are done, so a "do first"
  step on U6 would be unreachable. U6's terminal `make docs` drift check still depends on U1–U5.

No cycles. Suggested execution order: U1 (incl. `make docs` baseline) → U2, then U3/U4/U5 (any
order), then U6 terminal verification.

## Decisions and Rationale

- **Split README into U1 (layout) + U2 (facts).** The restructure is a prose/layout activity;
  the drift fix is a source-reconciliation activity. Separating them keeps each atomic and under
  the 2-hour rule, with a same-file dependency edge to prevent thrash.
- **Cluster the six non-README docs into three 2-file tasks** by theme (setup/config,
  install-path/migration, usage/positioning). Two files per task respects "fewer than 3 files"
  while minimizing task count.
- **No separate "surface inventory" task.** Each drift task consults the same in-repo
  authoritative sources directly; a shared inventory artifact would add overhead without a
  shippable milestone.
- **No automated prose-vs-surface drift test.** Recorded in the deliberation as a deferred
  follow-up; building it is a feature in its own right (scope discipline / YAGNI).
- **`docs/cli-reference/` stays generated.** Command doc corrections, if any surface, belong in
  Cobra source + regeneration, not in this docs feature.

## Risks and Caveats

- **Over-correcting intentional references.** Fix only against authoritative sources; when a
  reference is ambiguous, leave it and annotate. Mitigation baked into each unit's method.
- **README restructure dropping accurate content.** U1 must *relocate*, not delete, the CQRS
  material. Mitigation: explicit "move, don't delete" instruction + U6 read-through.
- **Docline lint regressions from frontmatter edits.** Body-preserving edits; quote scalars with
  `#`/`:`; U3–U5 each self-lint; U6 runs the repo-wide gate.
- **Hidden generated-reference drift surfaced by `make docs`.** Treated as an out-of-scope
  follow-up, not silently hand-patched.
- **Markdown lint not yet repo-provisioned.** P-008 (`workflow-policies.md`) declares a
  `.markdownlint.json` (MD001/MD025/MD041) precondition the repo does not actually ship. U6 pins that
  rule set and provisions the config per P-008 before linting; the broader tooling gap (config +
  Makefile target + CI wiring) is captured as a separate follow-up, out of this docs feature's scope.

## Constitution Check

- **Safety-First Go (MUST):** N/A — no Go production code changes; documentation only.
  U6 still runs `go vet`/`golangci-lint`/`make test` to confirm the build stayed green.
- **Test-First Development (NON-NEGOTIABLE):** N/A — prose docs have no unit-test harness.
  Verification is `backlogit docs lint`, the `make docs` drift check, and read-through (U6).
- **Workspace Isolation and Security Boundaries:** pass — all edits resolve within the workspace
  root; no secrets introduced.
- **CLI Workspace Containment (NON-NEGOTIABLE):** pass — every file is in-repo. The two
  external-autoharness-repo stash items (`7F0A6E89`, `6FA0829B`) are explicitly excluded from
  this feature.
- **Structured Observability:** pass — conventional commits + backlog traceability (commit
  association per task).
- **Single Responsibility:** pass — no new dependencies; documentation-only.
- **Destructive Command Approval (NON-NEGOTIABLE):** N/A — no destructive commands. Doc edits are
  non-destructive and git-revertible. U6's `make docs` regenerates `docs/cli-reference/` (a
  generated-file write within the workspace, git-tracked and revertible), which is not a
  destructive operation requiring approval.
- **Explicit Safety Modes for Elevated Risk (Principle VIII):** N/A for elevated-risk gating —
  blast radius is low (git-tracked, revertible documentation edits with no runtime/schema/security
  impact). No destructive command, production-impacting change, or uncertain root cause is
  involved, so no careful/freeze-scope/investigate-first mode is required. Ship may still scope
  edits to the enumerated doc set (an implicit freeze-scope) during implementation.
- **Git-Friendly Persistence:** pass — Markdown with YAML frontmatter.
- **Agent Context Efficiency:** pass.
- **Merge Commit History Preservation (NON-NEGOTIABLE):** pass — ships via a merge commit, never
  squash/rebase.
- **Capability overlays (backlogit):** pass — work items, deliberation, plan, and shipment are
  tracked through the backlogit backlog surface (registry-backed), not a parallel markdown tracker;
  the index is refreshed via `backlogit_sync_index`. No other overlay (intercom/engram/strict-safety)
  gates this docs-only change.

Constitution Check: pass

## Plan Hardening Signals

- Public API, schema, or contract change: **absent** — documentation only; no CLI/MCP contract
  changes.
- Security, auth, permission, or compliance-sensitive behavior: **absent**.
- Migration, backfill, destructive data/config action, or irreversible step: **absent** — doc
  edits are git-tracked and revertible.
- External integration, operator checkpoint, or external dependency: **absent** — the
  external-repo follow-ups are excluded from this feature.
- High runtime, rollout, or rollback risk: **absent** — no runtime behavior changes.

Requires plan hardening: no

## Runtime Verification and Closure

- **Runtime surface change:** none. No CLI/API/UI/background-job behavior changes; only
  human/agent-facing documentation content.
- **Runtime verification:** U6 read-through confirms Quick Start and install commands are
  copy-pasteable and accurate against the shipped v1.7.0 CLI; MCP tool names referenced in prose
  resolve to real registrations; the `make docs` diff is clean and `backlogit docs lint` reports
  0 violations.
- **Operational closure:** no monitoring, alerting, or rollback trigger required — a docs
  regression is corrected by a follow-up doc edit or `git revert`. Closure artifact is the merged
  PR plus this plan and its deliberation. Ownership: whoever ships the feature.

## Plan Review

- dispatch_mode: multi-agent-dispatch
- personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist, Learnings Researcher
- decision: PASS
- severity_tally (as reviewed): P0=0, P1=0, P2=7, P3=8
- gate_basis: No P0/P1 findings. All 7 P2 findings were resolved by plan revision (below); only
  P3 advisories remain (applied or consciously accepted), so the revised plan passes.
- operator_authorization: not required (decision is PASS; not an ADVISORY/FAIL gate)

### P2 findings — all resolved in this revision

1. **[Go Reviewer] MCP tool-name source incomplete.** Grepping `tools.go` alone misses tools in
   `dynamic.go`/`hook_tools.go`/`docs_tools.go`/`version_tool.go`. → **Resolved:** authoritative-sources
   block now names `srv.ToolDefs()`/`ListTools()` as the primary complete authority and requires any
   static grep to scope the whole `internal/mcp/` package; U3 updated likewise.
2. **[Go Reviewer] No mechanical gate catches prose drift** (the core deliverable). → **Resolved:**
   U6 now enumerates the canonical fact set once and cross-checks it across every edited doc, adds a
   mechanical `--help` command check, and records the residual manual dependency as accepted risk.
3. **[Go Reviewer] No Markdown-structure lint in U6.** → **Resolved:** U6 adds markdownlint over the
   edited README + `docs/**`.
4. **[Go Reviewer] Sequencing** — authors reconcile against `docs/cli-reference/` before it is
   confirmed drift-free. → **Resolved:** the one-time `make docs` baseline runs in **U1** (the first
   runnable unit, no dependencies) up front, not in terminal U6 — the dependency-aware queue hides
   U6 until U1–U5 finish, so a "do first" step there is unreachable; dependency graph and execution
   order updated. *(Refined after PR #297 Copilot review flagged the blocked-terminal-U6 prep.)*
5. **[Architecture Strategist] No cross-unit fact-consistency mechanism across U2–U5; U6 too
   README-narrow.** → **Resolved:** U6's enumerated-fact cross-check now spans all edited docs (same
   fix as #2).
6. **[Architecture Strategist] README relocation only checks intra-README anchors,** ignoring inbound
   cross-doc links to the relocated `## Overview`. → **Resolved:** U1 verify now prefers preserving
   heading slugs and sweeps `docs/**` + repo root for inbound anchors on rename/move.
7. **[Constitution Reviewer] Principle VIII (Safety Modes) missing from the Constitution Check.** →
   **Resolved:** added a Principle VIII entry (N/A — low blast radius; implicit freeze-scope
   available).

### P3 advisories — applied or consciously accepted (not gate-blocking)

- [Go Reviewer] `go.mod` not authoritative for the patch-level `go1.24.13` string. → **Applied:**
  sources block reconciles patch strings against `backlogit --version`.
- [Go Reviewer] Prefer `make test` over `go test ./...`. → **Applied** in U6.
- [Go Reviewer] Mechanical `--help` read-through. → **Applied** in U6.
- [Constitution Reviewer] Principle I label was NON-NEGOTIABLE (it is MUST). → **Applied.**
- [Constitution Reviewer] Extend Principle VII rationale to cover the `make docs` regeneration
  write. → **Applied.**
- [Constitution Reviewer] Capability overlays not addressed. → **Applied** (backlogit overlay
  bullet).
- [Scope Boundary Auditor] U1/U2 could optionally collapse into one task. → **Declined:** the split
  keeps each unit atomic under the 2-hour rule (see Decisions); accepted as advisory.
- [Scope Boundary Auditor] U6 runs full Go gates beyond R6's narrow need. → **Declined:** U6 is the
  green-build/consistency gate; running the gates confirms docs-only edits kept the build green.
