---
chunk_strategy: h1-h2-h3
description: Compacted Ship/Stage/Orchestrator session memory for release units 101-S through 114-S (2026-07-22 through 2026-07-31) — dark-mode P1 index bundle, plan-review dispatch enforcement, v1.7.0 cut, MCP list_items parity, return-to-queued transitions, markdownlint P-008 provisioning, docline relative-root fix, durable_writes fsync protocol and second-layer hardening, archive re-persist field-drop fix, complexity metadata, and formal-gate foundations (including the 114-S P-015 partial-feature-shipment incident and recovery).
doc_type: memory
docline:
    ms.date: 2026-08-01T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-08-01-ship-batch-101S-114S-compacted.md
title: Compacted memory - 101-S through 114-S ship batch
---

## Summary

Twelve consecutive shipments plus one dark-mode feature bundle and one platform
release, each shipped and closed via the standard staging→feature→closure PR
cycle (merge-commit-only, P-009):

* **101-S** (feature `118-F`) — DARK_MODE p1 index bundle. Added composite SQLite
  index `idx_items_parent_type_id` for the batched task-children rollup query
  plan, and corrected a misleading autocommit-read-path lock comment. Impl PR
  #281 merge `63827213` (Copilot clean first round, 4-persona adversarial panel
  found a verified-false-positive Constitution scope finding from a gitignored
  `diff.txt`); closure PR #282 merge `a9ba323a`. Retained the narrow
  `idx_items_parent` index as a conservative default alongside the new composite
  (planner preference with both present is not proof of non-redundancy).
* **102-S** (feature `119-F`) — end-to-end plan-review dispatch enforcement. PR
  #287 merge `130a10a1`. Established the durable machine-readable governance
  field contract: a plan's `## Plan Review` section must carry literal
  `dispatch_mode:`/`decision:`/`operator_authorization:` fields (prose alone is
  insufficient because PASS/FAIL/ADVISORY share dispatch_mode values); when
  multiple `## Plan Review` sections exist, only the final one in document order
  governs. 4 review-fix cycles used (cap 3) — accepted because all findings were
  substantive correctness issues.
* **103-S** (feature `122-F`) — MCP `backlogit_list_items` priority/owner filter
  parity with the CLI, decided via deliberation `053-DL`. PR #289 staging merge
  `8b4e924c`, PR #290 feature merge `311b3840`, PR #291 closure merge `7daf8c30`
  (also tagged **v1.7.0** — 6 platform binaries, 304 commits since v1.6.0, no
  breaking changes), PR #292 stash-intake merge `09708da8`. Data layer needed
  zero changes (`QueryFilters.Owner`/`Priority` already existed); only the MCP
  schema+handler needed wiring. New parity-lock test lives in `internal/cli`
  (avoids an import cycle) using a denylist of output-only flags so future CLI
  filter flags automatically fail the test without a maintained allowlist.
  Surfaced stash `BD8DBB85` (state-machine `blocked`/`active` has no path back to
  `queued`, contradicting the doctor doc) for the next Stage cycle.
* **104-S** (feature `124-F`) — resolved `BD8DBB85`: added `queued` as a valid
  target from both `active` and `blocked` in the default transition maps
  (`internal/config/defaults.go` + `internal/hooks/builtin_pre.go`, kept in sync
  by a `reflect.DeepEqual` guard test), plus a load-time
  `upgradeLegacyTransitions` normalizer so already-persisted configs adopt the
  wider map (deep-equal against a frozen prior-default snapshot; customized maps
  are preserved, not overwritten). PR #293 staging merge `70712c2e`, PR #294 code
  merge `96664088`, PR #295 closure merge `369e862a`. Gotcha: a ship subagent
  archived done tasks in the working tree without committing them to the feature
  branch — recovered by committing the reconciliation in the closure PR;
  convention is item done/archival committed on the feature branch, shipment
  queue→archive deferred to post-merge `ship_shipment`.
* **106-S** (feature `126-F`) — provisioned markdownlint (P-008) tooling
  repo-wide. PR #300 merge `59269785` (single consolidated
  `chore/stage-106-S` branch carrying both planning and implementation, operator
  directed). Root fix: docline artifacts carry both frontmatter `title:` and a
  body `# H1`; markdownlint's default MD025 `front_matter_title` regex counts
  `title:` as the H1 so the body H1 double-counts. Retargeting MD025's
  `front_matter_title` to a sentinel `_title` key (no artifact has it) — while
  leaving MD041 on its default so `title:` still credits it — collapsed 250
  violations to 21 with zero file edits, then to 0/1839 after 21 structural
  fixes (20 `SKILL.md` leading-H1, 1 heading-level jump). Gate is
  `scripts/md-lint.sh` (`npx markdownlint-cli2@0.23.1`), SHA-pinned Node 22
  action, hard-fails CI; promotion to a branch-protection required check
  deferred to stash `918BCDAF`. Non-converging Copilot reviewer (4→2→3→4→5→5→0
  finding counts) — operator directive was to fix genuine bugs/fail-open gate
  guards and accept pure doc-nits as backlog; terminal cycle landed 0 new
  threads. Closure anomaly: `shipment ship 106-S` failed on "refusing to write
  archived artifact without provenance" because linked deliberation `054-DL` was
  already archived; resolved via a direct `backlogit archive 106-S` call.
* **107-S** (feature `127-F`) — fixed `docline.collectInScopeDocs` computing
  `filepath.Rel(root, p)` against a possibly-relative `root` while walking an
  absolute base; under the MCP server default `RootPath == "."` this raised
  `can't make <abs> relative to "."` on Windows. Fix: absolutize the Rel base
  once (`filepath.Abs(root)`) before the WalkDir callback, mirroring the
  existing `ValidateApplyPath` idiom. PR #303 merge `8a757d5e`. Runtime-verified
  via both the CLI (already absolutized, no regression) and a live MCP stdio
  session (`backlogit_docs_lint` → `valid:true, 0 violations`) — the exact
  failing condition. No follow-up stashes.
* **109-S** (feature `123-F`) — opt-in `durable_writes` fsync durability
  protocol (9 TDD units). Shipped in an operator-directed dark-mode posture
  (activation-phrase deviation noted: literal command was "run in dark_mode",
  not the P-017 canonical trigger phrase — treated as a dark-mode request
  anyway, recorded as a process deviation). PR #308 feature merge `e0ae3546`, PR
  #309 closure merge `688056fa`. Established the durable_writes two-class
  contract (`internal/errors/durability_errors.go`): `ErrWriteNotApplied` =
  safe-retry/pre-commit, `ErrWriteIndeterminate` = never-roll-back/post-commit —
  "commit-then-surface" is the governing rule (compound
  `2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`). 5
  findings (F16-F20) deferred past the §1.8 three-cycle cap to follow-up stash
  `50471E28` (all triple-gated: durable ON + real fsync failure + retry). Closure
  review caught a real data-loss bug: archiving `123-F` dropped its `spike_ref`
  link to `120-F` — restored by hand, root cause filed as stash `7A965F8A`
  (`ArchiveItem` re-persist drops modeled `item_links` from frontmatter).
* **110-S** (feature `129-F`) — fixed the archive/shipment re-persist
  field-drop: `attachCommitToItems` now does a single `findArtifact` (Markdown)
  reload for both the archived-skip guard and the mutate-then-persist step
  (previously split-loaded, which raced DB vs. Markdown state), and skips
  already-archived items outright (re-stamping them would trip the
  archived-artifact write guard). PR #312 merge
  `f32c9f7847f9cee9428ff8b76f0d7778748f0944`. Both reconcile gates PROCEED.
* **111-S** (feature `130-F`, staged from stash `50471E28` — the 109-S
  deferral) — durable_writes second-layer hardening across 5 sites (U1-U5):
  `UnarchiveItem` non-git-restore indeterminate reconciliation, explicit
  dependency-caller indeterminate reconciliation, parent-flush re-attempt on
  durable append retry (moved pre-write into `mkdirAllDurable`, not post-write —
  post-write placement wrongly produces `ErrWriteIndeterminate` on retry),
  existing-dir re-fsync on `mkdirAllDurable` retry, and MCP `append_comment`
  durability-class-to-outcome mapping (`gate_errors.go` envelope
  `write_not_applied`/`write_indeterminate` + `retryable`). Plan-review needed 2
  attempts (attempt 1 FAIL: 2 P1s on the U5 outcome contract and an
  unimplemented exactly-once retry test claim). PR #315 merge `d1be5117`.
  Deferred: extract a shared durable-mkdir primitive into an `fsutil` leaf
  (stash `345297B2`, explicitly not done in 111-S scope).
* **113-S** (feature `132-F`, staged from stash `D46F3B0C`) — optional task
  complexity metadata (`trivial`/`low`/`medium`/`high`, task-only, stored in
  `custom_fields.complexity` projected to `items.complexity`). Built on branch
  `feat/132-F-complexity-metadata` through 8 tasks (`132.001-T`…`132.008-T`)
  spanning WIT metadata, body-preserving setter, schema evolution, SQLite
  projection/filtering, CLI/MCP surfaces, and docs. Local review found P1/P2s
  on legacy header-def upgrade, task-only projection enforcement, MCP
  non-string mutation clearing, and priority wording — fixed in `12ea3e36`,
  reaching `READY`. Confirmed archived (query verified 2026-08-01); full
  closure record at `docs/closure/2026-07-30-113-S-complexity-metadata-closure.md`.
* **114-S** (feature `106-F` tasks F2/F3 only — `106-F` itself stays `active`,
  spanning F1-F6 across future cycles) — canonical serialization + SHA-256 hash
  primitive (`internal/canonical/`, stdlib-only leaf; deliberate RFC 8785
  divergence: map keys sorted by Go/UTF-8 byte order) and an authoritative
  status taxonomy (`internal/core/status_taxonomy.go`; two PINNED, NOT-unified
  truth tables — 6-status cascade-terminal vs. 4-status releasable, differing
  by `shipped`/`abandoned`). PR #323 staging merge `809e741d`, PR #324 feature
  merge `f8870f864d596a1f3593405e54396d8129aa8871`, PR #325 closure. **P-015
  incident**: the closure agent mistakenly ran the native cascade
  `backlogit shipment ship 114-S` on this PARTIAL-feature shipment (`106-F`
  intentionally excludes unharvested F1/F4/F5/F6); the cascade archived `106-F`
  itself. Recovered via `git revert` of the cascade commit, re-verified `106-F`
  back in queue as `active`, then reclosed with 3 single-artifact ops
  (`update --commit`, `move --status done`, `archive`) instead of the cascade —
  surfaced to the operator for explicit authorization per P-015 (recovery is not
  self-authorizing). This incident is the direct root cause that shipment
  **115-S/feature 133-F** (the current release unit) fixes at the code level;
  see `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`
  (updated 2026-08-01 to record the fix) and the now-updated P-015 policy text
  in `.github/policies/workflow-policies.md`.

## Archived originals

* `docs/archive/memory/2026-07-22-118-F-p1-index-bundle-memory.md`
* `docs/archive/memory/2026-07-23-102-S-post-merge-closure-memory.md`
* `docs/archive/memory/2026-07-23-orchestrator-v170-release-session-memory.md`
* `docs/archive/memory/2026-07-23-shipment-103s-closure-memory.md`
* `docs/archive/memory/2026-07-23-shipment-104s-closure-memory.md`
* `docs/archive/memory/2026-07-26-106-S-post-merge-closure-memory.md`
* `docs/archive/memory/2026-07-27-107-S-post-merge-closure-memory.md`
* `docs/archive/memory/2026-07-28-110-S-ship-closure-memory.md`
* `docs/archive/memory/2026-07-28-111-S-closure-memory.md`
* `docs/archive/memory/2026-07-28-130-F-durable-writes-second-layer-hardening-stage-memory.md`
* `docs/archive/memory/2026-07-28-ship-109-S-durable-writes-closure-memory.md`
* `docs/archive/memory/2026-07-29-ship-sequence-spike-memory.md`
* `docs/archive/memory/2026-07-29-stage-complexity-metadata-memory.md`
* `docs/archive/memory/2026-07-30-ship-113-S-pr-ready-memory.md`
* `docs/archive/memory/2026-07-31-ship-114-S-formal-gate-foundations-memory.md`

## Decisions and learnings

* **Durable_writes two-class contract lineage (109-S → 111-S).** The
  never-roll-back-indeterminate invariant is the load-bearing rule across every
  durable-write call site; parent/ancestor re-fsync retries must happen
  pre-write (inside `mkdirAllDurable`), never post-write, or they wrongly
  produce `ErrWriteIndeterminate` on a retry that should be safely re-attemptable.
* **P-015 partial-feature-shipment lineage (114-S → 133-F).** 114-S's incident
  (native cascade archives a non-member covering feature on a partial-feature
  shipment) was first handled by agent-process discipline (single-artifact
  manual close, git-revert-on-cascade recovery). 133-F later fixed this at the
  code level in `collectArchiveCandidateIDs` (explicit-manifest-membership
  gating) with a snapshot/restore safety net
  (`snapshotNonMemberFeatureStatuses`/`restoreRolledUpNonMemberFeatures`) for the
  separate generic parent-status cascade. The manual procedure is now legacy
  defense-in-depth, not the primary control.
* **Machine-readable governance contracts (102-S).** When a governance rule has
  a machine consumer checking literal values, the producer must emit exact
  labeled fields, not prose satisfying the same intent.
* **Composite index does not obsolete a narrower one (101-S).** Keep the
  narrower index as a conservative default while both exist unless a
  post-removal query-plan benchmark proves it redundant.
* **Config default-map widening needs two layers (104-S).** A code default
  change alone does not reach already-persisted configs; add a load-time
  upgrade normalizer keyed off a frozen prior-default snapshot (deep-equal),
  so customized maps are preserved and only exact-default configs are upgraded.
* **markdownlint frontmatter/H1 double-count (106-S).** Retarget MD025's
  `front_matter_title` regex to a sentinel key rather than editing every file's
  heading structure; keep MD041 on its default.
* **Absolutize before `filepath.Rel` when walking absolute paths (107-S).**
  Match the walked path's own absoluteness rather than trying to make both
  sides relative.

## Failed approaches / gotchas (carried forward)

* **gofmt on Windows CRLF (recurring, 106-S/110-S/113-S/114-S).** Whole-repo
  `gofmt -l .` reports ~90-96 pre-existing files as unformatted due to
  `core.autocrlf=true`; the gate requires zero findings, verify on
  LF-normalized, BOM-free blob copies (`git show HEAD:file`) rather than
  relaxing the gate. See
  `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`.
* **`shipment ship` requires `shipment claim` first** (104-S) — shipping a
  `queued` shipment errors "shipment status conflict".
  **`shipment ship`/`ship_shipment` fails on already-archived linked
  deliberations** (106-S) — "refusing to write archived artifact without
  provenance"; resolve with a direct `backlogit archive <id>` call instead.
* **Ship subagents share the workspace but may not commit archival state to the
  feature branch** (104-S) — verify `.backlogit/` changes are actually committed,
  not just applied to the working tree.
* **`backlogit query` logs an `INFO config loaded` line to stderr** before JSON
  output (101-S) — redirect `2>$null`/`2>&1` before parsing.
* **Status transition graph is guarded**: `queued → done` direct is rejected;
  valid path is `queued → active → done`; `done → archived` only via archive /
  `shipment ship`, never a status move (101-S; widened by 104-S to add
  `→queued` from `active`/`blocked`).
* **GraphQL thread IDs in PowerShell**: assign `$t.id` to a scalar variable
  first — interpolating `-f tid=$t.id` inside a hashtable member access expands
  to the literal type name (109-S).
* **Copilot bot GraphQL login is `copilot-pull-request-reviewer`** (no `[bot]`
  suffix) — the suffixed form is REST-only (109-S/110-S).

## Follow-ups (status at each source session's close — re-verify via `fetch_stash`/`list_items` before acting; NOT re-audited during this compaction)

* `918BCDAF` (medium) — promote repo-wide `md-lint` to a branch-protection
  required check (admin action).
* `03EFBBAC` (medium) — dedicated remediation task for the 21-file markdownlint
  structural fixes; reconcile plan↔task unit numbering.
* `C63AF32E` (low) — residual doc-wording accuracy (`blocking/required` vs.
  `hard-fails CI`/`required-check`; `gitignore`-corpus phrasing).
* `7F0A6E89` / `6FA0829B` (low) — external autoharness `*.tmpl` template parity
  writes, blocked by Principle IV (outside workspace).
* `345297B2` (low) — extract shared durable-mkdir primitive into an `fsutil`
  leaf (109-S/111-S deferral).
* `7A965F8A` — `ArchiveItem` re-persist drops modeled `item_links` from
  frontmatter (109-S; related to prior stash `D04D63D0`).
* `016.001-R` — pre-existing orphaned review artifact, deferred triage
  (mentioned recurring across 101-S through 106-S doctor runs as unrelated
  noise).
* `131CEAE4` / `9D5BB492` (101-S) — durability/fsync redesign and crash-window
  exactly-once spikes, routed to standalone investigations (may already be
  superseded by the 109-S/111-S durable_writes work).
* `8CD8F46A` (101-S/102-S) — persona-dispatch-path deliberation routing.
* `001-SP` — ship-sequence manifest spike harvested from `16FD6CC0`; findings
  recommend deferring a standalone `ship_sequence.jsonl` manifest.
