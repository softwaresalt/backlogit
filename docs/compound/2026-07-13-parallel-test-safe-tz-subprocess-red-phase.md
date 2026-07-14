---
title: "Parallel-test-safe RED phase for timezone-dependent emission: hermetic TZ subprocess re-exec instead of a global time.Local override"
source: docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md
doc_type: learning
description: "To prove (RED phase) that a local-offset timestamp emission fails even on a UTC CI runner, a test must run the write under a controlled non-UTC zone. Overriding the process-global time.Local is a data race in any package that runs t.Parallel() tests (notably internal/cli). The safe alternative is a hermetic subprocess: re-exec the test binary targeting a helper test, pass TZ=America/Los_Angeles via the child env, have the child emit the serialized timestamp on stdout, and assert in the parent that it ends with exactly Z. Serial-test packages may use a time.Local override with defer-restore, but parallel packages must use the subprocess variant."
docline:
    date: 2026-07-13T00:00:00Z
    severity: medium
    tags:
        - testing
        - go
        - t-parallel
        - data-race
        - timezone
        - subprocess
        - hermetic
        - tdd
        - red-phase
---

# Parallel-Test-Safe RED Phase for Timezone-Dependent Emission

## Context

Surfaced by shipment 092-S (feature 103-F, PR #235, merge `4a90bf4`) while
writing TDD RED tests that prove item-artifact writers emit `created_at` /
`updated_at` in canonical UTC (`Z`). The RED assertion is only meaningful if the
write runs under a **non-UTC** zone: on a UTC CI runner, a buggy local-offset
writer and a correct UTC writer produce the *same* bytes, so the test would pass
against the bug. The tests must therefore inject a controlled non-UTC zone.
(Accepted as an explicit test-design caveat carried in from the PR #234 review.)

## Problem

The obvious way to force a non-UTC zone in Go is to set the process-global
`time.Local`:

```go
orig := time.Local
time.Local = time.FixedZone("PST", -8*3600) // or LoadLocation("America/Los_Angeles")
defer func() { time.Local = orig }()
```

`time.Local` is **process-global mutable state**. In any package that runs
`t.Parallel()` tests — notably `internal/cli` (task 103.010-T) — other parallel
tests read `time.Local` concurrently, so this mutation is a **data race**: it
corrupts sibling tests and trips `go test -race`. The `defer`-restore does not
help, because the window between set and restore overlaps concurrently-running
tests. So the exact zone control the RED phase needs is unsafe precisely in the
package that most needs it.

## Solution

Split by test-execution model:

* **Serial-test packages** (no `t.Parallel()` in the package): a scoped
  `time.Local` override with `defer`-restore is acceptable — there are no
  concurrent readers.

* **Parallel-test packages** (`internal/cli` here): drive the write in a
  **hermetic subprocess** and pass the zone through the child's environment.
  Re-exec the test binary at a helper test and read a marker line off stdout:

  ```go
  func TestUpdateCommand_SectionWrite_EmitsUTCUpdatedAt(t *testing.T) {
      t.Parallel()
      cmd := exec.Command(os.Args[0], "-test.run=^TestHelperUpdateSectionUTCChild$", "-test.v=true")
      cmd.Env = append(os.Environ(), "BACKLOGIT_UTC_HELPER=1", "TZ=America/Los_Angeles")
      out, err := cmd.CombinedOutput()
      require.NoErrorf(t, err, "hermetic subprocess failed:\n%s", out)
      // parse "UTC_UPDATED_AT=<value>" from out ...
      assert.True(t, strings.HasSuffix(value, "Z"), "updated_at must end with exactly Z, got %q", value)
      assert.False(t, zoneOffsetRE.MatchString(value), "must not carry a numeric zone offset, got %q", value)
  }
  ```

  The child test guards on the env sentinel, does the real write in
  `TZ=America/Los_Angeles`, and prints `UTC_UPDATED_AT=<serialized>` on stdout;
  the parent asserts on that value. The zone lives **only** in the child process,
  so the parent package's parallel tests are never touched. This is the standard
  Go "helper subprocess" pattern (as used by `os/exec`'s own tests), applied to
  timezone hermeticity. `TZ` is honored by Go's `time` package at process start
  on Unix CI runners; the child re-reads it fresh.

## Applicability

Any Go test that must exercise behavior under a specific timezone (or other
process-global env/state) inside a package that also runs `t.Parallel()`. Rule of
thumb: **never mutate `time.Local` (or any process-global) in a parallel-test
package** — push the environment-sensitive execution into a subprocess and assert
on its output. Prefer `TZ=` in the child env over `time.Local =` in-process; it
is race-free and closer to how the binary actually runs in production. Pair this
with asserting the *exact* canonical form (trailing `Z`), not a semantically
equal zero offset (`+00:00`) — see
`docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md`.
