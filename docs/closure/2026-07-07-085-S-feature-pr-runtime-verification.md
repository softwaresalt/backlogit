---
chunk_strategy: h1-h2-h3
description: 'Runtime verification for shipment 085-S (shipment-gate empty-head fail-closed hardening) feature PR. Exercises the REAL ship-gate entry point ShipShipment -> gateShipmentCompletion -> validateMemberGateEvidence, plus the new bounded repo-presence probe inGitWorktreeBounded, against REAL git repositories (real git rev-parse subprocesses). Demonstrates the full behavioral matrix end to end: (1) enforcement + real work tree + empty SHIPMENT head (unborn branch) -> REFUSES (fail closed, hole 1AEA2B0E); (2) enforcement + real work tree + empty MEMBER head_sha -> REFUSES (fail closed, hole B85DAEE8, R7 flipped); (3) genuine no-repo + empty shipment head -> still SHIPS (legitimate skip preserved); (4) no-repo empty member head -> still skipped; (5) legitimate members-with-evidence -> still SHIPS; (6) present-but-broken repo (broken .git pointer / empty .git dir) -> FAILS CLOSED (adversarial F1/N1); (7) expired probe context -> FAILS CLOSED. All scenarios green. Regression coverage lives in the committed unit tests.'
doc_type: closure
docline:
    ms.date: 2026-07-07T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-07T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-07-085-S-feature-pr-runtime-verification.md
title: 085-S Shipment-Gate Empty-Head Fail-Closed Hardening — Feature PR Runtime Verification
---

# 085-S Feature PR — Runtime Verification

**Shipment:** 085-S — Shipment-gate empty-head fail-closed hardening
**Branch:** `feat/085-shipment-gate-empty-head-fail-closed`
**Date:** 2026-07-07
**Verified commits:** `bf80557` (ST3 GREEN) + `586993f` (adversarial F1) + `203a4b1` (N1/N2)

## Method

The committed behavioral tests drive the **real** ship-gate entry point
`ShipShipment` → `gateShipmentCompletion` → `validateMemberGateEvidence` (the
functions `shipment ship` invokes), and the new bounded repo-presence probe
`inGitWorktreeBounded`, against **real** git repositories — the actual
`git rev-parse --is-inside-work-tree` subprocess executes (no mocking of the
probe). The gate broker is injected as `EnabledTrue` so `ev.Enforced == true`
(the enforced path). Two real git fixtures back the scenarios:

- `initGitRepoNoCommits` — a real work tree with an **unborn** HEAD (`git init`,
  no commit): `--is-inside-work-tree` = `true`, `rev-parse HEAD` empty. This is
  the load-bearing fixture that makes an empty shipment/member head arise **inside
  a real repo** under enforcement (the exact fail-open condition).
- `newGateTestWorkspace` bare temp dir (no `.git`) — the genuine no-repo case.

The branch-built repo-root `.\backlogit.exe` was used for the CLI smoke
(`shipment get 085-S` → active, 5 members) confirming the shipped binary wires
the gate.

## Results — full behavioral matrix green

Run: `go test ./internal/core/ -v -run '<matrix>'` (real git subprocesses).

| # | Scenario (real path) | Enforced | Repo state | Head | Expected | Observed |
|---|---|---|---|---|---|---|
| 1 | Empty **shipment** head in real work tree (1AEA2B0E) | yes | unborn-branch worktree | shipment head "" | **REFUSE** (fail closed) | `TestShipmentGate_EmptyShipmentHeadInRepo_Refused` PASS — "cannot resolve shipment head in repository"; state unchanged; `EventGateBlocked reason=empty-shipment-head` recorded ✅ |
| 2 | Empty **member** head_sha under resolved shipment head (B85DAEE8 / R7) | yes | real repo | member head_sha "" | **REFUSE** (fail closed) | `TestValidateMemberGateEvidence_StaleRefused` (R7) PASS — "no recorded head_sha"; `*GateBlockedError`; `EventGateBlocked reason=empty-member-head` ✅ |
| 3 | Empty **shipment** head, genuine no-repo | yes | no `.git` | "" | **SHIP** (legacy skip preserved) | `TestShipmentGate_EmptyShipmentHeadNoRepo_Skips` PASS — ships ✅ |
| 4 | Empty **member** head, empty (no-repo) shipment head | yes | no `.git` | "" / "" | **SKIP** (block not entered) | `TestValidateMemberGateEvidence_EmptyMemberHeadNoRepoSkipped` PASS ✅ |
| 5 | Legitimate members with evidence (control) | yes | no `.git` | resolved | **SHIP** | `TestShipmentGate_AllMembersHaveEvidence_Ships` PASS ✅ |
| 6a | Present-but-broken **.git pointer** (F1) | — | gitfile → missing gitdir | — | **FAIL CLOSED** | `TestInGitWorktreeBounded` (d) PASS — non-nil err ✅ |
| 6b | Present-but-**empty .git dir** (N1) | — | empty `.git` dir | — | **FAIL CLOSED** (message-independent) | `TestInGitWorktreeBounded` (e) PASS — non-nil err ✅ |
| 7 | Expired probe context | — | real repo | — | **FAIL CLOSED** (no silent false) | `TestInGitWorktreeBounded` (c) PASS — non-nil ctx err ✅ |
| 8 | Real work tree (control) / genuine no-repo (control) | — | worktree / none | — | (true,nil) / (false,nil) | `TestInGitWorktreeBounded` (a),(b) PASS ✅ |

### Verbatim (key lines)

```
--- PASS: TestInGitWorktreeBounded (a real / b no-repo / c expired-ctx / d broken-pointer / e empty-.git-dir)
--- PASS: TestShipmentGate_AllMembersHaveEvidence_Ships
--- PASS: TestShipmentGate_EmptyShipmentHeadInRepo_Refused
--- PASS: TestShipmentGate_EmptyShipmentHeadNoRepo_Skips
--- PASS: TestValidateMemberGateEvidence_StaleRefused        (R7 flipped: empty member head -> fail closed)
--- PASS: TestValidateMemberGateEvidence_EmptyMemberHeadNoRepoSkipped
PASS

CLI smoke (branch-built .\backlogit.exe): shipment get 085-S -> status "active", members [085-F, 085.001-T, 085.001.001/002/003-ST]
```

## Interpretation

- **Both fail-open holes are closed on the real path.** Scenario 1 proves an empty
  shipment head inside a real work tree under enforcement now **refuses** (was a
  silent skip). Scenario 2 proves an empty member head_sha under a resolved
  shipment head now **refuses** (R7 flipped). Both emit an `EventGateBlocked`
  monitoring signal (Constitution Principle V), so the refusal is observable, not
  silent.
- **Legitimate empty-head cases are preserved (no completion breakage).** Scenarios
  3–5 confirm a genuine no-repo empty shipment head still ships, a no-repo empty
  member head stays skipped, and legitimate members-with-evidence still ship. The
  discriminator is the bounded, fail-closed `inGitWorktreeBounded` probe — not
  `ev.Enforced` (which does not track repo presence).
- **Present-but-broken repos fail closed.** Scenarios 6a/6b (adversarial F1/N1)
  confirm a broken `.git` pointer and an empty/corrupt `.git` dir both **fail
  closed** — via a message-independent `os.Stat` guard plus the git-stable no-repo
  marker — rather than being misread as a legitimate no-repo skip.
- **Fail-closed on the probe itself.** Scenario 7 confirms an expired/cancelled
  bounded context refuses (never a silent false-negative that would re-open the
  hole).

## Verdict

**PASS.** The shipped empty-head fail-closed hardening behaves exactly as designed
on the real `ShipShipment`/`gateShipmentCompletion`/`validateMemberGateEvidence`
path with real git subprocesses: empty shipment/member heads in a real work tree
under enforcement refuse (holes closed, non-weakening), genuine no-repo / no-repo
member cases still ship/skip (legitimate-empty preserved), and every
broken-repo / timeout / cancel path fails closed. Confirmed non-weakening of the
084 ancestor-aware path (equality fast-path, `isAncestor`, malformed-guard all
green).
