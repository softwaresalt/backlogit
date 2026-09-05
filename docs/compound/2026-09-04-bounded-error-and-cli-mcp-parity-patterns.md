---
chunk_strategy: h1-h2-h3
description: Shared input validation and bounded error response patterns for CLI/MCP surface parity
doc_type: learning
docline:
    date: 2026-09-04T00:00:00Z
    severity: high
    tags:
        - mcp
        - cli
        - error-handling
        - parity
        - bounded-output
        - s3
source: docs/closure/2026-09-04-137s-post-merge-closure.md
title: S3 — Bounded error projection and CLI/MCP shared validation patterns
---

# S3 — Bounded Error Projection and CLI/MCP Shared Validation Patterns

## Context

Shipped in 137-S (S3). Resolves two parity gaps: (1) unbounded checkpoint unknown-field
error responses on CLI and MCP, and (2) absent MCP equivalent for acklogit docs classify.

## Key Patterns Learned

### 1. Neutral Leaf Ownership for Shared Structured-Error DTOs

**Problem**: A structured error DTO used by BOTH CLI and MCP must live in a package that
neither imports. internal/errors is the correct location because CLI imports MCP
(oot.go:22) and MCP cannot import CLI, making internal/errors the only cycle-free leaf.

**Pattern**: Put shared bounded-projection types and their constructors in internal/errors.
Both surfaces consume them; neither defines its own projection. Never put the DTO in
internal/mcp (CLI → MCP imports) or internal/cli/format (MCP → CLI would be a cycle).

### 2. Single Quoting/Escaping Site for BoundedFieldPathSet

**Problem**: Multiple Error() and FieldPathsForDisplay() methods duplicated the
same strconv.Quote + join + truncation-clause rendering of a BoundedFieldPathSet.

**Pattern**: Extract ormatBoundedFieldSet(BoundedFieldPathSet) string as an unexported
package-level helper. Every human-readable render delegates to it. The doc comment on the
helper is "the single quoting/escaping site" so no future method re-invents it.

### 3. rrors.As in Execute() for Structured CLI JSON-RPC Data

**Pattern**: In internal/cli/root.go Execute(), before the generic ormat.WrapError,
add rrors.As checks for typed errors that need structured data. The caller must
%w-wrap the error at every intermediate call site (not %v). ormat.WrapErrorData
takes ny data with json:"data,omitempty", and a reflect-based typed-nil guard
prevents data:null when a typed-nil interface is passed.

**Gotcha**: omitempty on an ny field omits a nil interface but NOT a non-nil
interface wrapping a typed-nil pointer/map. Use eflect.ValueOf(data).Kind() + IsNil()
to normalize before marshaling.

### 4. Shared Input Validation Helper for New CLI/MCP Surface Pairs

**Pattern**: When adding a new operation that exists on both surfaces, create a shared
validation helper (e.g., docline.ValidateClassifyPath) called by BOTH CLI and MCP handlers
BEFORE delegating to the pure function. The helper enforces the full input contract
explicitly — core.SafeResolve only validates the joined result and does NOT reject
empty, absolute, volume/UNC, or dot-segment raw path inputs.

**Dot-segment gotcha**: core.SafeResolve accepts in-root dot-segment paths (e.g.
docs/decisions/../reviews/x.md) since the joined result does not escape. Reject
"." and ".." segments explicitly to prevent lexical path misclassification.

### 5. CLI JSON Output for --jsonrpc Shape Parity

**Pattern**: CLI commands consumed by agents via --jsonrpc should emit a JSON
object (e.g., {"doc_type":"decision"}) via writeDocsJSON, NOT plain text. Plain-text
output cannot be parsed by PersistentPostRunE's json.Unmarshal fallback; it wraps
the output as a string scalar ({"result":"decision"}) rather than the structured shape
({"result":{"doc_type":"decision"}}) that agents expect when accessing esult.doc_type.

### 6. Governed Registry Markers Require Authoritative-Registry-Dispatching Fixtures

**Reminder** (compound 2026-08-15): A behavioral fixture for a governed:true registry
operation must invoke the REGISTERED handler (via in-process MCP or real CLI dispatch),
not the underlying core function directly. Use callToolForTest(t, s, mcpTool, args) and
cli.NewRootCommand() + .SetArgs(...) + .Execute(). When both surfaces use the same
workspace, use separate 	.TempDir() instances to prevent second-precision filename
collisions in checkpoint-create fixtures.
