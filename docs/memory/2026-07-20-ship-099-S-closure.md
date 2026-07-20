---
doc_type: memory
docline:
  date: 2026-07-20
  status: active
  tags: [ship, closure, 099-S, 108-F, shipment-gate, descope, archived, size-estimation]
schema_version: "1.0"
title: Ship 099-S — Closure via Ship-Gate Descoped-Member Exemption Fix (PR #263, #264)
---

# Ship 099-S — Closure Session

**Outcome:** `099-S` **shipped and archived**; feature `108-F` "Size estimation"
+ 13 member tasks closed. Blocking ship-gate bug fixed and merged.

## What happened

Post-merge `ship_shipment 099-S` was refused by the two-level ship gate:
`member 108.011-T missing passing gate evidence`. `108.011-T` is a
doctor-reconcile scaffold that was **descoped** (archived from `queued`,
excluded from the manifest), but `releaseScopeItemIDs` expands feature `108-F`
to ALL descendants (`IncludeArchived: true`), re-pulling it into release scope.
`validateMemberGateEvidence` then demanded gate evidence it never earned, and
`archived` is a terminal sink (no transitions) so it could not be force-gated →
permanent block, no operator recourse.

## Fix (PR #263 → merge `84915a4`)

- `internal/core/shipment_gate.go`: in `validateMemberGateEvidence`, at the
  `latest == nil` branch, exempt a member **only when archived from a
  NON-terminal status** (genuine descope), via new helper
  `archivedFromNonTerminalStatus` which reads `archived_status` from the
  Markdown source (the DB index projection omits it; `loadArtifact` is
  index-first). Missing `archived_status` fails closed.
- Copilot review cycle-1 (VALID, HIGH quality): the first draft exempted EVERY
  archived member. `ArchiveItem` accepts terminal items and preserves the
  pre-archive status, and `validate_status_transition` is registered only for
  `HookUpdateArtifact` (not `HookArchiveItem`), so a `done` member with only
  fail-open `EventGatePassed{ran:false}` evidence could be archived and wrongly
  exempted → F4 bypass. Refined to key on the pre-archive state; added
  `TestValidateMemberGateEvidence_ArchivedFromDoneNotExempt`. Replied + resolved
  the thread; fresh Copilot review on `014339e` clean.
- TDD: `TestValidateMemberGateEvidence_DescopedArchivedMemberExempt` (+2 safety
  guards) and the archived-from-done regression. Gates GREEN: `go test ./...`,
  `go vet ./internal/core/...`, `golangci-lint run internal/core/...`, `gofmt`.

## Closure (PR #264 → merge `28770ca`)

- Rebuilt `backlogit.exe` from merged HEAD (post-merge-fresh-binary rule),
  synced index, re-ran `backlogit shipment ship 099-S` → `shipment_status:
  shipped`; `archived_ids` = 13 tasks + `099-S` + `108-F` (15); `returned_ids:
  []`. `108.011-T` exempted.
- Post-mode reconcile PASS: queue drained of `099-S`/`108*`;
  `archive/099-S.md` `archived_status: shipped`; `108-F` `archived_status: done`.
- Committed workspace archival (queue→archive rename + status stamps +
  hooks_queue.jsonl) on `chore/close-099-S` (`1df8e2a`) → PR #264 → own §1.10
  P-014 gate (0 findings) → operator approved → merged.

## Design Q&A (operator)

- **archive reason vs. archive status:** a structured `archived_reason` enum is
  redundant. Disposition is already encoded by the status vocabulary and
  preserved in `archived_status`; the gate needs pre-archive STATE (not intent);
  a reason enum overlaps the status enum (rejected/abandoned) creating a
  two-source-of-truth smell. Open-ended "why" narrative belongs in a free-text
  comment/log event, not a field. Decision: **inference-only on existing state,
  no new field** — exactly what #263 implements.

## Files modified

Shipped in PR #263 / #264:

- `internal/core/shipment_gate.go` (+`archivedFromNonTerminalStatus`, refined
  exemption)
- `internal/core/shipment_gate_test.go` (2 new tests)
- `.backlogit/archive/099-S.md` (queue→archive), `108-F` + 13 tasks stamped,
  `.backlogit/hooks_queue.jsonl`

Closure follow-ups in PR #265 (this session):

- `docs/memory/2026-07-20-ship-099-S-closure.md` (this file)
- `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`
- `.backlogit/stash.jsonl` (files perf follow-up `47ED88ED`)
- `.backlogit/archive/099-S.md` (materializes the `099-S supersedes 100-S` link
  into frontmatter so it survives the next index rehydration)

## Compound learning

`docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md` —
descope as a first-class gate concept; key exemptions on pre-archive state; read
index-omitted fields from Markdown source; fail closed on missing provenance;
false-invariant-in-comment is a latent bug.

## Next steps / open items

- Deferred `get_queue` size-composition perf follow-up **filed to stash as
  `47ED88ED`** (`O(aggregates × members × all-artifacts)` `WalkDir` scan;
  108-F SE-4 / Copilot cycle-3 `SIEy9`). Earlier 099-S perf/parity defers are
  already stashed (`D5FA1EE9`, `387DE4BF`, `131CEAE4`, `9D5BB492`).
- Engram-first search policy honored where the daemon was reachable; a session
  note on engram usage lives in the session `files/`.
