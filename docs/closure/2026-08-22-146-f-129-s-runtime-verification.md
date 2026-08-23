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

**Surface**: `cli` (backlogit CLI — `checkpoint create`, `docs lint`)
**Mode**: manual (local build + representative invocations)
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

* Build artifact: local build from post-merge `main` (`go build ./cmd/backlogit`),
  commit `15ab30a2a394439f52e5338fc94d1c50e3f395ae` — matches the merged PR #373 HEAD.
* No external dependencies, ports, or credentials required.
* `go test ./...` (full suite) already green on this commit per PR #373's `test` CI check.

## Step 3 — Verification Mode

**Manual** CLI invocation of the built binary — the fastest, most direct verification for
CLI-surfaced behavior; browser/API modes do not apply.

## Step 4 — Targets and Scenarios

* `backlogit checkpoint create --state-dump <dump-with-unmodeled-context-keys>`
* `backlogit docs lint --path <corpus-with-one-malformed-frontmatter-file>`

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
removed after verification (not committed).

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

Observed (`docs/scratch/rv-docs-lint-output.txt`):
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
corpus removed after verification (not committed); the raw command output log is retained
at `docs/scratch/rv-docs-lint-output.txt` and `docs/scratch/rv-checkpoint-create-output.txt`
as evidence.

## Step 6 — Verdict

**PASS** for both scenarios. No follow-up runtime risk identified for these two defect
fixes specifically.

## Step 7 — Handoff to Operational Closure

* Verification verdict: **PASS**
* Runtime surfaces verified: CLI (`checkpoint create`, `docs lint`)
* Evidence: see Step 5 above and `docs/scratch/rv-*.txt`
* Blocked prerequisites: none for this verification
* Risky action state: none — this verification is read/write to local scratch state only,
  no destructive or production-impacting action was taken
* Follow-up recommendations: none new from runtime verification itself. The **unrelated**
  shipment-archival gate-evidence blocker for 146.006-T (stash `DD957688`) is a backlog/
  process concern, not a runtime regression, and is tracked separately in the closure
  artifact.
