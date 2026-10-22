# Ship 196-S: 196.006-T (U6) complete

* Wave 5 admission:
  * Served-Root Attestation re-ran and passed.
  * `ready_5` = {196.006-T}, a harness-required config task.
* Harness `abcffb63` (`TestUSR6_HarnessManifestDriftRecords` plus 3 subtests):
  * Compilation PASS.
  * Red CONFIRMED (3/3 assertion FAIL).
  * harness-ready label committed in `865d3b7c`, which is also the clean-tree baseline.
* Step 4.1b: start records went 0 -> 1. Telemetry is disabled; the topology gate passed.
* Deliverable `091a074e`: three manifest entries changed (checksums plus drift_reason).
  The in-scope P3 SKILL drift_reason accuracy fix is in `b2768e69`.
* AC1 recipe values equal the manifest values:

  | File | SHA-256 |
  |---|---|
  | orchestrator | `c608a6bd...e421b3` |
  | SKILL | `acef3aa9...f37f4` |
  | ship | `4757ccb9...35dfc` |

* Step 4.3:
  * USR6 PASS: top-level plus 3 subtests. U19R3 GREEN.
  * YAML parses (82 artifacts).
  * vet and lint PASS. The harness is gofmt-clean on LF.
* Step 4.4: P0=0, P1=0, P2=0, P3=2. Outcome READY_WITH_FOLLOWUPS.
  * Deferred harness hardening to `0F428816`. Discovery candidate 41FE00A1 was unrelated, so this is a new capture.
* Step 4.5: comment, track_commit x3, and the move to done (gate passed).
* E1.2 residual recorded. E1.4 rule 5 stash usage: none.