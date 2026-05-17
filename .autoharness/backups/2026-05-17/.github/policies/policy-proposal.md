---
policy_id: "{P-NNN}"
policy_type: "workspace-specific"
status: "proposed"
proposed_at: "{YYYY-MM-DD}"
evidence_count: {N}
---

# {Policy Title}

## Status: Proposed

This policy was auto-proposed by `auto-tune` based on recurring patterns in the compound
learning library. Review and decide: accept (copy to `.github/policies/` or append to
`workflow-policies.md`) or reject (delete this file with a note in the tuning report).

## Policy Definition

| Field | Value |
|---|---|
| **APPLIES_TO** | {agents/skills affected} |
| **GATE_POINT** | {Where in the pipeline this gate is enforced} |
| **PRECONDITION** | {What must be true before the gate} |
| **POSTCONDITION** | {What must be true after the gate} |
| **VIOLATION_ACTION** | {What happens when violated} |

## Rationale

{Why this policy is needed}

## Evidence

Derived from {N} compound learnings sharing the pattern `{pattern_key}`.

Evidence references:

{artifact references}

## Acceptance Path

To accept this policy:

1. Review the evidence references and validate the pattern is real and recurring.
2. Copy this file to `.github/policies/` or append a new entry to `.github/policies/workflow-policies.md`.
3. Update any agents or skills that should enforce this policy gate.
4. Run `autoharness verify-workspace` to confirm no dangling cross-references remain.
5. Record acceptance in the tuning report.

To reject: delete this file and note the rejection in the tuning report with a reason.
