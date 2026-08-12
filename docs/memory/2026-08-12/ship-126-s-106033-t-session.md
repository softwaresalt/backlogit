---
title: Ship session — shipment 126-S / task 106.033-T (repository-ref CAS/guard)
doc_type: memory
schema_version: "1.0"
---

# Ship session — 126-S / 106.033-T

## Task IDs completed

- `106.033-T` — Repository-ref CAS/guard for shipment completion window (post manifest-signing HEAD drift). Status: `done` → archived.
- `126-S` — Formal gate shipment-completion HEAD guard (repository-ref CAS) shipment. Status: `shipped` → archived.

## Pull requests / merge commits

| PR | Purpose | Merge commit | Merge method |
|---|---|---|---|
| #357 | Stage staging PR (manifest + memory) | `81813527` | merge commit |
| #358 | Implementation PR (CAS/guard fix + tests) | `49d742fc327ea2960ce08af2ac56db403c2365ac` | merge commit |
| #359 | Post-merge backlog closure PR (task/shipment done+archive) | `c9c010d1e484532e1048b6155cffe1d0999e27ed` | merge commit |

All three merges verified as true two-parent merge commits (`git log --merges --format="%H %P"`); no squash/rebase/force-push used anywhere in the session.

## Files modified (PR #358)

- `internal/core/shipment_gate.go` — `gateShipmentCompletion` now returns `(shipmentHead string, err error)` instead of just `error`, propagating the HEAD its own pre/post drift bracket validated as stable.
- `internal/core/shipment.go` — `moveShipmentStatusWithTopLevel` refactored into a thin wrapper over new `moveShipmentStatusWithHeadGuard(..., expectedHeadSHA string)`, which re-resolves HEAD immediately before the shipment-status persist and refuses (fail closed) on drift; appends a best-effort `EventGateBlocked` (with expected/observed heads) on refusal so the audit log never shows only a pass.
- `internal/core/shipment_lifecycle.go` — `ShipShipment` captures `gatedHead` from `gateShipmentCompletion` and threads it into the final `moveShipmentStatusWithHeadGuard` call.
- `internal/core/shipment_gate_manifest_test.go` — updated two direct callers of `gateShipmentCompletion` for its new two-value signature.
- `internal/core/shipment_gate_completion_cas_test.go` (new) — TDD coverage: `TestShipmentGate_HeadDriftBetweenGateCheckAndPersist_Refuses` (RED-first, injects a concurrent commit via a `HookMoveShipmentStatus` pre-hook, confirms refusal) and `TestShipmentGate_NoHeadDriftBeforePersist_ShipsCleanly` (happy-path regression).

## Design decisions

- Implemented stage-memory option **(a) + (c)**: an additional CAS-style HEAD re-check immediately before the shipment-status persist (a), paired with accepting the further-narrowed (not eliminated) residual window as a documented, monitored limitation (c). Option (b) (git-level advisory ref-lock) was **not** implemented, per the stage memory's assessment of its materially larger design surface.
- An empty `expectedHeadSHA` leaves the new guard inert — every existing call site (`ClaimShipment`, tests, etc.) is unaffected; only `ShipShipment`'s `active→shipped` transition passes a non-empty head.

## Copilot review cycles (PR #358)

Two rounds, 6 findings total:

1. Round 1 (5 findings, all addressed):
   - Fixed: append `EventGateBlocked` audit event on the new guard's refusal (previously only a pass was ever recorded).
   - Fixed: assert the git-checkout result in the test's drift-injection fixture instead of ignoring it.
   - Fixed: stale `moveShipmentStatusWithTopLevel` references in test doc comments → `moveShipmentStatusWithHeadGuard`.
   - Fixed: import grouping (stdlib/external/internal) in the new test file.
   - **Deferred** (not fixed in this PR): compensating rollback of `completeReleaseScope`/`returnUnreleasedFeatureItems`/`setArtifactStatus` mutations on a late `ShipShipment` failure — a pre-existing structural gap (not a regression), judged a materially larger design surface than 106.033-T's scope (comparable to already-deferred option b). Tracked as backlog follow-up **`3A649F8E`**.
2. Round 2 (1 finding, addressed via doc-comment correction + scope decline):
   - The doc comment understated the residual window ("a few, all in-process, instructions") — `persistArtifact` itself still does validation/path-resolution/registry-load/snapshotting before its first mutating write. Corrected the doc comment for accuracy; declined moving the guard inside `persistArtifact` itself (shared across every artifact type, not just shipments) as out of scope. Folded into the same follow-up `3A649F8E`.

All 6 review threads resolved via `gh api graphql resolveReviewThread`. Final pre-merge readiness gate (§1.9: no pending Copilot review request, latest review covers current HEAD, zero unresolved Copilot threads) passed before each merge (#358 at HEAD `6ee731d4`, #359 at HEAD `b8edc8bb`).

## Quality gates (all green before each merge)

- `go build ./cmd/backlogit` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (all packages, including new TDD tests GREEN)
- `golangci-lint run` — pass (zero warnings, full repo)
- `gofmt -l .` — flags nearly the entire repository due to pre-existing `core.autocrlf=true` CRLF/LF normalization on this Windows checkout (not a regression; no `gofmt` step exists in CI workflows). Not treated as a blocking finding.

## Follow-up backlog items created

- `3A649F8E` (task, medium priority): two related, deliberately-deferred findings — (1) no compensating rollback in `ShipShipment` for a late-failure partial release, (2) a repository-wide (not shipment-specific) `persistArtifact`-level CAS tightening. Both judged out of scope for 106.033-T per the same "materially larger design surface" rationale the stage memory already applied to option (b).

## Residual risk / known limitations

The repository-ref CAS/guard **narrows but does not eliminate** the HEAD-drift race: git has no atomic "read HEAD and complete our write" primitive, and `persistArtifact`'s own non-mutating preparation (path resolution, registry load, snapshotting) still runs after the final guard check. This is an accepted, documented, monitored limitation (option c), not a defect. The two deferred findings above are the concrete follow-up path if further narrowing is ever required.

## Next steps for future sessions

- Pick up backlog stash `3A649F8E` when scheduling further shipment-completion hardening work.
- No other open items for 126-S / 106.033-T; both are fully closed and archived.
