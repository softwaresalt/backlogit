# Stage memory — 154-S wave-admission remediation

- **Session date:** 2026-09-13T22:49 (local)
- **Agent:** Stage (backlog/planning remediation, scoped)
- **Mode:** CLI-fallback (backlogit MCP tools not in tool surface; registry-declared `backlogit` CLI used; `TOOL_DEGRADED`/CLI-fallback is a legitimate operating mode). `INDEX_SYNC_OK` at start and end.
- **Trigger:** Ship halted on active shipment `154-S` with `WAVE_NO_PROGRESS: active residual` — all seven members `173.001-T`..`173.007-T` were `active`, so the P-002.6 wave scheduler (`while($true){ $waveIndex++ }`, 1-based; halts before admission when any in-scope member is active) could compute no ready set. RED harness tasks `173.006-T`/`173.007-T` also lacked canonical `red-deliverable-contract` blocks.

## Scope (frozen)
Backlog/planning artifacts for feature `173-F` and its seven tasks only. Did NOT touch: application source, tests, active shipment manifest `154-S` (membership + status), git history, PRs, or unrelated backlog items.

## Authorization determination
- **Status reconcile active→queued:** VALID + AUTHORIZED. Gate broker (`UpdateArtifactWithGate`) engages only on task/subtask entry into a *terminal* status; queued is non-terminal, so the path is ungated. 144-F governance refusals guard only shipment→shipped, not member status. "Update backlog items" is within Stage's role; shipment manifest membership was untouched.
- **Contract blocks:** authoritative schema from `scripts/wave-scheduler-sim.ps1` `Read-RedDeliverableContract` (parser is the operational authority). Canonical form = `<!-- BEGIN:red-deliverable-contract -->` + blank line + ```` ```text ```` fence + exactly 5 keys in order (`red_deliverable`, `red_deliverable_reason`, `red_selector_command`, `green_maker_tasks`, `green_maker_closes_wave`) + fence close + `<!-- END:red-deliverable-contract -->`. Exemplar: archived `167.020-T`. (Bare/un-fenced form in `156.004-T` would FAIL the parser.)

## Actions taken
1. Reconciled member statuses `active`→`queued` via `backlogit move <id> --status queued` for all seven (173.001-T..173.007-T). This restored the committed HEAD baseline (all seven were committed as `queued`; Ship's claim had activated them in the working tree).
2. Appended canonical `text`-fenced `red-deliverable-contract` blocks to `173.006-T` and `173.007-T`.
3. `backlogit sync` → `Indexed 1472 artifacts` (`INDEX_SYNC_OK`).

## Dependency graph / wave map (derived; used for contract wave numbers)
- Wave 1: 173.006-T (U0a lifecycle RED), 173.007-T (U0b read-surface RED) — no deps
- Wave 2: 173.001-T (U1 persist marker; dep 006)
- Wave 3: 173.002-T (U2 read projection; deps 001,007), 173.003-T (U3 rollback clear; deps 001,006)
- Wave 4: 173.004-T (U4 concurrency/integration; deps 001,002,003), 173.005-T (U5 docs; dep 002)

## Contracts written (validated with the real parser — 0 errors each)
- **173.006-T** (lifecycle): `red_selector_command: go test -count=1 -run '^TestU0a_ClaimMarkerLifecycle$' ./internal/core`; `green_maker_tasks: 173.001-T, 173.003-T`; `green_maker_closes_wave: 3`.
- **173.007-T** (read surface, both transports): `red_selector_command: go test -count=1 -run '^TestU0b_ClaimMarkerReadSurface$' ./internal/cli ./internal/mcp`; `green_maker_tasks: 173.001-T, 173.002-T`; `green_maker_closes_wave: 3`.

## Verification
- `Read-RedDeliverableContract` (extracted from the live sim script) parsed both blocks with `Errors: <none>`, `red_deliverable=True`, green-makers resolving to real members, integer close-wave 3.
- `backlogit query`: 173-F=active (covering feature, unchanged); 173.001-T..173.007-T all `queued`.
- `154-S` re-read: status=active, all 8 items present, `updated_at` 2026-09-14T05:05:25 unchanged from intake → manifest preserved.

## Preserved / untouched (Ship-owned, cross-role — left as-is)
- `154-S.md` (active claim), `173-F.md` (active), `.backlogit/checkpoints/checkpoint-20260914-054250.json`, `.backlogit/reconcile/154-S-pre-20260914T053515Z.md`, `docs/memory/2026-09-13-ship-154s-wave-admission-blocked.md`.

## Handoff to Ship (resume)
Shipment `154-S` remains active/claimed. All seven members are `queued`; the two RED-deliverable tasks now carry canonical contract blocks. Ship can resume the harness/wave-admission step: wave 1 will admit {173.006-T, 173.007-T}. No further Stage action required.
