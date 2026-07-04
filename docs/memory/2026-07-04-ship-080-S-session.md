---
doc_type: memory
schema_version: "1.0"
title: "Ship session — shipment 080-S (release pipeline & docs hygiene)"
date: 2026-07-04
agent: ship
shipment: 080-S
feature: 080-F
branch: feat/080-release-docs-hygiene
---

# Ship session — 080-S

## Scope

Shipment `080-S` "Release pipeline and documentation hygiene" (feature `080-F`),
3 mutually-independent tasks, plan `docs/exec-plans/2026-07-04-release-docs-hygiene-plan.md`
(plan-review PASS, P3-only). Suggested order A -> B -> C.

- **080.001-T** (Unit A, ci/config): guard npm-publish job on `NPM_TOKEN` presence in
  `.github/workflows/release.yml` via env-indirection preflight step -> boolean output ->
  `if:` gate on the two publish steps. Retain `continue-on-error: true`. Never echo the token.
- **080.002-T** (Unit B, tests/shell): characterization-first shell test pinning
  `scripts/package-npm.sh` output (valid package.json x6, version stamped, optionalDependencies
  synced). jq-empty/JSON.parse is the required assertion; npm pack is OPTIONAL. 2-file stop rule.
- **080.003-T** (Unit C, docs): correct misleading `make docs-lint --path` wording in
  `docs/exec-plans/2026-07-02-stage-harvest-docline-frontmatter-hardening-plan.md` (~L102-107, ~L123-125)
  and `.backlogit/archive/076.002-T.md` (L22). Distinguish repo-wide `make docs-lint` (no args)
  from scoped `go run ./cmd/backlogit docs lint --path <file>`.

## Environment / tooling

- Tool gate: registry present; MCP not shell-invocable -> CLI fallback (repo-root
  `.\backlogit.exe` v1.2.0, has shipment/checkpoint cmds). DEGRADED_MODE (CLI fallback) = intended.
- Index sync OK. Intake reconcile PROCEED (all 4 items present + queued).
- Branch created from clean main (stash/pop preserved pre-existing unrelated operator WIP:
  .gitignore, start.ps1, agent .md files, hooks_queue.jsonl). Stage only 080-S files explicitly.
- actionlint installed via `go install` (GOPATH/bin) for Unit A lint.
- jq missing locally -> downloaded portable jq 1.7.1 to %TEMP%/ship-tools for Unit B local run.
  (package-npm.sh uses jq internally.)
- capability packs: continuous-learning NOT installed (no observe/learn/evolve skills);
  agent-intercom/engram/graphtor transports not reachable from shell -> broadcasts skipped
  (degraded, non-blocking); status reported directly to operator.

## Constraints

- P-014 / Principle VII: DO NOT merge. Halt at merge-ready; operator authorizes admin bypass.
- P-009 / Principle XI: merge-commit strategy only.
- Principle IV: stay in-tree; external npm/secret provisioning (stash 34F11E5A) and .tmpl edits
  (EED25928) are OUT of scope.
- Circuit breakers: build/fix-ci 5, review-fix 3, same-error 3.

## Progress log

- [x] Step 0.0 tool gate / 0.1 index sync / 0.5 shipment claim (080-S -> active)
- [x] Step 1 pre-flight (P-001 clear, compiles)
- [x] Unit A (080.001-T) -> commit 2258f68 (ci: guard npm-publish on NPM_TOKEN presence); done+archived
- [x] Unit B (080.002-T) -> commit dfbd2a1 (test: characterize package-npm.sh); done+archived
- [x] Unit C (080.003-T) -> commit 9b54b53 (docs: distinguish make docs-lint from scoped docs lint --path); done+archived
- [x] quality gate suite (see below)
- [x] review gate: PASS (no P0/P1/P2; 2 P3 advisories accepted)
- [x] mark 080-F feature done + chore(backlog) commit
- [ ] PR + CI + Copilot
- [ ] runtime-verification + operational-closure
- [ ] merge-ready halt

## Quality gates (all pass)

- `go test ./...` -> EXIT 0 (tests/integration package incl. new characterization wrapper passes; wrapper skips on Windows, runs on Linux CI).
- `go vet ./...` -> EXIT 0.
- `golangci-lint run` (full repo) -> EXIT 0.
- `gofmt`: new file `tests/integration/package_npm_characterization_test.go` is LF-clean (NOT in gofmt -l list). Repo-wide `gofmt -l .` flags all pre-existing .go files = Windows CRLF working-tree checkout artifact (autocrlf); committed blobs are LF; CI (Linux/LF) passes. Not a real finding.
- Unit A workflow lint: `actionlint` EXIT 0 + YAML parse OK on `.github/workflows/release.yml`. Diff purely additive.
- Unit C: `backlogit docs lint` scoped + repo-wide -> 0 violations; `backlogit sync` run after `.backlogit/archive/076.002-T.md` edit.

## Review gate (report-only, 4 personas)

- Constitution Reviewer: no P0/P1/P2. 1 P3 advisory -> verify `backlogit sync` after archive edit (DONE; sync does not bump updated_at for archived items).
- Go Reviewer: no P0/P1/P2. 1 P3 advisory -> exec.Command w/o context timeout (hang risk near-zero: jq --arg / jq empty <file>, no stdin, set -euo pipefail; CI global timeout). Deferred to respect P3-only scope + Unit B 2-file stop rule; will address if Copilot flags it.
- Security Reviewer: no findings. Secret handled via env-indirection (boolean-only $GITHUB_OUTPUT); SHA pins, permissions: contents:read, persist-credentials:false all preserved; continue-on-error unchanged; no injection surface. release.yml triggers on tags only (never PR).
- Scope Boundary Auditor: no scope findings. Exactly 5 files, on-scope; Unit B 2-file stop rule respected; scripts/package-npm.sh unmodified; no out-of-scope npm/secret/.tmpl edits.

## 079-S closure pattern (followed)

- Feature branch commit marks FEATURE + all tasks done (auto-archived queue->archive); shipment .md stays in queue.
- Post-merge `shipment ship 080-S --sha <merge-sha>` archives the shipment .md and stamps merge SHA onto every archived artifact (`| 7 +++-` churn), plus reconcile pre/post artifacts.

## Decisions

- Harness-first applies only to 080.002-T (characterization, green-from-start). A/C are
  apply-and-verify-via-gate per operator mandate. Only new test artifact = shell characterization
  test (+ thin Go wrapper so CI `go test` exercises it).
