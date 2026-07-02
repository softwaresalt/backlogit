# Ship checkpoint — 072-S doctor --target nil-HeaderDef hardening

- **Date**: 2026-07-01
- **Agent**: Ship
- **Shipment**: `072-S` — "doctor --target: fail closed on nil header-def (072-F)"
- **Feature**: `072-F` · **Task**: `072.001-T` (single)
- **Branch**: `feat/072-doctor-target-nil-headerdef` (off local `main` @ `14776b6` Stage commit)
- **Plan**: `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md`

## Items completed

- `072.001-T` — **done**, commit `cedfe94`.
  - `internal/core/doctor_target.go`: inverted the `if ws.HeaderDef != nil` guard in
    `ValidateDoctorTargetResolved` → explicit `ws.HeaderDef == nil` early return with
    `kind=DoctorTargetIO` (exit 3), `OK=false`, message
    "header definition not loaded; cannot perform required-field validation".
    Broadened the `DoctorTargetIO` constant doc comment.
  - `internal/core/doctor_target_test.go`: added `TestDoctorTarget_NilHeaderDefFailsClosed`
    (TDD red→green) with a loaded-vs-nil precedence pair (2 scenarios).
- Shipment `072-S`: claimed → **active**.

## Decisions (with rationale)

- **io (exit 3), not validation (exit 1)**: a nil workspace `HeaderDef` is a system/config
  precondition fault, not a user-correctable artifact defect. Reuses the shipped 071-S
  exit-code contract (0/1/2/3/4) with **no new kind, no exit-code table change, no schema-version
  bump**. Matches the plan Decision (Option B) and Ship's own 071-S thread ack
  (`PRRT_kwDORzozKM6Ngr6a`).
- CLI + MCP consistency is structural — both route through the single shared
  `ValidateDoctorTargetResolved`; the one-function fix covers both surfaces.

## Quality gates (full mandated set, in order)

- `go test ./...` → **PASS** (all packages incl. cli/mcp/contract/integration)
- `go vet ./...` → **PASS** (exit 0)
- `golangci-lint run` → **PASS** (exit 0)
- `gofmt -l .` → **PASS for change** (my 2 files LF-normalized clean; whole-tree CRLF flags are a
  pre-existing Windows working-tree artifact affecting all 361 tracked files; git stores LF via
  autocrlf, so CI gofmt on LF is clean)

## Review gate

- code-review (report-only): **No P0/P1**. One P3 (informational, out-of-scope): the create/update
  write paths in `internal/core/artifacts.go:224,514` share the same fail-open *shape*
  (`ValidateArtifactFields` behind `if ws.HeaderDef != nil`). Captured as a follow-up stash for Stage.

## Branch / PR state

- **PR #158**: https://github.com/softwaresalt/backlogit/pull/158 (`feat/072-doctor-target-nil-headerdef` → `main`), **HEAD `a76b717`**.
- Commits: `14776b6` (Stage, rides along — origin/main lacked it), `cedfe94` (fix), `30e0a35` (backlog state),
  `e3939f2` (closure docs), `64865e4` (fix-ci: docline frontmatter on closure docs), `a76b717` (review-fix:
  drop dated exec-plan path from doctor_target comment per Copilot).
- **Two post-`30e0a35` remediation cycles**:
  1. **fix-ci cycle 1/5** — `e3939f2` closure-docs push failed the Docline frontmatter gate;
     added required YAML frontmatter to both `docs/closure/*.md`; validated locally
     (`go run ./cmd/backlogit docs lint` → valid, 0 violations); pushed `64865e4`; CI green.
  2. **review-fix cycle 1/3** — Copilot re-review of `64865e4` raised 1 thread
     (`PRRT_kwDORzozKM6NzqKt`, doctor_target.go: dated exec-plan path in comment). Fixed
     (`a76b717`) → replied on thread → **resolved programmatically** via `gh api graphql`.
     Copilot re-review of `a76b717` (07:14:16Z) raised **0 new threads**.
- Copilot auto-re-review did NOT fire on the `a76b717` push; **re-requested** via REST
  `POST /pulls/158/requested_reviewers` with `copilot-pull-request-reviewer[bot]` (the working
  re-request path in CLI mode — `gh pr edit --add-reviewer` fails `'Copilot' not found`).
- **CI**: all 4 checks green on `a76b717` (`test 1.23`, `test 1.24`, Docline gate, CLI Reference Drift).
- **Copilot review**: `COMMENTED` on HEAD `a76b717`. **§1.9 gate PASS** — Check 1 (no pending request),
  Check 2 (latest Copilot review commit == headRefOid, fully fresh), Check 3 (0 unresolved Copilot
  threads, fully paginated `hasNextPage=false`). Fail-closed satisfied.
- **P-009**: satisfied (`allow_merge_commit=true`, squash/rebase=false).
- `reviewDecision=REVIEW_REQUIRED` — branch protection needs a formal approving review (operator/P-014 concern).
- Runtime verification: **PASS** (pass/io/scope exit codes intact via source build; new branch covered by unit test).
- Closure: `docs/closure/2026-07-01-072-S-doctor-nil-headerdef-{runtime-verification,closure}.md` — **READY**.
- Follow-up stashed for Stage: `266816CE` (artifacts.go write-path fail-open shape).
- **HALTED at P-014**: PR is merge-ready, awaiting explicit operator merge approval. Do NOT self-merge.
  Post-merge closure (shipment-reconcile → ship_shipment → knowledge graduation) runs in a later
  Orchestrator-routed session after operator approval.

## Circuit-breaker counters

- build-feature attempts: 1/5 · review-fix cycles: 1/3 · fix-ci cycles: 1/5 · consecutive failures: 0/3

## Post-merge closure (2026-07-02, operator P-014 approved)

- **Merged**: PR #158 via operator-authorized `--admin` **merge commit** (standard merge
  blocked by `PR-Review` ruleset needing a formal approving review that does not exist —
  same as #156/#157). P-009 preserved (`allowed_merge_methods: ["merge"]`; squash/rebase off).
  **Merge SHA `d3f0facf530592c526e261b3818dc6e0d0dd0ced`**, merged 2026-07-02T13:53:57Z.
- **Merge Confirmation Gate**: `state: MERGED`; `git merge-base --is-ancestor d3f0fac origin/main`
  → exit 0 (merge SHA is `origin/main` HEAD). §1.9 re-checked green at merge (HEAD `a76b717`, 0
  pending requests, latest review covers HEAD, 0 unresolved threads, fully paginated).
- **Closure branch**: `post-merge/072-doctor-nil-headerdef` (off `main` @ `d3f0fac`).
- **Shipment**: `072-F` finalized active→done; `backlogit shipment ship 072-S --sha d3f0fac`
  → **shipped**; archived `072.001-T`, `072-F`, `072-S` with merge SHA recorded. Reconcile
  pre+post → **PROCEED**; **P-007** archive integrity intact (no spurious deletions).
  Backlog archival committed `fcd9e5c`.
- **Knowledge graduation**: reinforced (UPDATE, not duplicate)
  `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md` — 072-S is a
  textbook second instance of the nil-zero-value-at-a-safety-boundary rule (validation
  precondition vs cache); recurrence noted at `artifacts.go:224,:514`.
- **Source artifact**: source stash `C16DBBEB` already archived/retired by Stage (forward-link
  intact); automated Step 6.7 retirement a no-op (072-F has no structured `source_stash_id`) —
  Stage-domain, flagged only.
- **Follow-up carried forward for Stage**: `266816CE` (active) — `artifacts.go` write-path
  fail-open shape. No new post-merge follow-ups.
- **Post-merge closure artifact**: `docs/closure/2026-07-02-072-S-doctor-nil-headerdef-post-merge-closure.md`.
- **Remaining**: closure PR opened for `post-merge/072-...`, awaiting separate operator P-014
  approval. No self-merge of the closure PR.

