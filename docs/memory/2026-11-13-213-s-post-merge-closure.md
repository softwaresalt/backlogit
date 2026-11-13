---
title: "213-S post-merge documentation handoff"
doc_type: memory
source: docs/memory/2026-11-13-213-s-post-merge-closure.md
ingested_at: "2026-11-13T07:16:50Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: 213-S
feature_id: 212-F
---

## Completed scope and current cursor

PR 647 merged `2026-11-13T06:35:19Z`, merge SHA
`b470866d02819c978216eb14c90e6a97dd805f11`. Parent verified safe main sync,
then established `post-merge/213-s-closure`. Archive HEAD is
`8fdede0043af465349fe620a4c611733c48e0110`, 16 exact allowed paths.
Governed pre/safe-close/post all passed under retained lock, released and
verified absent. Explicit members: `212-F`, `212.001-T`, `212.002-T`,
`212.003-T`, plus control `213-S`; no returned IDs or nonmember changes.
Shipment archived status is shipped; feature/tasks archived status is done.

Prior green head `a4f44381baf901c6c88df14ce5058d303f71e239` has all eight CI
checks SUCCESS, parent-confirmed P-018 and READY_WITH_FOLLOWUPS,
P0=0/P1=0/P2=8/P3=5. Independent feature/merge comparison found identical trees.
Merged-tip LineEndings passed at `06:35:43Z`; PR-only Go CI is not a
merged-tip run. The actual observation window ended `07:05:19Z`; retrospective
checks were healthy and smoke was explicitly late, not continuous monitoring.
`O-213-01` is recorded retrospectively as OPEN effective
`2026-11-13T07:05:19Z`, owned by Ship, for the missed in-window smoke; no
contemporaneous opening event is claimed.
At `2026-11-13T07:41:51Z`, parent Ship explicitly CLOSED it
through Ship-owner acceptance of late 14-pass smoke, within-window main
LineEndings and source-equivalent CI. Rationale: optional pure-library
validation with no live rollout. Not operator acceptance, a met/restarted
window or continuous monitoring; rollback authority stays with the operator.

Actual post-merge U9 selector ran `07:14:17.6328332Z` to `07:15:01.1071778Z`:
4 tests, 10 subtests, 14 run/pass events, no failures/skips, exit 0.
No live CLI/MCP/doctor wiring or unsupported HTTP/browser guarantee was tested.
Boundary/risk review must precede `214-S` U11 (`213.001-T`), the first CLI
consumer implementing `VerifyLauncherScripts` / `AuditRepositoryToolchain`
and `mcp status --toolchain` / `--strict`. U12 (`213.002-T`) separately
provisions `WriteToolchainManifest` / `RenderPinnedMCPConfig` and E2E isolation.

## Documents and decisions

Authored canonical
[closure](../closure/213-S-212-F-post-merge-closure.md),
[runtime verification](../closure/213-S-post-merge-runtime-verification.md),
and [compound refresh](../closure/213-S-compound-refresh.md).
Kept the verified actual-Markdown-bin learning. Marked the old
`source-artifact-archival-pattern-2026-04-20.md` stale with current-boundary
citations, retaining history. U10 architecture/design remain accurate.

P-020 `compact-context` was explicitly invoked with target all and defaults
14 days / 40 files / 500 KB. Assessment baseline: memory 356 files,
2,253,099 bytes; plans 0; closure 156 files, 1,011,858 bytes.
All 46 release memory records were preserved: 42 have incoming Markdown
citations; all contain gate/admission/recovery provenance, served-root state,
authoritative adjudication or completed-task continuity. Two completion-titled
records and every task's most recent checkpoint remain in place.
This was only a scoped preservation assessment, not global eligibility
resolution. The previous zero-global-candidate/done claim was incorrect.
Zero moves, zero compacted files, zero bytes recovered, zero decided plans
and zero compacted closure records are actual mutation counts only.
Fresh closure/evidence stay intact; unrelated release units and active context
were excluded from mutation. Current handoff memory is also protected.

Reinvoked `compact-context` target `all`, defaults 14 days / 40 files /
500 KB, for a bounded read-only global assessment at
`2026-11-13T07:41:51Z`. Current inventory before these corrections:
memory 357 files / 2,258,718 bytes (297 Markdown, 60 other);
configured plans 0; closure 159 files / 1,031,842 bytes
(156 Markdown, 3 other). Filename/ingestion-date screening before October 30
found 55 possible older memory and 71 possible older closure candidates;
189 memory and 69 closure Markdown files had no date by that screen.
These are not resolved eligibility counts, and newer completed/superseded
memory is not excluded by age alone.

Resolved example: the PR 402 September 3 closure records merged/READY,
so is a completed older closure candidate, not an automatic scope exclusion.
Exact filename scans found zero incoming `docs/**/*.md` references and the
read-only MCP query found zero queued/active/blocked indexed description or
reference-field matches. Other aliases, its observation and the remaining
global completion/reference/latest-checkpoint protections are unresolved.
All originals remain untouched. Final `compaction_status: degraded` honestly
records a partial pass, allowed non-blocking under P-020, not global success.
Parent follow-up: resolve global eligibility and obtain governed cross-release
summary/archive scope, including PR 402. No new stash/capture was written.

Parent confirmed the same broader maintenance work is already captured by
active Stage-triaged survivor `A17EE897`, re-read it and reused it without
editing or duplicating the entry. PR 648 / thread
`PRRT_kwDORzozKM62_vov` are the current residual-risk references. The 21
feature deferrals below remain unchanged.

## Residual inventory and provenance

No new captures or item/stash writes. Preserve all 21 unchanged IDs:

```text
00FD5168, 8841FD73, 1406FAE2, CF190A0C, 58AA0592, 5F7A81D7, 2C71DFA2,
D1455316, 3083FC1A, 01D3F26B, CBD2D21C, B7E3A649, 8AD5625B, 5688AB99,
600835CA, 0AF43C97, F12B587F, C446F206, 5B8497D4, 6FADB54A, EC589C00
```

`6FADB54A` stays Stage/operator P2/out of scope; `EC589C00` is Level 2;
relative-root spelling reuses `8AD5625B`. Retain exact qualifications:

* `DISCOVERY-STATUS: LOOKUP-UNAVAILABLE` where already recorded
* `C446F206`: `DISCOVERY-STATUS: AMBIGUOUS 036B0CB9`
* `5B8497D4`: `DISCOVERY-STATUS: AMBIGUOUS 48F1E692 6BF65C9D 5F7A81D7`
* `6FADB54A`: `DISCOVERY-STATUS: AMBIGUOUS 58AA0592 5F7A81D7`
* `EC589C00`: `DISCOVERY-STATUS: AMBIGUOUS D1455316 9FC3A6A6`

`212-F` declares neither source field. `082-DL` / `ECF33C79` are untouched
narrative references, not members. Immutable public close evidence remains
CASCADE/VERIFIED, post_close, real exit 0/postconditions PASS.
The untracked resolved checkpoint
`.backlogit/checkpoints/checkpoint-20261113-061326.json` is untouched and
excluded; parent decides later staging.

Policy is not an engine guarantee: read-only
`shipment_lifecycle.go:512-518` / `stash.go:789-804` confirm queued linked-stash
archival can touch active sources outside M. Future closes must preflight
ALL explicit members' linked sources and HOLD on ANY out-of-M archival.
For `213-S`, `disposition_byte_identical` and empty shared-reference drift
prove the recorded no-mutation outcome only; no retroactive preflight or
production fix is claimed.

## Degradation, failed approach and next steps

Engram PID 49128 intentionally remains stopped. Parent's post-suite
`set_workspace` retry failed IPC; bounded known-file fallback was used, not a
restart. Graphtor is unavailable; no index writes.
Continuous-learning is configured, but parent attempted a `learn` SKILL read
which failed, and `scripts/search.ps1 learn` found compound/refresh only.
Missing named `learn`/`observe`/`evolve` skills mean no invented observation,
instinct or promotion result. P-020 was invoked; global resolution remains
partial/degraded as corrected above.

One reference scan failed because PowerShell wildcard interpretation of an
existing bracketed archive filename broke `Get-Content`; a literal .NET read
retry succeeded, finding 42 cited release-memory files. No file was changed
by either scan. No new release risk was discovered.

PR 648 bounded review-fix iteration 1 corrects four C1 same-contract-surface
documentation findings at base HEAD `56329021c52e2b5ad19c021131f0c4787ba1397c`.
Changed paths: the canonical closure, post-merge runtime/refresh,
pre-merge readiness/runtime, both PR/post-merge handoff memories and historical
source-archival learning. No Stage plan, archived planning field, source,
schema, test, review artifact, evidence JSON or checkpoint was edited.
The first reference query failed because `references` needed SQL quoting;
the corrected read-only MCP query succeeded with zero matches. One guessed
runtime-report filename did not exist; the actual known pre-merge path was
read instead. No engine operation, claim, ship/dispose, Git/PR/backlog
mutation or additional agent was used. Parent owns post-push replies and
resolution for comments 4560806734, 4560806798, 4560806823 and 4560806870.

Parent owns Git, closure PR creation, current-HEAD local review and gates,
CI/P-018 where applicable, and that PR's individual merge approval.
No staging, commits, pushes, branch switches or PR actions occurred here.
Documentation readiness is not overall pipeline completion or successor claim.

Local validation at `2026-11-13T07:18:47Z`: actual Markdown CLI v0.23.1
scanned 4,157 files, zero issues; all five authored paths passed docline
frontmatter lint; canonical closure-evidence gate passed with no warnings.
Evidence SHA-256 remained
`d299968293821d733749949fcde274e3978fa5bebbe1f3dadfad874bae094982`;
excluded checkpoint SHA-256 remained
`7adc271772f12eb97c3f161e790104e713818018b507141455040bec73cb2ae0`.

## Bounded PR 648 remediation validation

Bounded remediation validation: PASS at `2026-11-13T07:45:58Z`.
The actual cached Markdown CLI v0.23.1 scanned all 4,157 repository Markdown
files with zero issues using the repository's `"**/*.md" "!.git/**"` selectors.
The initial invocation omitted the repository-defined Git metadata exclusion
and reported 23 MD041 findings confined to `.git` scratch; no such file was
edited. Correcting the invocation to match `scripts/md-lint.sh` passed.
Authoring-profile docline lint passed all eight corrected files with zero
violations; the canonical closure-evidence gate returned passed=true,
exit 0, no warnings. `git diff --check` passed.

Both immutable SHA-256 values above were rechecked unchanged, and the exact
21-ID inventory matches HEAD byte-for-byte. The diff contains only the eight
authorized Markdown paths; the index is unchanged. Go build/test/vet/lint
were not run: no production, schema or test surface changed. Advisory file
locks were acquired for these eight files and released after this validation.
No commit, push, staging, PR/backlog write, source guard or cross-release
compaction was performed. Parent owns governed capture of the global
compaction follow-up and all post-push thread replies/resolutions.
