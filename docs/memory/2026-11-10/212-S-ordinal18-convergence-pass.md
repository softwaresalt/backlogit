---
title: "212-S Ordinal 18 unfiltered convergence PASS"
doc_type: reference
shipment_id: "212-S"
feature_id: "211-F"
phase: "pre-pr-review"
---

## Convergence evidence

The single authorized Ordinal 18 run passed at execution HEAD
`2af30b0a4b5dbdaddc41595dcb0a13d4f3cc1079`, unchanged throughout execution.
Command: `go test -count=1 -p=1 -timeout=30m -json ./...`, through the
unchanged authorized private runner and pinned Go 1.24 environment.

* Result: `bin/_rr7c19a6f2/wave12-ordinal18/result.json`
* SHA-256: `6bd4d4e23d0c41b8bd72771f23b3faaa172549b6af556871e5e5da4b2546e4a2`
* Native exit: 0; suite native exit: 0; evidence valid: true
* Package results: 46 PASS; missing: []; extra: []
* Top-level passes: 3922; reported failures: 0
* Critical named receipts: one PASS each, no fail/skip
* Elapsed: 2282.113677 seconds; budget: 2700 seconds
* Poll gaps over five seconds: 0; sleep hold acquired and cleared
* Full test child launched and cycle consumed; no prelaunch-only result
* Ordinal: 18; historical failed count: 14, unchanged without reset
* External consumption receipt: `bin/tmp/212-ordinal18-7c19a6f2-consumption.json`

The bounded raw tail intentionally omits output after its retention cap;
the recorder consumed all 44041 events and retained complete package terminal
and critical-test inventories. No rerun or token reuse is authorized.

## Build and static checks

Full build passed:
`go build -o bin/tmp/_backlogit-212s.exe ./cmd/backlogit`.
The binary identified execution HEAD 2af30b0a; help smoke and provenance
were checked, then only that temporary binary was removed as requested.
Post-suite compile-only, vet and lint gates passed.

Literal `gofmt -l .` returned exit 2 with 1214 output lines under ignored
`bin/` dependency/toolchain caches and generated temporary build artifacts.
No formatter repair or deletion was attempted. A separate read-only,
Windows-argument-safe formatting check across all 1400 tracked Go files
returned exit 0 with no unformatted files. This is diagnostic evidence,
not a claim that the literal recursive formatting command passed.
Recursive formatting-gate disposition remains required before readiness.

## Runtime and release state

The runtime-verification and pre-merge operational-closure skills ran:

* `docs/closure/2026-11-10-212-s-211-f-runtime-verification.md`
* `docs/closure/2026-11-10-212-s-211-f-closure.md`

Both conservatively report BLOCKED: help/provenance smoke passed, but
independent end-to-end/manual lifecycle and applicable platform evidence,
monitoring ownership and rollback readiness are incomplete. No daemon
replacement, disposal, real-store mutation or rollout was performed.
The profile makes runtime validation optional; no mandatory manifest
requirement is invented. Conditions are recorded, not silently waived.

Next: current-HEAD standard report-only review, then the requested
three-reviewer report-only adversarial review of `origin/main...HEAD`.
PR creation remains conditional on passing local readiness and resolving
the recorded gates. Hosted CI and required Copilot review remain pending.
No PR is currently asserted ready and no merge approval exists.
All 22 task members remain done; open RED is empty. Remain on the same
212-S feature branch. Preserve the protected historical checkpoint.
