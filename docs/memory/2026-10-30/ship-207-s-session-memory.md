# Ship 207-S session memory (implementation phase)

* Branch: `feat/207-s-replan-queued-shipment-manifests-without-hand-edits-206-f`
* Wave 1 `206.001-T`: RemoveFromShipment/MoveShipmentItem (commits 42bdc3bc, 3aec010c)
* Wave 2 `206.002-T`: ReorderShipment (commits daeac3bf, 7f003072)
* Wave 3 `206.003-T`: CLI/MCP/registry/docs (commits 81b810c5, 3971d675)
* Local review: READY at 3971d675 (P0-P3 all 0)
* Local gates: build, vet, targeted TestU1-3 and registry parity pass; full suite deferred to CI
* Decisions: refusals wrap ErrShipmentConflict (no dedicated sentinels exist); reorder uses the manifestMutation journal protocol
* Next: push, PR, CI, Copilot gate, merge (--merge), post-merge closure
