---
title: "bufio.Scanner fix for oversized JSONL lines missed in db package"
problem_type: runtime_error
category: runtime_error
component: db_schema
root_cause: encoding_error
resolution_type: code_fix
severity: high
message: "bufio.Scanner 1MB limit fix applied to telemetry/ package but same pattern in db/telemetry_schema.go:RehydrateTelemetry was missed, leaving oversized JSONL lines silently dropped during SQLite rehydration"
file_path: "internal/db/telemetry_schema.go"
resolved: true
tags: [bufio-scanner, bufio-newreader, jsonl, oversized-lines, incomplete-fix, telemetry, rehydration, cross-package-search]
date: 2026-04-25
---

# bufio.Scanner fix for oversized JSONL lines missed in db package

## Problem

When fixing `bufio.NewScanner` → `bufio.NewReader + ReadString` to remove the
implicit 1 MB line limit when parsing JSONL in `internal/telemetry/parser.go`
and `internal/telemetry/harvest.go`, the **identical pattern in
`internal/db/telemetry_schema.go:RehydrateTelemetry` (lines 145–148) was
missed**. That function reads `telemetry-sessions.jsonl` to rebuild the SQLite
`telemetry_sessions` and `telemetry_tool_usage` tables. Any record whose
serialised JSON exceeds 1 MB was silently skipped, producing a rehydrated DB
that did not match the JSONL source of truth.

The fix was caught by the review gate Learnings Researcher persona — not by
the original implementer.

## Symptoms

- Partial data in `telemetry_sessions` / `telemetry_tool_usage` after `backlogit
  telemetry harvest` or `backlogit sync` when any JSONL line exceeds 1 MB
- No error message — `bufio.Scanner` silently drops oversized lines; only
  `scanner.Err()` at the end returns an error, and even that requires the
  explicit 1 MB buffer cap to be hit
- Discrepancy between what `backlogit telemetry list` reports (sourced from
  JSONL) and what `backlogit_query_sql` returns (sourced from SQLite)
- Bug missed during implementation because the search scope was limited to the
  `internal/telemetry/` package

## What Did Not Work

Fixing only the files that were immediately obvious (`parser.go`, `harvest.go`)
without searching the entire codebase for other call sites of `bufio.NewScanner`
applied to JSONL files. The rehydration path in `internal/db/` is a separate
package that reads the same file through independent I/O logic.

## Solution

### Before (`internal/db/telemetry_schema.go`)

```go
scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
for scanner.Scan() {
    line := scanner.Text()
    if line == "" {
        continue
    }
    // unmarshal and insert...
}
if err := scanner.Err(); err != nil {
    return fmt.Errorf("scan telemetry-sessions.jsonl: %w", err)
}
```

### After (`internal/db/telemetry_schema.go`)

```go
// Added import: "io"
reader := bufio.NewReader(f)
for {
    rawLine, readErr := reader.ReadString('\n')
    if readErr != nil && readErr != io.EOF {
        return fmt.Errorf("scan telemetry-sessions.jsonl: %w", readErr)
    }
    isEOF := readErr == io.EOF
    line := strings.TrimRight(rawLine, "\r\n")
    if line == "" {
        if isEOF {
            break
        }
        continue
    }
    // unmarshal and insert...
    if isEOF {
        break
    }
}
```

Note: `db/telemetry_schema.go` does not import `internal/errors`, so
`io.EOF == readErr` direct comparison is safe and idiomatic here (unlike
`internal/telemetry/harvest.go` which must avoid `errors.Is` due to the
internal/errors import shadowing the stdlib `errors` package).

## Why This Works

`bufio.NewReader` has no hard line-length limit — it grows its internal buffer
as needed. `bufio.NewScanner` with a fixed-size buffer drops any line that
exceeds the cap. `ReadString('\n')` returns the full line regardless of length,
with `io.EOF` as the termination signal for the last record (which may not end
in `\n`).

## Prevention

- **Always search the whole codebase** when fixing a pattern, not just the
  package that was the immediate focus.
  ```powershell
  Select-String -Path "**\*.go" -Pattern "bufio.NewScanner" -Recurse
  ```
- **Follow data flow**: for any JSONL file, trace every read site across all
  packages (telemetry/, db/, cli/, etc.) before claiming a fix is complete.
- **Add a cross-package test**: a test in `tests/integration/` that writes a
  >1 MB JSONL record, harvests it, then queries SQLite to confirm the record
  appears — this would catch the rehydration bug independently of the parser fix.

## Related Solutions

- See also `docs/compound/workflow-issues/` for the CLI Reference Drift Check
  failure that was also caught in the same review gate pass.
