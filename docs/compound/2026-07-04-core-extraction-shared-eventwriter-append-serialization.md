---
chunk_strategy: h1-h2-h3
description: 'One durable rule graduated from 079-S — when extracting an MCP write handler into a shared core function that both a long-lived MCP server and a one-shot CLI reuse, thread the caller''s *events.EventWriter through the function (MCP passes the server''s shared s.Events; CLI passes nil for a per-invocation writer). Minting a fresh EventWriter inside the core function on every call drops the server''s per-item JSONL append serialization, because EventWriter.mu only serializes callers that share one instance.'
doc_type: learning
docline:
    date: 2026-07-04T00:00:00Z
    severity: high
    tags:
        - core
        - mcp
        - cli
        - events
        - concurrency
        - append-serialization
        - eventwriter
        - refactor
        - parity
schema_version: "1.0"
source: docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md
title: 'Thread the shared EventWriter through a core extraction so a long-lived MCP server keeps append serialization while a one-shot CLI reuses the path (079-S)'
---

# Core Extraction Must Thread the Shared EventWriter

One durable rule graduated from shipment 079-S (feature 079-F, "CLI/MCP command
parity phase-2", PR #172, merge `a8e07ea38f8e153e9a29def264538bcab8222868`). This is
the *concurrency-correctness* sibling of the command-level parity rules in
`2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`: closing a
CLI/MCP parity gap by **extracting a shared core function** is only correct if the
extraction preserves the write-serialization guarantee the MCP server relied on.

## Rule — Thread the caller's `*events.EventWriter`; never mint a fresh one inside the shared core function

### Problem

To give `append_comment` a CLI fallback (U4), the write path was extracted into a
shared `core.AppendComment` that both surfaces call. The first extraction minted a
**fresh** `events.EventWriter` inside `AppendComment` on every call. That silently
broke the MCP server's append serialization:

- `events.EventWriter` guards its append with a mutex (`EventWriter.mu`, `stream.go`).
- That mutex only serializes callers that **share one** `EventWriter` instance.
- The MCP server holds a single long-lived shared writer (`Server.Events`,
  `server.go`) and routes all event appends through it — see the established
  `handleMoveItem` pattern (`tools.go`), which uses `s.Events.AppendEvent` +
  `IndexEvent`.
- A core function that constructs its own writer per call means each concurrent MCP
  `append_comment` gets its **own** mutex — no cross-call serialization — so
  concurrent appends can interleave and corrupt the per-item JSONL event stream.

Copilot flagged this on the first review round; it is a real defect, not a style nit.

### Rule

A shared `core` write function that backs **both** a long-lived server and a
one-shot CLI MUST accept the caller's writer:

```go
// core
func AppendComment(..., ew *events.EventWriter) error {
    if ew == nil {
        ew = events.NewEventWriter(events.WorkspaceLogsRoot(rootPath)) // one-shot
    }
    // ... use ew.AppendEvent / IndexEvent ...
}
```

- **MCP handler** passes the server's shared instance: `handleAppendComment` →
  `core.AppendComment(..., s.Events)`. All concurrent MCP calls now share one mutex.
- **CLI command** passes `nil`: `newCommentAddCmd` → `core.AppendComment(..., nil)`.
  A one-shot process has no concurrency to serialize, so a per-invocation writer is
  correct and cheap.

### Why it works

The serialization guarantee lives in the *instance*, not the function. Threading the
writer lets the long-lived caller keep exactly one instance (preserving the mutex's
cross-call serialization) while the short-lived caller gets a fresh instance for
free. The nil-guard keeps the CLI call site clean without leaking server lifecycle
concerns into `core`.

### Guardrails / gotchas

- **Preserve pre-extraction indexing semantics.** `core.AppendComment` intentionally
  keeps its zero-`Timestamp` indexing behavior — do **not** "fix" it to mirror
  `LinkCommit`'s timestamp during the extraction. Behavior-preserving means
  byte-for-byte behavior, including quirks.
- **Mirror the reference pattern, not a fresh design.** `handleMoveItem`
  (`s.Events.AppendEvent` + `IndexEvent` with `WorkspaceLogsRoot(RootPath)`) is the
  in-repo canonical shared-writer path; extractions should match it.
- **Generalizes** to any core extraction that backs a long-lived server + a one-shot
  CLI where the server depends on a shared, mutex-guarded singleton (writers, index
  handles, connection pools): thread the singleton, nil-guard for the one-shot.

## References

- PR #172 — feat: CLI/MCP command parity phase-2 (079-S / 079-F)
- Merge commit `a8e07ea38f8e153e9a29def264538bcab8222868`
- Fix commit `6257fab` — fix(core): thread shared EventWriter through AppendComment
- `internal/core/commits.go` — `AppendComment(..., ew *events.EventWriter)` (nil-guarded)
- `internal/mcp/tools.go` — `handleAppendComment` (passes `s.Events`); `handleMoveItem` (reference pattern)
- `internal/mcp/server.go` — `Server.Events` (shared long-lived writer)
- `internal/events/stream.go` — `EventWriter.mu` (instance-scoped append mutex)
- `internal/cli/comment.go` — `newCommentAddCmd` (passes `nil`)
- Test: `internal/mcp/append_comment_test.go`
- Runtime verification: `docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-runtime-verification.md`
- Related (complementary, kept): `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`
