---
title: "Build, Docs & CLI Parity"
date: 2026-04-11
origin: "stash entries FFF344F2, BC78CBDA, E3627E50, D7B72D92"
status: reviewed
---

## Problem Frame

Four current stash entries identify drift between the backlogit codebase and its
public-facing surfaces:

1. **FFF344F2** (formatting): 22 Go files fail `gofmt -l .`, and the Unix
   Makefile `fmt` target silently succeeds when files are unformatted.
2. **BC78CBDA** (docs accuracy): `docs/workflow.md`, `docs/installation.md`,
   `AGENTS.md`, and several instructions files reference `index.db` instead of
   `backlogit.db`, advertise Go 1.22 instead of 1.24, and show CLI flags or
   subcommands that do not exist.
3. **E3627E50** (stash docs): `README.md`, `docs/workflow.md`, and
   `docs/configuration.md` still reference the renamed `fetch-stash`
   subcommand and omit documentation for `get`, `edit`, and `remove`.
4. **D7B72D92** (CLI parity): `backlogit add` exposes 6 flags while MCP
   `backlogit_create_item` exposes 13 parameters. `backlogit update` exposes 6
   flags while MCP `backlogit_update_item` exposes 11 parameters. Both
   commands also contain duplicate persistence calls already performed by
   `core.CreateArtifact` and `core.UpdateArtifact`.

These are independent but thematically related: they address the gap between
what the tool does and what users and agents see. Shipping them together
reduces context-switching and lets a single review cycle validate the full
surface correction.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | All Go files pass `gofmt -l .` with zero output | FFF344F2 |
| R2 | Makefile `fmt` target fails the build when unformatted files exist | FFF344F2 |
| R3 | Documentation references correct database filename `backlogit.db` | BC78CBDA |
| R4 | Documentation references correct Go version requirement (1.24) | BC78CBDA |
| R5 | CLI examples in docs use only flags and subcommands that exist | BC78CBDA, E3627E50 |
| R6 | Stash documentation covers the current CLI surface (`add`, `list`, `get`, `edit`, `remove`, `harvest`) | E3627E50 |
| R7 | `backlogit add` accepts `--priority`, `--sprint`, `--assigned-to`, `--owner`, `--labels`, `--dependencies`, `--references`, `--commit` | D7B72D92 |
| R8 | `backlogit update` accepts `--description`, `--sprint`, `--assigned-to`, `--owner`, `--labels`, `--commit` | D7B72D92 |
| R9 | CLI `add` does not duplicate the `db.UpsertItem` call already performed by `core.CreateArtifact` | D7B72D92 |
| R10 | CLI `update` does not duplicate the `WriteArtifactFile` + `db.UpsertItem` calls already performed by `core.UpdateArtifact` via `persistArtifact` | D7B72D92 |

## Scope Boundaries

### In Scope

* Formatting all Go source files
* Fixing Makefile `fmt` target to fail on output
* Correcting documentation inaccuracies (db filename, Go version, CLI syntax)
* Updating stash documentation to reflect current CLI surface
* Adding missing flags to CLI `add` and `update` commands
* Removing duplicate persistence calls from CLI commands
* Tests for new CLI flags

### Non-Goals

* Adding new CLI subcommands (e.g., `backlogit dep` is out of scope)
* Changing MCP tool parameter schemas
* Modifying `core.CreateArtifact` or `core.UpdateArtifact` internals
* Fixing instructions files that reference `index.db` (constitution, sql-schema,
  go-mcp-server instructions are teaching artifacts whose update scope exceeds
  this shipment; BC78CBDA is partially addressed for user-facing docs only — a
  follow-up stash entry should track the remaining instructions file portion)
* CLI `--version` flag implementation (separate feature work)
* Stash `fetch-stash` CLI alias for backward compatibility

### Deferred to Implementation

* Exact wording for updated documentation paragraphs
* Whether `--labels` should accept comma-separated string or repeated flags
  (recommendation: comma-separated string to match MCP tool behavior)
* Whether `--dependencies` and `--references` on `add` should accept
  comma-separated strings (recommendation: yes, matching MCP)

## Implementation Units

### Unit 1: Format All Go Files and Fix Makefile

**Files:** all 22 unformatted `.go` files (bulk `gofmt -w`), `Makefile`
**Test files:** none (verified by `gofmt -l .` producing zero output)
**Effort size:** small
**Skill domain:** config
**Execution note:** format-first — run formatter, then fix build target
**Patterns to follow:** `make.ps1` line 46 (`if ($bad) { exit 1 }`)
**Dependencies:** none (should be first to establish clean baseline)

**Approach:**

1. Run `gofmt -w .` from the repository root to format all 22 files in place.
2. Update `Makefile` line 18 from `gofmt -l .` to a POSIX shell snippet that
   fails when output is non-empty. The Makefile is POSIX-only; Windows uses
   `make.ps1`:

   ```makefile
   fmt:
   	@bad=$$(gofmt -l .); test -z "$$bad" || { printf '%s\n' "$$bad"; exit 1; }
   ```

3. Verify: `make fmt` exits 0 on formatted code, exits 1 when a file is
   deliberately mis-formatted.

**Verification:**

* `gofmt -l .` produces no output
* `make fmt` exits 0 (POSIX only)
* `make.ps1 fmt` exits 0 (Windows)

### Unit 2: Fix Documentation Accuracy (General)

**Files:** `docs/workflow.md`, `docs/installation.md`, `AGENTS.md`
**Test files:** none (documentation)
**Effort size:** medium
**Skill domain:** docs
**Execution note:** characterization-first — audit current state, then fix
**Patterns to follow:** existing documentation style in each file
**Dependencies:** none (draftable in parallel with Unit 1; final CLI example
  verification should follow Units 4/5 since those change the CLI surface)

**Approach:**

`docs/installation.md`:

1. Line 16: change "Go 1.22" to "Go 1.24".
2. Line 24: change `go1.22` to `go1.24`.
3. Lines 71-75: remove or rewrite the `backlogit --version` paragraph. Replace
   with `backlogit help` to verify installation.
4. Line 120: change "Go 1.22" to "Go 1.24".

`docs/workflow.md`:

1. Line 81: already correctly says `backlogit.db` — no change needed.
2. Line 106: remove `backlogit list --label security` (no `--label` flag
   exists). Replace with a valid `backlogit list --type task --status active`
   example.
3. Line 161: fix `backlogit move T042 done` to valid syntax
   `backlogit move T042 --status done`.
4. Line 167: fix `backlogit dep add T042 --depends-on T038`. The CLI does not
   have a `dep` subcommand. Replace with the MCP equivalent or note that
   dependency management is through the MCP tool surface.

`AGENTS.md`:

1. Line 42: change `index.db` to `backlogit.db`.

**Verification:**

* No references to `index.db` remain in `AGENTS.md`
* No references to Go 1.22 remain in `docs/installation.md`
* All CLI examples in `docs/workflow.md` use flags that exist in the
  implemented CLI commands
* `backlogit --version` is no longer documented as available

### Unit 3: Fix Stash Documentation

**Files:** `README.md`, `docs/workflow.md`, `docs/configuration.md`
**Test files:** none (documentation)
**Effort size:** small
**Skill domain:** docs
**Execution note:** characterization-first
**Patterns to follow:** existing doc style; current CLI stash subcommands
  (`add`, `list`, `get`, `edit`, `remove`, `harvest`)
**Dependencies:** Unit 2 (both touch `docs/workflow.md`); final verification
  after Units 4/5 (CLI examples must match implemented flags)

**Approach:**

`README.md`:

1. Line 35: remove `fetch-stash` from the CLI command list.
2. Lines 77-83: update stash examples. Replace `stash fetch-stash
   --group-by-priority` with `stash list --group-by-priority`. Ensure `get`,
   `edit`, and `remove` examples are present.

`docs/workflow.md`:

1. Lines 135-136: replace `stash fetch-stash` with `stash list`.
2. Line 255: update `backlogit_fetch_stash` MCP tool reference to
   `backlogit_fetch_stash` (this name is correct in the MCP surface; only the
   CLI `fetch-stash` subcommand was renamed to `list`).

`docs/configuration.md`:

1. Lines 103-104: replace `stash fetch-stash` with `stash list`.
2. Line 114: `backlogit_fetch_stash` is the correct MCP tool name — no change.

**Verification:**

* No references to `fetch-stash` as a CLI subcommand remain
* Stash CLI examples include `add`, `list`, `get`, `edit`, `remove`, `harvest`
* MCP tool names remain correct (`backlogit_fetch_stash` is valid)

### Unit 4: CLI `add` Flag Parity and Duplicate Removal

**Files:** `internal/cli/add.go`
**Test files:** `internal/cli/add_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — write failing tests for new flags, then implement
**Patterns to follow:** existing flag wiring in `add.go` lines 85-90; existing
  `core.WithXxx` option functions in `internal/core/artifacts.go` lines 36-91
**Dependencies:** Unit 1 (code should be formatted before modification)

**Approach:**

1. Remove the duplicate `db.UpsertItem` call at line 77-79 of `add.go`.
   `core.CreateArtifact` already calls `db.UpsertItem` at
   `internal/core/artifacts.go:255`. The duplicate causes a harmless but
   unnecessary second write. Also remove the now-unused `internal/db` import.

2. Add flag declarations for the 8 missing parameters:

   ```go
   cmd.Flags().StringVar(&priority, "priority", "", "priority (low, medium, high, critical)")
   cmd.Flags().StringVar(&sprint, "sprint", "", "sprint ID")
   cmd.Flags().StringVar(&assignedTo, "assigned-to", "", "assignee")
   cmd.Flags().StringVar(&owner, "owner", "", "owner")
   cmd.Flags().StringVar(&labels, "labels", "", "comma-separated labels")
   cmd.Flags().StringVar(&dependencies, "dependencies", "", "comma-separated dependency IDs")
   cmd.Flags().StringVar(&references, "references", "", "comma-separated reference paths")
   cmd.Flags().StringVar(&commit, "commit", "", "commit SHA")
   ```

3. Wire each flag to its corresponding `core.WithXxx` option in the `RunE`
   body. For comma-separated fields (`labels`, `dependencies`, `references`),
   use a shared `splitCSV` helper that trims whitespace, drops empty entries,
   and returns `[]string`.

4. Keep `--section` behavior as-is (only `description` override). General
   section-body parity with MCP is out of scope for this unit; it requires a
   shared core/parser abstraction that does not exist yet.

**Required failing tests (write before implementation):**

* `TestAddCommand_Priority` — verify `--priority high` persists in frontmatter
* `TestAddCommand_AssignedTo` — verify `--assigned-to agent` persists
* `TestAddCommand_Labels` — verify `--labels "a,b,c"` persists as list
* `TestAddCommand_EmptyLabels` — verify `--labels ""` does not persist empty
  string entry
* `TestAddCommand_AllFlags` — smoke test with all 8 new flags set

**Verification:**

* All new tests pass via `go test ./internal/cli/...`
* `backlogit add --help` shows all new flags
* No unused imports or variables (`golangci-lint run`)

### Unit 5: CLI `update` Flag Parity and Duplicate Removal

**Files:** `internal/cli/update.go`
**Test files:** `internal/cli/update_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — write failing tests for new flags, then implement
**Patterns to follow:** existing flag wiring in `update.go` lines 153-158;
  `core.UpdateArtifact` update map pattern at
  `internal/core/artifacts.go:349-396`
**Dependencies:** Unit 1 (code should be formatted before modification)

**Approach:**

1. Remove redundant persistence in the frontmatter-update branch (lines 92-101).
   `core.UpdateArtifact` already calls `persistArtifact` which calls both
   `WriteArtifactFile` and `db.UpsertItem`. The CLI currently:
   - Calls `FindArtifactPath` (line 85) — redundant for frontmatter-only updates
   - Calls `WriteArtifactFile` (line 96) — redundant
   - Calls `db.UpsertItem` (line 99) — redundant

   Simplify the frontmatter branch to: call `core.UpdateArtifact` and handle
   the error. Remove `FindArtifactPath` from the frontmatter branch.

2. **Critical: re-resolve path after relocation.** When both frontmatter and
   section updates are requested (e.g., `--status done --section notes=...`),
   `core.UpdateArtifact` may relocate the file via `persistArtifact`. The
   section branch must re-resolve the artifact path AFTER the frontmatter
   update completes, not before. Move `FindArtifactPath` to the section branch
   entry and call it after the frontmatter branch has completed.

3. Add flag declarations for the 6 missing parameters:

   ```go
   cmd.Flags().StringVar(&description, "description", "", "new description")
   cmd.Flags().StringVar(&sprint, "sprint", "", "sprint ID")
   cmd.Flags().StringVar(&assignedTo, "assigned-to", "", "assignee")
   cmd.Flags().StringVar(&owner, "owner", "", "owner")
   cmd.Flags().StringVar(&labels, "labels", "", "comma-separated labels")
   cmd.Flags().StringVar(&commit, "commit", "", "commit SHA")
   ```

4. Wire each flag into the `updates` map in the `RunE` body using the
   `cmd.Flags().Changed()` pattern already established for `title`, `status`,
   and `priority`. For `labels`, use the same `splitCSV` helper from Unit 4 to
   produce `[]string` for the `updates["labels"]` type assertion in
   `core.UpdateArtifact`.

**Required failing tests (write before implementation):**

* `TestUpdateCommand_Description` — verify `--description "new"` persists
* `TestUpdateCommand_AssignedTo` — verify `--assigned-to agent` persists
* `TestUpdateCommand_Labels` — verify `--labels "x,y"` persists as list
* `TestUpdateCommand_EmptyLabels` — verify `--labels ""` is a no-op
* `TestUpdateCommand_StatusAndSection` — verify combined `--status active
  --section notes=...` works correctly after file relocation (regression for
  the relocation hazard)
* `TestUpdateCommand_NoDuplicateWrites` — verify artifact file is written
  exactly once (not triple-written)

**Verification:**

* All new tests pass via `go test ./internal/cli/...`
* Combined `--status` + `--section` update works after relocation
* `backlogit update --help` shows all new flags
* No unused imports or variables (`golangci-lint run`)

## Dependency Graph

```text
Unit 1 (formatting)
  └─> Unit 4 (CLI add)
  └─> Unit 5 (CLI update)

Unit 2 (docs accuracy) — draftable in parallel with Unit 1
  └─> Unit 3 (stash docs) [both touch docs/workflow.md]
  └─> Final verification after Units 4/5

Unit 3 (stash docs) — final verification after Units 4/5
```

Units 1, 2 can proceed in parallel. Units 4, 5 can proceed in parallel after
Unit 1. Unit 3 follows Unit 2. Final documentation verification follows all
code units.

## Final Verification Gate

After all units are complete, run the mandatory Go quality gates:

```bash
gofmt -l .            # Must produce zero output
go vet ./...          # Must pass with zero findings
golangci-lint run     # Must pass with zero findings
go test ./...         # Must pass all tests
```

All four gates must pass before the shipment is eligible for PR creation.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Use comma-separated strings for `--labels`, `--dependencies`, `--references` | Matches MCP tool input format; simpler than repeated flags | Repeated `--label` flags (inconsistent with MCP) |
| D2 | Remove duplicate persistence calls rather than adding `skipDB` flags | Eliminates dead code and double-writes; `core.CreateArtifact` and `core.UpdateArtifact` already handle full persistence | Adding a `NoPersist` option (over-engineering) |
| D3 | Do not update instructions files that reference `index.db` | Constitution, sql-schema, and go-mcp-server instructions are teaching/reference artifacts. Updating them requires a separate review cycle with broader scope | Updating all in this shipment (scope creep) |
| D4 | Do not add `--version` flag | Separate feature work requiring version injection at build time | Quick hack with hardcoded string (unmaintainable) |
| D5 | Keep `FindArtifactPath` in update.go section branch only, re-resolved after frontmatter update | Section updates need the file path to read/modify body content; frontmatter updates via `persistArtifact` may relocate the file, so path must be re-resolved post-update | Resolving path once before all updates (stale path after relocation) |
| D6 | Remove general `--section` parity from Unit 4 add.go | Section-body persistence lives in MCP tools, not in a shared core abstraction. Adding it to CLI crosses a module boundary and creates a second persistence path | Implementing section parity on add (scope creep, needs prerequisite refactor) |
| D7 | Use `splitCSV` helper for comma-separated flags | Trims whitespace, drops empty entries, returns `[]string`. Avoids `StringSliceVar` quoting issues and matches MCP comma-separated input format | `StringSliceVar` (shell quoting complications), raw `strings.Split` (no whitespace handling) |

## Risks and Caveats

1. **Double-upsert removal in add.go**: The duplicate `db.UpsertItem` is
   harmless today (idempotent upsert). Removing it is safe because
   `core.CreateArtifact` handles the upsert atomically with file creation.
   If the upsert fails, `CreateArtifact` removes the file to prevent orphans.

2. **Triple-write removal in update.go**: The redundant `WriteArtifactFile` and
   `UpsertItem` are harmless but wasteful. `persistArtifact` handles relocation
   logic that the CLI's manual write bypasses. Removing the duplicates actually
   improves correctness for status-change relocations.

3. **Section update branch in update.go**: This branch (lines 105-145) reads
   the file directly and writes back. It still needs `FindArtifactPath` and its
   own `UpsertItem`. This branch is NOT redundant and must be preserved. A
   future refactor could move section writing into `core.UpdateArtifact`, but
   that is out of scope. **Critical**: when combined with a status change
   (`--status done --section notes=...`), the path must be re-resolved AFTER
   `core.UpdateArtifact` completes because `persistArtifact` may have relocated
   the file.

4. **Documentation-only changes risk**: Doc fixes are low risk but should be
   verified against the actual CLI `--help` output to avoid introducing new
   inaccuracies. Final documentation verification should follow CLI code
   changes (Units 4/5).

5. **Import cleanup**: Removing `db.UpsertItem` from add.go will make the
   `internal/db` import unused. This must be removed in the same change or
   `golangci-lint` will fail.

## Learnings Applied

* Stage intercom protocol requires broadcasting substantive triage content, not
  just lifecycle transitions. See
  `docs/compound/workflow-issues/stage-intercom-missing-triage-content-2026-04-11.md`

## Standards Check

* **Type-safe Go (I)**: CLI flag additions use typed variables and existing
  `core.WithXxx` option functions. No `any` without justification.
* **Test-first (III)**: Units 4 and 5 specify test-first execution posture.
* **Workspace containment (IV)**: No changes to file routing or path handling.
* **Structured observability (V)**: No new logging needed; existing `slog`
  usage in core functions is sufficient.
* **Single-binary simplicity (VI)**: No new dependencies.
* **CQRS (VII)**: Duplicate persistence removal actually improves CQRS
  compliance by ensuring writes go through a single path.
* **Git-friendly persistence (VIII)**: No changes to file format.
