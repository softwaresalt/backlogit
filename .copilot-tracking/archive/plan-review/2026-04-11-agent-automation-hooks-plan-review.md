---
title: "Plan Review: Agent-Automation Hooks for MCP Event Signals"
date: 2026-04-11
plan: "docs/exec-plans/2026-04-11-agent-automation-hooks-plan.md"
gate: fail
reviewers: [constitution-reviewer, go-quality-reviewer, architecture-strategist, scope-boundary-auditor]
---

## Gate Decision: FAIL

1 P0, 8 P1, 8 P2 findings after deduplication of 24 raw findings across 4 reviewers.
Plan must be revised to address P0 and P1 findings before proceeding to harvest.

## Summary

| Severity | Count | Action          |
|----------|-------|-----------------|
| P0       | 1     | Must fix        |
| P1       | 8     | Should fix      |
| P2       | 8     | Advisory        |
| P3       | 0     | —               |

Core themes: sequence counter race condition, negative-sequence/ack model conflict,
package layering violations, scope mismatch between claimed and delivered event types,
and YAGNI in config for deferred features.

## Findings

### P0: Critical (must fix before proceeding)

**MF1: Sequence counter race condition in multi-process scenarios**
Units: 1
Sequence derived from line count at writer startup. Two processes can cache the same
next-sequence, then append duplicate sequence numbers despite per-write file locking.
**Fix:** Allocate sequence under the cross-process lock from durable state (count lines
while holding lock on every append, not just at init).

### P1: High (should fix before proceeding)

**MF2: Negative sequence numbers break the ack model**
Units: 3, 6
Derived events use negative sequences mixed into the same poll/ack stream as persisted
events. Acking a negative sequence regresses checkpoints or makes through_seq semantics
undefined. Derived events intentionally reappear, but ack is designed to be monotonic.
**Fix:** Separate derived signals from persisted queue in poll response. Do not include
them in the ack stream. Return them in a separate response field or mark non-ackable.

**MF3: MCP tool schemas not strongly typed**
Units: 6
event_types as comma-separated string and raw array response weaken schema-first MCP
contract. No typed Go request/response structs defined.
**Fix:** Define typed request/response structs, make event_types a []string, validate at
handler boundary, derive contract tests from structs. Add visibility test for pre-init.

**MF4: No explicit 90%+ coverage gate**
Units: 1-6
Plan names unit tests and contract tests but does not make 90%+ coverage an acceptance
gate per the constitution's NON-NEGOTIABLE testing requirement.
**Fix:** Add coverage requirement to each unit's verification criteria.

**MF5: Consumer checkpoints stored in Git-tracked path**
Units: 2
hooks_consumers.jsonl in .backlogit/ causes constant diff churn and merge conflicts.
Consumer checkpoints are runtime state, not source of truth.
**Fix:** Store in .gitignore'd runtime path (e.g. .backlogit/runtime/) or mark ephemeral.

**MF6: Derived signals in reader couples events transport to SQLite**
Units: 3
HookEventReader queries SQLite for blocked_stale, coupling the transport layer to a
disposable cache. Poll results can change without source-of-truth changes.
**Fix:** Inject a narrow signal-provider interface into the reader rather than direct
SQLite dependency. Move computation to a core/hooks service or use dependency injection.

**MF7: Emitters in internal/events inverts package layering**
Units: 4
internal/events is currently low-level append infrastructure. Placing workflow-aware
emitters there makes it depend on 007-DL's lifecycle hook engine.
**Fix:** Create internal/hooks/ package for emitter registration and orchestration. Let
internal/events stay as primitive read/write storage.

**MF8: Scope mismatch — claims 5 event types, delivers 3**
Units: 4, overall
Plan traces to "five built-in event types" but only delivers 2 queued emitters plus 1
derived signal. stash_overflow and shipment_ready are deferred.
**Fix:** Explicitly reduce v1 scope to 3 supported signals throughout the plan. Update
requirements trace, in-scope list, and config defaults to match.

**MF9: Config adds fields for deferred events (YAGNI)**
Units: 5, 7
Default subscriptions include stash_overflow and shipment_ready. Config defines
thresholds for deferred events. Agents would poll for signals that can never arrive.
**Fix:** Trim v1 config to implemented signals only. Add deferred config in follow-up.

### P2: Moderate (advisory)

**MF10: Untyped maps and missing boundary validation**
Units: 1, 5, 6
HookEvent.Payload is map[string]any. AgentSubscriptions lacks validation for unknown
event names. No checkpoint regression rejection.
Advisory: Consider typed payload structs per event type and validation tags.

**MF11: Missing structured logging requirements**
Units: 1-6
No explicit slog requirements for emit/poll/ack paths, lock recovery, or post-hook
write failures. Fire-and-forget post-hooks risk silent failure.
Advisory: Add slog requirements for observable outcomes in each unit.

**MF12: Reader API has too many string parameters**
Units: 3
PollHookEvents takes 5 params including 3 strings. Invites argument-order bugs.
Advisory: Use a reader type with constructor-injected paths and a typed filter struct.

**MF13: Missing sentinel error mapping**
Units: 1-4, 6
No specification for how hook/checkpoint failures map to ErrConfig/ErrValidation/ErrMCP.
Advisory: Define error contracts up front per the existing sentinel hierarchy.

**MF14: Missing failure mode tests**
Units: 1-3, 6
No tests for truncated JSONL, stale lock recovery, concurrent poll+ack, ack regression,
restart-resume, or multi-process scenarios.
Advisory: Add table-driven tests for these failure modes.

**MF15: Full log scan per poll**
Units: 1-3
Every consumer poll scans the entire hooks_queue.jsonl. Linear growth in latency.
Advisory: Use byte-offset checkpoints or segmented queues if volume grows.

**MF16: Dependency understatement for Unit 6**
Units: 4, 6
Unit 6 can ship without Unit 4, but then MCP tools have no real producer.
Advisory: Gate MCP tool exposure on at least one producer being wired.

**MF17: Units 4 and 6 may be underestimated**
Units: 4, 6
Unit 4 depends on unfinished 007-DL contracts. Unit 6 includes two tools, handlers,
server wiring, and contract tests.
Advisory: Consider splitting during execution if either exceeds 2-hour budget.

## Reviewer Attribution

| Finding | Reviewer(s)                                                      | Model(s)                    |
|---------|------------------------------------------------------------------|-----------------------------|
| MF1     | Constitution, Go Quality, Scope Boundary                        | claude-opus-4.6, gpt-5.4   |
| MF2     | Constitution, Go Quality, Scope Boundary                        | claude-opus-4.6, gpt-5.4   |
| MF3     | Constitution                                                     | claude-opus-4.6             |
| MF4     | Constitution                                                     | claude-opus-4.6             |
| MF5     | Constitution                                                     | claude-opus-4.6             |
| MF6     | Architecture Strategist                                          | gpt-5.4                     |
| MF7     | Architecture Strategist                                          | gpt-5.4                     |
| MF8     | Scope Boundary                                                   | gpt-5.4                     |
| MF9     | Constitution, Scope Boundary                                     | claude-opus-4.6, gpt-5.4   |
| MF10    | Constitution                                                     | claude-opus-4.6             |
| MF11    | Constitution                                                     | claude-opus-4.6             |
| MF12    | Go Quality                                                       | claude-opus-4.6             |
| MF13    | Go Quality                                                       | claude-opus-4.6             |
| MF14    | Go Quality, Scope Boundary                                       | claude-opus-4.6, gpt-5.4   |
| MF15    | Architecture Strategist                                          | gpt-5.4                     |
| MF16    | Architecture Strategist                                          | gpt-5.4                     |
| MF17    | Scope Boundary                                                   | gpt-5.4                     |

## Next Steps

Plan must be revised to address MF1-MF9 before proceeding to harvest.
