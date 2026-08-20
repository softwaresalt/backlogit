---
type: session-memory
timestamp: 2026-08-10T07:30:00Z
agent: ship
shipment: 120-S
---

# 120-S Ship Session — F5 Idempotent Multi-Mutation Envelope

## Status: POST-MERGE CLOSURE IN PROGRESS

## Implementation PR
- PR #340: feat(core): F5 — idempotent multi-mutation envelope (120-S)
- Merge commit: 1f046895e8058e97c5eec1fe5784fba758df7ce2
- Merged at: 2026-08-10T06:51:40Z

## Tasks Completed
- 106.025-T (U1): characterization test
- 106.026-T (U2): MutationEnvelope + MutationPartialError
- 106.027-T (U3): wrap AssociateCommit
- 106.028-T (U4): wrap CreateArtifact + AddDependency
- 106.029-T (U5): wrap AddItemToShipment
- 106.030-T (U6): doctor CheckPartialMutations + CLI/MCP surface
- 106.031-T (U7): governed-mutation-recovery-contract.md

## Review Cycles
- Cycle 1: 2 local review findings (P1: pre-existing commit_links row, P2: ContinuationApplied)
- Cycle 1 Copilot: 8 threads total — all resolved in 2 rounds of fixes
  - Key fixes: persistArtifact for hook bypass, dep type restore, WithoutCancel context, shipment JSONL error propagation, MCP routing, doctor shipment check

## Files Modified
- internal/errors/mutation_errors.go (new: MutationPartialError + ContinuationApplied)
- internal/core/mutation_envelope.go (new: MutationEnvelope)
- internal/core/mutation_envelope_characterization_test.go (new: U1 tests)
- internal/core/commits.go (wrap AssociateCommit; use persistArtifact not UpdateArtifact)
- internal/core/dependencies.go (wrap AddDependency with oldEdgeType restore)
- internal/core/artifacts.go (wrap CreateArtifact dependency steps)
- internal/core/shipment.go (wrap AddItemToShipment)
- internal/core/doctor.go (CheckPartialMutations, detectInconsistentShipmentMembership)
- internal/cli/doctor.go (--check-partial-mutations flag)
- internal/mcp/tools.go (mutation_partial mapping, handler routing)
- docs/design-docs/governed-mutation-recovery-contract.md (new: U7)
- docs/ARCHITECTURE.md (pointer to design doc)
- docs/cli-reference/backlogit_doctor.md (regenerated)

## Shipment Status
- 120-S: archived (shipped with commit 1f046895)
- 106-F: archived
- 106.025-T through 106.031-T: archived
- 106.001-T, 106.002-T: gate force events written, then archived

## Issues Encountered
- Gate evidence missing for 106.001-T and 106.002-T (pre-gate tasks from F2/F3)
  - Fixed by writing EventGateForced events with head_sha=1f046895
- ShipShipment succeeded after gate force

## Next Steps
- Commit backlogit changes on post-merge closure branch
- Push and create closure PR
- CI + Copilot review
- Merge closure PR
- Verify final state on origin/main
