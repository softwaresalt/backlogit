---
chunk_strategy: h1-h2-h3
description: "Ship 109-S (feature 123-F durable_writes) executed in dark factory mode: build via TDD, four Copilot review cycles (F1-F15 fixed, F16-F20 deferred to 50471E28), dark-mode merge PR #308 at e0ae3546, and two-PR post-merge closure with closure PR #309 merged at 688056fa (restored a dropped 123-F->120-F spike_ref link; codec bug filed as 7A965F8A)."
doc_type: memory
docline:
  date: 2026-07-28T00:00:00Z
  ms.topic: closure
schema_version: "1.0"
source: docs/memory/2026-07-28/ship-109-S-durable-writes-closure-memory.md
title: "Ship 109-S durable_writes — dark-mode closure session memory"
---

# Ship 109-S durable_writes — dark-mode closure session memory

## Scope

Shipped queued shipment **109-S** (feature **123-F** — opt-in `durable_writes`
fsync durability protocol; 9 TDD units U1-U9) in **dark factory mode (P-017)**.
Command: `Ship next; run in dark_mode`.

## Task IDs

- Feature: **123-F**; tasks 123.001-T .. 123.009-T (U1-U9), all `done` + archived.
- Shipment: **109-S** — `archived_status: shipped`, commit `e0ae3546`.

## Merge / commit SHAs

- Feature merge (PR #308): merge commit **`e0ae3546`** (feature HEAD `93f8501f`).
- Closure PR #309: merge commit **`688056fa`** (closure HEAD `ca44df71`).

## Files modified (closure)

- Backlog state: `109-S.md` queue->archive; 123-F + 123.001-T..009-T commit-stamped.
- `docs/closure/2026-07-28-109-S-durable-writes-closure.md` (new).
- `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` (new).
- `.backlogit/archive/123-F.md` — restored dropped `spike_ref` link to 120-F.

## Decisions and rationale

- **Cycle-4 deferral (F16-F20):** past the §1.8 three-cycle review-fix limit,
  and all five findings are triple-gated (durable ON + real fsync failure +
  retry), inert by default, and reconcilable (MD is source of truth; SQLite
  index self-heals on sync). Dispositioned as P2 follow-up stash **50471E28**
  rather than a fourth fix cycle.
- **Dark-mode merges:** in-scope 109-S PRs (#308, #309) had
  `merge_approval_pre_authorized=true`; the dark-mode activation record is the
  P-014 signal per github-pr-automation §1.9.6. Both were `NORMAL_MERGE_READY`
  (CLEAN) so no admin fallback (which was NOT pre-authorized). P-001/P-009/
  P-014/P-016 and local review readiness were preserved, not waived.
- **Closure-PR review (4 findings, all valid, all fixed in `ca44df71`):**
  (1) relabel `93f8501f` as reviewed feature HEAD (merge is `e0ae3546`);
  (2) expand Monitoring into the required release-observability manual
  checklist (pack is enabled); (3) qualify `DARK_MODE_COMPLETE` as gated on
  #309 merge; (4) **real data loss** — archiving 123-F dropped its `spike_ref`
  link to 120-F. Restored the link block by hand (verified rehydration into
  `item_links` after sync) and filed the root-cause codec bug as stash
  **7A965F8A**.

## Failed approaches / gotchas

- GraphQL per-thread reply+resolve: PowerShell `$t.id` inside `-f tid=$t.id`
  expands to the literal `System.Collections.Hashtable.id`. Assign
  `$tid = $t.id` to a scalar first.
- `gh pr edit 309 --add-reviewer copilot` fails (`'' not found`); request the
  Copilot bot via GraphQL `requestReviews(botIds:["BOT_kgDOCnlnWA"])`.
- gofmt-on-Windows CRLF noise: verify on LF-normalized BOM-free blobs.

## Follow-ups (open)

1. **50471E28** — durable_writes second-layer hardening (5 acceptance criteria).
2. **7A965F8A** — `ArchiveItem` re-persist drops modeled `item_links` from
   frontmatter (related to D04D63D0).

## Dark-mode events (local records; intercom unreachable)

`DARK_MODE_START` -> `DARK_MODE_SCOPE` -> `LOCAL_REVIEW_READY` ->
`DARK_MODE_MERGE_AUTHORIZED` (x2, #308 + #309) -> `DARK_MODE_COMPLETE`.

## Next steps

- Release unit 109-S is CLOSED. No further Ship work queued.
- Two open follow-up stashes (50471E28, 7A965F8A) are candidates for the next
  `stage next` cycle, alongside pre-existing D04D63D0.
