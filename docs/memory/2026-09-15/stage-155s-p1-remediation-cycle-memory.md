# Stage memory — 155-S current-HEAD P1 remediation cycle (2026-09-15)

Branch: `chore/stage-155`  ·  Base HEAD at start: `36aae220`  ·  Stage route: anthropic/claude-opus-4.8/high

## Scope
Bounded, single current-HEAD remediation of five P1 findings from final code review against
feature `174-F` / shipment `155-S` (queued) and `146-S`/`164-F`. Stage-owned planning + backlog
artifacts ONLY — no source/tests, no `154-S` edit, no shipment claim, no PR. Append-only review
history and blocked/superseded tasks preserved.

## Findings resolved
1. **F1 RED contracts** — added canonical `red-deliverable-contract` blocks to `174.039-T` (R1),
   `174.040-T` (R2), `174.041-T` (R3). Selectors `go test -count=1 -run '^TestUR{1,2,3}_'
   ./internal/core`. Green-makers/closing waves: R1={043,044}@4, R2={042,045,051}@5, R3={047,048}@6.
   Validated with the actual `scripts/wave-scheduler-sim.ps1` parser (`Read-RedDeliverableContract`
   + `Test-TaskScopedCommandShape`) → 3/3 clean.
2. **F2 governed block commit** — plan §0.2 / decision §6.2 / spec §0.3 now require commit of the
   FULL governed output (`154-S` record + changed member artifacts + intent/preimage/snapshot
   recovery state + per-item event logs), not `154-S`+snapshot only.
3. **F3 164.002-T retired** — parked-era shipment-record-only `queued→active` forward-repair is
   incompatible with rev3 exclusive activation; subsumed by R9(047)/R11(049). Set `blocked`,
   obsolete `→174.050-T` dep removed, history preserved.
4. **F4 release-unit ordering** — F3 dep removal decouples `146-S` from `155-S`; no shipment-level
   `blocks` edge needed (consistent with retire choice). Verified 146-S members (164-F, 164.002-T)
   carry no dep to 155-S.
5. **F5 174.045-T split** — R7→R7a(`174.045-T`, bypass WRITE paths)+R7b(new `174.051-T`,
   create-as-active/activation refusal). Each ≤5 fns, ~2h, RED-before-GREEN. `174.051-T` deps
   `[040,042,044]`, added to `155-S` manifest after `174.045-T`. §0.1 table/graph/topo updated in
   plan + spec.

## 155-S manifest (14 members, all queued)
174-F, 174.039-T, 174.040-T, 174.041-T, 174.042-T, 174.043-T, 174.044-T, 174.045-T, 174.051-T,
174.046-T, 174.047-T, 174.048-T, 174.049-T, 174.050-T.

## Validation
- `backlogit sync` → parse_failures=0, 1525 artifacts indexed.
- `backlogit doctor --check-orphans --check-duplicates` → 23 pre-existing findings (016.x/106.x),
  NONE on touched artifacts.
- `wave-scheduler-sim.ps1 -VerifyAgainstQueue` → WAVE_SIM_OK 186/186.
- RED contract parser → 3/3 clean, selector shape OK.
- `backlogit docs lint` on all 3 docs → findings: [].

## Residual
- P0 = 0, P1 = 0.
- Observation (P2, out of scope): `146-S`/`164-F` now has no live actionable task member
  (164.001-T removed earlier; 164.002-T retired). Whether to retire `146-S`/`164-F` entirely is a
  separate operator decision — flagged, not actioned (avoid scope creep beyond the blocked-lifecycle ask).

## Files changed
.backlogit/queue/{155-S,164.002-T,174.039-T,174.040-T,174.041-T,174.045-T,174.051-T}.md;
docs/{exec-plans,product-specs,decisions}/2026-09-14-resumable-shipment-blocked-lifecycle-*.md
