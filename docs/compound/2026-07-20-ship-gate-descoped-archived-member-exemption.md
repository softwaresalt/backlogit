---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Ship-gate member-evidence must exempt descoped members by their PRE-archive state, read from the Markdown source"
source: docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md
doc_type: learning
description: "A two-level shipment gate that expands each manifest item to all descendants (IncludeArchived: true) will pull a descoped child task into the release scope even when the manifest excludes it, then demand per-member gate evidence the descoped task never earned. Because `archived` is a terminal sink with no allowed status transitions, the child cannot be force-gated or un-archived, permanently blocking the parent feature's ship with no operator recourse. The fix exempts members archived from a NON-terminal status (a genuine descope), NOT every archived member: `ArchiveItem` also accepts terminal items and preserves the pre-archive status in `archived_status`, so a `done` member with only fail-open `EventGatePassed{ran:false}` evidence could be archived and wrongly exempted, bypassing the F4 predicate. The discriminator is the PRE-archive state, and it must be read from the Markdown source because the DB index projection omits `archived_status` (loadArtifact is index-first). Missing provenance fails closed."
docline:
    date: 2026-07-20T00:00:00Z
    severity: high
    tags:
        - ship
        - shipment-gate
        - archived
        - descope
        - gate-evidence
        - release-scope
        - index-projection
        - fail-closed
        - dogfooding
---

# Ship-Gate Descoped-Member Exemption

## Context

Surfaced during shipment **099-S** post-merge closure (feature `108-F`
"Size estimation" + 13 member tasks). The deferred `ship_shipment 099-S`
(queue → archive) refused with:

```
member 108.011-T missing passing gate evidence: gate blocked: 108.011-T remains active
```

`108.011-T` was a doctor-reconcile scaffold that got **descoped** during the
`108-F` build: archived directly from `queued` (`archived_status: queued`),
never started, never gated, and correctly excluded from the `099-S` manifest.
Yet the shipment refused to ship.

## Problem

Two independent mechanisms combined into a permanent, unrecoverable block:

1. **Release-scope expansion pulls archived descendants back in.**
   `releaseScopeItemIDs` expands each manifest item to itself **plus all
   descendants**, and `descendantItems` queries with `IncludeArchived: true`.
   So shipping the manifest feature `108-F` re-collected the archived,
   manifest-excluded `108.011-T` into the release scope. Manifest exclusion is
   NOT descope exclusion.

2. **Per-member evidence demand + terminal sink = no recourse.**
   `validateMemberGateEvidence` requires every task/subtask member to be
   terminal AND carry passing gate evidence. The descoped child is terminal
   (`archived`) but has no evidence (it was never completed through the gate).
   `archived` is a terminal sink: the `validate_status_transition` hook reports
   "status archived has no allowed transitions", so the member cannot be
   force-gated (`move --force-gates` fails the pre-hook) and there is no
   un-archive command. The descoped child **permanently blocked the parent
   feature's ship** with no supported operator action.

## Fix

Exempt a member with no passing evidence from the per-member requirement **only
when it was archived from a NON-terminal, in-flight status** — a genuine
descope. Not every archived member.

```go
if item.Status == models.StatusArchived {
    descoped, err := archivedFromNonTerminalStatus(ctx, ws, id)
    if err != nil { return fmt.Errorf("validate member evidence: %s: %w", id, err) }
    if descoped { continue }
}
return shipmentMemberEvidenceError(id, "missing passing gate evidence")
```

`archivedFromNonTerminalStatus` reads `archived_status` from the **Markdown
source** and returns `!isTerminalReleaseStatus(archived_status)`; a missing/empty
`archived_status` fails closed (reported as NOT a descope).

## Two traps that shaped the fix

### 1. The over-broad first draft (caught by Copilot review)

The initial fix was a bare `if item.Status == models.StatusArchived { continue }`
with a rationale asserting "there is no production path to a done/accepted
deliverable without emitting gate evidence." **That invariant is false.**
`ArchiveItem` accepts terminal items too and preserves the pre-archive status in
`archived_status` (the `validate_status_transition` hook is registered only for
`HookUpdateArtifact`, NOT `HookArchiveItem`, so `done → archived` is allowed). A
member driven to `done` whose only "pass" is a fail-open
`EventGatePassed{ran:false}` — rejected by the F4 predicate
(`TestValidateMemberGateEvidence_FailOpenNoRunRejected`) — would be **rejected
before archival but accepted after archival** by the bare check. That is an
evidence-predicate bypass. The discriminator must be the **pre-archive state**
(`archived_status`), not the bare `archived` status.

### 2. The DB index omits `archived_status` (index-first field-absence trap)

`selectCols` / `scanArtifactRow` do not project `archived_status`, and
`loadArtifact` is index-first (`bldb.GetItem`, disk fallback only on
`ErrNotFound`). So `item.ArchivedStatus` is **empty** whenever the gate loads a
member through the normal path. Any gate/decision logic that needs a field the
index does not carry MUST read the Markdown source directly (via
`FindArtifactPath` + `os.ReadFile` + `models.ParseFrontmatter`), mirroring
`UnarchiveItem`. Trusting the index projection here would have silently made the
"non-terminal" predicate always false-ish and re-broken the exemption.

## Rules

- **Descope is a first-class gate concept.** When a gate expands scope via a
  descendant walk with `IncludeArchived: true`, a manifest-excluded but
  descoped descendant WILL re-enter scope. Decide exemption at the gate, not by
  assuming manifest exclusion removes it.
- **Key evidence exemptions on the PRE-archive state**, never the bare
  `archived` status — `archived` erases the distinction between "descoped before
  completion" and "completed then retired."
- **Read fields the index does not project from the Markdown source.**
  Index-first loaders + partial `selectCols` silently return zero-valued fields;
  gate logic that depends on such a field must go to disk.
- **Fail closed on missing provenance.** A missing `archived_status` is not a
  proven descope; refuse rather than exempt.
- **A false safety invariant in a comment is a latent bug.** The over-broad
  draft was "correct" only under an invariant that the archive path violates.
  State invariants precisely and test the boundary (added
  `TestValidateMemberGateEvidence_ArchivedFromDoneNotExempt`).

## Evidence

- Fix PR #263 (merge `84915a4`); closure PR #264 (merge `28770ca`).
- `internal/core/shipment_gate.go`: `validateMemberGateEvidence`,
  `archivedFromNonTerminalStatus`.
- `internal/core/shipment_gate_test.go`:
  `TestValidateMemberGateEvidence_DescopedArchivedMemberExempt`,
  `TestValidateMemberGateEvidence_ArchivedFromDoneNotExempt`,
  `TestValidateMemberGateEvidence_FailOpenNoRunRejected` (unchanged, still green).
