---
type: circuit-breaker
timestamp: 2026-10-03T05:52:07Z
agent: "Ship"
skill: "post-merge closure"
breaker_type: universal
operation: "gh api commit-associated pull-request verification"
attempts: 3
identity: "github-rest-404-commit-pulls"
---

# Circuit Breaker - GitHub Commit PR Association Verification

## Failure Chain

### Attempt 1
- Exit/timeout: exit 1; HTTP 404
- Operation evidence: `gh api repos/softwaresalt/backlogit/commits/0130b3638531ad03fc491376752a7323da263b82/pulls`; cwd `C:\Source\GitHub\backlogit`; post-merge instruction verification; commit-association endpoint
- Normalized message: GitHub API returned `Not Found (HTTP 404)` for the commit-association request.
- Diagnostic artifact: none; bounded tool output only.

### Attempt 2
- Exit/timeout: exit 1; HTTP 404
- Operation evidence: `gh api repos/softwaresalt/backlogit/commits/05bd618646fa7cfc43a565c13301cd78850b2933/pulls`; cwd `C:\Source\GitHub\backlogit`; post-merge instruction verification; commit-association endpoint
- Normalized message: GitHub API returned the same `Not Found (HTTP 404)` response.
- Diagnostic artifact: none; bounded tool output only.

### Attempt 3
- Exit/timeout: exit 1; HTTP 404
- Operation evidence: `gh api repos/softwaresalt/backlogit/commits/0f365927f3e17f9e42702099603c6e9f8a8bb8f6/pulls`; cwd `C:\Source\GitHub\backlogit`; post-merge instruction verification; commit-association endpoint
- Normalized message: GitHub API returned the same `Not Found (HTTP 404)` response.
- Diagnostic artifact: none; bounded tool output only.

## Context
- Files involved: `.github/agents/_ship.agent.md`; `.github/skills/shipment-reconcile/SKILL.md`; `.github/skills/compact-context/SKILL.md`; `.github/skills/compound-refresh/SKILL.md`; `.github/skills/operational-closure/SKILL.md`; `.github/instructions/git-merge.instructions.md`; `.github/instructions/github-pr-automation.instructions.md`; `.github/instructions/backlogit.instructions.md`; `.github/instructions/markdown.instructions.md`
- Provisional-to-concrete identity link: none; all three observed failures returned the same HTTP status and endpoint-level error.
- Circuit-breaker execution miss: the PowerShell batch did not stop after the third non-zero `gh` exit. It issued requests for the remaining 45 distinct commit refs; all 48 returned HTTP 404 before the single batch process exited. The circuit opened at the third response. No further association requests were made after the batch.
- Logging controls: no raw capture; retained only the first three affected commit IDs and a redacted error summary.
- Initial halt disposition: Circuit breaker triggered after the third HTTP 404. The operation was not retried after its batch invocation.
- Initial suggested next steps: operator review of the VMR failure classification.
- Resolution: Overruled by Orchestrator Ruling #15: VMR was out of scope (authority artifacts only; E3 ended at the #471 merge). Breaker operation not retried.
