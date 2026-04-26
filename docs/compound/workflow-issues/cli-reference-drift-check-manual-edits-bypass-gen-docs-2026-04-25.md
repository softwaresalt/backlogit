---
title: "CLI Reference Drift Check fails when docs/cli-reference/ files are edited manually"
problem_type: workflow_issue
category: workflow_issue
component: cli
root_cause: schema_mismatch
resolution_type: code_fix
severity: medium
message: "Manually editing docs/cli-reference/*.md bypasses gen-docs and causes CLI Reference Drift Check CI failure; documentation source must live in the Cobra command Long field"
file_path: "internal/cli/telemetry.go"
resolved: true
tags: [cli-reference, cobra, long-field, gen-docs, make-docs, ci-drift, documentation-generation, workflow]
date: 2026-04-25
---

# CLI Reference Drift Check fails when docs/cli-reference/ files are edited manually

## Problem

The backlogit CI runs a **CLI Reference Drift Check** that regenerates all CLI
reference docs via `make docs` (`go run ./cmd/gen-docs docs/cli-reference`) and
then asserts `git diff --exit-code docs/cli-reference/` is clean. Any content
that was added directly to a `docs/cli-reference/*.md` file without a
corresponding source in the Go command definition is overwritten by the
generator and the diff check fails.

In practice: sections (`### Behavior`, `### Checkpoint`) were manually added to
`docs/cli-reference/backlogit_telemetry_harvest.md` as part of documentation
task 047.003-T, but the `newTelemetryHarvestCmd` in `internal/cli/telemetry.go`
had no `Long:` field — so `gen-docs` regenerated the file without those
sections, producing a drift.

## Symptoms

CI failure on the "CLI Reference Drift Check" step:

```
::error::CLI reference docs are out of date. Run 'make docs' locally and commit the updated files.
```

The diff shows sections present in the committed file are absent in the
regenerated version (the generator knows nothing about them).

## What Did Not Work

Directly editing `docs/cli-reference/backlogit_telemetry_harvest.md` and
committing the result. The file is a generated artifact. The CI check
immediately detected it diverged from what `gen-docs` would produce.

## Solution

1. **Add a `Long:` field** to the Cobra command struct in `internal/cli/telemetry.go`:

```go
cmd := &cobra.Command{
    Use:   "harvest",
    Short: "Parse Copilot CLI logs and write telemetry-sessions.jsonl",
    Long: `Parse Copilot CLI logs and write telemetry-sessions.jsonl

Each harvest run performs two writes:

1. Primary output: appends new session_summary and tool_usage JSONL records to
   .backlogit/telemetry-sessions.jsonl. Incremental by default...

### Checkpoint

A harvest checkpoint is saved to .backlogit/telemetry-checkpoint.json...`,
    RunE: func(cmd *cobra.Command, _ []string) error {
```

Note: avoid Markdown emphasis (`**`, backticks) inside the `Long` string if
you need the text to render cleanly in both CLI `--help` output and generated
Markdown. Plain text with `###` headings works for both.

2. **Regenerate the docs**:

```powershell
go run ./cmd/gen-docs docs/cli-reference
# or
make docs
```

3. **Commit both files together**:
   - `internal/cli/telemetry.go` — the documentation source
   - `docs/cli-reference/backlogit_telemetry_harvest.md` — the generated output

## Why This Works

`cmd/gen-docs` uses `cobra/doc.GenMarkdownTreeCustom` to produce markdown from
the Cobra command tree. The Cobra fields it uses are:

| Cobra field | Rendered as |
|---|---|
| `Short` | YAML frontmatter `description` + first line in the page body |
| `Long` | Synopsis block below the command usage line |
| `Flags()` | Options section |
| `PersistentFlags()` | Options inherited from parent commands section |

When `Long` is set, the generated markdown gains a `### Synopsis` section with
the full content. The CI check then regenerates an identical file and the diff
is clean.

## Prevention

- **Never edit `docs/cli-reference/*.md` directly.** These are generated
  artifacts. Treat them as read-only.
- **Add content via the `Long:` field** in the corresponding Cobra command
  struct.
- **Always run `make docs` locally** after editing any CLI command definition
  and commit the regenerated files in the same commit as the Go change.
- **Pre-commit check**: run `make docs && git diff --exit-code docs/cli-reference/`
  before pushing any branch that touches `internal/cli/`.

## Related Solutions

- `docs/compound/workflow-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md`
  — similar pattern: local tests pass, CI detects drift because a generated or
  registered artifact was not committed.
