---
title: "PR 645 halted after Git became unavailable during Ordinal 21"
doc_type: reference
source: "docs/memory/2026-11-10/212-S-pr645-git-tool-unavailable-checkpoint.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: "212-S"
feature_id: "211-F"
phase: "step5-blocked-git-unavailable"
---

## Halt

TOOL_UNAVAILABLE: Git is unavailable on PATH and at the standard installed
locations. It was available when Ordinal 21 began, but later tests and the
recorder's final HEAD lookup could not execute it. The cause is unestablished.
No installation, external filesystem repair, push workaround or automatic
retry was attempted. GitHub CLI and backlogit MCP remain reachable.

The operator requested Step 5 CI/Copilot completion, not a merge.
The requested all-pass state was not reached. No merge is approved or executed.

## Local and remote state

* Open PR: <https://github.com/softwaresalt/backlogit/pull/645>
* Independently queried remote HEAD:
  `c574669b0a016040a5ffb0582e2482a6b505289d`
* Last verified local commit, not pushed:
  `74c09baeea828231ccbae811100bc4259f4f2335`
* Reviewed local tree:
  `05fe49f07ae86f9349ae1b2f329f8e89f8228c5b`
* Post-Ordinal-21 local HEAD and Git worktree status cannot be certified
  because Git is missing. Do not silently substitute filesystem inference
* Existing feature branch/worktree retained; no checkout, reset or stash

All 22 frozen task members remain complete; no lifecycle replay occurred.
Shipment 212-S remains active pending delivery and post-merge closure.

## Completed C1 remediation

1. `62064e89`: portable U4/U5/U6 relative-root and socket-length fixtures.
2. `c574669b`: inherited Linux pipe/socket input cancellation so the shim
   can join workers after peer EOF or cancellation.
3. `74c09bae`: test-only private stdout duplication so the owned shim
   output does not close Go coverage's global stdout descriptor.

The third correction was reproduced with coverage-only atomic RED and
GREEN; all six pipe/socket cases passed ten repetitions after the fix.
Local race compilation is unavailable because WSL lacks a C compiler;
hosted race verification remains required, not waived. No new tool was
installed. Standard review, full build, vet/lint and exact tree binding
passed for the last local commit. The carried Windows private-wrapper
P2 remains open; it is not misclassified as a C2 scope expansion.

The production shutdown repair also passed targeted three-reviewer
adversarial validation with zero new HIGH/MEDIUM/LOW findings. The
socketpair completion was proven RED before its minimal correction.

## Full-suite evidence and counters

* Ordinal 19: consumed PASS on `62064e89`, 46 packages and 3,922 top-level
  passes. No reuse for later source changes
* Ordinal 20: consumed PASS on `c574669b`, same census, zero failures,
  exits 0 and valid evidence
  * Result: `bin/_rrc2645a20/pr645-ci2-ordinal20/result.json`
  * SHA-256:
    `5be6ffa55ab7d9c39c9c3261332ccf074b56fae4d27fe19bff8bcdd2405af6e0`
* Ordinal 21: consumed failed/invalid evidence on last-known local
  `74c09bae`, native/suite exits 1, 14 reported failure events, 3,903
  top-level passes, 44 package passes and two package failures
  * Failures in `tests` and `tests/integration` require missing `git`
  * Final HEAD lookup: `FileNotFoundError`; `head_after=null`
  * Result: `bin/_rrc3645a21/pr645-ci3-ordinal21/result.json`
  * SHA-256:
    `b3b9f756c80d56a8e1305e0825347649e3d6fcd2e28c5b08a181c45effa1070f`
  * Consumption:
    `bin/tmp/212-ordinal21-c3645a21-consumption.json`
  * Elapsed 2,156.459294 seconds; no timeout, drain error or cleanup error

Historical full-suite failed count is now **15**, increased from 14 by
the consumed Ordinal 21 failure. It is not reset or excused as prelaunch.
The test child ran. No next full-suite attempt is authorized by this halt.

Fix-CI repair iteration: **3**. Two pushed repair iterations failed the
same hosted `test` check; the initial baseline failure is separately
retained, making three observed hosted failures. The third repair is not
pushed. Do not reset counters or treat this halt as a fresh iteration.
A third failed repair iteration on the same hosted check must halt.

## Copilot capture and current gates

Copilot review 5849123381 identified the valid U1k Git-ceiling defect.
M22/U1k explicitly mandates the production rule, so changing it fails
P-021 C1 and requires Stage deliberation.

Stash **1CED4AD6** was created and re-read before reply **4530424016**
cited it. Bot thread `PRRT_kwDORzozKM615Onq` was then resolved and the
PR residual-risk body updated. The single-write entry was not amended.
It preserves `DISCOVERY-STATUS: AMBIGUOUS 60D799D9` and
`DISCOVERY-STATUS: LOOKUP-UNAVAILABLE` for archived-stash discovery.
The earlier candidate was not positively confirmed and was not reused.

All Copilot review bodies were inspected. There are no threadless
findings or missing trusted disposition markers. Review 5850382383 on
the current pushed HEAD has no open issues and lists the prior issue
under resolved-since-last-review. Review 5849232603 is the owner's empty
reply review, not an additional Copilot finding.

Final independent remote verification:

* P-018: **SATISFIED**, exit 0, HEAD `c574669b`, zero open Copilot threads
* Required hosted `test`: **FAILURE**, run `42913035454`,
  job `130168437175`, regression coverage stdout defect
* Other required checks: PASS
* Section 1.9 query fully paginated: Check 1 PASS, Check 2 **FAIL**
  (Outcome BLOCKED, P1=1), Check 3 PASS, Check 4 PASS, Check 5 P-018 PASS
* `reviewDecision`: null

The PR body is explicitly BLOCKED; it does not claim CI success or
all-five-check readiness. Local P-014 and P-012 halt events were recorded
through telemetry. No force/admin/merge command was used.

Existing residual IDs remain:
C9331068, 0BB19B05, D2FCA0FB, 368E21A2, 396A018C, 54D5F9CC,
4B30E19C, 264ACCED, 9ECF5511, F0973A97, 0D073C21 and 1CED4AD6.
The five historical LOW-confidence independent-review observations
remain advisory under the operator's disposition. Monitoring ownership,
rollout/rollback readiness and the validation window remain conditional.

## Resume boundary

Restore stable Git availability through the operator's environment
maintenance path. Then explicitly select and confirm this Ship checkpoint;
do not auto-resume, resolve other checkpoints or repeat the failed run.
First verify local/remote identities and preserve the unpushed commit,
all evidence and the failed-count 15 baseline. Obtain a fresh bounded
verification decision before another full suite; never reuse Ordinal 21.

After fresh local validation, push the existing correction and require
new hosted CI, current-HEAD local readiness and Copilot/P-018 completion.
Only after all required gates pass may merge approval be requested.
P-014 approval is absent; do not merge.

Protected historical checkpoint `checkpoint-20261106-210317.json`
remains byte-identical:
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
No historical checkpoint was resolved or modified in this continuation.
This final memory and the new recovery checkpoint remain local and
uncommitted because Git is unavailable.
Recovery checkpoint: `checkpoint-20261110-232635.json`.
Its MCP re-read confirmed valid V1, conforming, agent ship and active,
with the frozen task set, counters, root bindings and gate outcomes intact.
