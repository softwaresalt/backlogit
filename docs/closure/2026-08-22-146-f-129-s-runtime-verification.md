---
chunk_strategy: h1-h2-h3
description: "Runtime verification for the 146-F success-shaped evidence loss fixes (checkpoint context preservation, docs lint decode-failure containment), PR #373 / shipment 129-S."
doc_type: closure
docline:
    date: 2026-08-22T00:00:00Z
    status: accepted
    tags:
        - runtime-verification
        - checkpoint
        - docline
        - 146-F
        - 129-S
schema_version: "1.0"
source: docs/closure/2026-08-22-146-f-129-s-runtime-verification.md
title: "146-F / 129-S — Runtime Verification"
---

# Runtime Verification: 146-F success-shaped evidence loss (PR #373, shipment 129-S)

**Surface**: `cli` and `mcp` (backlogit CLI — `checkpoint create`, `docs lint`; MCP tools —
`backlogit_create_checkpoint`, `backlogit_docs_lint`). PR #373 changed both transports (the
CLI's own result path AND MCP-specific response shaping for `context_keys` and the
`decode_error`-carrying successful-result contract), so both must be exercised — CLI-only
verification is insufficient for this change's stated scope.
**Mode**: manual (CLI, local build + representative invocations) + automated (MCP, existing
in-tree regression tests dispatched through the registered tool handlers)
**Context**: PR #373 merged to `main` at `15ab30a2a394439f52e5338fc94d1c50e3f395ae`. Two governed
diagnostic-path defects were fixed:

1. `events.CreateCheckpoint` silently dropped every unmodeled `context` key and every
   unknown top-level key on the create round-trip.
2. `docline.LintTree` returned `nil, err` on the first per-file frontmatter decode
   failure, aborting the whole corpus scan with no findings printed.

## Step 1 — Verification Depth

Both defects are CLI/MCP-surface, deterministic, and fully exercisable locally without
external services, credentials, or network access. Manual CLI invocation against a local
build is sufficient; no browser, API host, or background job is involved.

## Step 2 — Environment Prechecks

* Build artifact: local build from this post-merge closure branch (`go build ./cmd/backlogit`),
  which descends from merge commit `15ab30a2a394439f52e5338fc94d1c50e3f395ae` (PR #373's
  merged tree, identical to its CI-covered head `de8efd694dbfb6909eae0f04161deee8592073e8`
  — the merge introduced no additional commits).
* No external dependencies, ports, or credentials required.
* `go test ./... -count=1` (full suite, all packages) was independently re-run locally
  during this closure session on the current tree and passed clean — every package `ok`,
  no failures. PR #373's own `test` CI check was also green at its head `de8efd69` prior
  to merge.

## Step 3 — Verification Mode

**Manual** CLI invocation of the built binary for the CLI transport; **automated in-process
MCP tool dispatch** (via `go test`, run fresh with `-count=1` to bypass the test cache) for
the MCP transport — the same technique the existing in-tree test suite uses to exercise the
registered MCP tool handlers end-to-end (`s.handleCreateCheckpoint` /
`s.handleDocsLint` through `mcplib.CallToolRequest`, never the underlying core functions
directly). Browser/API modes do not apply.

## Step 4 — Targets and Scenarios

* `backlogit checkpoint create --state-dump <dump-with-unmodeled-context-keys>` (CLI)
* `backlogit docs lint --path <corpus-with-one-malformed-frontmatter-file>` (CLI)
* `backlogit_create_checkpoint` MCP tool with a `context` object carrying a modeled key
  (`shipment_id`) and an unmodeled key (`pr_number`) (MCP)
* `backlogit_docs_lint` MCP tool against a corpus containing one file with malformed
  frontmatter (MCP)

## Step 5 — Execution and Evidence

### Scenario 1 — Checkpoint context key preservation (Defect 1)

Command:
```
backlogit checkpoint create --state-dump '{"schema_version":1,"agent":"ship","session_id":"rv-session-1","phase":"build","context":{"custom_unmodeled_key":"preserve-me","another_extra":42}}'
```

Expected: the response reports both unmodeled context keys in `context_keys`, and the
persisted checkpoint file preserves both keys verbatim under `context`.

Observed:
```json
{
  "context_keys": [
    "another_extra",
    "custom_unmodeled_key"
  ],
  "path": "C:\\Source\\GitHub\\backlogit\\.backlogit\\checkpoints\\checkpoint-20260823-022749.json"
}
```
Persisted file content (context object):
```json
"context":{"another_extra":42,"custom_unmodeled_key":"preserve-me"}
```
**PASS** — both previously-dropped keys are reported and durably persisted. Test artifact
removed after verification (not committed); the full command output is captured inline
above (Observed).

### Scenario 2 — Docs lint corpus-wide decode-failure containment (Defect 2)

Local scratch corpus: one well-formed doc (`good.md`, missing a required `source` field
on purpose to prove the scan reaches it) and one doc with malformed YAML frontmatter
(`bad-frontmatter.md`, an unterminated `[` list).

Command:
```
backlogit docs lint --path docs/scratch/rv-docline-corpus --format json
```

Expected: the malformed file surfaces as a `decode_error` finding (not a silent abort),
and the scan continues to lint `good.md`, producing its own finding too. Exit code
remains non-zero (a corpus containing a decode error is not a clean tree).

Observed:
```json
{
  "valid": false,
  "violation_count": 2,
  "findings": [
    {
      "file": "docs/scratch/rv-docline-corpus/bad-frontmatter.md",
      "rule": "decode_error",
      "severity": "error",
      "fix": "docline.decodeDoc: decode docs/scratch/rv-docline-corpus/bad-frontmatter.md: docline: frontmatter decode failed: mdfront.Decode: parse frontmatter: yaml: line 1: did not find expected ',' or ']'"
    },
    {
      "file": "docs/scratch/rv-docline-corpus/good.md",
      "field": "source",
      "rule": "required",
      "severity": "error",
      "fix": "required field \"source\" is missing or empty"
    }
  ]
}
```
Exit code: 1 (non-zero, expected — corpus is not clean).

**PASS** — the scan did not abort on the first decode failure; both files were linted and
both findings were surfaced. This is the exact regression the defect fix targets. Test
corpus removed after verification (not committed); the full command output is captured
inline above (Observed).

### Scenario 3 — MCP `backlogit_create_checkpoint` reports `context_keys` (Defect 1, MCP transport)

Command:
```
go test ./internal/mcp/... -run TestHandleCreateCheckpoint_ResultIncludesContextKeys -v -count=1
```

This test dispatches `backlogit_create_checkpoint` through the registered
`s.handleCreateCheckpoint` handler (the same `mcplib.CallToolRequest` path a real MCP
client uses — not `events.CreateCheckpoint` directly) with a `context` object carrying one
modeled key (`shipment_id`) and one unmodeled key (`pr_number`), and asserts the tool
result's `context_keys` array contains both.

Observed:
```
--- PASS: TestHandleCreateCheckpoint_ResultIncludesContextKeys (0.09s)
```
(full `go test` transcript reproduced verbatim above is the complete evidence; no
additional output beyond the pass line and standard `go test` framing was produced)

**PASS** — the MCP transport reports the same previously-dropped keys as the CLI transport
(Scenario 1), confirming the fix is not CLI-only.

### Scenario 4 — MCP `backlogit_docs_lint` returns `decode_error` in a successful result (Defect 2, MCP transport)

Command:
```
go test ./internal/mcp/... -run TestDocsLintTool_DegradedCorpus_SuccessfulResultNotInternalError -v -count=1
```

This test dispatches `backlogit_docs_lint` through the registered `s.handleDocsLint`
handler against a corpus containing one malformed-frontmatter file, and asserts (a) the
tool result is a **successful** result (`IsError: false`) rather than the pre-fix bare
`InternalError`, (b) the decoded `docline.LintReport` contains a `decode_error` finding for
the broken file, and (c) the MCP payload is byte-for-byte identical (after JSON
unmarshalling) to the underlying `docline.LintTree` result for the same corpus —
cross-surface parity with the CLI path.

Observed:
```
--- PASS: TestDocsLintTool_DegradedCorpus_SuccessfulResultNotInternalError (0.01s)
```

**PASS** — the MCP transport no longer returns a bare `InternalError` on a corpus decode
failure; it returns a successful result carrying the `decode_error` finding, matching the
CLI transport's behavior (Scenario 2).

## Step 6 — Verdict

**PASS** across all four scenarios (CLI and MCP transports, both defect fixes), reinforced
by a clean full-suite `go test ./... -count=1` run on the current tree (every package
reported `ok`, zero failures). No follow-up runtime risk identified for these two defect
fixes specifically.

## Step 7 — Handoff to Operational Closure

* Verification verdict: **PASS**
* Runtime surfaces verified: CLI (`checkpoint create`, `docs lint`) and MCP
  (`backlogit_create_checkpoint`, `backlogit_docs_lint`)
* Evidence: see Step 5 above (all command/test output reproduced inline in this report;
  no separate evidence files are retained per this workspace's ephemeral-scratch
  convention — `.github/instructions/context-efficiency.instructions.md`)
* Blocked prerequisites: none for this verification
* Risky action state: this verification touches the two `ActionRisk: high` surfaces this
  release changed — PA-8 (hand-written checkpoint codec on the shared read path) and PA-3
  (docs-lint CI-gate semantics) — and confirms both behave as the plan specified at the
  merged HEAD; see the closure artifact's Risky Action Record for the full approval/result
  history
* Follow-up recommendations: none new from runtime verification itself. The **unrelated**
  shipment-archival gate-evidence blocker for 146.006-T (stash `DD957688`) is a backlog/
  process concern, not a runtime regression, and is tracked separately in the closure
  artifact.
