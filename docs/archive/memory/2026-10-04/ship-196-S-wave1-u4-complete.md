# 196-S wave 1 — U4 complete

- **Shipment / feature:** `196-S` / `196-F`
- **Task:** `196.004-T` — U4 explicit-feature shipment-release characterization
- **Status:** `done`
- **Branch:** `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`
- **Completion HEAD:** `ebf010d41cff8f0971c90394abd18c11e9f8214e`
- **Commits associated:** `0d094fc755b4e880641482aec2fb0cd12ef830e8`, `ebf010d41cff8f0971c90394abd18c11e9f8214e`

## Evidence

- The exact `verification-only` completion command passed and printed
  `EXEMPT_VERIFY_OK:196.004-T`, including the named test and both required
  subtests.
- The pre-work probe had failed as required because the test file did not yet
  exist. The clean exemption baseline was
  `ee04d2318af5500bf33dcc1ae59360fb2c8743c7`.
- P-002.4 path and content checks passed: the only task delta is the authorized
  new file `internal/core/shipment_explicit_feature_release_test.go`; no
  production Go or repository configuration changed.
- CI-pinned golangci-lint v1.64.8 passed with zero findings. Local
  golangci-lint v2.13.2 reported zero new findings across the four Go files
  changed by the release branch relative to merge-base
  `2741626afdd92e16262c365c689a54545e298f6e`.
- LF-canonical `gofmt -l .` on the committed tree returned no files.
- The report-only review covered current HEAD
  `ebf010d41cff8f0971c90394abd18c11e9f8214e` and was `READY`, with no
  unresolved P0/P1/P2/P3 findings. Two P3 suggestions from the initial review
  were incorporated: required table-driven subtests and the persisted shipment
  archive status (`shipped`).
- Engram was unavailable after a readiness retry and its symbol/map/impact
  calls timed out. The documented degraded fallback used exact-symbol local
  lookups; structural coverage is recorded as degraded. Relevant compound
  guidance confirmed archive-aware metadata reads and did not require an
  unlisted sibling outside this task's explicit-member fixture.

## Wave and next steps

Wave 1 is not converged. `196.001-T` and `196.003-T` are also `done` as
red-deliverable tasks. Their selectors, plus U8's, remain intentionally open:

- `^TestUSR1_` — green-maker `196.002-T` in wave 2.
- `^TestUSR3_` — green-maker `196.005-T` in wave 2.
- `^TestUSR8_` — green-maker `196.009-T` in wave 3.

Next, execute `196.007-T` using its exact verification-only contract, then
`196.008-T` as the wave-1 red deliverable. Repeat the R14 served-root
attestation, including `pragma_database_list`, at each task claim before raw
log reads. Run wave-1 convergence only after all five members are individually
complete; repo-wide vet and the remaining declared gates are still pending.
Continue with waves 2–4 on this same branch and worktree.
