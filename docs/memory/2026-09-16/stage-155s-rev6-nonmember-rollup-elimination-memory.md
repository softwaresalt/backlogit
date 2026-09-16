# Stage session memory — 155-S rev6: eliminate non-member ancestor rollup race (P1 concurrency)

- **Date:** 2026-09-16
- **Branch:** `chore/stage-155-flat-shipment-scope`
- **Start HEAD:** `8f4d32e5`
- **End HEAD:** `7dcca553`
- **Agent:** Stage (docs/backlog only; no source, no PR)
- **Shipment:** `155-S` (covering feature `174-F`, status `queued`)

## Objective

Fix one current P1 concurrency defect in the planned flat-manifest shipment rollback,
staging docs/backlog only.

## The defect (P1 lost update)

`internal/core/shipment_lifecycle.go`, `ShipShipment`: rev5 task `174.057-T` removes unlisted
covering-feature ancestors from the outer artifact-mutation lock set (`rollbackIDs` /
`lockArtifactMutations`, `:603-612`), but the SEPARATE non-member status-rollup path was
explicitly preserved — `snapshotNonMemberFeatureStatuses` (`:598`) + `restoreRolledUpNonMemberFeatures`
(in-line `:684`, deferred fallback `:496`, and inside `classifyShippedEventAppendFailure` `:927`).
Because the ancestor is no longer locked, a concurrent mutation to it between snapshot and restore is
silently overwritten (lost update). Root cause: `completeReleaseScope` → `setArtifactStatus` →
`cascadePersistedParentStatuses` (`:1267`) walks UP the parent chain marking any ancestor `done` with
no shipment-membership awareness.

## Resolution (Option B — eliminate the rollup, authoritative flat-manifest rule SBLK-R28)

Eliminate the non-member ancestor rollup side effect entirely rather than re-lock/CAS-protect it.
No separate protected CAS needed. Two coordinated moves in ONE impl task (atomic — no
rolled-up-but-unreverted intermediate state):

1. Bound `cascadePersistedParentStatuses` to explicit members via an unexported context boundary
   key sourced from `explicitScopeSet`; stop the up-walk at a non-member parent. Absent key ⇒
   unchanged global cascade for every non-ship caller. Injection point (per review F1): on the exact
   ctx threaded into `completeReleaseScope`, the `:648` member loop, and `returnUnreleasedFeatureItems`
   (`:735`), AFTER the `lockArtifactMutations` ctx reassignment (`:610`), BEFORE `completeReleaseScope`.
2. Delete `snapshotNonMemberFeatureStatuses` + `restoreRolledUpNonMemberFeatures` +
   `featureStatusSnapshot`/`nonMemberFeatureSnapshots`; drop from `ShipShipment` and
   `classifyShippedEventAppendFailure` signature. Preserve the `:648` membership-guarded
   `setArtifactStatus(featureID, StatusDone, "feature released")` — listed member features still get
   governed `done`.

## Tasks created

- **`174.061-T`** — SCOPE-RED-E RED harness (`^TestUNonMemberRollupSafe_`, 3 scenarios). dep `174.044-T`
  → wave 7. red-deliverable-contract: `green_maker_tasks: 174.062-T`, `green_maker_closes_wave: 10`.
  Uses existing production seams `persistArtifactPreLockHook`, `persistArtifactWriteFn`. Per review F3,
  scenarios 2 & 3 anchor the concurrent-writer injection to a LISTED MEMBER's persist boundary (not `F`,
  which never persists post-fix) so assertions are RED-now / GREEN-after, not vacuous.
- **`174.062-T`** — SCOPE-IMPL-3 (the exact contract change; the fix above). deps
  `{174.061-T, 174.057-T, 174.058-T}` → wave 10. ≤5 functions (`cascadePersistedParentStatuses`,
  `ShipShipment` governed closure, `classifyShippedEventAppendFailure`, delete
  `snapshotNonMemberFeatureStatuses` + `restoreRolledUpNonMemberFeatures`). Domain: code.

## Coupled tasks updated

- `174.057-T` — removed "status-rollup preserved/UNCHANGED" claim; path left in place pending
  elimination by `174.062-T`.
- `174.059-T` — scoped to artifact set only; cross-references `174.062-T`/`174.061-T`.
- `174.058-T` — projection count 22 → 24.

## Docs updated

- Plan `docs/exec-plans/2026-09-14-...-plan.md` — §0.4 rev6 addendum + **Plan Review — Revision 6**
  (dispatch_mode: multi-agent-dispatch; decision: PASS).
- Spec `docs/product-specs/2026-09-14-...-status.md` — §0.5 point 8, SBLK-R28 augmentation, §0.6 rev6.
- Decision `docs/decisions/2026-09-14-...-deliberation.md` — §6j (Option A rejected / Option B chosen).

## Manifest

`155-S` now 25 members (`174-F` + 24 tasks), parent-first, acyclic. `174.061-T`/`174.062-T` appended
after `174.058-T`. Dep edges: `061→044`, `062→{061,057,058}`. Waves 9 → 10.

## Validations (all green)

- `backlogit docs lint` plan/spec/decision → `valid: true`, 0 violations.
- `backlogit sync` → 1536 artifacts, `parse_failures=0`.
- `doctor --target` on `174.057/058/059/061/062-T` + `155-S` → exit 0, `ok: true`, `kind: pass`.
- `wave-scheduler-sim.ps1` fixture **WAVE_SIM_OK 164/164**; `-VerifyAgainstQueue` **WAVE_SIM_OK 186/186**.
- Dependency/manifest: acyclic, parent-first, 25 members, edges verified.

## Plan review

Concurrency Reviewer subagent (independent, read-only over source + task contracts).
**Verdict: PASS, High confidence.** No residual P0/P1. 2 P2 advisories (F1 ctx-key injection point,
F3 member-boundary anchor for RED scenarios) + 3 P3 (F2 return-to-backlog path, F4 member-above-non-member
invariant, F5 atomicity) folded into task contracts before recording PASS.

## Residual

- **P0 = 0, P1 = 0** for this scope.
- Unrelated baseline: pre-existing `doctor` orphan findings in `016.xxx`/`106.xxx-T` bands (out of scope).

## Handoff

`155-S` remains `queued`, ready for Ship to implement SBLK-R28 test-first: RED `174.061-T` →
GREEN `174.062-T`.
