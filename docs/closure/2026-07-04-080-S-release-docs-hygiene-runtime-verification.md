---
chunk_strategy: h1-h2-h3
description: 'Pre-merge runtime verification for shipment 080-S — release pipeline and documentation hygiene. The only true runtime surface is the tag-triggered release workflow (.github/workflows/release.yml, Unit A): it fires only on push of a v*.*.* tag and cannot be exercised end-to-end in-tree without provisioning an external npm org + NPM_TOKEN (out of scope, Principle IV; human-only stash 34F11E5A). It is therefore verified statically — actionlint EXIT 0, YAML parse OK, and a logic walkthrough of both guard branches (token-absent -> has_token=false -> both publish steps skipped, no red X; token-present -> has_token=true -> publish runs) with SHA pins, contents:read, persist-credentials:false, and continue-on-error:true all preserved. Unit B (retired packaging characterization script + retired packaging characterization test) IS exercised: green locally (Git Bash + jq 1.7.1, isolated temp workspace, real package tree untouched) and green in CI on Linux (test 1.23 + 1.24). Unit C (docs wording) verified via backlogit docs lint (0 violations). Verdict PASS WITH FOLLOW-UP — the guard is statically proven and CI-green; end-to-end tag-triggered publish behavior is deferred to observation on the next real release.'
doc_type: closure
docline:
    ms.date: 2026-07-04T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-04T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-04-080-S-release-docs-hygiene-runtime-verification.md
title: 080-S release pipeline & docs hygiene — Pre-Merge Runtime Verification
---

# Runtime Verification — 080-S release pipeline & docs hygiene

- **Date**: 2026-07-04
- **Shipment**: `080-S` · Feature `080-F` · Tasks `080.001-T`, `080.002-T`, `080.003-T`
- **PR**: #174 — https://github.com/softwaresalt/backlogit/pull/174 · **not merged** (halted at merge-ready per P-014 / Constitution Principle VII)
- **Branch**: `feat/080-release-docs-hygiene` · code-review HEAD `afab513` (closure commit is docs-only, re-reviewed)
- **Surface**: `background-job` (tag-triggered CI/CD workflow, Unit A) + `cli`/test artifact (Unit B) · **Mode**: static workflow verification + isolated shell-test execution + whole-suite gates
- **Verdict**: **PASS WITH FOLLOW-UP**

## Affected runtime surfaces

- **Unit A — `.github/workflows/release.yml`** (tag-triggered `npm-publish` job). This is the
  only genuine runtime surface. The workflow triggers **only** on `push` of a `v*.*.*` tag —
  never on `pull_request` — so it cannot be exercised end-to-end from a feature branch, and
  triggering it would require an external npm org + a real `NPM_TOKEN` secret, which is
  **out of scope** for 080-S (Principle IV; human-only stash `34F11E5A`). Verified statically.
- **Unit B — `retired packaging characterization script` + `retired packaging characterization test`**.
  A characterization test that pins `retired packaging script` output. Fully exercisable and
  exercised (see below).
- **Unit C — docs wording** (`docs/exec-plans/2026-07-02-...-hardening-plan.md`,
  `.backlogit/archive/076.002-T.md`). Not a runtime surface; validated by `backlogit docs lint`.

## Verification approach

### Unit A — release workflow guard (static)

| Check | Command / method | Result |
|---|---|---|
| Workflow lint | `actionlint .github/workflows/release.yml` | exit 0 |
| YAML parse | Python `yaml.safe_load` | OK (valid document) |
| Diff shape | `git diff` inspection | purely additive (preflight step + two `if:` gates); no deletions |

**Guard-logic walkthrough** (both branches reasoned through, since execution is tag-gated):

| Condition | `preflight` step | `has_token` output | Publish steps (`if: ... == 'true'`) | Observed run behavior |
|---|---|---|---|---|
| `NPM_TOKEN` **absent** | env-indirection maps empty secret → `NPM_TOKEN` env; `[ -n "$NPM_TOKEN" ]` false | `has_token=false` + logs `NPM_TOKEN absent — skipping` | condition false | both publish steps **skipped** — no red X |
| `NPM_TOKEN` **present** | env var non-empty; `[ -n "$NPM_TOKEN" ]` true | `has_token=true` | condition true | both publish steps **run** normally |

Security invariants preserved (confirmed by inspection + Security Reviewer): the secret value
is never echoed (only the boolean `has_token` reaches `$GITHUB_OUTPUT`); all `uses:` remain
full-SHA pinned; top-level `permissions: contents: read` and every checkout's
`persist-credentials: false` are unchanged; the pre-existing job-level `continue-on-error: true`
is neither removed nor broadened.

### Unit B — characterization test (executed)

| Scenario | Command | Observed |
|---|---|---|
| Isolated run (local) | `bash scripts/package-npm.characterization.sh` (PATH incl. jq 1.7.1) | **PASS** — 6 `package.json` valid + version-stamped; wrapper `optionalDependencies` synced |
| Isolation invariant | inspect real `npm/**/package.json` after run | unchanged (test copies into `mktemp -d`, `trap cleanup EXIT`) |
| Optional npm-pack path | `RUN_NPM_PACK=1 bash …` | passes (off by default; non-fatal) |
| CI (authoritative) | GitHub Actions `test (1.23)` + `test (1.24)` on Linux | **PASS** (Go wrapper runs the shell test; skips only on Windows) |

### Unit C — docs wording (validated)

| Check | Command | Result |
|---|---|---|
| Scoped lint | `backlogit docs lint --path <each edited plan/archive file>` | 0 violations |
| Repo-wide lint | `make docs-lint` / `backlogit docs lint` | 0 violations |
| Index consistency | `backlogit sync` after archive-file edit | Indexed 715 artifacts |

## Whole-suite gates (code-review HEAD `afab513`)

| Gate | Command | Result |
|---|---|---|
| Build/compile | `go test -run=^$ -count=1 ./...` | exit 0 |
| Tests | `go test ./...` | **PASS** (all packages ok, incl. contract + integration) |
| Vet | `go vet ./...` | exit 0 |
| Lint | `golangci-lint run` | exit 0 (0 findings) |
| Format | `gofmt -l .` | see note |

**gofmt note**: the local Windows working tree is checked out with CRLF line endings (no
`.gitattributes`, `core.autocrlf` on, blobs stored LF), so `gofmt -l` flags pre-existing `.go`
files as a line-ending artifact, not a content issue. The new
`retired packaging characterization test` is LF-clean (absent from `gofmt -l`).
The authoritative format/vet gates run on LF in CI and are green.

## CI evidence (PR #174, code-review HEAD `afab513`)

- CI: **4/4 green** — `test (1.23)`, `test (1.24)`, `CLI Reference Drift`, `Docline frontmatter gate`.
- Copilot review at `afab513`: **COMMENTED**, "reviewed 16 out of 16 changed files … generated no comments" — 0 inline comments, 0 review threads.
- No fix-ci cycle required this shipment.

## Load-bearing invariants confirmed

- **Guard is non-suppressing**: the `if:` only *skips* publish when the token is absent; when
  present, publish runs exactly as before. Legitimate failures are not masked by the new guard.
- **Secret never logged**: env-indirection emits only a boolean presence flag.
- **Supply-chain pinning intact**: no third-party action pin was downgraded.
- **Characterization stability**: the shell test passes against an isolated copy and never
  mutates tracked `retired package metadata`; CI exercises it on Linux.
- **Docs honesty**: `make docs-lint` (no args, repo-wide) is now clearly distinguished from the
  scoped `go run ./cmd/backlogit docs lint --path <file>`; docline gate stays green.

## Handoff to operational-closure

- Verification verdict: **PASS WITH FOLLOW-UP**
- Surfaces verified: release-workflow guard (static: actionlint + YAML + both-branch logic);
  characterization test (executed local + CI-green); docs wording (docs-lint clean).
- BLOCKED prerequisites: none for the in-scope verification. Full end-to-end execution of
  `release.yml` requires a real `v*.*.*` tag push with a provisioned `NPM_TOKEN` — intentionally
  **out of scope** (Principle IV; human-only stash `34F11E5A`).
- Risky action state: none — additive workflow guard + new test artifact + docs prose. No
  destructive, migration, or persistence-schema change.
- Follow-up recommendations:
  - **Observe the next real tagged release** to confirm the guard behaves as designed:
    `has_token=false` cleanly skips publish (no red X) when `NPM_TOKEN` is absent, and publish
    runs when it is present.
  - External npm-org / `NPM_TOKEN` provisioning (stash `34F11E5A`) remains human-only, out of
    scope for 080-S.
  - External `.tmpl` edits (stash `EED25928`) remain out of scope (out-of-tree; Principle IV).
