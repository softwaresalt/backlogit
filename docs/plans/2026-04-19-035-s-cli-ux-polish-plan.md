---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-19T00:00:00Z
    origin: docs/closure/2026-04-19-034-s-cli-ux-output-formatting-closure.md
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-19-035-s-cli-ux-polish-plan.md
title: 'Shipment 035-S: CLI UX Review Follow-ups'
---

## Problem Frame

Two P3 review findings from the just-shipped 034-S CLI UX work were deferred to a polish shipment:

1. **WF-001 (P3)** — `.github/workflows/cli-reference-drift.yml` declares `permissions:` at the workflow level. The repo CI security instructions (`.github/instructions/ci-security.instructions.md` and `.github/instructions/workflows.instructions.md`) require job-level least-privilege permissions.
2. **AP-001 (P3)** — `format.TileRenderer.Bold` requires the caller to pass a TTY-detection result. The current `internal/cli/list.go:newRenderer` helper hard-codes `false`, so ANSI bold is never applied even when output is a terminal. A `// TODO` comment marks this in `list.go` line 42.

Both are localized, low-risk fixes with no API change, no migration, no rollout. Scope is bounded to two files (plus optionally one test file).

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Workflow-level `permissions:` block in `cli-reference-drift.yml` is removed; `permissions: contents: read` is declared on the `drift` job | Review finding WF-001, task 033.011-T |
| R2 | A reusable `isTerminal(w io.Writer) bool` helper exists and is wired into `newRenderer` so `format.NewTileRenderer` receives the correct bold setting | Review finding AP-001, task 033.012-T |
| R3 | Helper returns `false` when `w` is not a `*os.File`, so non-file writers (e.g., test buffers, piped output) stay plain | Recommended fix in 033.012-T |
| R4 | All existing tests for `list` and `tile` formats continue to pass; new behavior is covered by a small unit test | Constitution III (TDD) |

## Scope Boundaries

### In Scope

- Edit `.github/workflows/cli-reference-drift.yml` to move permissions to job level.
- Add `isTerminal(w io.Writer) bool` helper in `internal/cli/` (likely `list.go` or a new `tty.go` if shared with other commands later).
- Update `newRenderer` in `internal/cli/list.go` to call the helper and pass the result to `format.NewTileRenderer`.
- Remove the `TODO (AP-001)` comment from `newRenderer`.
- Add a small unit test for `isTerminal` covering the non-`*os.File` path (returns `false`).

### Non-Goals

- Wiring TTY detection into `get`, `queue view`, `stash`, or `shipment` commands. Those commands do not currently use `TileRenderer`; if/when they do, they call the same helper.
- Auto-detecting `NO_COLOR` or `TERM=dumb` environment variables. The helper only checks file descriptor terminal status; richer color policy is out of scope.
- Changing `TileRenderer`'s API. The renderer keeps its `Bold bool` field; only the call site changes.
- Adding `golang.org/x/term` if a simpler `*os.File` cast plus `term.IsTerminal` from `golang.org/x/sys/unix` (or equivalent) is preferred — implementer's choice based on what is already available. Note that `golang.org/x/sys` is already an indirect dep; `golang.org/x/term` would be a new direct dep but is the canonical, cross-platform approach.

### Deferred to Implementation

- **Helper location**: `internal/cli/list.go` (private to the file) versus a new `internal/cli/tty.go` (shared). Implementer picks; the latter is preferred if a second call site is anticipated within this shipment.
- **TTY detection mechanism**: `golang.org/x/term.IsTerminal(int(f.Fd()))` is recommended (cross-platform, well-known, single dep). Falling back to `os.Stdout` identity comparison is acceptable but weaker (does not detect redirection).

## Implementation Units

### Unit 1: Move workflow permissions to job level (033.011-T)

**Files:** `.github/workflows/cli-reference-drift.yml`
**Test files:** none — actionlint and the existing `Test-DependencyPinning.ps1` / workflow lint gates cover this
**Effort size:** small (≤15 minutes)
**Skill domain:** config
**Execution note:** edit-and-verify (no test-first; CI validation provides the gate)
**Patterns to follow:** other workflows in `.github/workflows/` that already declare permissions at the job level
**Dependencies:** none

**Approach:**

1. Delete the workflow-level `permissions:` block (lines 15–16).
2. Add `permissions: \n  contents: read` under the `drift:` job, before `runs-on:`.
3. Run actionlint locally if available (or rely on CI) to confirm the workflow still parses.

**Verification:**

- `cli-reference-drift.yml` no longer has a top-level `permissions:` key.
- The `drift` job declares `contents: read` and nothing more.
- The workflow continues to run successfully on the next push (CI green).

### Unit 2: Wire TTY detection into `newRenderer` (033.012-T)

**Files:** `internal/cli/list.go` (modify `newRenderer`, add helper or import from new file), optionally `internal/cli/tty.go` (new, if helper is shared)
**Test files:** `internal/cli/list_test.go` or `internal/cli/tty_test.go` (new test for the helper's non-file path)
**Effort size:** small (≤45 minutes including test)
**Skill domain:** code
**Execution note:** test-first for the helper; refactor the call site once helper is in place
**Patterns to follow:** existing renderer wiring in `newRenderer`; test patterns in `internal/cli/*_test.go`
**Dependencies:** none

**Approach:**

1. Add `isTerminal(w io.Writer) bool`:
   - Type-assert `w` to `*os.File`. If the assertion fails, return `false`.
   - Otherwise call `golang.org/x/term.IsTerminal(int(f.Fd()))` and return the result.
2. Update `newRenderer(f string, w io.Writer) format.Renderer` (note: signature gains a writer parameter so the helper can inspect the actual destination, not assume `os.Stdout`).
3. In the `list` `RunE`, pass `cmd.OutOrStdout()` as the second arg.
4. Pass `isTerminal(w)` into `format.NewTileRenderer(...)`.
5. Remove the `// TTY detection for the TileRenderer is a follow-up task (AP-001).` comment.
6. Add unit test: `TestIsTerminal_NonFileWriterReturnsFalse` passes a `bytes.Buffer` and asserts `false`.

**Alternative shape (if implementer prefers minimal signature change):**

- Keep `newRenderer(f string)` and have the helper call `isTerminal(os.Stdout)` directly. This is acceptable but slightly less testable and ignores `cmd.OutOrStdout()` redirection in tests.

**Verification:**

- `go test ./internal/cli/...` passes, including the new helper test.
- Manual smoke: `backlogit list --format tile` in a real terminal shows ANSI bold; `backlogit list --format tile | cat` does not.
- `go build ./...`, `go vet ./...`, `golangci-lint run` all clean.

## Dependency Graph

Both units are independent and can be executed in either order or in parallel. Unit 1 is a YAML edit; Unit 2 is a Go change. No shared files.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Use `golang.org/x/term.IsTerminal` for TTY detection | Cross-platform, canonical Go idiom, single small dep | `os.Stdout` identity check (weaker, misses redirection); manual `syscall.IsATTY` (non-portable) |
| D2 | Helper returns `false` for non-`*os.File` writers | Test buffers, pipes, and other writers must stay plain; matches the recommended fix in 033.012-T | Returning `true` by default (would break captured output in tests and pipes) |
| D3 | Pass writer into `newRenderer` rather than hard-coding `os.Stdout` | Honors `cmd.OutOrStdout()` redirection used by tests and cobra's testing harness | Hard-coding `os.Stdout` (works in production but ignores test wiring) |
| D4 | No new helper file unless a second consumer appears in this shipment | YAGNI — keep helper colocated until reuse is concrete | Pre-emptively creating `internal/cli/tty.go` |

## Risks and Caveats

- **`golang.org/x/term` is a new direct dep** (currently only `golang.org/x/sys` is present, indirect). Trivial, well-trusted Go subrepo; constitution VI ("prefer stdlib when adequate") notes external deps must be justified — TTY detection is not adequately solved by stdlib alone.
- **Signature change to `newRenderer`** affects 4 call sites within `internal/cli/` (list, queue view, shipment list, stash list — all already pass `cmd.OutOrStdout()` to `.Render(...)`, so threading the same writer into `newRenderer` is mechanical). Verified via `Select-String "newRenderer" internal/cli/*.go` before planning.
- **Workflow permissions edit is YAML-sensitive** — `permissions:` must be a mapping under `drift:`, not a scalar.

## Plan Hardening Signals (REQUIRED)

- public API, schema, or contract change: **absent** — `TileRenderer` API is unchanged; CLI flags are unchanged
- security, auth, permission, or compliance-sensitive behavior: **absent** — moving workflow permissions to job level is a hardening improvement, not a permission expansion
- migration, backfill, destructive data/config action, or irreversible step: **absent**
- external integration, operator checkpoint, or external dependency: **absent** (new `golang.org/x/term` dep is a Go subrepo, not an external service)
- high runtime, rollout, or rollback risk: **absent** — both fixes are revertable with a single-commit `git revert`

Conclude: **Requires plan hardening: no**

## Runtime Verification and Closure

- **Unit 1 runtime surface**: GitHub Actions CI workflow. Verification: the next push to a PR runs `cli-reference-drift` workflow and completes successfully with the new job-level permissions. No monitoring or rollback artifact needed beyond the standard CI gate.
- **Unit 2 runtime surface**: `backlogit list --format tile` CLI output. Verification: manual smoke (TTY shows bold; pipe stays plain) plus the new unit test. No production monitoring needed; output formatting failures are user-visible immediately.

No release-observability monitoring plan, pre-deploy audit, or rollback trigger needed beyond the standard `git revert` path. Both changes are localized and low-blast-radius.

## Learnings Applied

No relevant `docs/compound/` entries surfaced for these specific fixes; both follow standard repo patterns already established (job-level permissions: see other workflows; TTY detection in Go: standard `golang.org/x/term` usage).

## Standards Check

- `.github/instructions/workflows.instructions.md`: Unit 1 directly satisfies the job-level permissions requirement.
- `.github/instructions/ci-security.instructions.md`: Unit 1 directly satisfies the least-privilege rule.
- `.github/instructions/go.instructions.md`: Unit 2 follows GoDoc, error-handling, and TDD conventions; helper is exported only if needed (currently package-private).
- Constitution III (TDD): Unit 2 has a test-first implementation note for the helper.
- Constitution VI (single-binary simplicity): adding `golang.org/x/term` is justified by a concrete requirement (TTY detection); no stdlib equivalent.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Type-Safe Go | ✅ helper has explicit `(w io.Writer) bool` signature; no `any` |
| II. MCP Protocol Fidelity | N/A — no MCP surface change |
| III. Test-First | ✅ helper test specified before implementation |
| IV. Workspace Containment | N/A — no `.backlogit/` writes |
| V. Structured Observability | N/A — no logging changes |
| VI. Single-Binary Simplicity | ⚠️ adds `golang.org/x/term` direct dep; justified by D1 |
| VII. CQRS Data Architecture | N/A |
| VIII. Git-Friendly Persistence | N/A |
| IX. Agent Context Efficiency | N/A |

No violations. One justified expansion (D1).
