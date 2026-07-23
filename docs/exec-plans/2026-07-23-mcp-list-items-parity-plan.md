---
chunk_strategy: h1-h2-h3
description: 'Add priority and owner filter parameters to the MCP tool backlogit_list_items to reach CLI/MCP request-contract parity (053-DL, v1.7.0).'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-23-mcp-list-items-parity-plan.md
title: 'MCP list_items priority/owner filter parity'
---

Source decision: `docs/decisions/2026-07-23-cli-mcp-list-items-parity-deliberation.md`
(Option A, promoted to plan; target release v1.7.0).

## Problem Frame

The MCP tool `backlogit_list_items` (`internal/mcp/tools.go:78-84`) exposes only
`type`, `status`, `assigned_to`, and `sprint`. Its handler
`handleListItems` (`internal/mcp/tools.go:472-496`) reads only those four
arguments into `db.QueryFilters`. The CLI `backlogit list` already offers
`--priority` (`internal/cli/list.go:250`) and `--owner`
(`internal/cli/list.go:252`), which flow through the same data layer. The data
layer already supports both filters: `QueryFilters` declares `Owner`
(`internal/db/queries.go:22`) and `Priority` (`:23`), and `QueryItems` applies
`owner = ?` (`:463-465`) and `priority = ?` (`:467-469`). The only gap is the
MCP request contract. Closing it brings MCP to parity with the CLI.

## Requirements Trace

| Requirement (from decision) | Implementation action |
|---|---|
| Expose `priority` on MCP `list_items` | Add `mcplib.WithString("priority", ...)` to the tool schema (`tools.go:78-84`) |
| Expose `owner` on MCP `list_items` | Add `mcplib.WithString("owner", ...)` to the tool schema (`tools.go:78-84`) |
| Wire params into filters | Add two `if v, ok := request.Params.Arguments[...].(string); ok { filters.Priority/Owner = v }` reads in `handleListItems` (`tools.go:472-488`) |
| Backward-compatible / additive | New params are optional; omission preserves current four-filter behavior |
| Prevent generated-doc / drift gate failure | Regenerate any MCP tool-reference / docline doc so "CLI Reference Drift" and "Docline frontmatter gate" CI jobs stay green (Ship regenerates during build) |
| Verify behavior (TDD) | Add a failing MCP-handler test in `internal/mcp/*_test.go` asserting priority/owner filters are applied, then implement |
| Lock the contract (prevent re-divergence) | Add/extend a CLI/MCP request-contract **parity test** that asserts the MCP `list_items` filter-param set equals the CLI `list` filter-flag set, so a future CLI-only filter addition fails CI instead of silently re-opening the asymmetry (compound: `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`, `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`) |
| Both-surfaces verification | Explicitly confirm priority/owner reads exist on BOTH the CLI and MCP surfaces and are covered by MCP-path tests (compound: `docs/compound/2026-05-07-mcp-cli-config-parity.md`) |

## Implementation Units

### Unit 1 (single unit): Add priority/owner params to `backlogit_list_items` (schema + handler + tests)

* **Domain:** Go / MCP (single skill domain).
* **Execution posture:** test-first (TDD).
* **What changes:**
  1. **Test-first (red):** add a table-driven MCP-handler test in
     `internal/mcp/` (e.g. `internal/mcp/tools_list_items_parity_test.go` or an
     existing `*_test.go`) that invokes `handleListItems` with `priority` and
     `owner` arguments against a seeded workspace and asserts the filtered
     result set. Confirm it fails (params not yet read).
  2. **Schema (green):** in `internal/mcp/tools.go:78-84`, add
     `mcplib.WithString("priority", mcplib.Description("Filter by priority"))`
     and `mcplib.WithString("owner", mcplib.Description("Filter by owner"))`,
     mirroring the existing four param declarations.
  3. **Handler (green):** in `handleListItems` (`internal/mcp/tools.go:476-488`),
     add two reads mirroring the existing pattern:
     `if v, ok := request.Params.Arguments["priority"].(string); ok { filters.Priority = v }`
     and the equivalent for `owner` → `filters.Owner`. In the schema
     `WithString` descriptions, note that `owner` filters the owner field and is
     **distinct from `assigned_to`** (mirroring the CLI's separate `--owner` /
     `--assigned-to` flags) to prevent agent misuse.
  4. **Parity-lock test (regression guard) — correct seam:** the test lives in
     **`internal/cli`**, which is the correct dependency direction — `internal/cli`
     **already imports** `internal/mcp`, so there is **no import cycle** and **no
     new exported API is needed**. The test inspects the real `newListCommand`
     flag set directly (same package), normalizes the CLI kebab-case flag names to
     snake_case (replace `-` with `_`, e.g. `assigned-to` → `assigned_to`), and
     compares that set against the `backlogit_list_items` parameter names extracted
     from the **existing** `(*mcp.Server).ToolDefs()` accessor
     (`internal/mcp/server.go:101-106`), which already returns the registered tool
     definitions including param schemas. The canonical (snake_case) filter
     contract is: `type`, `status`, `assigned_to`, `sprint`, `priority`, `owner`.
     Deriving the CLI side from the live `newListCommand` flags (rather than a
     hard-coded constant list) is what makes this real drift protection: a future
     CLI-only filter addition FAILS the test instead of silently re-diverging — the
     "close the gap AND lock it with a drift test" idiom from the compound learnings
     above. Confirm the priority/owner reads exist on both surfaces (both-surfaces
     checklist). No new `internal/mcp` accessor is created (planning only here — no
     Go is written in this Stage step).
  5. **Docs/drift:** regenerate any generated MCP tool-reference doc and confirm
     docline frontmatter validity, so the "CLI Reference Drift" and "Docline
     frontmatter gate" CI jobs stay green. (Ship regenerates during build; call
     it out so it is not missed.)
* **Files affected (<3 source files):** `internal/mcp/tools.go` (two schema
  params + two handler reads); one **new `internal/cli/*_test.go`** for the
  parity-lock test (correct dependency direction — `internal/cli` already imports
  `internal/mcp`; uses the existing `(*mcp.Server).ToolDefs()` accessor, so **no
  new `internal/mcp` accessor is required**); one `internal/mcp/*_test.go` for the
  functional MCP-handler test; and (if a drift/docline gate requires it) a
  generated MCP tool-reference doc. No production API is added — the schema+handler
  change stays confined to `internal/mcp/tools.go`.
* **Tests:** the new MCP-handler test (priority filter applied; owner filter
  applied; omitted params preserve prior behavior) plus the CLI/MCP filter-set
  parity-lock test.
* **Scope note:** CLI `list --group-by` is output-shaping (not a request-contract
  filter) and stays out of scope — agents receive full JSON and can group
  client-side; the parity test asserts filter-set equivalence only.
* **Atomic milestone:** `go test ./internal/mcp/...` passes with the new
  coverage; `go build ./cmd/backlogit` succeeds.

## Dependency Graph

Single unit; no internal dependencies. No cross-unit sequencing.

## Decisions and Rationale

* **Reuse the existing filter pattern** rather than introducing a new argument
  parser — the four existing reads establish the idiom; two more keep the
  handler uniform and low-risk.
* **No data-layer change** — `QueryFilters.Priority/.Owner` and the `QueryItems`
  WHERE clauses already exist; touching them would be scope creep.
* **TDD** — a failing MCP-handler test first proves the params were previously
  ignored and guards against regression.

## Risks and Caveats

* **Generated-doc / drift gate:** adding tool params can trip "CLI Reference
  Drift" or the "Docline frontmatter gate" if a generated reference doc is not
  regenerated. Mitigation: regenerate during build; CI gates verify. Low.
* **Behavioral regression:** additive change; existing four-filter behavior is
  untouched and covered by the new test. Very low.

## Constitution Check

* **Safety-First Go (NON-NEGOTIABLE):** pass — production code stays in Go; the
  two handler reads follow the existing pattern; no new error paths beyond the
  existing `QueryItems` wrapping; no `unsafe`.
* **Test-First Development (NON-NEGOTIABLE):** pass — a failing MCP-handler test
  is written and observed to fail before the schema/handler change lands.
* **Workspace Isolation and Security Boundaries:** pass — no filesystem or path
  handling changes; filters are passed to the existing parameterized SQL layer
  (no injection surface introduced).
* **CLI Workspace Containment (NON-NEGOTIABLE):** N/A — no file creation/deletion
  outside the working tree; MCP request-contract change only.
* **Structured Observability:** N/A — no change to logging/observability
  surfaces.
* **Single Responsibility:** pass — no new dependencies; reuses existing data
  layer and MCP helpers.
* **Destructive Command Approval (NON-NEGOTIABLE):** N/A — no destructive
  commands.
* **Explicit Safety Modes:** N/A — low blast radius, additive.
* **Git-Friendly Persistence:** N/A — no persisted format change.
* **Agent Context Efficiency:** pass — improves agent ergonomics by letting the
  MCP surface filter server-side instead of returning unfiltered rows.
* **Merge Commit History Preservation (NON-NEGOTIABLE):** pass — Ship merges via
  a merge commit; no squash/rebase.

Constitution Check: pass

## Plan Hardening Signals

* Public API, schema, or contract change: **present** — adds two optional params
  to the MCP `backlogit_list_items` request contract. Additive and
  backward-compatible; no removal or behavior change for existing callers.
* Security, auth, permission, or compliance-sensitive behavior: **absent** —
  filters pass through the existing parameterized query layer; no auth surface.
* Migration, backfill, destructive/irreversible step: **absent**.
* External integration, operator checkpoint, external dependency: **absent**.
* High runtime, rollout, or rollback risk: **absent** — rollback is a plain
  revert; no data/format change.

The single hardening signal (additive contract change) is low-risk and
backward-compatible; it does not warrant a `plan-harden` pass.

Requires plan hardening: no

## Runtime Verification and Closure

* **Runtime surface changed:** the MCP tool `backlogit_list_items` request
  contract (agent-facing).
* **Runtime verification:** exercise `backlogit_list_items` with `priority` and
  `owner` arguments against a seeded workspace (via the new MCP-handler test and,
  during Ship, a live MCP invocation) and confirm the filtered result set
  matches the CLI `list --priority/--owner` output for the same inputs.
* **Operational closure:** confirm CI "CLI Reference Drift" and "Docline
  frontmatter gate" jobs are green after any generated-doc regeneration. No
  monitoring/rollback artifact needed beyond revert (additive, no persisted
  state). Ownership: Ship agent through v1.7.0 closure.

## Verification

* `go test ./internal/mcp/...` — new MCP-handler test AND CLI/MCP filter-set
  parity-lock test pass (both were red).
* `go build ./cmd/backlogit` — builds.
* `go vet ./...`, `golangci-lint run`, `gofmt -l .` — clean.
* CI gates: "CLI Reference Drift" and "Docline frontmatter gate" green after any
  generated MCP tool-reference regeneration.

## Plan Review

dispatch_mode: multi-agent-dispatch

decision: PASS

**Gate rationale.** Reviewer personas were dispatched as independent sub-agents
(multi-agent-dispatch; capability probed and available). All seven selected
personas completed and returned findings: the four always-on personas
(Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings
Researcher) plus three triggered cross-model personas — Architecture Strategist
(always triggered), Agent-Native Parity Reviewer (plan exposes an MCP tool), and
Security Lens Reviewer (plan changes an API-surface request contract).

The initial pass surfaced two **P1** findings from the Learnings Researcher (a
highly-relevant prior solution the plan ignored). Both were **resolved by
revising the plan before this gate decision** — the plan now requires a CLI/MCP
request-contract parity-lock test and the both-surfaces verification checklist
(Implementation Unit 1 steps 3-4, Requirements Trace, Tests, Verification). With
those additions there are **no outstanding P0 or P1 findings**, only P2/P3
advisories, so the gate is **PASS**.

**Plan hardening:** the plan's Plan Hardening Signals section concludes
`Requires plan hardening: no`. The one present signal (additive MCP
request-contract change) is low-risk and backward-compatible; no `plan-harden`
pass is required. This is consistent with the gate.

**Constitution Check:** the plan's `## Constitution Check` section maps all
engaged principles and concludes with the recognized verdict `Constitution
Check: pass`. Structural gate requirement satisfied.

### Findings by severity

**P0 / P1 (resolved before gate):**

* ~~P1 (Learnings Researcher)~~ — RESOLVED. The plan added only a functional
  filter test and no parity-lock/drift test, ignoring the compound "close the
  gap AND lock it with a drift test" pattern
  (`docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`,
  `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`).
  **Fix applied:** Unit 1 step 4 now requires a CLI/MCP filter-set parity-lock
  test; Requirements Trace, Tests, and Verification updated.
* ~~P1 (Learnings Researcher)~~ — RESOLVED. The both-surfaces verification
  checklist (`docs/compound/2026-05-07-mcp-cli-config-parity.md`) was not applied
  explicitly. **Fix applied:** Requirements Trace adds a "both-surfaces
  verification" row and Unit 1 step 4 confirms priority/owner reads exist on both
  surfaces with MCP-path coverage.

**P2 (advisory, acknowledged):**

* Learnings Researcher — the `list_items` response-shape contract
  (`arrays-always-[]`, `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`)
  is out of this plan's request-contract scope but touches the same
  `handleListItems` path; implementation must not regress response marshaling.
  Not blocking.

**P3 (advisory, acknowledged):**

* Constitution Reviewer — Width Isolation wording: the unit bundles schema +
  handler + tests + doc regen. Mitigated: single Go/MCP skill domain, ~8 LOC +
  tests, TDD. Non-blocking.
* Go Reviewer — `.(string); ok` reads silently drop wrong-typed args (idiomatic,
  matches existing four reads); table-driven test should use `t.Run` + `t.Helper`;
  name the generated-doc regeneration command for determinism.
* Architecture Strategist — six repeated `Arguments[key].(string)` extractions;
  consider a `stringArg` helper only if the filter set keeps growing (do NOT add
  now).
* Agent-Native Parity Reviewer — note group-by is intentionally out of scope
  (added to plan); clarify `owner` vs `assigned_to` in param descriptions (added
  to Unit 1 step 3); consider a standing CLI/MCP lockstep parity policy (captured
  as an Unresolved Question in the decision artifact).
* Security Lens Reviewer — no findings; parameterization preserved, no injection
  or data-exposure surface introduced.

### Runtime verification and closure

The plan includes a `## Runtime Verification and Closure` section covering the
changed MCP request-contract surface (verify filtered output matches CLI parity;
confirm CI drift + docline gates green; rollback = revert). No gaps.

Reviewed personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
Security Lens Reviewer (7/7 covered).

