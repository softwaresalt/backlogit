---
chunk_strategy: h1-h2-h3
description: "Runtime verification and completed post-merge observation for shipment 140-S and PR #436."
doc_type: closure
docline:
    date: 2026-09-12T02:44:05Z
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

**PASS**

The compiled CLI artifacts built successfully from exact PR HEAD
`5a337b9f8e8f2a80a9c80f27f8e08443d60e73e4`. Both the `go run` target and
the saved `faultline-analyze.exe` exposed FL001-FL005 and analyzed the
shipment-owned analyzer and compatibility-corpus packages with exit code 0 and
no diagnostics. Representative compatibility-corpus tests passed under the Go
race detector. All six hosted CI checks reported success for the same HEAD.

The post-merge release-observability follow-up is complete. Final PR HEAD
`6b7029eadf6418c70045aae552335670be89c7c6` passed six hosted checks, and
merge commit `c5978bc26343a1ca6e3894bcab7a0fd74f7b8215` has that commit as
its second parent with the same Git tree
`4cf3c79d19af2f679774bc1eff9e37c25e61bf5a`. Local `main` and
`origin/main` were synchronized at the merge SHA before the closure branch was
created. More than 30 minutes elapsed after merge with no failure signal.

No merge-commit CI run is claimed. The CI workflow triggers only on pull
requests to `main`, while the release workflow triggers only on version tags.
The terminal verdict uses final PR-head CI, merge-content identity, synchronized
`main`, the compiled smoke evidence below, and the healthy observation window.

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

The merge reached `main` at `2026-09-12T02:12:48Z`. The required minimum
30-minute window elapsed at `2026-09-12T02:42:48Z`, and the
operator-provided threshold of `2026-09-12T02:43:19Z` also elapsed. The
terminal observation at `2026-09-12T02:44:05Z` found no failure or rollback
signal.

Repository CI is intentionally PR-only. The six successful checks on final PR
HEAD `6b7029ea` are retained as hosted evidence, and merge parent/tree identity
confirms that the checked content reached `main`.

### Rollback trigger and procedure

Rollback trigger: a reproducible post-merge rerun of either command below
produces a non-zero exit, unexpected analyzer diagnostic, race, panic, or hang,
or retained evidence shows that the merged tree differs from the successfully
checked final PR tree:

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

## Post-Merge Local Smoke

The closure branch contains no source-code change after merge. Current compiled
smoke was repeated from the closure branch after the observation window:

```text
go build -o .copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\post-merge-backlogit.exe ./cmd/backlogit
go build -o .copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\post-merge-faultline-analyze.exe ./cmd/faultline-analyze
.\.copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\post-merge-backlogit.exe version --format json
.\.copilot\session-state\3c64d86d-f670-44b2-be32-15c6bdd2d747\files\post-merge-faultline-analyze.exe .\internal\faultline\analyzer\scannerdiscipline .\internal\faultline\analyzer\errwrap .\internal\faultline\analyzer\failopen .\internal\faultline\analyzer\auditsuccess .\internal\faultline\analyzer\locktimeout .\internal\faultline\compatcorpus
```

Both builds and both executions exited 0. The analyzer emitted no diagnostics.

## Post-Merge Follow-Up Disposition

| Evidence | Outcome |
|---|---|
| PR merge | `c5978bc2` merged at `2026-09-12T02:12:48Z` |
| Final PR-head checks | Six successful checks for `6b7029ea` |
| Merge content | Final PR HEAD is second parent; merge and PR HEAD trees match |
| Main synchronization | Local `main` and `origin/main` matched `c5978bc2` |
| Merge-commit CI | Not applicable; no push-to-main workflow trigger exists |
| Observation window | Healthy after more than 30 minutes |
| Failure signals | None observed |

The original follow-up is closed with outcome **healthy**. Releasability is
**READY**.

## Operational Closure Handoff

* Verification verdict: **PASS**
* Runtime surfaces verified: compiled CLI help and execution, analyzer
  registration, shipment-owned package analysis, compatibility-corpus tests,
  and hosted current-HEAD CI
* Evidence: exact commands, hashes, outputs, and CI check table in this report
* Blocked prerequisites: none
* Follow-up disposition: completed healthy; no closure condition remains
* Monitoring baseline: analyzer exit 0 with zero diagnostics; focused race
  tests pass; final PR-head hosted checks succeed; merge tree matches final PR
  tree
* Rollback readiness: corrective revert PR; no deploy or data rollback needed
* Recommended next action: retain the residual P-021 IDs for future Stage
  triage
