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

---

## Follow-up cycle — PR #444 contract-alignment P1 (2026-09-15, HEAD `5e3e1001`)

Bounded single in-scope P1 remediation on `chore/stage-155`. Stage-owned docs/backlog ONLY —
no source/tests, no `154-S` edit, no shipment claim, no PR; caller owns GitHub thread replies.

### Finding resolved
**P1 — unblock-confirmation + resume-flag divergence between the authoritative product contract
and the live [local] API tasks.** The authoritative contract (spec SBLK-R5/R13, plan L335/L827,
decision) requires (a) explicit `--confirm` for BOTH `blocked→queued` AND `blocked→active`, and
(b) CLI flag `--resume-checkpoint`. The exact-shape task surfaces still carried the pre-contract
"active-only confirmation" wording, and R8 mapped the stale flag `--resume-ref`. Aligned all live
task surfaces to the contract; plan/spec/decision already agreed and were left unchanged.

### Surfaces aligned (task bodies + comments; no struct-field additions)
- `174.039-T` (R1s) — `UnblockOptions.Confirm` comment: REQUIRED for BOTH targets (active also
  needs the free-active-slot check). Struct/type names unchanged, so the go/ast source-shape
  harness stays intact.
- `174.052-T` (Rd) — `Confirm` stub comment: REQUIRED for BOTH targets.
- `174.053-T` (R1b behavior RED) — target-aware unblock assertion: BOTH targets require
  `Confirm==true`; active additionally requires free-active-slot.
- `174.044-T` (R6) — governed UnblockShipment: both targets require `opts.Confirm==true` (refuse
  when absent); active additionally requires free-active-slot.
- `174.046-T` (R8 CLI/MCP parity) — block mapping `--resume-checkpoint`/`resume_checkpoint_ref`
  → `BlockOptions.ResumeCheckpointRef`; unblock `--confirm`/`confirm` → `Confirm` (required for
  BOTH targets). MCP param `resume_checkpoint_ref` already matched the spec field name.

### Resume-flag semantics
`ResumeCheckpointRef` belongs to `BlockOptions` metadata only (block path). Unblock has no resume
flag — no field invented; `--resume-checkpoint` maps to `BlockOptions.ResumeCheckpointRef` on the
block command. Consistent with spec SBLK-R13 and audit field `resume_checkpoint_ref`.

### Final exact API shape (unchanged struct fields; semantics clarified)
- `BlockOptions{ Reason (required non-empty→blocked_reason), BlockedBy (→blocked_by),
  ResumeCheckpointRef (optional→resume_checkpoint_ref) }`
- `UnblockOptions{ Target (ShipmentQueued|ShipmentActive), Confirm (REQUIRED true for BOTH
  targets), UnblockedBy }`
- `BlockShipment(ctx, ws, shipmentID string, opts BlockOptions) (*models.Artifact, error)`
- `UnblockShipment(ctx, ws, shipmentID string, opts UnblockOptions) (*models.Artifact, error)`
- CLI: block `--reason`/`--by`/`--resume-checkpoint`; unblock `--to queued|active` `--confirm`
  (both targets) `--by`. MCP: `reason`/`by`/`resume_checkpoint_ref`; `target`/`confirm`/`by`.

### Validation
- `backlogit docs lint` → valid, 0 findings.
- `backlogit sync` → parse_failures=0, 1528 artifacts indexed.
- `scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue` (actual parser + wave sim) → WAVE_SIM_OK
  186/186 across 21 scenarios.
- `backlogit doctor` (dependency/manifest structural) → 23 pre-existing orphans (016.x/106.x),
  NONE on touched artifacts; target-mode schema validation of all 5 edited files → exit 0.
- `shipment get 155-S` → manifest membership intact (174-F first; all 5 edited tasks present).
- Targeted text/contract assertions → ALL_ASSERTIONS_PASS (no residual `--resume-ref` /
  active-only wording; confirm-for-both + `--resume-checkpoint` present on all current surfaces).

### Residual
- P0 = 0, P1 = 0 (in-scope).

### Files changed (this cycle)
.backlogit/queue/{174.039-T,174.044-T,174.046-T,174.052-T,174.053-T}.md; this memory file.
