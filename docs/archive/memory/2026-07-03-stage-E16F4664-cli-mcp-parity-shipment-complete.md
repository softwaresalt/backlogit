# Stage session — E16F4664 CLI/MCP command parity — COMPLETE

- **Date:** 2026-07-03
- **Agent:** Stage (stash-to-backlog pipeline)
- **Status:** COMPLETE — queued shipment `078-S` handed off
- **Stash processed:** `E16F4664` (feature, medium) + `7ECBAC7E` (task, low, folded in)

## Outcome (handoff token)

- **Queued shipment: `078-S`** — status `queued`, 15 items, parent-first ordered. Hand off to Ship after operator go-ahead.
- **Covering feature: `078-F`** — "CLI/MCP command parity: honest fallback map + highest-value gap fills".

## Backlog created

| ID | Unit | Domain | Notes |
|---|---|---|---|
| 078-F | — | feature | Covering feature |
| 078.001-T | U1 | docs | Parity audit matrix → docs/reviews/ |
| 078.002-T | U2 | config+test | Registry correction + drift test |
| 078.002.001-ST | U2 | test | Drift test (red) |
| 078.002.002-ST | U2 | config | Correct registry YAML → green |
| 078.003-T | U3 | code+test | `shipment add` CLI |
| 078.003.001-ST | U3 | test | shipment add tests (3 scenarios) |
| 078.003.002-ST | U3 | code | newShipmentAddCmd |
| 078.004-T | U4 | code+test | shipment-list items null→[] (folds 7ECBAC7E) |
| 078.004.001-ST | U4 | test | cross-surface items-array guard test |
| 078.004.002-ST | U4 | code | normalize items in list handler |
| 078.005-T | U5 | docs | MCP→CLI fallback + discoverability guide → docs/design-docs/ |
| 078.006-T | U6 | code+test | `checkpoint create` CLI |
| 078.006.001-ST | U6 | test | checkpoint create tests |
| 078.006.002-ST | U6 | code | newCheckpointCreateCmd |

**Dependency edges (blocks):** 078.002-T ← 078.001-T, 078.003-T, 078.006-T; 078.004-T ← 078.003-T; 078.005-T ← 078.001-T, 078.002-T. Execution order: U1 → U3 → U4 → U6 → U2 → U5.

## Artifacts

- Deliberation: `docs/decisions/2026-07-03-cli-mcp-command-parity-deliberation.md` (Option B) + native `050-DL` (linked to E16F4664). Lints clean.
- Plan: `docs/exec-plans/2026-07-03-cli-mcp-command-parity-plan.md` — **plan-review PASS (attempt 2)**. Lints clean (0 violations). `Requires plan hardening: no` (P-006 gate: plan-harden correctly skipped).
- Triage checkpoint: `docs/memory/2026-07-03-stage-E16F4664-cli-mcp-parity-triage.md`.

## Plan-review history

- **Attempt 1:** FAIL (6 personas). P0 = generated-docs collision (matrix/guide slated for `docs/cli-reference/`, a gen-docs zone). P1 = drift test parsing `manifest` text instead of the typed MCP tool set.
- **Revision:** relocated docs to `docs/reviews/` + `docs/design-docs/`; drift test driven from `mcp.Server.ListTools()`.
- **Attempt 2:** Learnings Researcher PASS, Architecture Strategist PASS, Agent-Native Parity PASS (after fix). New P1 raised + resolved: U5/U2(iv)/R5 pointed the agent at `metadata export-command-map` for the MCP→CLI mapping, but that command reads the `.backlogit` routing registry (verified: nothing in internal/ or cmd/ reads `.autoharness/backlog-registry.yaml`) and renders two disjoint lists. Repointed the fallback mapping's source of truth to `.autoharness/backlog-registry.yaml` (guarded by the U2 drift test); demoted export-command-map to a discoverability aid. **Gate: PASS.**

## 7ECBAC7E decision: FOLD IN

Folded into `078.004-T` (U4). Rationale: no `link` CLI command exists to link instead, and folding avoids duplicated scope (E16F4664 explicitly stated it is broader than 7ECBAC7E). Archived with a forward-ref marker embedded in its stash text.

## Deferred to phase-2

New follow-up stash **`6C6ACE00`** (feature, low): net-new CLI command surfaces — link group, poll/ack_hook_events, save_memory, append_comment, get_wit_metadata/list_types/list_templates, merge_sync; plus the optional export-command-map pairing-render enhancement. `log_telemetry` flagged as likely intentional-permanent MCP-only.

## Stash archival

- `E16F4664` → archived, forward-ref to 078-F/078-S/050-DL embedded.
- `7ECBAC7E` → archived, forward-ref to 078.004-T embedded.

## Grounded audit summary (56 MCP tools vs cobra CLI)

- Class A STALE (9): registry cli_command empty but CLI exists (deliberate, metadata catalog, export-command-map, version, telemetry harvest, stash get/edit/archive/remove).
- Class B MISSING (3): docs_lint/migrate/scope have MCP+CLI but no registry rows.
- Class C OVER-CLAIM (3, dangerous): add_link/remove_link/get_links map to a non-existent `backlogit link` command.
- Class D TRUE GAPS (14 incl link): highest-value add_to_shipment (→U3); create_checkpoint (→U6); rest deferred.
- Class E CLI-only (intentional): init, mcp, manifest, completion, etc.

## Risks / notes for Ship

- U2/U3/U6 ordering: `shipment add` (U3) and `checkpoint create` (U6) MUST land before U2 sets their `cli_command` rows (drift test asserts cli_commands resolve).
- U3 + U4 both edit `internal/cli/shipment.go` (different funcs) — sequential, not parallel.
- `mcp.Server.ListTools()` already exists (internal/mcp/dynamic.go:52); `internal/cli`→`internal/mcp` import already established — no new accessor, cycle-free.
- gofmt -l false-positives on all .go on this Windows CRLF checkout — do NOT mass-reformat; go vet + golangci-lint + CI (LF) authoritative.
- Do NOT author docs in `docs/cli-reference/` (gen-docs regenerated zone, gated by cli-reference-drift.yml).

## Role-boundary confirmation

Stage produced planning artifacts + backlog structure + a queued shipment only. No production code written, no feature branch, no build/PR. Operator in-flux files untouched. Shipment left in `queued`.
