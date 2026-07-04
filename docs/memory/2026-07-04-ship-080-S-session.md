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
- [x] mark 080-F feature done + chore(backlog) commit (afab513)
- [x] PR #174 created; CI 4/4 green; Copilot reviewed 16/16 files, 0 comments, 0 threads; §1.9 gate PASS
- [x] runtime-verification (PASS WITH FOLLOW-UP) + operational-closure (READY WITH CONDITIONS)
- [x] merge-ready halt (operator approved)
- [x] MERGE #174 (admin, merge-commit) -> d0ebb4f; feature+closure branches cleaned up
- [x] shipment ship 080-S -> shipped (merge SHA stamped); reconcile pre/post PROCEED; P-007 clean
- [x] post-merge closure + compound-refresh (all-keep) + memory finalize
- [ ] closure PR held at merge-ready (awaiting separate operator P-014 approval)

## Post-merge closure (Step 6)

- Feature merge: PR #174 MERGED 2026-07-04T18:03:49Z, merge commit `d0ebb4f` (true 2-parent
  merge: af26c71 + e718a81; P-009 merge-commit preserved). Confirmed in origin/main
  (merge-base --is-ancestor EXIT 0). Admin bypass of REVIEW_REQUIRED ruleset (operator-authorized).
- Branch cleanup: remote feat/080-release-docs-hygiene deleted by gh --delete-branch; local
  feature branch deleted (git branch -d, was e718a81). Now on post-merge/080-S.
- shipment ship 080-S --sha d0ebb4f: status=shipped; archived_ids=[080.001-T,080.002-T,080.003-T,
  080-F,080-S]; returned_ids=[]. Commit add3aae `chore: archive 080-S backlog artifacts`.
- Reconcile: pre PROCEED (4 items pre-archived), post PROCEED (all 5 archived; P-007 no deletions).
- Knowledge graduation: no ARCHITECTURE/AGENTS/design-doc/product-spec changes (pure hygiene).
  compound-refresh: F013 SHA-pinning / docline-contract / npm-resolver / f015-shipment all KEEP;
  no supersession; no new capture (gofmt-CRLF gotcha kept in closure docs, not promoted).
- Source-artifact cleanup: 080-F has no structured source_stash_id/source_deliberation_id;
  source stashes 9140F65C + B55985DD already retired by Stage; deliberation is a docs/decisions
  design record (retained). Deferred stashes 34F11E5A / EED25928 / 21E17BFC left untouched.
- Post-merge artifacts (docline-clean): post-merge-closure.md, compound-refresh.md.
- Closure PR: opened on post-merge/080-S, Copilot review + §1.9 gate, HELD at merge-ready
  (needs separate operator P-014 approval per §1.10).

## PR / CI / review

- PR: #174 — https://github.com/softwaresalt/backlogit/pull/174 · base `main` · code-review HEAD `afab513`.
- PR commit set (vs origin/main af26c71): 40bd121 (Stage harvest, rides along) + 2258f68 + dfbd2a1 + 9b54b53 + afab513. 16 files.
- CI at afab513: 4/4 green (test 1.23, test 1.24, CLI Reference Drift, Docline frontmatter gate).
- Copilot review at afab513: COMMENTED, "reviewed 16 out of 16 changed files ... generated no comments"; 0 inline, 0 threads.
- §1.9 readiness gate at afab513: Check1 (no pending req) PASS; Check2 (freshness commit.oid==HEAD) PASS; Check3 (0 unresolved Copilot threads) PASS. reviewDecision=REVIEW_REQUIRED (branch-protection PR-Review ruleset; needs operator admin-bypass — expected, same as 078/079-S).
- P-009 verified: repo allows merge_commit only (squash+rebase disabled).
- Closure artifacts (docs/closure/, docline-clean):
  - 2026-07-04-080-S-release-docs-hygiene-runtime-verification.md (PASS WITH FOLLOW-UP)
  - 2026-07-04-080-S-release-docs-hygiene-closure.md (READY WITH CONDITIONS)
- Closure commit is docs-only -> re-request Copilot + re-run §1.9 on the closure HEAD before halt.
- Follow-up (deferred): observe guard on next real tagged release; P3 exec.CommandContext timeout in test wrapper; external stash 34F11E5A + EED25928 out of scope (Principle IV).

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
