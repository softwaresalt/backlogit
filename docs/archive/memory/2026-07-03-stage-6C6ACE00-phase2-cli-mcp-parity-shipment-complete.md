# Stage session — 6C6ACE00 CLI/MCP command parity phase-2 — COMPLETE

- **Date:** 2026-07-03
- **Agent:** Stage (stash-to-backlog pipeline)
- **Status:** COMPLETE — queued shipment `079-S` handed off
- **Stash processed:** `6C6ACE00` (feature, low) promoted; `2827CB5F` (deliberation, low) ride-along corrected + archived
- **Predecessor:** 078-S (phase-1 CLI/MCP parity) merged via PRs #170/#171; `main` HEAD `8bf53eb` at session start.

## Outcome (handoff token)

- **Queued shipment: `079-S`** — status `queued`, 15 items, parent-first + dependency-ordered. Hand off to Ship after operator go-ahead.
- **Covering feature: `079-F`** — "CLI/MCP command parity phase-2: build the deferred CLI fallbacks worth building".

## Triage decisions

| Stash | Kind | Decision | Rationale |
|---|---|---|---|
| `6C6ACE00` | feature | **PROMOTE** | Highest-value coherent continuation of 078-F. Build deferred CLI fallbacks for mcp_only tools with clean shared core paths. |
| `2827CB5F` | deliberation | **RIDE-ALONG (done)** | Stage-domain deliberation-record correction (P-010). Applied 3 fixes to the phase-1 record; archived. |
| `9140F65C` | task | **DEFER** | npm-publish release-workflow infra (`@backlogit` scope + NPM_TOKEN). Disjoint domain; own shipment later. |
| `EED25928` | task | **DEFER + FLAG** | Part (a) harvest-commit topology partly Ship-owned. **Part (b) NOT ACTIONABLE HERE** — targets external autoharness `.tmpl` sources outside this workspace tree (Constitution Principle IV: no out-of-tree writes). |
| `B55985DD` | task | **DEFER** | `make docs-lint --path` wording cleanup; disjoint doc/cleanup domain. |
| `21E17BFC` | feature | **DEFER (contingency)** | Singleton MCP server — trigger condition not met (2026-04-23 triage). Do not promote unless multi-process contention recurs. |

Step 1.5 grouping: the 3 deferred task-shaped entries (9140F65C, EED25928, B55985DD) span 3 disjoint domains (release-infra / harvest-topology / docs-wording) → no coherent covering feature; deferred (documented, not silently skipped).

## Backlog created (feature 079-F → 8 tasks + 6 subtasks = 15 items)

| ID | Unit | Domain | Notes |
|---|---|---|---|
| 079-F | — | feature | Covering feature |
| 079.001-T | U1 | code+test | link CLI group (add/remove/list); extract `core.GetLinks` (LARGEST) |
| 079.001.001-ST | U1a | test | link add/remove/list parity tests (red-first) |
| 079.001.002-ST | U1b | code | link cmd + `core.GetLinks` extraction |
| 079.002-T | U2 | code+test | hooks poll/ack CLI (single test-first task) |
| 079.003-T | U3 | code+test | memory save CLI (single test-first task) |
| 079.004-T | U4 | code+test | comment add CLI + `core.AppendComment` extraction |
| 079.004.001-ST | U4a | test | comment add + append-parity tests (red-first) |
| 079.004.002-ST | U4b | code | extract `core.AppendComment` + comment cmd |
| 079.005-T | U5 | code+test | metadata types/wit/templates subcommands (single test-first task) |
| 079.006-T | U6 | config+test | flip 10 registry rows + drift-test flag-parity assertion |
| 079.006.001-ST | U6a | test | flag-parity drift-test assertion (red-first) |
| 079.006.002-ST | U6b | config | flip 10 mcp_only→cli_command rows + sync |
| 079.007-T | U7 | docs | update parity matrix + fallback guide (discoverability) |
| 079.008-T | U8 | docs/codegen | regenerate `docs/cli-reference` for new commands |

**Dependency edges (blocks):** 079.006-T ← 079.001-T, 079.002-T, 079.003-T, 079.004-T, 079.005-T; 079.007-T ← 079.006-T; 079.008-T ← 079.001-T, 079.002-T, 079.003-T, 079.004-T, 079.005-T. Subtask ordering: 079.001.002 ← 079.001.001; 079.004.002 ← 079.004.001; 079.006.002 ← 079.006.001.
**Suggested execution order:** U1‖U2‖U3‖U4‖U5 (independent) → U6 → U7; U8 after U1–U5 (parallel to U6/U7). Within each split task: test subtask before code subtask.

Decomposition rationale: split test/code subtasks ONLY for the heaviest/dual-domain units (U1 4 files, U4 extraction+refactor, U6 config+test). U2/U3/U5 collapsed to single test-first tasks (Scope Auditor endorsed collapsing thin units). U7/U8 single docs tasks.

## Artifacts

- Deliberation: `docs/decisions/2026-07-03-cli-mcp-command-parity-phase2-deliberation.md` (Option B) + native `051-DL` (linked to 6C6ACE00). Lints clean.
- Plan: `docs/exec-plans/2026-07-03-cli-mcp-command-parity-phase2-plan.md` — **plan-review PASS**. 8 units (U1–U8), Requirements Trace R1–R10, Dependency Graph, Constitution Check. `Requires plan hardening: no` (per-signal justified → P-006 gate: plan-harden correctly skipped).

## Plan-review history

- **Single gate, PASS.** 5 personas in parallel (Agent-Native Parity, Scope Boundary Auditor, Go Reviewer, Architecture Strategist, Constitution Reviewer). Result: **0 P0, 0 P1, 6 P2, 12 P3.** All actionable P2/P3 findings incorporated into the plan via edits before the gate stamp: added U8 (cli-reference regeneration to satisfy cli-reference-drift gate), Constitution Check section, flag-parity drift-test assertion (U6), D6, fixed a U4 contradiction, corrected file counts. Re-linted clean. `## Plan Review` appended with gate **PASS**.

## Scope decision (Option B)

- **BUILD (5 families):** U1 link add/remove/list; U2 hooks poll/ack; U3 memory save; U4 comment add; U5 metadata types/wit/templates — each over its shared core path, test-first with MCP output-shape parity, then U6 flips 10 registry rows mcp_only→cli_command guarded by the (extended) drift test; U7 docs; U8 regenerate cli-reference.
- **DEFER to phase-3 (documented, not dropped):** `merge_sync` CLI (writes by default; 078-S Rule-4 dry-run-default + runtime verification needed) and the export-command-map per-tool cli_command/mcp_only enhancement (requires resolving the deliberate binary-vs-`.autoharness` decoupling).
- **KEEP permanent MCP-only:** `log_telemetry` (agent-internal event logging).

## Ride-along 2827CB5F (deliberation-record reconciliation) — DONE

Corrected 3 drifts in `docs/decisions/2026-07-03-cli-mcp-command-parity-deliberation.md` (phase-1 record) and added a dated Reconciliation section (P-010: Stage owns deliberation edits). Docs lint clean.
1. Parity-matrix location `docs/cli-reference/` → `docs/reviews/` (as shipped; cli-reference is a generated zone).
2. Task count "five tasks" → "six tasks"; added T6.
3. `checkpoint create` removed from the deferred list and recorded as shipped T6.

## Stash archival

- `6C6ACE00` → archived, forward-ref to 079-F/079-S/051-DL embedded in stash text.
- `2827CB5F` → archived, forward-ref (reconciliation DONE) embedded.
- Remaining active stash (4, all deferred this session): `9140F65C`, `EED25928` (part b not-actionable/out-of-tree), `B55985DD`, `21E17BFC` (contingency).

## Environment note for Ship (IMPORTANT)

- The repo-root `backlogit.exe` (**v1.2.0**) is **stale — a pre-078-S build**. It lacks the `backlogit shipment add` subcommand that phase-1 (078-S) delivered and that `.autoharness/backlog-registry.yaml` maps `add_to_shipment` → `backlogit shipment add`. Because incremental `shipment add` was unavailable, Stage assembled `079-S` via `shipment create --items <full 15-item CSV>` (delete+recreate of the initial feature-only shipment). The resulting manifest is correct (verified via `shipment get`: 15 items, parent-first). **Ship should rebuild the binary from `main` HEAD before execution** so registry-declared CLI fallbacks resolve.
- The stale binary also means U6's drift test (which asserts registry cli_commands resolve against the cobra tree) must be run against a freshly built binary, not the checked-in one.

## Risks / notes for Ship

- U6 depends on U1–U5 landing first (drift test asserts the flipped cli_commands resolve). U7 depends on U6. U8 (regenerate cli-reference) depends on U1–U5.
- `get_links` + `append_comment` have NO existing core helper — U1/U4 require refactor-preserving extractions (`core.GetLinks`, `core.AppendComment`). `core.LinkCommit` (commits.go:27) is the exact template for `core.AppendComment`; preserve zero-Timestamp value passing (do not pre-stamp). No import cycle: core already imports db+events.
- U6 also renames the append_comment registry param `task_id`→`item_id` and adds a flag-parity assertion to `internal/cli/registry_parity_test.go` (the existing drift test checks command-path resolution + mcp_only exclusivity, NOT flag names).
- New cobra commands REQUIRE `docs/cli-reference/` regeneration (U8) or the `cli-reference-drift.yml` CI gate goes red. Do NOT hand-author under `docs/cli-reference/`.
- gofmt -l false-positives on Windows CRLF checkout — do NOT mass-reformat; CI (LF) authoritative.
- merge_sync + log_telemetry stay mcp_only (must remain mutually-exclusive in the registry after U6's flip).

## Role-boundary confirmation

Stage produced planning artifacts + backlog structure + a queued shipment only. **No production code written, no feature/chore branch created, no build run, no PR opened.** Stayed in-tree (Principle IV honored — EED25928 part (b) out-of-tree work flagged not-actionable, not attempted). Backlog mutations via registry-backed `backlogit` CLI (no parallel markdown trackers). Shipment left in `queued`. Git commits (planning/backlog artifacts) on `main` only — within Stage's git boundary.
