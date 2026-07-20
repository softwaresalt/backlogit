---
type: session-memory
timestamp: 2026-07-19T00:00:00Z
agent: orchestrator/ship
skill: build-feature
shipment: 099-S
feature: 108-F
branch: feat/108-F-size-estimation
---

# Ship 099-S — build-feature complete (108-F size estimation)

## Status

build-feature loop COMPLETE. All 9 code tasks + docs implemented, all harness
tests green, full quality gates pass. F6 (post-create rollback) was scaffolded
here but subsequently DROPPED during PR #261 review (cycle-4) — no production
`CreateArtifact` post-create rollback remains. Next: review skill → pr-lifecycle
→ merge → closure.

## Tasks completed (all done, 108-F tree = 14 done + 1 archived)

- SE-1 (108.001) config size+provenance schema — c041aab
- SE-7a (108.009) config-load containment — 9654380
- SE-7b (108.010) lookup containment — 1da188f (F6 post-create rollback was
  scaffolded in this commit but later removed during PR #261 review; dropped/deferred)
- SE-3a (108.002) + SE-3b (108.006) persist provenance + estimate_history event — da76ae8
- SE-4 (108.003) computed-on-read composition rollups — 977d767
- SE-5 (108.007) + SE-6 (108.008) MCP masquerade reject + size_composition projection — 113c91a
- SE-8 (108.004) sizing-contract design doc — 6653a84
- F6 lint fix DeleteItem→DeleteItemCascade — fbda4ac (on the since-removed F6 branch)
- backlog status updates (all done) — af66691
- SE-2 (108.005) codec round-trip guard was already green from harness

## Key decisions

- Rule R: an explicit size_source requires an accompanying size_ruleset_version
  (derived from harness-fitting; ruleset VALUE is not enum-validated — presence only).
  Documented in docs/design-docs/2026-07-19-size-estimation-contract.md.
- Did NOT auto-stamp size_source from actor when Size set + Source nil (matches
  930-c3 test "absent source not rewritten to human"). Plain size set writes no source.
- SetArtifactSize kept as thin wrapper (existing artifact_size_test.go depends on it);
  plan's "retire compat wrapper" interpreted as "migrate production callers", not delete.
- MCP masquerade: explicit size_source=human over MCP transport is rejected outright
  (ValidationFailed), independent of Rule R.
- SE-6 read projection: handleGetItem marshals feature/shipment artifact to a map and
  attaches computed size_composition (never persisted).

## Gates (all pass)

- go test ./... — all ok
- go vet ./... — exit 0
- golangci-lint run — exit 0 (fixed DeleteItem deprecation)
- gofmt -l . — my 4 changed files clean; other flags are local CRLF false positives (CI Linux clean)

## Env notes

- Windows PowerShell; .\backlogit.exe (rebuild: go build -o backlogit.exe ./cmd/backlogit)
- Commit emoji via temp file .git\COMMIT_MSG_TMP.txt + git commit -F (surrogate-pair issue with [char]0x)
- Strip logs via Select-String -NotMatch 'level=INFO'
- Task done transition is queued→active→done (queued→done rejected by validate_status_transition);
  active→done runs the pre-task-completion gate (~30-60s each).

## Next steps

1. review skill (multi-persona) on the 10-commit diff
2. pr-lifecycle — PR + Copilot review + CI (P-014 gate, P-009 merge-commit)
3. operator-approved merge
4. runtime-verification + operational-closure
5. post-merge: ship_shipment 099-S (queue→archive), shipment-reconcile, compound-refresh, compact-context
