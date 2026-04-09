---
title: "Plan Review: Token and Context Window Efficiency Telemetry"
date: 2026-04-09
plan: "docs/exec-plans/2026-04-09-token-telemetry-plan.md"
gate: fail
reviewers: [constitution-reviewer, go-quality-reviewer, architecture-strategist, scope-boundary-auditor]
---

## Gate Decision: FAIL

1 P0, 6 P1, 7 P2 findings after deduplication. Plan must be revised before
proceeding to harvest.

## Summary

21 raw findings across 4 personas reduced to 14 unique findings after
deduplication. Key themes:

* Workspace containment boundary violation (reading `.copilot/` from backlogit)
* CQRS architecture tension with dual write paths and mutable checkpoint state
* Session-store.db dropped as a data source despite being in the origin doc
* Type-safety and schema collision in telemetry JSONL storage
* Parser interface too narrow for streaming and future adapters
* Verification criteria missing edge cases and error scenarios

## Findings

### P0: Critical (must fix before proceeding)

**F1: Workspace containment violation**
*Sources: CR-1*
*Units: 1, 2, 6, 7, 8*

Principle IV requires all file-system operations to resolve within `.backlogit/`.
The plan reads from `.copilot/logs/`, `.copilot/session-state/`, and
`.copilot/session-store.db`, which are outside the containment boundary. The
Standards Check incorrectly marks Principle IV as compliant.

**Recommendation:** Either amend the constitution to allow read-only access to
workspace-scoped external data (`.copilot/` via `COPILOT_HOME`), or add an
explicit import/promote step that copies telemetry data into `.backlogit/`
before processing. A constitution amendment is the cleaner path since `.copilot/`
is workspace-scoped and read-only access poses no mutation risk.

### P1: High (should fix before proceeding)

**F2: Checkpoint file violates CQRS and is YAGNI for v1**
*Sources: CR-2, SB-2*
*Units: 6, 7*

The `.backlogit/telemetry-checkpoint.json` introduces mutable operational state
that fits none of the constitution's four storage layers (Markdown, SQLite,
append-only JSONL, transient JSONL queues). It also represents premature
optimization: checkpoint/offset tracking, force mode, advisory locking, and
re-harvest semantics are infrastructure that should be deferred until full
re-harvest is proven too slow.

**Recommendation:** Defer R10 for v1. Ship with idempotent harvest-all or a
simple timestamp-based watermark stored as a JSONL entry. Add file-offset
checkpoints only after real-world performance validates the need.

**F3: Session-store.db dropped as data source**
*Sources: GQ-1, SB-1*
*Units: 2, 4, 5*

The origin doc identifies `.copilot/session-store.db` as one of three primary
data sources, yet no implementation unit reads it. Session metadata (branch,
repository, timing) is partially substituted by `session.start` event parsing,
but this leaves `tokens_per_task` time-overlap attribution underdefined without
reliable session boundaries.

**Recommendation:** Add a focused unit or expand Unit 2 to read session metadata
from `session-store.db`. Alternatively, explicitly reduce v1 scope to
session-level metrics and defer per-task attribution until session boundaries
are reliably sourced.

**F4: Telemetry JSONL schema collision and type-safety violation**
*Sources: GQ-2, AS-2*
*Units: 5, 6*

Reusing `events.TelemetryWriter` (which persists `map[string]any` payloads)
for harvested session summaries creates two problems: schema collision with
existing agent telemetry entries, and type-safety violation (Principle I
prohibits `any` in public API without justification).

**Recommendation:** Define typed `SessionSummaryRecord` and `ToolUsageRecord`
structs. Write to a dedicated `.backlogit/telemetry-sessions.jsonl` file with
a purpose-built writer, keeping existing `telemetry.jsonl` for generic agent
telemetry.

**F5: Dual write path and hidden dependency cycle (Units 5-6)**
*Sources: GQ-3, AS-1*
*Units: 5, 6*

Unit 6 upserts directly into SQLite during harvest, while Unit 5 introduces
`RehydrateTelemetry` to rebuild from JSONL. This creates two competing write
paths for the same read model and a hidden dependency cycle. The persisted JSONL
schema is never defined as a concrete type before these units split.

**Recommendation:** Single authority path: harvest writes only to the durable
JSONL stream, then rehydration rebuilds SQLite from that stream. Promote a
concrete `SessionSummaryRecord` type into Unit 4 and have both JSONL persistence
and SQLite rehydration consume it.

**F6: Parser interface too narrow for streaming and scale**
*Sources: AS-3*
*Units: 1, 6*

`LogParser.Parse(r io.Reader) ([]ModelCall, []ToolCall, error)` forces full
in-memory accumulation of 50MB+ files, assumes only two event types, and does
not support incremental checkpoint-aware parsing.

**Recommendation:** Move to a streaming/callback/iterator boundary where the
parser emits normalized events incrementally. Keep host-specific parsing behind
the adapter but make the core consume events without full-file accumulation.

**F7: Tool usage table lacks uniqueness constraint**
*Sources: AS-4*
*Units: 5*

`telemetry_tool_usage` uses `AUTOINCREMENT` with no natural key. Repeated
harvests or partial rebuilds duplicate rows for the same
`(session_id, server_name, tool_name)` tuple, breaking idempotency claims.

**Recommendation:** Use `PRIMARY KEY (session_id, server_name, tool_name)` or
an explicit summary grain key. Design the schema for deterministic rebuilds.

### P2: Moderate (user discretion)

**F8: MCP tool contract testing gaps**
*Sources: CR-3, GQ-5*
*Units: 8*

Missing tests: unconditional tool visibility before init, pre-init descriptive
error, and real handler invocation through the server. Unit 8 says "validates
required params" but the tool has no required params.

**Recommendation:** Specify contract tests following existing `tests/contract/`
patterns: `ListTools()` visibility, optional param acceptance, success payload
shape, and workspace/`.copilot/` missing error responses.

**F9: Correlator coupled to backlogit log internals**
*Sources: AS-5, SB-3*
*Units: 4*

The correlator directly scans `.backlogit/logs/*.jsonl` for `status_changed`
events, coupling the telemetry domain to backlogit's internal log layout. The
origin doc specifies `.backlogit/events.jsonl` as the normalization source.

**Recommendation:** Keep v1 normalization on `.backlogit/events.jsonl`. Put
task-completion extraction behind a backlogit-owned adapter rather than having
the telemetry package parse backlog logs directly.

**F10: GoDoc and lint not in unit exit criteria**
*Sources: CR-4*
*Units: 1-8*

The constitution requires GoDoc on all exports and clean lint/vet. No unit
includes these in verification criteria.

**Recommendation:** Add `gofmt -l .`, `go vet ./...`, `golangci-lint run` to
final verification. Require GoDoc on all exported symbols.

**F11: No telemetry-specific sentinel errors**
*Sources: GQ-4*
*Units: 1, 2, 5, 6, 7, 8*

The plan does not define sentinel errors for parse failures, missing sources,
checkpoint corruption, or harvest/rehydration failures.

**Recommendation:** Add telemetry error types to `internal/errors/errors.go`
with `errors.Is`-based mapping in CLI/MCP entrypoints.

**F12: YAML attribution override is premature**
*Sources: SB-4*
*Units: 3*

`.backlogit/telemetry-attribution.yaml` adds config loading and override
precedence before default mappings are validated.

**Recommendation:** Hardcode the observed registry for v1. Defer YAML overrides.

**F13: Report surface scope drift**
*Sources: SB-5*
*Units: 7*

`report` adds `--format table|json` (not requested) while dropping `--by tool`
(requested in origin doc).

**Recommendation:** JSON-only for v1. Preserve all grouping axes from the
origin doc.

**F14: Verification criteria missing edge-case tests**
*Sources: SB-6, AS-6*
*Units: 1, 2, 4, 6, 7, 8*

Missing: corrupt/truncated JSON, empty sessions, zero tool calls, zero model
calls, no compaction events, corrupt checkpoint files, partial `.copilot/`
directories. Dependency graph has hidden dependencies.

**Recommendation:** Expand each unit's verification with explicit negative and
edge-path tests. Add checkpoint/lock design and task-completion source as
explicit Unit 6 dependencies.

## Reviewer Attribution

| Finding | Reviewer(s)              | Model(s)             |
|---------|--------------------------|----------------------|
| F1      | Constitution Reviewer    | Claude Opus 4.6      |
| F2      | Constitution, Scope      | Claude Opus, GPT-5.4 |
| F3      | Go Quality, Scope        | Claude Opus, GPT-5.4 |
| F4      | Go Quality, Architecture | Claude Opus, GPT-5.4 |
| F5      | Go Quality, Architecture | Claude Opus, GPT-5.4 |
| F6      | Architecture Strategist  | GPT-5.4              |
| F7      | Architecture Strategist  | GPT-5.4              |
| F8      | Constitution, Go Quality | Claude Opus 4.6      |
| F9      | Architecture, Scope      | GPT-5.4              |
| F10     | Constitution Reviewer    | Claude Opus 4.6      |
| F11     | Go Quality Reviewer      | Claude Opus 4.6      |
| F12     | Scope Boundary Auditor   | GPT-5.4              |
| F13     | Scope Boundary Auditor   | GPT-5.4              |
| F14     | Scope, Architecture      | GPT-5.4              |

## Next Steps

Plan must be revised to address all P0 and P1 findings before proceeding to
harvest. P2 findings are at user discretion but recommended for plan quality.

Key revisions needed:

1. Address F1 (P0): Amend constitution or add import step for `.copilot/` reads
2. Address F2 (P1): Defer checkpoint system; use simple harvest-all for v1
3. Address F3 (P1): Add session-store.db reader or reduce scope
4. Address F4 (P1): Dedicated typed JSONL stream for harvested telemetry
5. Address F5 (P1): Single write path (JSONL → rehydrate → SQLite)
6. Address F6 (P1): Streaming parser interface
7. Address F7 (P1): Composite primary key for tool usage table
