# Ship 196-S: 196.009-T (U9) complete

* Wave 4 admission:
  * Served-Root Attestation PASS: catalog `workspace.root_path` and `workspace.storage_root`
    equal the served roots, and the pragma returns one `main` row `.backlogit\backlogit.db`.
  * 009 and 006 are claim-assigned (marker 196-S, 0 start records). `ready_4` = {196.009-T}.
* Step 2a/4.1a: covered-by, owner 196.008-T (a dependency, not exempt). The owner is harness-ready
  with Compilation PASS / Red CONFIRMED, and harness commit `413dff42` is an ancestor.
  * The destructive screen is read-only; the only match was the stream merge `2>&1`.
  * The probe exited 1 with no marker. `exempt_baseline_sha` = `6539216f` (clean tree).
* Step 4.1b: start records went 0 -> 1. Telemetry is disabled; the topology gate passed.
* Deliverable `0cdb4f0a`: three U9 edits to `.github/agents/_ship.agent.md`.
  The in-scope P3 alignment to the U9 Change 1 text is in `3a8a7744`.
* Step 4.3:
  * The exempt command exits 0 with `EXEMPT_VERIFY_OK:196.009-T`; USR8 is 4/4 PASS.
  * Path pass shows only `_ship.agent.md`. vet and lint PASS; markdownlint is clean.
* Step 4.4: P0=0, P1=0, P2=1, P3=6. Outcome READY_WITH_FOLLOWUPS.
  * Deferred: `7C9340AC` (DISCOVERY-STATUS: AMBIGUOUS CDBCB258).
* Step 4.5: comment, track_commit x2, and the move to done (gate passed).
* E1.2 residual recorded. E1.4 rule 5 stash usage: none.