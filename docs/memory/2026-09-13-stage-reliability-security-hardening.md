# Stage Session Memory — 2026-09-13 reliability + PR#425 security hardening

**Mode**: P-017 dark-factory, DARK_MODE_ACTIVE. Stage-only (no source/build/branch/PR).
**Commit**: 88c54c7a (local `main`, +1 ahead of origin/main, NOT pushed — integration reserved for Ship / operator).

## Stash triage (8 scoped IDs, all archived `state: removed`)

| Stash | Kind | Route | Disposition |
|---|---|---|---|
| 6D53A33F | bug | deliberate | Cross-workspace (autoharness gate presentation). No in-repo item. Non-blocking. Decision doc recorded. |
| CC0EBB59 | bug | deliberate→plan→harvest | 173-F / 154-S (enabling precondition) |
| 1E0C2251 | bug | deliberate→plan→harvest | 172-F / 153-S (with FE440C62) |
| FE440C62 | bug | deliberate→plan→harvest | 172-F / 153-S (with 1E0C2251) |
| 3B661FCE | feature | deliberate→plan→harvest | 168.007-T/168.008-T → 149-S |
| BFF76433 | feature | deliberate→plan→harvest | 169.007-T/169.008-T → 150-S |
| BF18DA1D | feature | deliberate→plan→harvest | (merged w/ BFF76433) 169.007-T/169.008-T → 150-S |
| 4E210DB4 | feature | deliberate→plan→harvest | 170.008-T → 151-S |

## Grouping (Step 1.5)
- Group A (concurrency, same code surface): FE440C62 + 1E0C2251 → 172-F / 153-S. Internal edge FE→CAS.
- Group B (shipment lifecycle surface): CC0EBB59 → 173-F / 154-S.
- Group C (PR#425 workspace-writer threat model): 3B661FCE / BFF76433+BF18DA1D / 4E210DB4 → hardening tasks into existing 168/169/170 + 149/150/151-S.

## Artifacts created
- Decisions: 6d53a33f-disposition, concurrency-writer-hardening-deliberation, shipment-claim-scheduler-reconciliation-deliberation, pr425-workspace-writer-hardening-deliberation.
- Plans: concurrency-writer-hardening-plan, shipment-claim-scheduler-reconciliation-plan, pr425-workspace-writer-hardening-plan.

## Gates
- Plan hardening (P-006): all three plans declared + carry `## Plan Hardening`. Satisfied.
- Plan review (multi-agent-dispatch): attempt 1 FAIL (P1 each), remediated, attempt 2 PASS all three. Records appended to each plan.

## Shipments
- 153-S (queued, root): 172-F + 172.001–007-T.
- 154-S (queued, root): 173-F + 173.001–005-T.
- 149-S (queued): +168.007-T, +168.008-T.
- 150-S (queued): +169.007-T, +169.008-T.
- 151-S (queued): +170.008-T.

## Dependency edges (real contracts; NO numeric adjacency added)
- 172: 002←001; 003←002; 004←002,003; 005←002; 006←002; 007←005,006.
- 173: 002←001; 003←001; 004←001,002,003.
- 168.007←168.001,168.003; 168.008←168.007; 169.007←169.002,169.003; 169.008←169.007; 170.008←170.003.
- Existing DAG (141-S chain; 149/150/151 edges) unchanged. Reliability shipments are independent roots; reliability-first execution order is a scheduling preference only.

## Blockers / caveats for the dark run
- 152-S claimability: autoharness pre_claim implicit numeric predecessor still active (6D53A33F remedy is out-of-workspace; `--force` NOT authorized). Ship-time claim caveat, not a Stage blocker.
- 170.008-T: Option A required before Ship claims 151-S, else block 151-S (no document-only closure).

## Next steps (Ship, not Stage)
- Claim shipments in reliability-weighted order; confirm hardening-task sequencing before claiming 149/150/151-S.
