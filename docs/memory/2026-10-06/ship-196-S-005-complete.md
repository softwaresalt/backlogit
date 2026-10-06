# Ship 196-S: 196.005-T (U5) complete

* Shipment: 196-S (dark mode, P-017; scope [196-S] only). Wave 3, E1.4 order.
* Deliverables:
  * `d64afe6a` changes `.github/skills/shipment-reconcile/SKILL.md`: the
    `feature-pending-governed-completion` row, the `status-mismatch` qualifier, the PROCEED
    rule, the ShipShipment sentence, and the A3.2.2 Safe-Close step 2. It also changes one
    sentence of `_ship.agent.md` Step 6 item a.
  * `f39fd009` is the Step 4.4 fix: classification order.
* `exempt_baseline_sha`: `a1effc4a` (clean tree).
* Step 4.1a: the owner has red evidence, and the probe exited 1 without the marker.
* Step 4.1b: exactly one start record.
* Step 4.3:
  * `EXEMPT_VERIFY_OK:196.005-T`. USR3 and USR4 are GREEN.
  * USR8 is RED only in its declared subtests.
  * The path and content passes are within the covered-by surface. The preserved
    invariants still hold.
  * Vet, lint, and markdownlint are clean.
* Step 4.4: READY_WITH_FOLLOWUPS. P0=0, P1=0, P2=1, P3=6.
  * The P2, that Ship Step 6 bypasses safe-close, reuses `52D18E44`.
  * Out-of-scope P3 wording items are captured as `6387A6A2`. That entry also covers the
    U2 P3s.
* `2D682258`: its concern is discharged, because the Step 6 item a orphan clause was removed.
* E1.2 residual is recorded. Stash usage (E1.4 rule 5): none.

Next: the wave-3 Step 4.6 convergence gate.
