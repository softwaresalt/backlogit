---
title: Shipment 212-S M11 prerequisites complete and convergence circuit open
date: 2026-11-06
shipment_id: 212-S
feature_id: 211-F
phase: wave1-convergence-circuit-open
status: blocked
---

## Completed work and immutable scope

U1, U1b, U1c, U1d, and U1e are done. U2-U7 remain claim-assigned,
unstarted, with zero start records. The formal M11 set, S order,
dependency edges, and eleven-task budget are unchanged.
No new claim, restore, branch, worktree, push, PR, merge, or successor
execution occurred.

Stage amendment commit: `bf6bf2f6e3503c916dabb1ac40580e8ad9cc5ca8`.
Harness commit: `310e87b4e85f6794f39b2f6e0685389b860382e7`.
U1c source and corrective evidence: `23581762`, RED `404e8e6a`,
final error-chain fix `286defdf`.
U1d: `0a92846e3e97298f81ad985042122dc4e731f23f`.
U1e and unchanged current HEAD:
`2c87ae6fdf64480ab6767e9015cae51f24abc2e6`.

All three new tasks passed their guarded commands, original-assertion
preservation, compile, vet, pinned five-minute lint, format, actual persona
reviews, official completion gates, and commit association.
One U1c P2 was corrected test-first; current task reviews have zero
unresolved findings. This is not full-branch adversarial or merge approval.

After all five wave-1 members were terminal, parent reran compile, vet,
pinned lint, formatting over 1,388 tracked/new Go files, and all five
declared scoped commands: PASS. No convergence red exception exists.

## Complete uncached unfiltered result

```text
go test -count=1 -p=1 -timeout=30m -json ./...
```

Cached Go 1.24.0, readonly/offline modules, workspace-contained caches,
private TEMP, package serialization only, unchanged within-package
scheduling and 30-minute per-binary timeout. No known competing contained
test runner was present.

Native exit: 1. Before/after HEAD matched 2c87ae6f.
All fifty discovered packages produced terminal evidence:
42 PASS, 4 FAIL, 4 no-test-file package skips.
There were 3,871 top-level passes, nine top-level failures and 21 failure
events including subtests. No timeout or panic occurred.
Core completed in 1,575.7 seconds, below its unchanged budget.
Seventy-eight existing test-skip events were reported honestly; no skip
flag, short mode, filtering, or new t.Skip was introduced.

Complete bounded capture:
`bin/tmp/1793952023849-copilot-tool-output-7128-9c0c1662-babc-4f5a-8b00-a8285271ca6d.txt`.
Human-readable failure tails:
`bin/tmp/1793952036460-copilot-tool-output-7128-7105f6a3-3b76-491f-b782-69835264279a.txt`.
Failure inventory is complete and untruncated; package census has no
missing or extra result, and no non-JSON infrastructure output occurred.

None of the nine prior full-run failing top-level names remained failed
or appeared in the skip inventory. The original deterministic blockers
were not bypassed.

## Current full-run top-level failure inventory

| Package | Test |
| --- | --- |
| internal/cli | TestU7_CustodyCLIMCPParityAndRegistryContract |
| internal/core | TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery |
| internal/core | TestGovernedShipmentDispositionEdge |
| internal/core | TestP021ClaimSerialization_RepeatedMembershipAndGenericWritersContend |
| internal/core | TestValidateShipmentReconcilePreconditions_RawManifestMustBeUniqueNonBlankStrings |
| internal/core | TestU2_ReconstituteShipmentsAtomicExecutionAndRollback |
| internal/core | TestUpdateArtifact_KeepsBlockedReasonWhileStillBlocked |
| internal/mcp | TestU6_MCPCustodyToolsCanonicalDirectAndColdServer |
| tests | TestReconcileAttestationFlagEndToEnd |

This table reports existing full-suite evidence; no Stage-only disposition
tool or CLI was invoked as a targeted diagnostic.

## Owned runner-path error and bounded correction

The private TEMP prefix
`bin/_authoringtmp/212-M11-wave1-full-20261105T231529817`
caused explicit Git `Filename too long` errors creating fixture ref locks
and objects. This is an owned test-runner environment error, not evidence
requiring custody source or Git-security changes.

Ship corrected only its private ignored TEMP to `bin/_qf4781986`
(41 absolute-path characters). No global Git longpaths setting, registry
change, antivirus change, ambient Git passthrough, fake repository
boundary, source edit, or unknown-process kill occurred.

One coherent count-1 package-serial diagnostic covered eight prior
failing top-level names across CLI/core/MCP/tests. It deliberately did
not target the Stage-only disposition case.
Seven passed; no selected test skipped.
The native command still exited 1 because reconstitution's concurrent
associate-commit case failed again. This is not full-suite success and
short TEMP is not a verified remedy for all AccessDenied failures.

## Repeated replacement denial and bounded read-only diagnosis

Remaining reproduced subtest:
`TestU2_ReconstituteShipmentsAtomicExecutionAndRollback/concurrent_associate_commit_and_post_hook_isolation`.
Failure at `internal/core/shipment_reconstitute_test.go:499`:

```text
associate commit: mutation partial (not-applied):
failed at frontmatter-scalar with no prior steps applied:
associate commit step 1 (frontmatter scalar):
move temp over destination:
backlogit: durable write not applied: Access is denied.
```

The short-TEMP probe executed this subtest once (3.55 seconds), with its
top-level test ending in 26.53 seconds. This is a second observed failure
of that case across full run and bounded comparison, not a stress loop.

A read-only source trace found a concrete in-process overlap candidate:

* Both writers acquire lifecycle-global before mutation, but
  `reconstituteShipments` calls `planReconstitution` before global
  acquisition (`shipment_reconstitute_exec.go:319-345`)
* The preliminary plan reads artifacts through `findArtifact` and
  `scanCanonicalArtifacts`, reaching `parseFile` / `os.ReadFile`
  (`shipment_reconstitute_plan.go:293-305,475-476`,
  `canonical_scan.go:93`, `artifacts.go:1156-1157`)
* AssociateCommit can therefore hold global and replace task 001.006-T
  while that unlocked preliminary plan reads it
* Fresh configured Concurrency Reviewer analysis proves that Go 1.24
  syscall.Open uses FILE_SHARE_READ and FILE_SHARE_WRITE without
  FILE_SHARE_DELETE; the failure-time denying handle, numeric error,
  attributes, and overlap remain uncaptured
* Explicit member-reader and journal helpers already allow delete sharing
* The atomic writer closes its temporary file before MoveFileEx; no
  own-temp-handle defer-order bug was established
* The fixture joins its two goroutines; its synchronous hook only updates
  a mutex-protected counter and does not open files

The source-derived short-run target is the queued `001.006-T.md`.
With a ten-digit temporary suffix, its destination is about 191 characters
and its atomic temporary path about 205, not an established MAX_PATH
explanation. Exact failed absolute spelling and attributes remain uncaptured.

This is a plausible reader/replacer sharing interaction, not proof of AV,
a retained offending handle, or a production patch.
Next evidence should identify numeric Windows error, destination/temp
attributes and ACLs, handle share modes/open-close ordering, and preliminary
planning versus mutation ordering. Do not remove intended concurrency,
add blind retries, weaken durability, or globally change sharing without
the proper contract and security review.

## Independent reasoning escalation clarification

The configured independent Concurrency Reviewer analysis completed
read-only on claude-sonnet-5.5/anthropic/high with long_context, distinct
from Ship's gpt-6.1-sol/openai/medium route. Fresh routing resolution is
recorded in the formal circuit checkpoint and JSON payload linked below.
No new analysis or test was dispatched during final bookkeeping.

The source-proved reader-sharing contract gap is P2/manual.
Attribution of this actual AccessDenied is only estimated at 50-60 percent:
the denying handle, numeric error, attributes, and interleaving were not
captured. The writer's own temporary file is closed; no own-handle bug
was proved, and an ignored-.md scanner observation is not attributed as
the denying actor.

Existing delete-sharing helpers also change containment, reparse handling,
read bounds, and error semantics. Do not copy them globally or hoist all
reads under global to eliminate intended concurrency. A future
Stage-owned diagnostic/repair unit needs deterministic Open/Replace proof
and specific actor attribution before source changes.

The P3 parse-error-skipping observation is unresolved analysis, not an
established defect: bulk scanning may intentionally tolerate bad files.
The escalation did not audit the other eight full-run failing names.
Seven bounded passes and the untargeted disposition case do not establish
that all nine failures are fixed.

The first of the three convergence commands was terminated after confirmed
CLI failure. Its normal overall native exit is unknown, not invented as 1.
The second and third Go commands returned normal overall exit 1.
All three remain counted convergence failures without resetting the limit.

Formal bookkeeping:

* [Wave-convergence circuit checkpoint](212-S-wave-convergence-circuit-checkpoint.md)
* [P-013.6 reasoning payload](212-S-wave-convergence-escalation-payload.json)

Terminal Engram delivery is unconfirmed: ESCALATION_DEGRADED.
The installed tools expose no targeted payload write; a legitimate
operator/asynchronous file fallback is prepared. No bulk graph mutation,
flush, or read query was misrepresented as a write or receipt.
This reasoning handoff provides no retry, repair, claim, or merge authority.

## Residual identity and authority

Reuse `96F1DFFD`: this is the same Windows durable replacement investigation
already carried through 086-DL with a pending-verification no-code
disposition. Its original capture remains untouched.
New failure names, short-TEMP results, and read-only causal evidence belong
in these Ship-owned residual records, not an edited or duplicate stash.
Stage owns any new persistence/reader-coordination work-unit decision.

## Circuit and resumption boundary

Cumulative counters: five tasks attempted, zero consecutive task failures,
three wave-convergence failures, one U1c review-fix cycle, zero CI-fix cycles.
The third convergence failure opens the convergence circuit; no fourth
unfiltered run or U2 admission was attempted or authorized by Ship.
The bounded comparison adds one unexpected native failure, not another
full-suite invocation. No counters were reset by M11 or the short-TEMP fix.

Preserve the same feature branch, HEAD, all unstaged tracking/capture state,
and archive/queue deletion pairs. Staging is empty.
The parent receives an actionable owner-corrected environment and bounded
reader/writer hypothesis for governed continuation, not a guessed first-error
handoff. Further full-suite execution needs explicit handling of the open
circuit; no source-free complete remedy has been established.

U2-U7, final actual persona plus three-model adversarial review, PR/Copilot,
required CI, full local build, normal implementation merge, runtime evidence,
safe-close, P-020 closure, closure PR merge, and safe main sync remain pending.

The sole current Ship checkpoint is `checkpoint-20261106-081656.json`,
officially read back valid and conforming. Its predecessor
`checkpoint-20261106-073313.json` was individually resolved after the
running suite reached its recorded terminal result. No historical restore,
cross-role checkpoint operation, or broad resolution sweep occurred.
