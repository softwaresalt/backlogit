---
title: U2j scoped GREEN evidence
task: 211.016-T
shipment: 212-S
source_head: ad2de5f3cc40c923f8f2c6b1aa709b8212418092
historical_red_baseline_sha: 30908ecb8438c0b3e7fe8bd514fa3680e35d90e5
---

## Scope and custody

Only the six authorized coupled source/control files changed. No production Go
edits, backlog mutations, task completion, U2i dispatch, U3 execution, full-suite
test execution, push, PR or child agents. Telemetry remained disabled.
The caller's hook/task changes and unrelated caller memory remain unstaged.
Existing RED evidence and protected task015/start history remain unchanged.

Careful/freeze-scope: ProposedAction = bounded snapshot remediation;
ActionRisk = moderate; ActionResult = applied under parent authorization.
Containment is the explicit six-file diff and existing committed RED scaffold.
No source lock conflict was observed; no concurrent implementation writer was used.
Parent quality/review and task016 completion remain pending.

## Native gates

All Go commands used `bin/tmp/212-go-env.ps1`: cached Go 1.24.0,
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, readonly modules.
Each execution used fresh owned ignored TEMP/TMP. Final roots used `bin/_j*`
to exclude in-flight temporary directories from Go package discovery.

| Gate | Native exit | Evidence |
|---|---|---|
| `go test -run=^$ -count=1 ./...` | 0 | Baseline and final whole compile; no tests executed |
| `go test -json -v -timeout=30m -count=1 -race -run '^TestU2j_' ./tests/integration` | 0 | 31 named RUN, 31 PASS, zero SKIP/FAIL |
| Exact AC companion command below | 0 | Integration package passed in 148.792s |
| `go vet ./...` | 0 | Final sequential rerun |
| `bin/golangci-lint.exe run` | 0 | Pinned v1.64.8, zero findings |
| `gofmt -l` on all tracked/new Go files | 0 | 1,390 files, batches of 40, zero findings |
| PowerShell AST parser | 0 | Zero parse errors |
| `pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1` | 0 | 182/182 assertions; all 26 original scenarios |
| Fixture JSON, prompt frontmatter, UTF-8/canonical LF and scoped diff check | 0 | Valid; original 26 scenarios and every original field of 18 controls unchanged |

```text
go test -timeout=30m -count=1 -race -run '^(TestUCS3_WaveSimClaimStart|TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract)$' ./tests/integration
```

Two additive tracked controls pin caller-zero-leaf-write acceptance and missing
snapshot refusal. Real-Git tests execute independent captures and comparison:
caller bytes, staging, index mode, tracked/untracked deletions, clean tracked
mutation, new tests/config/docs, production pre-dispatch/leaf deltas, raw NUL
bytes, ignored scratch, HEAD mutation and an actual junction/reparse escape.
Paused replay preserves the started member, waits for explicit prerequisite,
resumes once, registers RED before convergence and rejects six invalid amendment
bindings. No original expectation was weakened.

## Counters and failed observations

Five scoped test invocations total; no counter reset:

1. Full U2j race/verbose: native 1, sole failure was paused replay
   `compile_order_violations`, all real-Git tests passed
2. Anchored paused subcase diagnostic: native 1, expected 0 versus actual 4
   compile-order violations; completed prerequisites lacked replay timestamps
3. Full strengthened U2j race/verbose: native 0, 148.51s
4. Exact AC companion command: native 0
5. Final U2j race/JSON/verbose: native 0, 143.74s, 31 RUN/PASS, zero SKIP

Two quality execution issues were retained, not hidden:

* Parallel vet initially returned native 1 because Go package discovery saw an
  in-flight `bin/tmp/j3-.../go-link-...` directory disappear. Sequential vet
  returned native 0; subsequent fresh roots used underscore-prefixed directories.
* One unbatched format launch failed before running gofmt due to Windows native
  invocation limits (`StandardOutputEncoding ... redirected`). Batches of 40
  completed with native 0 and zero findings across all 1,390 source files.

Historical shipment full-suite failures are untouched; this dispatch ran no
unfiltered/full-package test suite. Tolerated sibling selector remains exactly
`^TestU2i_`; carried open-red selectors and green-regression commands remain empty.

## Source fingerprints

SHA-256 below identifies canonical UTF-8 LF bytes committed by Git.
Repository `core.autocrlf=true` initially supplied CRLF for existing PS/prompts.
After the named test runs, the eight owned files received newline-only UTF-8 LF
formatting; canonical source fingerprints did not change. Parser and simulation
were rechecked after formatting. Caller files were excluded from formatting.

| Source | LF SHA-256 |
|---|---|
| `tests/integration/claim_start_wave_sim_contract_test.go` | `ccad8c26cee7bf8a6d8a76069aa9f21f9012193c87e098d5abf737355eb08aaa` |
| `scripts/wave-scheduler-sim.ps1` | `0c0e1931255f87a76c990334b0464be181efd6c5b9e4362c4a51cb95b7ea87fd` |
| `tests/simulation/wave-scheduler-contract.json` | `c4d9e9cc963e9451261784b3d5f06b9780f2562632e9d0cf1b603e13a1f06da5` |
| `.github/skills/build-feature/SKILL.md` | `21ab387c050bc15b103b95846c19985996e382f7e709719c0b6f23b3fffb6e0a` |
| `.github/agents/_ship.agent.md` | `494badfbc2ecd7995475b0ef90c7d6d35f500d5c1379718956d322aead7efe84` |
| `.github/policies/workflow-policies.md` | `2296c3573bdfb93afc42f93eb7e445fe990c3e7bdbcda79176f22f5ddb8faae8` |

Raw worktree SHA-256 at test time for the four initially CRLF files:

```text
scripts/wave-scheduler-sim.ps1 d206d7ba74b3978101a0e065f99a861a8f8a4f908e97fa2485b0e8ecfa309c26
.github/skills/build-feature/SKILL.md 1906387e153aba098ab23798523eb2209b561a60d2e13dbf1275adefab1cc474
.github/agents/_ship.agent.md 922b2f1652840f1c8a6599ff87f4c2185d455301f6b22b8851e554b8188ae717
.github/policies/workflow-policies.md 49e5ba10ad25f63204b0ed341d29b394393499b14c829327cb871b13ba6909c4
```

See `native-proof.txt` for selected native output and the final JSON-stream digest.
