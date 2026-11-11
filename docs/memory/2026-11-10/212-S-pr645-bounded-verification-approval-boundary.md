---
title: "PR 645 bounded verification and approval-boundary handoff"
doc_type: reference
source: "docs/memory/2026-11-10/212-S-pr645-bounded-verification-approval-boundary.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: "212-S"
feature_id: "211-F"
phase: "pr-approval-boundary-pre-current-head-review"
---

## Recovery and authority

The operator explicitly selected and confirmed recovery checkpoint
`checkpoint-20261110-232635.json`. Its pre-restore SHA-256 matched
`114b60be2c988d8272d3fad8228b2c34cb2a13b98c564753784defe3efa7f5e7`.
The full checkpoint enumeration reported no validation/quarantine
anomalies. Bound engram state was reachable and read before bounded
summarization; its semantic hits were unrelated, so exact checkpoint and
Stage evidence remained authoritative.

The restored release cursor, frozen 22-task set, gate verdicts, unpushed
source commit and historical failure count were preserved. Only the
selected checkpoint was resolved after successful recovery. The
unselected `checkpoint-20261110-183215.json` was not resolved.

Protected `checkpoint-20261106-210317.json` remains byte-identical:
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.

Stage authority:
[bounded verification handoff](stage-212-s-089-dl-ordinal21-bounded-verification-authority.md),
SHA-256 `2257fa667e647354df2ea60c587f41b41db281d8ff78a362191b01e57d5dde98`.
All eight listed Stage-authored file hashes matched before any mutation.
Ship transports those approved changes without editing planning fields.

## Served-root and tool handoff

Independently confirmed through MCP metadata and `pragma_database_list`:

* Served workspace: `.`
* Served storage: `.backlogit`
* Main index: `.backlogit/backlogit.db`
* Shipment 212-S: active, explicit 23-member manifest unchanged
* All 22 task members remain complete; open RED is empty
* Same feature branch and worktree retained

Git is restored as `2.55.0.windows.5`, resolving through `bin/git.exe`.
No system installation or configuration change was made by Ship.
Live routing configuration validated. Startup index sync passed, hook
queue was empty, and graphtor-docs was reachable without workspace ingestion.
Intercom is not installed; visibility remains local and PR-backed.

## Bounded verification

Source baseline: `74c09baeea828231ccbae811100bc4259f4f2335`.
The sole Go-file difference from the Ordinal 20 source commit is
`internal/cli/mcp_session_input_unix_test.go`, excluded from Windows
builds. Stage formally authorizes the following evidence composition;
no Ordinal 22 Windows full suite was launched.

All commands used the existing pinned Go 1.24 environment:

| Check | Result |
| --- | --- |
| `go test -timeout=10m -count=1 ./tests ./tests/integration` | PASS; tests 503.935s, integration 245.207s |
| `GOOS=linux go test -c -o bin/tmp/_cli_linux.test ./internal/cli` | PASS; metadata confirmed GOOS=linux |
| `go build -o bin/tmp/_backlogit-212s.exe ./cmd/backlogit` | PASS; GOOS=windows, source baseline confirmed |
| `go vet ./...` | PASS |
| `golangci-lint run --timeout=10m` | PASS |
| `backlogit_docs_lint` | Valid, zero violations |
| `go test -timeout=5m -count=1 -v -run '^TestU175_001_BaselineEol$' ./tests` | PASS; 1759 checked, zero non-LF/mixed rows |

Both generated binaries were verified and removed at their exact paths.
The pre-commit build correctly recorded `vcs.modified=true` for the
approved Stage metadata working tree. A clean final-HEAD build and the
current-HEAD readiness record belong in the PR evidence after commit.

Paired Ordinal 20 result:
`bin/_rrc2645a20/pr645-ci2-ordinal20/result.json`,
SHA-256 `5be6ffa55ab7d9c39c9c3261332ccf074b56fae4d27fe19bff8bcdd2405af6e0`.
It remains 46/46 Windows packages PASS, 3,922 top-level passes, zero
failures, exits 0 and valid evidence. Stage also accepts the prior WSL
Linux coverage-mode six-case, ten-repetition PASS.

Ordinal 21 remains a consumed failed cycle, not reclassified or omitted.
Historical failed count is **15**. No reset, token reuse, new wave,
task reclaim, restart or re-completion occurred.

## Review and operational handoff

The before-commit source review at 74c09bae covers the fixture correction;
the new metadata/docs commit still requires a current-HEAD addendum.
Do not fabricate that future result or commit again merely to record its SHA.
Record final review, CI and P-018 outcomes in the PR body and session output.

Carry the twelve residual identities:
C9331068, 0BB19B05, D2FCA0FB, 368E21A2, 396A018C, 54D5F9CC,
4B30E19C, 264ACCED, 9ECF5511, F0973A97, 0D073C21 and 1CED4AD6.
The five historical LOW-confidence independent-review observations
remain advisory under the operator's disposition. The existing Windows
private-wrapper P2 remains an explicitly open same-contract issue.

1CED4AD6's capture-before-reply-before-resolution history is preserved.
Its candidate 60D799D9 was not positively confirmed and was not reused.
No stash entry was edited after capture, re-triaged or re-prioritized.

The reusable coverage/stdout ownership lesson was captured through the
compound skill in
`docs/compound/test-failures/private-stdio-for-reexecuted-go-coverage-2026-11-10.md`.

Runtime/closure notes carry the new bounded evidence without claiming
hosted Linux UDS success or activated monitoring. Operational closure
remains conditional; post-merge compaction has not begun.

## Approval boundary

Create one current-session active checkpoint before the final metadata
commit. Then require an empty worktree, current-HEAD local readiness,
full-build evidence, lifecycle topology PASS, push, fresh required CI,
fresh Copilot/P-018 completion and section 1.9 verification.

The fix-CI counter remains on repair iteration 3 after two failed pushed
repair iterations; the initial baseline failure is separately retained.
The upcoming push is the third repair iteration, not a counter reset.
A third failed repair iteration on the same hosted check must halt.

No merge is approved. Remain on the feature branch and wait for an
explicit P-014 signal after presenting the actual final gate state.

Final current-session checkpoint: `checkpoint-20261111-000724.json`.
Its MCP re-read confirmed valid V1, conforming, agent ship and active.
It intentionally predates the final commit, reviews and hosted gates;
the actual current-HEAD PR record remains authoritative for those outcomes.
