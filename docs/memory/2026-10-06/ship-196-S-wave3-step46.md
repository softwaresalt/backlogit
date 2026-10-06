# Ship 196-S: wave-3 Step 4.6 convergence gate

* Census over frozen `M` (9 tasks): 001, 002, 003, 004, 005, 007, and 008 are `done`.
  006 and 009 are `active` and claim-assigned (marker 196-S, no start record). Nothing is
  blocked or in an unsupported status.
* `ready_3` = {196.002-T, 196.005-T}. Both are `done`.
* Always-on checks:
  * Compile `go test -run='^$' -count=1 ./...`: PASS.
  * `go vet`: PASS. `golangci-lint` v1.64.8: PASS.
  * gofmt: no `*.go` changed since `72a074f1`, so the wave-2 LF result (0 paths) still holds.
* Scoped commands: the 196.002-T and 196.005-T exempt commands exit 0 with their markers.
* `newly_closed_3` = {196.001-T `^TestUSR1_`, 196.003-T `^TestUSR3_`}. Both are GREEN
  (4/4 PASS each).
* Open red after recomputation = {196.008-T `^TestUSR8_`}. It is RED as required: 1 PASS
  (PreservedInvariants) and 3 FAIL. Its green-maker is 196.009-T (wave 4). The declared
  closing wave is 3 and the effective one is 4; the drift was recorded at item 21. Wave
  index 3 does not exceed 3, so the budget holds.
* `FULL_SUITE_DEFERRED: wave 3`: the open selector `^TestUSR8_` (owner 196.008-T),
  green-maker 196.009-T, scheduled for wave 4. Compile, vet, lint, format, and every
  declared scoped command passed. The open red was re-confirmed RED and the newly closed
  selectors GREEN. The only failing selector is the declared open red.
* Next: wave 4 = {196.009-T}.