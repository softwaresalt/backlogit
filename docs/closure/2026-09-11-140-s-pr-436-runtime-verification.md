---
chunk_strategy: h1-h2-h3
description: "Manual compiled-CLI runtime verification for shipment 140-S and PR #436 at HEAD 5a337b9f."
doc_type: closure
docline:
    date: 2026-09-12T01:00:11Z
    status: accepted
    tags:
        - runtime-verification
        - cli
        - faultline
        - 140-S
        - pr-436
schema_version: "1.0"
source: docs/closure/2026-09-11-140-s-pr-436-runtime-verification.md
title: "Shipment 140-S / PR #436 Runtime Verification"
---

## Verdict

**PASS WITH FOLLOW-UP**

The compiled CLI artifacts built successfully from exact PR HEAD
`5a337b9f8e8f2a80a9c80f27f8e08443d60e73e4`. Both the `go run` target and
the saved `faultline-analyze.exe` exposed FL001-FL005 and analyzed the
shipment-owned analyzer and compatibility-corpus packages with exit code 0 and
no diagnostics. Representative compatibility-corpus tests passed under the Go
race detector. All six hosted CI checks reported success for the same HEAD.

The only follow-up is the release-observability post-merge observation window.
There is no deployment, browser, API, background-job, migration, or production
runtime surface to validate for this shipment.

## Inputs and Verification Depth

* Surface: `cli`
* Mode: `manual`
* Target: `go run ./cmd/faultline-analyze`
* Context: shipment `140-S`, PR `#436`, current HEAD `5a337b9f`
* Verification depth: local compiled-CLI execution, package analysis, focused
  package tests, and current-HEAD hosted CI confirmation

`.autoharness/workspace-profile.yaml` declares:

```yaml
runtime_validation:
  validator_manifest:
    surfaces: []
  validation_expectations:
    required: false
    surfaces_expected: []
    minimum_verdict: "PASS"
```

The empty automatic validator manifest does not waive the manual compiled-CLI
verification required by PR #436's local review readiness record.

## Environment Prechecks

| Precheck | Observation |
|---|---|
| Local branch | `feat/140-s-s6-compatibility-corpus-fuzzing-and-static-analysis` |
| Local HEAD | `5a337b9f8e8f2a80a9c80f27f8e08443d60e73e4` |
| PR #436 head | `5a337b9f8e8f2a80a9c80f27f8e08443d60e73e4` |
| Base branch | `main` |
| Working tree before verification | Clean |
| Toolchain | `go version go1.26.5 windows/amd64` |
| External services or credentials | Not required |
| Browser or deployment target | Not applicable |

Engram indexed discovery was unavailable because its local daemon IPC endpoint
was not running. This did not block verification because the branch, PR, target
commands, and package paths were explicit and were confirmed directly.

## Built Artifact Evidence

Artifacts were written under the ignored session-state directory so the build
did not dirty the repository.

| Artifact | Size | SHA-256 |
|---|---:|---|
| `.copilot/session-state/3c64d86d-f670-44b2-be32-15c6bdd2d747/files/backlogit.exe` | 25,567,744 bytes | `4CB5FC0F8A282EAAEE3547EA77E6B5787DD3123FDF9A85098C2A6166826E5F1F` |
| `.copilot/session-state/3c64d86d-f670-44b2-be32-15c6bdd2d747/files/faultline-analyze.exe` | 10,391,552 bytes | `3DFFF7D67288E7A4B185EC6360524D7735227300C3EAD5452549847F83B687F7` |

Commands:

```text
go build -o .copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\backlogit.exe ./cmd/backlogit
go build -o .copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\faultline-analyze.exe ./cmd/faultline-analyze
```

Observed: both commands exited 0 and both expected PE executables existed with
the sizes and hashes above.

The compiled backlogit artifact was also started:

```text
.\.copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\backlogit.exe version --format json
```

Observed: exit 0; version
`1.10.1-0.20260912004159-5a337b9f8e8f`; Go version `go1.26.5`.

## CLI Scenarios

### Help and registered analyzer inspection

Commands:

```text
go run ./cmd/faultline-analyze -help
.\.copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\faultline-analyze.exe -help
```

Observed: both commands exited 0 and listed the five expected analyzers:

```text
-FL001scannerdiscipline
-FL002errwrap
-FL003failopen
-FL004auditsuccess
-FL005locktimeout
```

No analyzer was missing or duplicated in the help surface.

The source-enumeration contract was checked independently:

```text
go test ./cmd/faultline-analyze -run TestMulticheckerEnumeratesAll -count=1 -v
```

Observed:

```text
--- PASS: TestMulticheckerEnumeratesAll (0.01s)
PASS
ok github.com/softwaresalt/backlogit/cmd/faultline-analyze
```

### Shipment-owned package analysis

The target command was run against the five analyzer implementation packages
and the compatibility-corpus package:

```text
go run ./cmd/faultline-analyze ./internal/faultline/analyzer/scannerdiscipline ./internal/faultline/analyzer/errwrap ./internal/faultline/analyzer/failopen ./internal/faultline/analyzer/auditsuccess ./internal/faultline/analyzer/locktimeout ./internal/faultline/compatcorpus
```

Observed: exit 0 and no stdout or stderr diagnostics.

The same scenario was run through the saved compiled executable:

```text
.\.copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\faultline-analyze.exe .\internal\faultline\analyzer\scannerdiscipline .\internal\faultline\analyzer\errwrap .\internal\faultline\analyzer\failopen .\internal\faultline\analyzer\auditsuccess .\internal\faultline\analyzer\locktimeout .\internal\faultline\compatcorpus
```

Observed: exit 0 and no stdout or stderr diagnostics.

This is a clean result. The deliberately bad `testdata` fixtures were not
included because they exist to produce expected analyzer findings in
`analysistest`, not to represent clean shipment-owned implementation packages.

### Representative compatibility-corpus tests

Command:

```text
go test -race ./internal/faultline/compatcorpus -run '^(TestCorpusRunner|TestConcurrencyFixtures|TestFuzzSeedCorpusCommitted)$' -count=1 -v
```

Observed: exit 0. All selected tests and subtests passed, including:

* lock contention and busy-sentinel behavior
* context cancellation without hangs
* ambiguous gate input failing closed
* committed native fuzz-seed presence
* strict adapter outcomes for every representative corpus case
* deterministic sorted reports
* cancellation classification
* adapter panic containment

Package result:

```text
PASS
ok github.com/softwaresalt/backlogit/internal/faultline/compatcorpus 7.070s
```

## Hosted CI Evidence

PR #436 reported these completed successful checks for
`headRefOid=5a337b9f8e8f2a80a9c80f27f8e08443d60e73e4`:

| Check | Conclusion |
|---|---|
| Detect code changes | `SUCCESS` |
| Markdown lint (P-008) | `SUCCESS` |
| pipeline-topology (ambient) | `SUCCESS` |
| test | `SUCCESS` |
| Docline frontmatter gate | `SUCCESS` |
| CLI Reference Drift | `SUCCESS` |

Hosted workflow run: [CI run 34662505116](https://github.com/softwaresalt/backlogit/actions/runs/34662505116).

## Release Observability

### Monitoring plan

The decided plan classifies this shipment as internal analyzer and test/CI
tooling with no production behavior, deployment, migration, or
high-rollout-risk change:

* `docs/exec-plans/2026-09-11-s6-compatibility-corpus-decided-plan.md`
  consolidates the governing plan and harness supplement

| Signal | Baseline | Investigation threshold | Observation source | Owner |
|---|---|---|---|---|
| Analyzer CLI execution on owned packages | Exit 0, zero diagnostics | Any non-zero exit or diagnostic | Re-run the package-analysis command in this report | Operational closure owner |
| Compatibility-corpus representative tests | All selected tests pass under `-race` | Any failed test, race, panic, or hang | Focused `go test -race` command in this report | Operational closure owner |
| Hosted current-head checks | Six checks successful | Any failed, cancelled, timed-out, or missing required check | PR #436 / GitHub Actions run | PR owner |

### Pre-deploy audit

No deployable service, feature flag, data migration, external integration, or
dependent service is affected. A production pre-deploy audit is therefore not
applicable. Build, test, analyzer, and CI evidence are the relevant
releasability evidence.

### Observation window

Operational closure should observe the first hosted CI run on the merge commit
through completion, with a minimum 30-minute window after merge. Do not close
the window early on success. Halt early only when a failure or rollback signal
requires intervention. This future merge-commit observation is the reason for
the `PASS WITH FOLLOW-UP` verdict.

### Rollback trigger and procedure

Rollback trigger: the first merge-commit CI run fails because of shipment
140-S, or a reproducible rerun of either command below produces a non-zero exit,
unexpected analyzer diagnostic, race, panic, or hang:

```text
go run ./cmd/faultline-analyze ./internal/faultline/analyzer/scannerdiscipline ./internal/faultline/analyzer/errwrap ./internal/faultline/analyzer/failopen ./internal/faultline/analyzer/auditsuccess ./internal/faultline/analyzer/locktimeout ./internal/faultline/compatcorpus
go test -race ./internal/faultline/compatcorpus -run '^(TestCorpusRunner|TestConcurrencyFixtures|TestFuzzSeedCorpusCommitted)$' -count=1
```

Rollback procedure: revert PR #436 with a normal merge-commit-preserving
corrective PR, then rerun the repository quality gates. No deployment rollback,
data restoration, or migration reversal exists because the shipment changes
only development-time analyzer and compatibility-corpus surfaces.

## Unsupported Evidence

No browser, deployed-service, public API, background-job, migration, or
production telemetry evidence is claimed. Those surfaces are absent from both
the workspace profile and this shipment.

No destructive or high-risk action was performed during verification. All
commands were local builds, read-only metadata checks, analyzer executions
without `-fix`, and tests.

## Operational Closure Handoff

* Verification verdict: **PASS WITH FOLLOW-UP**
* Runtime surfaces verified: compiled CLI help and execution, analyzer
  registration, shipment-owned package analysis, compatibility-corpus tests,
  and hosted current-HEAD CI
* Evidence: exact commands, hashes, outputs, and CI check table in this report
* Blocked prerequisites: none
* Follow-up: observe the first hosted CI run on the merge commit for a minimum
  of 30 minutes; halt early only on a failure or rollback signal, then record a
  healthy, degraded, or rolled-back outcome
* Monitoring baseline: analyzer exit 0 with zero diagnostics; focused race
  tests pass; expected hosted checks succeed
* Rollback readiness: corrective revert PR; no deploy or data rollback needed
* Recommended next action: feed this report into operational closure and retain
  the merge-commit observation as a closure condition
