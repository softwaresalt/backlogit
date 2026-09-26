# Shipment 155-S — Wave 7 early-green blocker

Status: blocked at the Wave 7 convergence gate

## Resumed execution

- Restored `checkpoint-20260921-212008.json`.
- Branch: `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`
- Current HEAD before this checkpoint: `eda253453726ce051eeca3426baaf6525e48819a`
- Shipment `155-S` remains the sole active shipment.
- Shipment `154-S` remains queued and was not mutated.
- PR #449 and `stage/baseline-convergence-decomposed` were not touched.

## Tasks completed during this continuation

- Wave 4: `174.042-T`, `174.054-T`
- Wave 5: `174.043-T`
- Wave 6: `174.044-T`
- Wave 7 RED contracts: `174.055-T`, `174.056-T`, `174.059-T`, `174.060-T`, `174.061-T`, `174.063-T`
- Wave 7 implementation: `174.045-T`, `174.046-T`, `174.047-T`, `174.051-T`

## Blocker

`WAVE_RED_DELIVERABLE_EARLY_GREEN`

The declared open RED selector `^TestUR3_`, owned by `174.041-T`, is GREEN at Wave 7 convergence:

```text
go test -count=1 -run '^TestUR3_' ./internal/core
ok github.com/softwaresalt/backlogit/internal/core
```

Its canonical red-deliverable mapping names both `174.047-T` and `174.048-T` as green-makers and
declares closing wave 8. `174.047-T` is complete, but `174.048-T` is still active and scheduled for
Wave 8. P-002.6 requires every still-open selector to remain RED; therefore Wave 7 cannot converge
and Wave 8 cannot be admitted without a Stage-owned contract/manifest amendment or other explicit
resolution of the mapping inconsistency.

No attempt was made to widen Wave 7, pull `174.048-T` forward, weaken the selector, or revert the
valid R9 recovery implementation.

## Passing evidence at the blocker

- `go test -run=^$ -count=1 ./...`
- `go test -count=1 -run '^TestUR1B_' ./internal/core`
- `go test -count=1 -run '^TestUR2_' ./internal/core`
- `go test -count=1 -run '^TestUR8_' ./internal/cli`
- `go vet ./...`
- CI-pinned golangci-lint v1.64.8

The six flat-manifest RED selectors remain RED as declared:

- `^TestUReleaseScopeFlat_`
- `^TestUReleaseScopeRegression_`
- `^TestURollbackScopeFlat_`
- `^TestUArchiveCandidateFlat_`
- `^TestUNonMemberRollupSafe_`
- `^TestUPostShipHookCascadeGlobal_`

## Required next step

Route the inconsistent `^TestUR3_` red-deliverable closing contract to Stage for amendment. Resume
Ship only after the authoritative mapping/schedule makes Wave 7 convergence satisfiable.

## Report-only review

Reviewed HEAD `edfc04948104130b24cda18c9f84ae7c264df259` is **NOT READY**:

- P1: `ClaimShipment` does not serialize with the existing per-shipment membership writers.
- P1: startup rollback recovery restores preimages without a CAS/drift check and may overwrite
  post-crash edits.
- P1: `backlogit_normalize_blocked_shipment` has no backlog-registry operation mapping; registry
  parity fails.
- P2: generic creation can create a shipment directly in `blocked`.
- P2: generic updates may return failure after the artifact mutation is already committed when the
  final audit append fails.

These findings require P-021 scope classification before remediation. No review fix was attempted
after the convergence halt.
