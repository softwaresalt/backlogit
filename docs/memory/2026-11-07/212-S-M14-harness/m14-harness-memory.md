---
title: "M14 U1g and U1h harness-only memory"
date: "2026-11-07T22:23:04Z"
---

* Scope: harness-architect for 211.013-T and 211.014-T only; no implementation, task starts, full suite, or checkpoint mutation
* U1g commit: 9bed9a0916007f5a29b8591a6bb07d08820cb20c
* U1g: whole compile passes for 50 packages; scoped and race selectors fail only the unchanged-task subtest on genuine assertions; two fail-closed cases pass
* U1h: three named assertion failures, zero skips/errors; native exit 1, Red observer exit 0; Green observer intentionally rejects missing PASS with exit 2
* Static checks: pinned Go 1.24.0, core vet, pinned lint 1.64.8, Go format, Python AST, and PowerShell parse pass
* Preserved: source 183, all old test assertions, 13 Stage hashes before labels, baseline runner 8058bd5e, official checkpoint 210317 and six historical failure counters
* Correction: new Python/PowerShell files initially had CRLF; normalized only new harness/helpers to LF and repeated U1h evidence; original environment helper preexisted and was preserved
* Files: new U1g test in existing core file, tracked U1h observer, ignored private Python harness, and JSON proofs/reproduction in this directory
* Canonical green-regression blocks are absent; both arrays are exactly empty; broader U1g regressions remain later producer/runtime evidence
* Compact-context: invoked, no eligible candidates within the current-wave-only scope; prior memory/plans/checkpoints untouched
* Next: labels-only attestation after both proof commits; Root retains claim, build, runtime, native Windows power verification, and full-suite authority
