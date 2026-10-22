# Ship 196-S: wave-4 Step 4.6 convergence gate

* Census over frozen `M` (9 tasks): 001, 002, 003, 004, 005, 007, 008, and 009 are
  `done`. 006 is `active` and claim-assigned. Nothing is blocked or in an unsupported
  status. `ready_4` = {196.009-T}, which is `done`.
* Always-on checks:
  * Compile `go test -run='^$' -count=1 ./...`: PASS. vet and lint: PASS (run at the
    009 Step 4.3).
  * gofmt: no `*.go` changed since `72a074f1`.
* Scoped commands: the 009 exempt command exits 0 with `EXEMPT_VERIFY_OK:196.009-T`.
  `^TestUSR7_` (`./internal/mcp`) is GREEN (009 AC3).
* `newly_closed_4` = {196.008-T `^TestUSR8_`}. It is GREEN, 4/4 PASS.
* The open-red set after recomputation is empty, so the unfiltered full suite ran:
  `go test -timeout=30m ./...` exited 0 with 40 `ok` packages.
  The wave-3 `FULL_SUITE_DEFERRED` is discharged.
* Deferral budget (item 4): 008 declared `green_maker_closes_wave` 3. The entry was still
  open at wave 3, where index 3 does not exceed 3. It closed at the wave-4 recomputation
  before the item-4 check, so it is not open past its closing wave and
  `WAVE_OPEN_RED_UNCLOSED` does not apply. The declared-vs-effective drift (3 vs 4) was
  recorded in the item-21 telemetry.
* Next: wave 5 = {196.006-T}, harness-required. harness-architect scaffolds `TestUSR6_`.