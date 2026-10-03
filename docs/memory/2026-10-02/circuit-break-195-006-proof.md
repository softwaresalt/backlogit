---
type: circuit-breaker
timestamp: 2026-10-02T21:46:00Z
agent: "Orchestrator"
skill: "build-feature (Ship proof protocol, plan 2A355F83)"
breaker_type: skill-managed
operation: "195.006-T claim-start proof run"
attempts: 5
identity: "195.006-T-proof-driver"
---

# Circuit Breaker - 195.006-T claim-start proof run

Held in `logs/carry-forward/` because a tracked memory write would enter the
open 195.006-T verification-only exempt delta. Re-apply to
`docs/memory/2026-10-02/` at the wave-3 closure commit.

## Failure Chain

All five aborts are PowerShell proof-driver defects. None is a backlogit
product defect or a plan defect. Step-2 containment matched the baseline on
every abort (status unchanged; 4,072-entry SHA-256 manifest equal).

| # | Run | Step | Token | Root cause |
|---|---|---|---|---|
| 1 | `195.006-T-20261002T203955Z` | 6 | PROOF_PRECHECK_FAILED | `$args` automatic-variable shadowing |
| 2 | `195.006-T-20261002T204945Z` | 1 | PROOF_PRECHECK_FAILED | `git check-ignore` output misparsed (false negative) |
| 3 | `195.006-T-20261002T205205Z` | 4 | PROOF_PRECHECK_FAILED | PowerShell ParserError (missing `)`) |
| 4 | `195.006-T-20261002T213757Z` | 4 | PROOF_PRECHECK_FAILED | Result recorder rejected empty `git clone` stdout |
| 5 | `195.006-T-20261002T214202Z` | 5 | PROOF_PRECHECK_FAILED | `list --format json` `[]` unrolled to `$null`, failed parameter binding |

Error identities are distinct, so the universal same-error breaker did not
trip; the build-feature per-task limit (5) is exhausted.

## Context

- Files involved: none tracked; all run roots retained under ignored `logs/2a355f83-proof/`.
- Branch HEAD: `9f8d77569f254a0aae7b012e8a858a4f7cb36f68`; tree clean.
- 195.006-T: `active` with one start record (Ship cannot move to `blocked` under its role boundary).
- 195.007-T: claim-assigned, unstarted. No PR; no merge.
- Checkpoints set aside in `logs/carry-forward/checkpoints/` (hashes recorded on 195.006-T).
- Resolution: Circuit breaker triggered. Awaiting operator guidance.
- Suggested next step: operator-approved fresh 5-attempt budget conditioned on an
  end-to-end driver dry run (all steps 1-8, including empty-stdout and empty-list
  cases) in a scratch fixture before any counted live attempt.
