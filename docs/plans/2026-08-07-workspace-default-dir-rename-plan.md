---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for stash 9370A18C: change the default workspace storage directory to .backlog while keeping every existing .backlogit workspace resolvable, with a closed override set, symlink-safe realpath containment, an immutable resolved root, a complete safety-path inventory, and a previewable idempotent migration.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-07-workspace-default-dir-rename-plan.md
title: 'Default workspace directory rename to .backlog with dual-root compatibility'
---

# Default workspace directory rename to `.backlog` with dual-root compatibility

Source deliberation:
`docs/decisions/2026-08-07-workspace-default-dir-rename-deliberation.md`.
Origin: stash `9370A18C`.

This release unit ships **after** all formal-gate work (F1, F4, F6, F5).

<!-- plan-review-attempt: 2 -->

## Problem Frame

The workspace storage directory is named `.backlogit`. The operator wants the
default to become `.backlog`. The primary seam is
`internal/core/workspace.go:55-62` (`WorkspaceStorageRoot`), with secondary
literals at `internal/cli/root.go:297` and `internal/cli/migrate.go:127`.

Discovery (`resolveWorkspaceRoot`, `internal/core/workspace.go:216-250`) keys on
config-file presence with the directory name supplying the candidate path, and
performs no upward parent walk.

**The risk is not the rename.** Discovery and containment are Principle III/IV
security surfaces; two candidate roots roughly doubles the exposure of the
safety-critical scan set; and the claim that *everything* derives from
`WorkspaceStorageRoot` is **false** — security-relevant paths are additionally
hardcoded in `internal/core/archive.go` (restore path),
`internal/core/shipment_verify.go` (post-ship scan),
`internal/core/migrate_queue.go`, several telemetry read/write paths, and the MCP
server's lazy-init/logs/telemetry/hooks/resources/memories/checkpoints paths.

### Success Criteria

* `backlogit init` creates `.backlog`.
* Every existing `.backlogit` workspace resolves indefinitely with no operator
  action.
* Both roots present with no override → **deterministic hard error** naming both
  paths and both supported resolutions.
* The override cannot express traversal, and no candidate root can be a symlink
  or reparse point that escapes the workspace.
* The resolved storage root is chosen **once** and is immutable for the process,
  so config, database, log, and write paths cannot split.
* Every safety-critical and state-bearing path is inventoried and routed through
  the resolved root.
* `migrate --workspace-dir --dry-run` previews; applying is idempotent; the apply
  step requires explicit operator approval.
* Discovery is verified through the **MCP production path**, not only the CLI.

### Scope Boundaries

**In scope:** the storage-root resolver and its immutability; the closed override
set and its validation; symlink-safe realpath containment; conflict refusal; the
safety-path inventory and its regression guard; `init` default; the migration
mode; the doctor conflict check and its read-only pre-resolution route; MCP
production path resolution and structured errors; agent-facing instruction
updates; user documentation.

**Out of scope:** renaming the module, binary, or MCP server name. Changing the
config file name or the internal layout. **Renaming this repository's own
`.backlogit` directory.** Removing `.backlogit` support. Arbitrary
operator-chosen directory names — the override is a **closed set** (see below).

## Requirements Trace

| Requirement | Source | Unit |
|---|---|---|
| Dual-root resolution with fixed precedence | Deliberation | U2 |
| Closed override set; no arbitrary names | Review B-01 | U2 |
| Symlink/reparse rejection + realpath containment | Review B-01 | U2 |
| Resolve once, store immutably on `Workspace` | Review B-02 | U2 |
| Probes distinguish not-found from indeterminate | Review B-02 | U2 |
| Typed error carrying both candidate paths | Review WR-01 (Go) | U2 |
| Candidate names exported from one owner package | Review Arch P2, WR-02 (Go) | U2, U3 |
| Complete safety-path inventory + literal guard | Review B-03 | U3 |
| New workspaces use `.backlog` | Stash `9370A18C` | U4 |
| Previewable, idempotent, approval-gated migration | `docs/migration-guide.md` | U5 |
| Doctor conflict finding reachable pre-resolution | Review scope P2 | U6 |
| MCP **production** path resolution + structured errors | Review Workspace-1/2 (parity) | U7 |
| Agent-facing instruction inventory | Review Workspace-3 (parity) | U9 |

## Implementation Units

### U1 — Failing resolution and validation matrix (tests)

Red matrix over workspace resolution and override validation. Must be **observed
failing at HEAD**.

Cases: `.backlog` only; `.backlogit` only; both present; neither; override set to
each allowed value; override unset vs empty string (distinguished); override with
`/` or `\`; override `.`; override `..`; override absolute; override
volume-qualified or drive-relative (`C:foo`); override UNC or device path;
override containing NUL or control bytes; override with a Windows case alias
(`.BACKLOG`); a candidate root that is a symlink or junction pointing outside the
workspace; a candidate whose `config.yaml` is unreadable (indeterminate, must not
collapse to "absent").

Files: `internal/core/workspace_dualroot_test.go`.
Posture: test-first (RED).

### U2 — Resolver: closed override set, symlink-safe containment, immutable root (code)

Back the candidate set with a private array and expose it through an accessor
that returns a defensive copy, so no importing package can mutate the
safety-critical scan set or override allowlist at runtime:

```go
// workspaceRootCandidates lists the supported storage-root directory names in
// precedence order. Private and never derived from config; accessed only
// through WorkspaceRootCandidates.
var workspaceRootCandidates = [...]string{".backlog", ".backlogit"}

// WorkspaceRootCandidates returns a fresh copy of the supported storage-root
// directory names in precedence order. Callers must not rely on a shared
// backing array; each call allocates its own copy so no caller can mutate the
// package's closed set.
func WorkspaceRootCandidates() []string {
    out := make([]string, len(workspaceRootCandidates))
    copy(out, workspaceRootCandidates[:])
    return out
}
```

**Override is a closed set.** `BACKLOGIT_WORKSPACE_DIR`, when set and non-empty,
must equal one of `WorkspaceRootCandidates()` exactly (case-sensitively). Any
other value is a hard error. This eliminates the entire traversal class rather
than trying to validate arbitrary names, and is the simplest design consistent
with the operator's simplicity-over-complexity policy. Unset and empty are
distinguished: unset means "use precedence"; empty means "misconfigured" and is
an error.

Precedence: validated override → `.backlog` → `.backlogit`.

Candidate probing must:

* use `Lstat` and **reject** a candidate that is a symlink or reparse point;
* realpath-resolve the candidate and confirm lexical **and** resolved containment
  inside the workspace root;
* require `config.yaml` to be a regular file;
* distinguish "not found" from an indeterminate `os.Stat` error and **fail
  closed** on indeterminate rather than falling through to a lower-precedence
  root;
* treat Windows case-insensitive aliases correctly using `os.SameFile` semantics
  so `.BACKLOG` and `.backlog` are not counted as two distinct roots.

Both candidates present with no override → return
`errors.AmbiguousWorkspaceRootError{Roots []string}` (a **typed** error in
`internal/errors/workspace_errors.go`, so the doctor finding and the MCP mapping
can read the paths programmatically).

**Resolve once.** `NewWorkspace` resolves and canonicalizes the storage root and
stores it on `Workspace`. Every subsequent path derives from that stored value;
nothing re-reads the environment or re-probes the filesystem mid-process.

Files: `internal/core/workspace.go`, `internal/errors/workspace_errors.go`.
Scenarios: U1's matrix turns green.
Posture: test-first.

### U3 — Safety-path inventory and literal guard (code + tests)

Produce a repository-wide inventory of every read, write, scan, lock, restore,
rehydration, telemetry, migration, and post-ship check that resolves a storage
path, and route each through the resolved root or `WorkspaceRootCandidates()`.
Known targets beyond the canonical scan: `internal/core/archive.go` (restore
path), `internal/core/shipment_verify.go` (post-ship scan),
`internal/core/migrate_queue.go`, and the telemetry paths.

The canonical scan set, archive-destination guard, and ID-collision guard consume
`WorkspaceRootCandidates()` **plus** any active override, hardcoded in code and
never config-derived. Add a regression test that **fails on any safety-critical
`.backlogit` string literal** outside the candidate list and outside test
fixtures.

Files: `internal/core/canonical_scan.go`, `internal/core/archive.go`,
`internal/core/shipment_verify.go`.
Scenarios: artifact under either root is scanned; cross-root ID collision
detected; post-ship scan covers both roots; the literal guard fails on a
reintroduced hardcode.
Posture: test-first.

### U4 — `init` default and second-root refusal (code)

`backlogit init` creates `.backlog`. `init` in a directory that already holds a
`.backlogit` workspace **refuses** rather than creating a second root.

Files: `internal/cli/root.go`, `internal/config/defaults.go`.
Scenarios: fresh init creates `.backlog`; init over an existing `.backlogit`
refuses with an actionable message; init remains idempotent.
Posture: test-first.

### U5 — `migrate --workspace-dir` (code)

Red-first: the migration tests are written and observed failing before the
migration code exists. A migration mode following the established UX contract:
`--dry-run` preview writing nothing, apply only after operator approval,
idempotent on re-run, refusing when the destination already exists. `git mv` for
tracked content with a filesystem fallback, mirroring the existing git-aware
artifact move. **The applied move is a pure move with no content rewrites and no
rehydration**, so git rename similarity stays above threshold and
`git log --follow` survives; any index rebuild is a separate, explicit `sync`
invoked by the operator afterwards.

Files: `internal/cli/migrate.go`, `internal/core/migrate_workspace_dir.go`.
Scenarios: dry-run leaves a clean `git status`; apply moves only; re-run is a
byte-identical no-op; destination exists → refusal.
Posture: test-first (RED before implementation), migration-shaped.

### U6 — Doctor conflict check with a read-only pre-resolution route (code)

Add `CheckWorkspaceRootConflict` and a finding type through the existing
`DoctorOptions` boolean pattern. Because U2 makes an ambiguous workspace fail
normal resolution, this check must be reachable through an explicit **read-only
pre-resolution route** — `doctor` detects the conflict from the candidate list
without requiring a resolved workspace — and that route is tested through the
real CLI and MCP entry points, not only through a direct core call. The MCP
`backlogit_doctor` tool has an explicit, hardcoded schema and handler
(`internal/mcp/tools.go`, tool registration and `handleDoctor`'s manual
`core.DoctorOptions` field mapping): without adding a
`check_workspace_root_conflict` boolean to both the tool's parameter schema and
the handler's option mapping, the new check cannot be requested over MCP even
after the `core.DoctorOptions` field exists. Advisory, never blocking.

Files: `internal/core/doctor.go`, `internal/cli/doctor.go`,
`internal/mcp/tools.go`.
Scenarios: conflict detected via CLI `doctor`; conflict detected via the MCP
`backlogit_doctor` tool with `check_workspace_root_conflict: true`; no conflict
→ no finding on either surface; schema/handler mapping test asserting the new
boolean reaches `core.DoctorOptions`.
Posture: test-first.
Scenarios: both roots → finding via the CLI entry point; one root → none; check
disabled → none.
Posture: test-first.

### U7 — MCP production path resolution and structured errors (code)

Route the MCP server's **production** paths — prechecks, lazy initialization,
logs, telemetry, hooks, resources, memories, checkpoints — through the shared
resolved root rather than directly constructing `.backlogit` paths. Add a
structured `workspace_root_ambiguous` MCP error carrying both candidate paths,
the supported resolutions, override guidance, and `retryable: false`, and update
`workspace_not_initialized` so its message no longer names only `.backlogit`.

Files: `internal/mcp/server.go`, `internal/mcp/errors.go`.
Scenarios: `.backlog`-only workspace works end to end over MCP including the
relative `RootPath: "."` default; legacy `.backlogit` works; conflict surfaces
the structured error; no MCP path splits across roots.
Posture: test-first.

### U8 — User-facing documentation (docs)

Update `README.md`, `docs/migration-guide.md`, and `AGENTS.md` for the new
default, the precedence rule, the closed override set, the conflict behavior, and
the migration command. Verify every storage-contract sentence against the actual
read/write code path — a path-existence-only audit already passed factually wrong
prose once.

Files: `README.md`, `docs/migration-guide.md`, `AGENTS.md`.
Posture: documentation.

### U9 — Agent-facing instruction and registry inventory (config)

Inventory **every** `backlogit*.instructions.md` file and every registry consumer
that names a storage directory, and update each to state `.backlog` as the
default, `.backlogit` as legacy-and-supported, the conflict refusal, and the
override behavior — while preserving legacy references that correctly describe
existing workspaces. The inventory is a deliverable: the unit lists the exact
files it changed. No wildcard sweep.

Files: `.github/instructions/backlogit.instructions.md`,
`.github/instructions/backlogit-sql-schema.instructions.md`,
`.github/instructions/backlogit-yaml-header-tooling.instructions.md`,
`.github/instructions/backlog-integration.instructions.md`,
`.autoharness/backlog-registry.yaml`.
Posture: configuration.

## Dependency Graph

```text
U1 ──> U2 ──> U3 ──> U4 ──> U5 ──> U6 ──> U7 ──> U8 ──> U9
```

Strictly sequential. **U3 deliberately precedes every unit that can create or
move a second root.** U7 follows the code units so it validates converged
behavior. U8 and U9 last.

## Decisions and Rationale

* **Dual-root over a hard rename** — every existing workspace, including this
  repository's own, would otherwise break with no diagnostic.
* **Closed override set over a validated free-form name** — simplest design that
  removes the entire traversal class. An arbitrary custom root would also have to
  be threaded into every safety guard, which is complexity with no demonstrated
  need.
* **Refuse rather than pick a winner** — an ordering-dependent winner is a silent
  data-loss vector.
* **Fail closed on indeterminate probes** — collapsing every `os.Stat` error to
  "absent" would silently fall through from an unreadable higher-precedence root.
* **Resolve once, store immutably** — a long-lived MCP process that re-resolves
  could split config, database, and log paths mid-session.
* **Exported candidate list** — keeps the dual-root concept owned by one package
  instead of leaking literals across modules, and gives the literal guard
  something to whitelist.
* **Pure-move commit, no rehydration** — mixing the move with content rewrites or
  an index rebuild can push git rename similarity below threshold.
* **Both names keep test coverage** — dual-root resolution is what needs
  covering, so a blanket find-and-replace would delete the coverage.

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| Symlink or junction redirects writes outside the workspace | **high** | `Lstat` rejection plus realpath containment in U2, tested |
| Malformed override selects an unintended root | **high** | Closed override set; unset vs empty distinguished; hard error otherwise |
| Omitted hardcoded paths split state or bypass a safety scan | **high** | U3 inventory plus a literal-guard regression test |
| Live re-resolution splits paths mid-process | high | Resolve once in `NewWorkspace`, store immutably |
| Indeterminate stat collapses to "absent" | high | Probes distinguish the cases and fail closed |
| Discovery defect ships green via CLI-only CI | **high** | U7 exercises the MCP production path including relative `RootPath` |
| Silent winner when both roots exist | high | Typed `AmbiguousWorkspaceRootError`, asserted |
| Doctor finding unreachable after U2 | medium | U6 adds an explicit read-only pre-resolution route, tested through real entry points |
| `git log --follow` breaks | medium | Pure-move commit; `git mv` with filesystem fallback |
| Agents directed to the wrong root | high | U9 inventories every agent-facing instruction file and registry consumer |
| Docs left factually wrong | medium | U8 requires verification against read/write code paths |
| Merging does not make the rename live for this repo | low | Recorded as residual exposure; this repo keeps `.backlogit` by design |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Safety-First Go | No `unsafe`. Typed `AmbiguousWorkspaceRootError` in `internal/errors`; wrapped errors. |
| II. Test-First | U1 is an explicit RED matrix; U5 is explicitly RED before implementation. |
| **III. Workspace Isolation** | **Primary review lens.** Closed override set, `Lstat` symlink rejection, realpath containment, fail-closed indeterminate probes, immutable resolved root. |
| IV. CLI Containment | The migration moves a directory **inside** the workspace tree only. |
| V. Structured Observability | Conflict refusal is a typed error carrying both paths; MCP surfaces it structurally. |
| VI. Single Responsibility | No new dependencies; closed override set avoids speculative generality. |
| IX. Git-Friendly Persistence | Pure-move commit preserves `git log --follow`. |
| X. Context Efficiency | No change to query shape. |

No violations.

## Plan Hardening Signals

* default change with live-workspace impact — **yes**
* migration with a destructive move step — **yes**
* touches a security surface (path resolution and containment) — **yes**
* irreversible if performed incorrectly — **yes**

Requires plan hardening: yes

## Runtime Verification and Closure

* **Verification surface:** `init`; every command that resolves a workspace; the
  **MCP production path** with relative `RootPath`; `migrate --workspace-dir`;
  `doctor`.
* **Scenarios:** the full U1 matrix; migration dry-run → apply → re-run; doctor
  conflict finding through CLI and MCP; a legacy workspace untouched by an
  upgrade; MCP read/write against `.backlog`, legacy, and conflict states.
* **Rollback:** `.backlogit` remains fully supported, so rollback is a plain
  revert. A migrated workspace is rolled back by moving the directory back — the
  internal layout is unchanged.
* **Closure artifact:** must record the precedence rule, the closed override set,
  the conflict behavior, the forward-compatibility guarantee, the safety-path
  inventory, and the pinned-binary residual exposure.

## Plan Hardening

Hardening was required (four signals). This is the highest-risk unit in the
staging cycle.

### Protected Invariants (must not regress)

1. An existing `.backlogit` workspace resolves with **no** operator action,
   indefinitely.
2. Path containment is never weakened; the override cannot express traversal and
   no candidate root may be a symlink or reparse point escaping the workspace.
3. The canonical scan set, archive-destination guard, ID-collision guard, and
   post-ship scan cover **both** roots, hardcoded in code, never config-derived.
4. Every state-bearing path — CLI **and** MCP — derives from the one resolved
   root.
5. Both roots present without an override is a **refusal**, never a silent choice.
6. An indeterminate probe fails closed; it never falls through to a
   lower-precedence root.
7. The migration is idempotent, refuses when the destination exists, and requires
   explicit operator approval before apply.
8. The internal layout inside the storage root is unchanged.
9. This repository's own `.backlogit` directory is **not** renamed by this unit.

### Learnings and Instructions Consulted

* Precedent `5f86ee9d` — the safety-critical scan set must not derive from
  mutable config.
* MCP relative-root lesson — MCP defaults `RootPath` to `"."`; CLI-only CI let a
  `filepath.Rel` defect ship green.
* `docs/compound/go-patterns/f015-shipment-stash-patterns.md` — dual-reader rule;
  document the collision winner explicitly.
* `docs/compound/best-practices/git-aware-backlog-artifact-archival-preserves-follow-history-2026-07-10.md`
* `docs/migration-guide.md` — the established migration UX.
* `internal/config/loader.go:62-86,132-150` — containment guard and legacy-upgrade
  shim style.
* `.github/instructions/constitution.instructions.md` (III, IV, VII),
  `.github/instructions/strict-safety.instructions.md`

### Risky Actions (carry forward to Ship)

| # | ProposedAction | Targets | change_kind | ActionRisk | rollback | approval_required |
|---|---|---|---|---|---|---|
| A1 | Change workspace path resolution | `internal/core/workspace.go` | security surface (Principle III) | **high** | Plain revert | **yes** |
| A2 | Add an environment-driven root selector | `internal/config/loader.go` | security surface | **high** | Unset the variable; plain revert | **yes** |
| A3 | Move a workspace directory on disk | `migrate --workspace-dir` | **destructive** | **destructive** | Move the directory back; layout unchanged | **yes — operator approval required before any apply run; `--dry-run` must be run and reviewed first** |
| A4 | Change the `init` default | `internal/cli/root.go` | default change | moderate | Plain revert | no |
| A5 | Route MCP production paths through the resolver | `internal/mcp/server.go` | behavior change on the agent surface | **high** | Plain revert | **yes** |
| A6 | Inventory-bounded text updates across docs and instructions | docs, `.github`, `.autoharness` | documentation | low | Plain revert | no |

`ActionResult` for every entry starts `planned`.

### Deepened Verification and Rollback (for Ship)

* **Order of work is a safety property.** U3 must be green before U4 or U5 —
  before anything can create or move a second root.
* **Negative-path first.** Land and observe failing tests for the conflict
  refusal, every rejected override form, symlink rejection, and the indeterminate
  probe before any happy path.
* **MCP coverage is mandatory.** A CLI-only green is explicitly insufficient, and
  U7 covers the **production** MCP paths, not only a discovery test double.
* **Migration proof obligations:** dry-run leaves a clean `git status`; apply then
  re-run is a byte-identical no-op; destination-exists refuses; the move commit
  contains only the move.
* **Literal-guard obligation:** the regression test must fail if a safety-critical
  `.backlogit` literal is reintroduced outside the candidate list.
* **Docs obligation:** every storage-contract sentence is checked against the
  read/write code path.
* **Rollback trigger:** any workspace failing to resolve, any artifact found
  outside the scan set, or any path observed splitting across roots in the first
  validation window → revert immediately.
* **Validation window:** one fresh `init`, one legacy-workspace run, one MCP
  session, and one `doctor` run, owned by the operator.

### Unresolved Operator Decisions

* Whether and when `.backlogit` support is ever deprecated. Left open; needs
  adoption data.
* Whether this repository's own backlog directory is ever renamed. Out of scope;
  a separate operator-initiated action requiring the pinned binary to be updated
  first.

## Plan Review

* **dispatch_mode: multi-agent-dispatch** (Constitution Reviewer, Scope Boundary
  Auditor, Security Lens Reviewer, Architecture Strategist, Go Reviewer,
  Agent-Native Parity Reviewer, Learnings Researcher — cross-model).
* **Cycle 1 decision: FAIL.** P1: `ensureContainedRelPath` is not a
  single-segment validator and provides only lexical containment, so the override
  and the candidate roots were open to `.`, nested paths, drive-relative forms,
  UNC/device paths, NUL bytes, Windows case aliases, and symlink/junction escape
  (B-01); the claim that all paths derive from `WorkspaceStorageRoot` is false —
  the archive restore path, post-ship scan, queue migration, and telemetry paths
  hardcode `.backlogit` (B-03); MCP **production** path resolution was not
  included, only a discovery test, so a `.backlog`-only workspace could split
  state (Workspace-1); `ErrAmbiguousWorkspaceRoot` had no structured MCP surface
  and no machine-readable payload (Workspace-2, WR-01); the instruction sweep
  omitted agent-facing `backlogit*.instructions.md` files and registry consumers
  (Workspace-3); the scan-set implementation target was unnamed (WR-02). P2: the
  resolved root could be re-read mid-process and probes collapsed all stat errors
  to "absent" (B-02); the pure-move unit also performed rehydration (scope); the
  doctor finding was unreachable because resolution now fails on conflict
  (scope); U9 was an unbounded wildcard sweep mixing domains (scope); dual-root
  literals leaked across modules (Architecture); the migration posture did not
  commit to a RED phase (WS-3).
* **Resolutions:** the override became a **closed set** equal to
  `WorkspaceRootCandidates()`, eliminating the traversal class outright, with unset
  distinguished from empty; `Lstat` symlink/reparse rejection plus realpath
  containment plus `os.SameFile` case-alias handling added; probes now
  distinguish not-found from indeterminate and fail closed; the root is resolved
  once in `NewWorkspace` and stored immutably; the candidate set is backed by a
  private array and exposed only through the `WorkspaceRootCandidates()`
  accessor, which returns a defensive copy so no importing package can mutate
  the closed override set or safety-critical scan set at runtime, and is
  consumed by every guard;
  U3 rewritten as a complete safety-path inventory plus a literal-guard
  regression test naming `archive.go`, `shipment_verify.go`, `migrate_queue.go`,
  and telemetry; `AmbiguousWorkspaceRootError` declared as a typed error in
  `internal/errors/workspace_errors.go`; U7 rewritten to cover MCP **production**
  paths and to add structured `workspace_root_ambiguous` and corrected
  `workspace_not_initialized` errors; U6 given an explicit read-only
  pre-resolution route tested through real entry points; rehydration removed from
  the pure-move unit; U9 bounded to an enumerated inventory including every
  `backlogit*.instructions.md`; U5 posture made explicitly RED-first; A1/A2/A3/A5
  set to `approval_required: yes` with A3 classified `destructive`.

### Cycle 2 Decision

decision: PASS

* dispatch_mode: multi-agent-dispatch
* P0: 0 — P1: 0 — remaining P2/P3 accepted as advisory follow-ups.
