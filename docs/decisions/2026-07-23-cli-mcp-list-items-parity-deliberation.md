---
title: "CLI/MCP list_items request-contract parity for priority/owner filters"
description: "Decision to add priority and owner filter parameters to the MCP tool backlogit_list_items to reach CLI/MCP request-contract parity (053-DL)."
source: docs/decisions/2026-07-23-cli-mcp-list-items-parity-deliberation.md
doc_type: decision
topic: "MCP list_items priority/owner filter parity with CLI list"
depth: lightweight
decision_status: decided
promoted_to: plan
linked_artifacts:
  - "docs/exec-plans/2026-07-23-mcp-list-items-parity-plan.md"
  - ".backlogit/queue/053-DL.md"
tags:
  - "mcp"
  - "cli-mcp-parity"
  - "list-items"
  - "request-contract"
---

## Problem Frame

The CLI command `backlogit list` already exposes `--priority` and `--owner`
filters (`internal/cli/list.go:250`, `:252`), which flow through the shared data
layer. The MCP tool `backlogit_list_items` (`internal/mcp/tools.go:78-84`) does
NOT expose `priority`/`owner` parameters, so an agent using the MCP surface
cannot filter by these fields without falling back to `backlogit_query_sql`.

This is an existing CLI/MCP request-contract **asymmetry**: the CLI already has
the filters and MCP lacks them. It was deferred from the 117-F adversarial
review as Parity-P2. Compound learnings caution against unilaterally *widening*
request-contract asymmetry between surfaces without an explicit, documented
decision. Adding the two filters to MCP `list_items` **closes** the asymmetry
(moves toward parity) rather than widening it — but the decision must still be
deliberate and recorded, which is what this artifact does.

Scope boundaries: this decision covers ONLY the MCP `backlogit_list_items`
request contract for `priority` and `owner`. It does not change the data layer,
the CLI surface, or any other MCP tool.

## Research Findings

Grounded in verified code facts (file:line):

* **Data layer already supports both filters.**
  `internal/db/queries.go:16-27` — the `QueryFilters` struct already declares
  `Owner` (`:22`) and `Priority` (`:23`) fields.
  `internal/db/queries.go:434-470` — `QueryItems` already applies the
  `owner = ?` (`:463-465`) and `priority = ?` (`:467-469`) WHERE clauses. No
  data-layer change is required.
* **MCP surface is the only gap.**
  `internal/mcp/tools.go:78-84` — the `backlogit_list_items` schema currently
  exposes only `type`, `status`, `assigned_to`, `sprint`.
  `internal/mcp/tools.go:472-496` — `handleListItems` reads only those four
  arguments into `db.QueryFilters`; it does not read `priority`/`owner`.
* **CLI parity reference.** `internal/cli/list.go` exposes `--priority` (`:250`)
  and `--owner` (`:252`) and flows through the same `db.QueryItems` data layer,
  proving the filters are already wired end-to-end on the CLI side.
* **Implementation size.** The change is ~8 lines of MCP code: two
  `mcplib.WithString(...)` schema params plus two
  `if v, ok := request.Params.Arguments[...].(string); ok { filters.X = v }`
  handler reads, mirroring the existing four. TDD MCP-handler test coverage is
  added under `internal/mcp/*_test.go`. A generated MCP tool-reference / docline
  doc may need regeneration so the "CLI Reference Drift" or "Docline frontmatter
  gate" CI jobs stay green (Ship regenerates during build; noted in the plan).

## Options Evaluated

### Option A: Add `priority` and `owner` filters to MCP `backlogit_list_items`

Reach CLI/MCP parity by closing the existing asymmetry. Additive and
backward-compatible (new optional params; existing callers unaffected). The data
layer already supports both filters, so the change is confined to the MCP schema
and handler plus test coverage.

* Pros: closes the asymmetry, improves agent ergonomics (filter without raw
  SQL), tiny additive scope, no data-layer or CLI change, backward-compatible.
* Cons: none material; adds two params to one tool's contract.
* Effort: low (~8 LOC + tests).
* Fit: directly satisfies the parity goal and the compound-learning caution
  (moves toward parity, does not widen asymmetry).

### Option B: Intentionally keep the asymmetry

Document the rationale for MCP omitting the filters (e.g. agents should prefer
`backlogit_query_sql` for filtered lookups).

* Pros: zero code change.
* Cons: leaves a real agent-ergonomics gap; agents must hand-write SQL for a
  filter the CLI already offers; perpetuates a documented asymmetry.
* Effort: none.
* Fit: poor — does not resolve the parity concern the deliberation was raised to
  settle.

### Option C: Narrow the CLI surface to reach parity from the other direction

Deprecate/remove CLI `--priority`/`--owner` so both surfaces match by removal.

* Pros: achieves parity.
* Cons: removes working, shipped CLI functionality users rely on; strictly worse
  ergonomics; regressive.
* Effort: low-to-medium (plus deprecation cost and user impact).
* Fit: undesirable — sacrifices working functionality to reach parity.

## Trade-off Comparison

| Criterion | Option A (add to MCP) | Option B (keep asymmetry) | Option C (narrow CLI) |
|---|---|---|---|
| Complexity | Low (~8 LOC + tests) | None | Low–medium |
| Risk | Low (additive, backward-compatible) | None (no change) | Medium (removes shipped functionality) |
| Reaches parity | Yes | No | Yes (by removal) |
| Agent ergonomics | Improved | Unchanged (gap remains) | Worse |
| Alignment with parity goal | Strong | Weak | Regressive |

## Decision

**Chosen: Option A** — add `priority` and `owner` filter parameters to the MCP
tool `backlogit_list_items`, implemented into the upcoming **v1.7.0** release.

Rationale:

* **Closes the asymmetry.** Adding the filters to MCP brings the request
  contract to parity with the CLI, directly satisfying the compound-learning
  caution (movement toward parity, not widening).
* **Additive and backward-compatible.** New optional parameters do not affect
  existing MCP callers; on-contract behavior is unchanged when the params are
  omitted.
* **Data layer already supports it.** `QueryFilters.Owner`/`.Priority` and the
  `QueryItems` WHERE clauses already exist, so the change is confined to the MCP
  schema + handler + tests — no data-layer risk.
* **Improves agent ergonomics.** Agents gain first-class filtering without
  falling back to raw `backlogit_query_sql`.

## Rejected Alternatives

* **Option B (keep asymmetry)** — rejected: leaves the agent-ergonomics gap
  unresolved and perpetuates a documented request-contract asymmetry the
  deliberation was raised to close.
* **Option C (narrow CLI)** — rejected: removes working, shipped CLI
  functionality and worsens ergonomics to reach parity from the wrong direction.

## Unresolved Questions

* Should CLI and MCP request contracts always evolve in lockstep going forward?
  Out of scope for this decision; a broader parity policy could be captured
  separately if the team wants a standing rule.
* No known downstream consumers depend on the current MCP contract omitting the
  filters (additive change poses no compatibility risk).

## Risks and Mitigations

* **Risk: request-contract drift / generated docs.** Low. Adding tool params can
  trip the "CLI Reference Drift" or "Docline frontmatter gate" CI jobs if a
  generated MCP tool-reference doc is not regenerated. *Mitigation:* the plan
  and harvested task flag regeneration; Ship regenerates during build and CI
  gates verify.
* **Risk: behavioral regression.** Very low. The change is additive; existing
  four-filter behavior is untouched and covered by the new TDD MCP-handler test
  (failing test first).
* **Risk: scope creep into the data layer.** Low. The data layer already
  supports the filters; the task is explicitly bounded to the MCP schema +
  handler + tests.
