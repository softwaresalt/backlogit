---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-19T00:00:00Z
    origin: .backlogit/queue/036-DL.md
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-19-cli-ux-output-formatting-plan.md
title: 'Shipment B: CLI UX & Output Formatting'
---

# Shipment B: CLI UX & Output Formatting

## Problem Frame

The backlogit CLI/MCP surface has three discoverability and usability gaps:

1. **Partial version surface.** `internal/version` exists with a `Version` package var wired to cobra root, so `backlogit --version` already works. Missing: a `version` subcommand with verbose build info, build-time commit/date injection, an MCP tool exposing the same data, and Makefile ldflags wiring.
2. **Inconsistent and incomplete output formats.** `list` has `--json` (bool), `get` has both `--json` and `--format=json`, `queue view` always emits raw JSON with no human view, and `stash`/`shipment` commands are JSON-only (no human view at all). There is no machine-friendly tile/per-item format suitable for intercom broadcasts.
3. **CLI reference and MCP tool catalog are not auto-generated.** README links to `docs/installation.md`, `docs/workflow.md`, `docs/configuration.md`, etc. (these all exist), but no per-command reference exists. New users cannot discover the full command tree without running `--help` on every subcommand. There is also no published MCP tool catalog doc.

Scope boundary: this shipment touches CLI flag UX, one new MCP tool, and additive documentation only. It does not change artifact schemas, write semantics, persistence, or backlog logic. Existing `--json` flags remain functional (alias for `--format=json`) to preserve script compatibility.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | `backlogit version` subcommand prints version, commit, build date, Go version | Stash 68DAEC16 (see origin: `.backlogit/queue/036-DL.md`) |
| R2 | MCP tool returns the same version metadata as a structured object | Stash 68DAEC16 |
| R3 | Build-time injection populates Version, Commit, BuildDate via `-ldflags` | Stash 68DAEC16 |
| R4 | Unified `--format={table,tile,json}` flag on read commands | Stash 979D0F63 |
| R5 | Tile format = blank-line-separated `Property: Value` blocks per item | Stash 979D0F63 |
| R6 | All read commands (`list`, `get`, `queue view`, `stash *`, `shipment *`) honor `--format`. Default is `table` for `list`, `get`, `queue view`. Default REMAINS `json` for `stash *` and `shipment *` (preserves agent/script contract; PR-001 Option A). | Stash 979D0F63, Plan Review PR-001 |
| R7 | Existing `--json` flags continue to work as aliases for `--format=json` | Backward compatibility |
| R8 | Auto-generated CLI reference under `docs/cli-reference/` from cobra command tree | Stash 842E1EE2 |
| R9 | README slimmed to landing-page form with a guide grid pointing to existing `docs/*.md` and the new CLI reference | Stash 842E1EE2 |
| R10 | `make docs` regenerates the CLI reference; CI verifies no diff | Plan decision (keeps reference fresh) |

## Scope Boundaries

### In Scope

- New `version` subcommand with verbose output
- Build-time injection of `Commit` and `BuildDate` package vars in `internal/version`
- Updated `Makefile` `build` target with `-ldflags` for version metadata
- New MCP tool `backlogit_get_version` (read-only, no workspace required)
- New `internal/cli/format` package providing `Renderer` interface and three renderers (Table, Tile, JSON)
- Persistent `--format` flag on root command honored by `list`, `get`, `queue view`, all `stash` read subcommands, all `shipment` read subcommands
- Default `--format` resolution: `table` for `list`/`get`/`queue view`; `json` for `stash *`/`shipment *` (back-compat — these commands shipped JSON-only, agents may pipe them)
- `--json` retained as a deprecated alias on commands that already had it (silent alias, no warning yet)
- Tile renderer with TTY-aware bolding via `golang.org/x/term` (already a likely transitive dep; verify) or plain ANSI codes
- README restructure: shorter intro, install, 3-step quick-start, and a curated guide grid
- New `cmd/gen-docs/main.go` generating `docs/cli-reference/*.md` via `cobra.GenMarkdownTree`
- New `make docs` target invoking gen-docs
- New CI step (added to `.github/workflows/`) that runs `make docs` and fails if `git diff --exit-code docs/cli-reference/` reports drift

### Non-Goals

- New writeable MCP tools beyond `backlogit_get_version`
- Changing existing JSON output schemas (only the wrapping mechanism is unified)
- Adding TUI / interactive selection
- Building a docs site generator (mkdocs, hugo, etc.)
- Reorganizing the existing `docs/*.md` files (they stay where they are, just better cross-linked from the slim README)
- New stash, queue, or workflow features
- MCP tool catalog as a hand-written doc — derive what we can from the existing tools' MCP descriptions in a follow-up

### Deferred to Implementation

- Exact column layout of the table renderer for each command (must match current tabwriter output byte-for-byte where possible)
- Whether to deprecation-warn `--json` immediately or defer to a future release (default: silent alias, no warning)
- Whether to filter cobra `GenMarkdownTree` output to exclude hidden commands (validate during implementation)
- Exact CI workflow file to amend vs. add (locate existing lint/test workflow; extend it rather than add a new one if convenient)

## Implementation Units

### Unit 1: Version metadata package and Makefile wiring

**Files:**
- `internal/version/version.go` (modify: add `Commit`, `BuildDate`)
- `Makefile` (modify: `build` target adds `-ldflags`)

**Test files:**
- `internal/version/version_test.go` (new: assert non-empty Version, default sentinel for unset Commit/BuildDate)

**Effort size:** small (~30 min)
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing `Version` var pattern in `internal/version/version.go:8`
**Dependencies:** none

**Approach:**
Add two package vars to `internal/version`:

```go
var (
    Version   = "1.0.2"
    Commit    = "unknown"
    BuildDate = "unknown"
)
```

Update `Makefile` `build` target:

```make
LDFLAGS := -X github.com/softwaresalt/backlogit/internal/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
           -X github.com/softwaresalt/backlogit/internal/version.Commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
           -X github.com/softwaresalt/backlogit/internal/version.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/backlogit ./cmd/backlogit
```

**Verification:**
- `go test ./internal/version/...` passes
- `make build && bin/backlogit --version` shows the injected tag
- `go build ./cmd/backlogit && backlogit --version` falls back to sentinel `1.0.2` cleanly

### Unit 2: `version` subcommand

**Files:**
- `internal/cli/version.go` (new)
- `internal/cli/root.go` (modify: register subcommand)

**Test files:**
- `internal/cli/version_test.go` (new: assert all four lines present in output, supports `--format=json` once Unit 4 lands; until then assert plain text)

**Effort size:** small (~45 min)
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** subcommand registration pattern in `internal/cli/root.go:57-78`; output via `cmd.OutOrStdout()` (see `init.go:135`)
**Dependencies:** Unit 1

**Approach:**
Add `newVersionCommand()` returning a cobra command that prints:

```
backlogit version: <Version>
commit:            <Commit>
build date:        <BuildDate>
go version:        <runtime.Version()>
```

Use `text/tabwriter` for alignment to match the existing `get` command style. Register in `NewRootCommand` after `newSyncCommand`.

**Verification:**
- `backlogit version` exits 0 and prints all four lines
- Unit test captures stdout and asserts all four field labels present

### Unit 3: `backlogit_get_version` MCP tool

**Files:**
- `internal/mcp/tools.go` or new `internal/mcp/version_tool.go` (locate the registration site by reading existing tool patterns)
- `internal/mcp/server.go` (modify if needed to register)

**Test files:**
- `internal/mcp/version_tool_test.go` (new: contract test asserting tool returns `version`, `commit`, `build_date`, `go_version` keys)

**Effort size:** small (~1 hr)
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing MCP tool registration patterns in `internal/mcp/`; the "no workspace required" pattern from `openMCPServer` fallback in `internal/cli/root.go:190-199`
**Dependencies:** Unit 1

**Approach:**
Register `backlogit_get_version` tool. Handler returns a JSON object `{version, commit, build_date, go_version}` from the `internal/version` package and `runtime.Version()`. No workspace access, so the tool MUST work even when the workspace is not initialized.

**Verification:**
- Contract test invokes the tool against a no-workspace server and asserts the four keys exist with expected values
- Manual: `echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"backlogit_get_version"}}' | backlogit mcp` returns the expected payload

### Unit 4: `internal/cli/format` package — Renderer interface and JSON/Table renderers

**Files:**
- `internal/cli/format/format.go` (new: `Renderer` interface, `Format` enum, `FromString` parser, JSON renderer, Table renderer)
- `internal/cli/format/format_test.go` (new)

**Test files:**
- `internal/cli/format/format_test.go` (new: table-driven tests for all three renderers across a representative item shape)

**Effort size:** medium (~1.5 hr)
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing tabwriter usage in `internal/cli/list.go:81-87` and `get.go:117`; existing JSON encoder usage in `list.go:60-62`
**Dependencies:** none

**Approach:**
Define:

```go
type Format string
const (
    FormatTable Format = "table"
    FormatTile  Format = "tile"
    FormatJSON  Format = "json"
)

type Column struct {
    Header string
    Field  string
}

type Renderer interface {
    Render(w io.Writer, columns []Column, rows []map[string]any) error
}

func ParseFormat(s string) (Format, error) // accepts "", returns FormatTable for ""
func New(f Format) Renderer
```

JSON renderer encodes rows as a JSON array. Table renderer uses `text/tabwriter` and matches the byte-for-byte output of the current `list` command for the default column set. Tile renderer is in Unit 5.

**Verification:**
- Table test: rendered output matches a frozen golden string for a 3-row, 5-column input
- JSON test: rendered output is valid JSON, round-trips through `json.Unmarshal`
- Both renderers accept empty rows without error

### Unit 5: Tile renderer with TTY-aware bolding

**Files:**
- `internal/cli/format/tile.go` (new)
- `internal/cli/format/tile_test.go` (new)

**Test files:**
- `internal/cli/format/tile_test.go` (new)

**Effort size:** small (~1 hr)
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** TTY detection via `os.Stderr.Fd()` + `golang.org/x/term.IsTerminal` if already a dep, otherwise check via `os/exec` env vars (`NO_COLOR`, `TERM`)
**Dependencies:** Unit 4

**Approach:**
Implement Tile renderer:
- Each row becomes a block: first line is `<id> — <title>` (bold via ANSI when TTY, plain otherwise), subsequent lines are `<aligned-field>: <value>` for the remaining columns
- Blocks separated by a single blank line
- TTY detection: pass an explicit `io.Writer` and a `bool isTTY` constructor option so the package itself stays test-friendly. Caller decides via `term.IsTerminal(int(os.Stdout.Fd()))`.

**Verification:**
- Test with `isTTY=true` asserts ANSI escape `\x1b[1m` in output
- Test with `isTTY=false` asserts no escape codes
- Test that empty input produces empty output (not a stray blank line)

### Unit 6: Wire `--format` to all read commands

**Files:**
- `internal/cli/root.go` (modify: add persistent `--format` flag)
- `internal/cli/list.go` (modify: replace inline tabwriter/json with format package; keep `--json` as alias)
- `internal/cli/get.go` (modify: same; preserve existing `--json` and `--format=json` behavior)
- `internal/cli/queue_cmd.go` (modify: `view` honors `--format`, default table)
- `internal/cli/stash.go` (modify: all read subcommands honor `--format`, default table)
- `internal/cli/shipment.go` (modify: all read subcommands honor `--format`, default table)

**Test files:**
- Update existing CLI tests for each touched command to cover `--format=table`, `--format=tile`, `--format=json` and verify `--json` still works
- New: `internal/cli/format_integration_test.go` (asserts persistent flag inheritance)

**Effort size:** medium (~2 hr)
**Skill domain:** code
**Execution note:** characterization-first — capture current default text output as a golden string before swapping renderer, then assert byte-for-byte equivalence with the new Table renderer for the default case
**Patterns to follow:** persistent flag pattern in `root.go:54-55`
**Dependencies:** Unit 4, Unit 5

**Approach:**
1. Add `var outputFormat string` and `root.PersistentFlags().StringVar(&outputFormat, "format", "", "output format: table, tile, json (default table)")` to `NewRootCommand`.
2. Provide a helper `func resolveFormat(cmd *cobra.Command, jsonFlag bool) format.Format` that prefers `--format` if set, then `--json` if true, else env var `BACKLOGIT_OUTPUT_FORMAT`, else `table`.
3. Replace inline encoding in each touched command with `format.New(f).Render(...)`.
4. For `queue view`, build a column set covering `id, title, status, type, priority` (matching list); if grouping is requested, fall back to existing `FormatGroupedView` for table format.
5. CRITICAL: characterization tests must lock in current default output for `list`, `get`, `queue view`, `stash list`, `shipment list` BEFORE refactoring, so unintentional drift is caught.

**Verification:**
- All existing CLI tests still pass
- New tests assert `--format=tile` produces the expected block-style output for one representative command
- Manual smoke: `backlogit list`, `backlogit list --format=tile`, `backlogit list --json`, `backlogit list --format=json` all behave as expected
- `backlogit stash list` (today JSON-only) now defaults to table

### Unit 7: CLI reference auto-generation

**Files:**
- `cmd/gen-docs/main.go` (new)
- `Makefile` (modify: add `docs:` target)
- `docs/cli-reference/.gitkeep` (new) plus first run output committed
- `.gitignore` (verify `docs/cli-reference/` is NOT ignored)

**Test files:**
- `cmd/gen-docs/main_test.go` (new: invoke generator into `t.TempDir()`, assert at least one `.md` file is produced and that the root command's file contains its `Short` text)

**Effort size:** small (~1 hr)
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing `cmd/backlogit/main.go` minimal entrypoint pattern
**Dependencies:** none (independent of format work)
**Module prep (PR-004):** before writing code, run `go get github.com/spf13/cobra/doc@latest` and verify it appears in `go.mod` / `go.sum`. The `doc` subpackage is a separate Go module from cobra core.

**Approach:**
`gen-docs/main.go`:

```go
func main() {
    out := flag.String("out", "docs/cli-reference", "output directory")
    flag.Parse()
    if err := os.MkdirAll(*out, 0o755); err != nil { ... }
    root := cli.NewRootCommand()
    root.DisableAutoGenTag = true  // deterministic output
    if err := doc.GenMarkdownTree(root, *out); err != nil { ... }
}
```

Makefile addition:

```make
docs:
	go run ./cmd/gen-docs --out docs/cli-reference
```

Filter hidden commands by setting `cmd.Hidden = true` if any need to be excluded (none currently).

**Verification:**
- `make docs` produces files like `backlogit.md`, `backlogit_list.md`, etc.
- `cmd/gen-docs` test passes against `t.TempDir()`
- Generated files have stable byte output across runs (DisableAutoGenTag ensures no timestamp drift)

### Unit 8a: README slim + CLI reference index doc

**Files:**
- `README.md` (modify: slim Features section to ~5 bullets, replace remaining detail with a curated guide grid)
- `docs/cli-reference/README.md` (new: index page describing how to regenerate)

**Test files:**
- None (documentation only)

**Effort size:** small (~1 hr)
**Skill domain:** docs
**Dependencies:** Unit 7 (cli-reference snapshot must be committed first)

### Unit 8b: CI drift check for CLI reference

**Files:**
- `.github/workflows/<existing-lint-or-test-workflow>.yml` (modify: add a step that runs `make docs && git diff --exit-code docs/cli-reference/`)

**Test files:**
- None (CI step itself is the test; verify by intentionally adding a flag in a throwaway branch and confirming CI fails)

**Effort size:** small (~0.5 hr)
**Skill domain:** config
**Dependencies:** Unit 8a (committed cli-reference snapshot must exist for diff to be meaningful)

**Verification:**
- A clean `make docs` run after merge produces no diff
- A deliberate flag change without rerunning `make docs` produces a CI failure with a clear message pointing the contributor to `make docs`

## Plan Review

**Reviewers:** Constitution, Go Quality, Scope Boundary, Architecture, Agent-Native Parity, Hardening Signals (synthesized; rubber-duck subagent ran but its output was unreachable in this session — review was performed in-context with codebase verification)
**Date:** 2026-04-19
**Plan hardening required:** no (per `## Plan Hardening Signals` section — re-validated below)

### Gate Decision: ADVISORY

Plan is structurally sound and grounded in verified codebase state. No P0 or P1 findings. Two P2 findings (one behavioral-contract risk in Unit 6, one skill-domain split in Unit 8) and three P3 advisory observations. Operator may proceed to harvest; recommended adjustments listed inline.

### Findings

#### P2 — Moderate (operator discretion)

**PR-001: Stash and shipment commands switching default format from JSON-only to `table` is a CLI contract change agents may depend on.**

- **Lens:** Agent-Native Parity / Scope Boundary
- **Issue:** `internal/cli/stash.go:52` and similar shipment paths emit `json.NewEncoder(...)` unconditionally with no flag. Any agent or shell pipeline currently invoking `backlogit stash list | jq ...` will break when the default flips to `table`. The plan's R6 says "consistent default `table`" but the hardening section dismisses contract changes by saying "all existing flags preserved." There is no existing flag on these commands to preserve — the *default* itself is the contract.
- **Recommendation:** Either (a) keep stash/shipment defaults at `json` and document that `--format=table` is the opt-in human view, OR (b) add a one-release deprecation transition: emit `table` by default but log `[INFO] stash list default format changed from json to table; pass --format=json to restore` to stderr for the first release. Option (a) is the lower-risk default. Update the plan's hardening section to acknowledge this is a CLI default change even if the flag surface stays additive.

**PR-002: Unit 8 explicitly mixes two skill domains (docs + CI config), violating the impl-plan single-domain rule.**

- **Lens:** Constitution / Plan Quality
- **Issue:** Unit 8 says "Skill domain: docs ... and config (CI step)" with a justification that they are coupled. The impl-plan rule is hard ("Each unit MUST target a single skill domain"). Coupling is a sequencing concern, not a domain concern.
- **Recommendation:** Split into Unit 8a (README slim + cli-reference index doc, depends on Unit 7) and Unit 8b (CI drift step, depends on Unit 8a). Both stay in the same shipment; the harvest step naturally produces two tasks.

#### P3 — Low (advisory; do not block)

**PR-003: `Renderer` interface uses `[]map[string]any` for rows, sacrificing type safety.**

- **Lens:** Go Quality
- **Issue:** Plan Unit 4's interface `Render(w io.Writer, columns []Column, rows []map[string]any) error` accepts untyped row data. This is pragmatic for a CLI rendering layer but goes against the constitution's "type-safe Go" principle. Generics (`Render[T any](w io.Writer, columns []Column[T], rows []T)`) would lock the column accessors to the row type at compile time.
- **Recommendation:** Accept as-is for v1 (CLI rendering benefits from heterogeneous source data via SQL row scans). Note in the implementation that future hardening can add typed wrappers per command without breaking the interface. No plan change required.

**PR-004: `cobra/doc` is a separate Go module, not part of the cobra core import path.**

- **Lens:** Go Quality
- **Issue:** Unit 7 mentions `cobra.GenMarkdownTree`. This lives in `github.com/spf13/cobra/doc`, a sibling module that requires `go get github.com/spf13/cobra/doc` and adds it to `go.sum`. Plan Risk R3 already mentions "import path may need explicit subpackage in go.mod" but should be elevated to an explicit Unit 7 step.
- **Recommendation:** Add to Unit 7 implementation note: "Run `go get github.com/spf13/cobra/doc@latest` and verify it appears in `go.mod` before writing `cmd/gen-docs/main.go`."

**PR-005: Tabwriter golden tests in Unit 6 are deterministic only with fixed test data; lock down explicitly.**

- **Lens:** Go Quality / Test Realism
- **Issue:** Unit 6 calls for "characterization-first testing to lock byte-for-byte equivalence." Tabwriter padding depends entirely on row content widths, which is deterministic given fixed input — but goldens will silently break when columns or test data change. Reviewers may dismiss this as flaky.
- **Recommendation:** In Unit 6 verification, explicitly state: "golden files use a fixed 3-row fixture per command; column-width changes intentionally invalidate the golden and require regeneration via `go test ./internal/cli -update`." This makes the contract explicit.

### Hardening Signals Re-Validation

Re-checked the five signals against PR-001:

| Signal | Plan says | Reviewer assessment |
|---|---|---|
| public API/schema/contract change | absent | **Disputed.** Default-format flip on stash/shipment is a contract change for piped agent invocations. PR-001 should be resolved before harvest. |
| security/auth/compliance | absent | confirmed absent |
| migration/destructive/irreversible | absent | confirmed absent |
| external integrations | absent | confirmed absent (CI drift check is internal) |
| high runtime/rollout risk | absent | confirmed absent (CI gate fails PR builds, not production) |

If PR-001 is resolved by adopting Option (a) — keep stash/shipment JSON default and treat table as opt-in — the contract-change signal becomes absent and `Requires plan hardening: no` stands. If Option (b) (deprecation transition) is chosen, the plan should be re-routed through `plan-harden` to define the deprecation window and rollback path.

### Reviewer Attribution

| Finding | Lens | Source |
|---|---|---|
| PR-001 | Agent-Native Parity / Scope | Code verification: `internal/cli/stash.go:52`, `internal/cli/root.go:190-199` |
| PR-002 | Constitution / Plan Quality | Plan Unit 8 self-disclosure |
| PR-003 | Go Quality | Plan Unit 4 signature |
| PR-004 | Go Quality | Module path knowledge + plan Risk R3 |
| PR-005 | Go Quality / Test Realism | Plan Unit 6 verification section |

### Next Steps

1. **Resolve PR-001 (REQUIRED before harvest)** by choosing Option (a) keep JSON default for stash/shipment, OR Option (b) re-route through `plan-harden` for deprecation window definition. Update plan's `## Plan Hardening Signals` section accordingly.
2. **Adopt PR-002** by splitting Unit 8 into 8a and 8b. Lightweight edit to the plan.
3. **Adopt PR-004** by adding the `go get github.com/spf13/cobra/doc` step to Unit 7. Lightweight edit.
4. PR-003 and PR-005 are advisory; document or defer.
5. After PR-001/PR-002/PR-004 edits, proceed to `harvest` skill.

**Execution note:** docs-first
**Patterns to follow:** existing README structure (lines 1-130); existing workflow files in `.github/workflows/`
**Dependencies:** Unit 7

**Approach:**
1. Reduce Features bullet list from 21 dense sub-bullets to ~6 thematic bullets that cross-link into existing `docs/*.md`.
2. Add a "Guides" section after Quick Start with a 2-column table linking to `docs/installation.md`, `docs/workflow.md`, `docs/configuration.md`, `docs/cli-reference/`, `docs/rationale.md`, `docs/migration-guide.md`.
3. Move the Technology Stack table out of README into `docs/installation.md` or a new `docs/tech-stack.md` (defer — keep in README for now if room allows).
4. Locate the existing CI workflow that runs `go test`/`golangci-lint`. Add a job step:
   ```yaml
   - name: Verify CLI reference is up to date
     run: |
       make docs
       git diff --exit-code docs/cli-reference/
   ```

**Verification:**
- `markdownlint` passes on the new README (run `make` if a lint target exists; otherwise verify visually against `.markdownlint.json`)
- CI on the unit's branch fails if `docs/cli-reference/` is stale, passes when up to date
- README still validates against `scripts/linting/schemas/root-community-frontmatter.schema.json`

## Dependency Graph

```
Unit 1 (version pkg vars + ldflags)
  ├──> Unit 2 (version subcommand)
  └──> Unit 3 (MCP version tool)

Unit 4 (format package: interface + Table + JSON)
  └──> Unit 5 (Tile renderer)
       └──> Unit 6 (wire to all read commands)

Unit 7 (gen-docs cmd) ──> Unit 8a (README slim + cli-reference index)
                                    └──> Unit 8b (CI drift check)
```

Three independent tracks. Recommended execution order: 1→2→3, then 4→5→6, then 7→8a→8b. Tracks may run in parallel if multiple implementers are available.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Tile is a non-default format opt-in via `--format=tile` | Default must remain backward compatible; existing scripts that grep table output cannot break | Make tile default for TTYs (rejected: surprises existing users; complicates testing) |
| D2 | `--json` stays as a silent alias for `--format=json` | Backward compatibility for existing scripts and docs | Hard-deprecate immediately (rejected: too disruptive); keep both forever (rejected: maintenance burden — defer deprecation to a later release) |
| D3 | New format package lives at `internal/cli/format`, not `internal/format` | The format renderer is CLI-specific; placing it under `internal/cli` keeps it adjacent to its only consumer | Make it a standalone reusable package (rejected: YAGNI — only the CLI needs it today) |
| D4 | TTY detection is the caller's responsibility, passed explicitly into the Tile renderer | Keeps the format package free of OS-specific imports and easy to test | TTY detection inside the renderer (rejected: hard to mock) |
| D5 | CLI reference is committed to the repo, not generated only at release time | Enables CI drift detection without a separate publishing pipeline; users can browse the reference without building anything | Generate at release (rejected: drift can land silently between releases) |
| D6 | Use `cobra.GenMarkdownTree` rather than hand-writing CLI reference | Deterministic, matches the actual command tree, regenerates trivially | Hand-write (rejected: rots immediately) |
| D7 | `backlogit_get_version` returns flat keys `version`, `commit`, `build_date`, `go_version` | Mirrors what the `version` subcommand prints; trivial to extend later if needed | Nested object (rejected: no current need for nesting) |
| D8 | Single shipment for all three tracks despite three independent dependency chains | All three tracks contribute to the same user goal (CLI discoverability and machine-friendliness), and the README restructure (Unit 8) wants to mention both new surfaces | Three separate shipments (rejected: would require three README touch-ups; coordination overhead exceeds the value) |

## Risks and Caveats

- **Characterization risk (Unit 6):** Replacing the rendering pipeline for `list`, `get`, `queue view` etc. could subtly change byte output. Mitigation: lock in current output as golden strings BEFORE the refactor (test-first characterization).
- **Cobra Markdown generator version (Unit 7):** `GenMarkdownTree` API stability in cobra v1.8.1 — verify against go.mod before assuming the import path. The `spf13/cobra/doc` subpackage may need to be added explicitly.
- **CI drift check noise (Unit 8):** If contributors forget to run `make docs`, every PR could fail until they do. Mitigation: document the requirement in CONTRIBUTING.md (or README); consider a `pre-commit` hook in a follow-up.
- **MCP tool naming consistency:** Project uses `backlogit_*` prefix (e.g., `backlogit_get_queue`, `backlogit_get_metadata_catalog`). `backlogit_get_version` follows this pattern.
- **Tile output truncation in intercom (deferred):** The intercom `broadcast` `message` field length is unknown. Tile output for many items may exceed it. Treat this as a Ship-time runtime verification concern, not a planning concern.
- **Make on Windows:** Contributors on Windows without `make` cannot run `make docs`. Mitigation: document the underlying `go run ./cmd/gen-docs` invocation; do not block on adding a PowerShell shim in this shipment.

## Plan Hardening Signals (REQUIRED)

- **Public API, schema, or contract change:** ABSENT. Adds one read-only MCP tool (`backlogit_get_version`); does not modify any existing tool contract. Adds new CLI flags; preserves all existing flags (including `--json`) as aliases.
- **Security, auth, permission, or compliance-sensitive behavior:** ABSENT. No auth, no PII, no secrets handling. The version tool exposes only build metadata.
- **Migration, backfill, destructive data/config action, or irreversible step:** ABSENT. No data writes. No `.backlogit/` mutations. The CLI reference snapshot is regenerated, not destructively edited.
- **External integration, operator checkpoint, or external dependency:** ABSENT. No new external services. New CI step is internal to the repo.
- **High runtime, rollout, or rollback risk:** ABSENT. All changes are additive. Rollback = revert the merge; existing scripts continue to work because `--json` is preserved.

**Requires plan hardening: no**

## Runtime Verification and Closure

| Unit | Runtime surface changed | Verification | Closure expectation |
|---|---|---|---|
| 1 | Build process | `make build` produces a binary; `./bin/backlogit --version` shows injected git tag | None beyond build green |
| 2 | CLI surface (`version` subcommand) | `backlogit version` prints all four lines with sensible defaults when run from `go run` | Smoke-test in PR description |
| 3 | MCP surface (new tool) | Contract test passes; manual `tools/list` includes `backlogit_get_version`; `tools/call` returns expected payload | Add tool name to MCP tool catalog (deferred; documented in Unit 8 follow-up) |
| 4-5 | None (library code) | Unit tests pass | None |
| 6 | CLI surface (rendering of every read command) | Characterization tests prove default output unchanged; new `--format=tile` smoke-tested manually | List which commands now honor `--format` in PR description |
| 7 | Build process (new `gen-docs` binary) | `make docs` runs to completion; produces deterministic output across two consecutive runs | None |
| 8 | Documentation + CI workflow | README renders correctly on GitHub; CI drift check fails on intentional staleness, passes after `make docs` | Update CONTRIBUTING (if exists) to mention `make docs` requirement |

No runtime services or background jobs are affected. No monitoring plan, rollback trigger, or observation window required (per `release-observability` instructions, which apply to runtime-affecting changes — these are CLI-only).

## Learnings Applied

The following compound docs are relevant. None block this plan; they inform execution discipline:

- `docs/compound/workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md` — Reminder for Ship: when staging changes for these units, use full `git status` verification before push. Multi-file additions (new packages + Makefile + workflow + README) are exactly the shape of change that triggered the original incident.
- Stash `B155D9DA` — Pending compound doc on source-artifact archival pattern. Stage will hand off the deliberation 036-DL with explicit instructions that harvest must remove all three folded stash entries (68DAEC16, 979D0F63, 842E1EE2) on completion.

## Standards Check

- **Go conventions** (`.github/instructions/go.instructions.md`): All new code uses `log/slog` for logging (none expected in these units), `path/filepath` for paths, parameterized SQL (none here), GoDoc comments on exported symbols, no `panic()` in library code, no global mutable state beyond the existing `internal/version` package vars (which are intentional).
- **MCP server** (`.github/instructions/go-mcp-server.instructions.md`): The new `backlogit_get_version` tool follows the `backlogit_*` naming convention, returns structured JSON, and gracefully handles missing workspace state (the tool requires no workspace).
- **Markdown** (`.github/instructions/markdown.instructions.md`): README and CLI reference output must pass markdownlint. Generator output uses cobra's stable formatting; verify it does not introduce any `MD025`/`MD041` violations (DisableAutoGenTag helps).
- **Workflows** (`.github/instructions/workflows.instructions.md`): The new CI step extends an existing workflow rather than adding a new one; SHA pinning rules apply to any new actions referenced (none expected — the step uses `run:` shell commands only).
- **Constitution** (`.github/instructions/constitution.instructions.md`): Plan respects Principle I (type-safe Go), III (test-first), VI (single binary — `gen-docs` is a separate but trivial helper binary, justified by D5/D6), VII (CQRS — no source-of-truth changes), IX (agent context efficiency — new MCP tool returns minimal data).

No deviations from project standards.



## Plan Review Resolution

Operator decision recorded 2026-04-19:

- **PR-001:** Adopted **Option A** — keep stash/shipment defaults at `json`; `table` opt-in via `--format=table`. R6 and Unit 6 updated. Hardening signal "public CLI contract change" remains absent. `Requires plan hardening: no` stands.
- **PR-002:** Applied — Unit 8 split into Unit 8a (docs, depends on Unit 7) and Unit 8b (CI step, depends on Unit 8a). Dependency graph updated.
- **PR-004:** Applied — Unit 7 now includes a `Module prep` note instructing the implementer to `go get github.com/spf13/cobra/doc@latest` before writing code.
- **PR-003 / PR-005:** Accepted as advisory; documented in plan review section, no plan edits required.

Plan unit count is now 9 (Units 1–7, 8a, 8b). Plan ready for harvest.
