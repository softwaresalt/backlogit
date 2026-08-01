---
type: session-memory
date: 2026-07-31
agent: ship
shipment: 114-S
feature: 106-F
tasks:
  - 106.001-T
  - 106.002-T
branch: feat/106-F-formal-gate-foundations
---

# Ship 114-S — Formal-gate foundations (F2 + F3)

## Scope

Shipment **114-S** ("Formal-gate foundations") executing two bounded tasks of the
F-series (parent feature **106-F**, which stays `active` — more F-tasks remain):

- **F2 (106.001-T)** — canonical serialization + SHA-256 hash primitive.
- **F3 (106.002-T)** — authoritative status taxonomy with context-specific predicates.

## Work completed

- PR #323 (staging, `chore/stage-114-S`) merged earlier → merge commit `809e741d` on `main`.
- Feature branch `feat/106-F-formal-gate-foundations` cut from clean `main`.
- Shipment 114-S claimed (queued→active).
- **F2** implemented via Go Engineer subagent (strict TDD). Review gate passed.
  Commit `a7199174`. Task 106.001-T → done (relocated to `.backlogit/archive/`).
- **F3** implemented via Go Engineer subagent (characterization-first TDD).
  Review gate passed. Commit `d512dea4`. Task 106.002-T → done (archived).
- Backlog state committed `d470724c`.

## F2 technical decisions

- `internal/canonical/canonical.go` — stdlib-only leaf, zero internal imports.
  `Canonicalize(v any) ([]byte, error)`, `Hash(v any) (string, error)`.
- Byte contract: UTF-8 no BOM; string CRLF->LF then lone-CR->LF (never deleted);
  minimal JSON escaping (control chars `\u00xx` lowercase, `/` not escaped);
  integers only (fail-closed via `ErrNonIntegerNumber`; integral floats accepted
  via round-trip; json.Number integer-only); map keys sorted by Go/UTF-8 byte
  order (deliberate RFC 8785 divergence); array order preserved; exactly one
  trailing LF before hashing.
- `gateReportHash` (internal/core/gate_evidence.go) deliberately NOT re-routed —
  F1 owns that. Pinned by `canonical_characterization_test.go`.
- baseline+allowlist `crypto/sha256` guard scoped to `internal/core` +
  `internal/gateevidence`, allowlisting only `internal/core/gate_evidence.go`.
- Deviation (accepted, P3): leaf tests use stdlib `testing` not testify (leaf is
  stdlib-only for cohesion).

## F3 technical decisions

- `internal/core/status_taxonomy.go` — named context-specific predicates over
  UNEXPORTED immutable sets. Two truth tables PINNED and NOT unified:
  - 6-status cascade `{done,accepted,archived,shipped,abandoned,rejected}` —
    `IsCascadeTerminalStatus` / `IsNoLongerBlockingStatus`.
  - 4-status releasable `{done,accepted,rejected,archived}` (omits
    shipped/abandoned) — `IsReleasableStatus`.
- `IsGateTargetStatus(status, configuredTerminalStatuses)` — the ONLY
  parameterized predicate; wired to `gate_transition.go isGateTerminalStatus`.
- `CascadeTerminalStatuses()` accessor returns a sorted COPY; immutability
  test asserts external mutation cannot corrupt the backing set.
- Exported mutable `var TerminalStatuses` REMOVED. Migrated 3 real use-sites
  (grep-verified; the summary's "queue.go:250" was stale index noise):
  blocking_cascade.go, queue.go `filterByResolvedDependencies`, mcp/tools.go
  `handleMoveItem`.
- `isTerminalReleaseStatus` and `isGateTerminalStatus` re-implemented as thin
  delegations — behavior-preserving; the 5 release-progression call sites and
  `isDescopeEligibleStatus`/`isRecognizedReleaseStatus` unchanged.
- archived treated LITERALLY; archived_status ignored (restore-path scoped).

## Quality gates (all green)

- `go build ./...` ok, `go vet ./...` ok
- `golangci-lint run ./internal/core/... ./internal/mcp/...` ok (0 findings)
- `go test ./...` ok (every package, incl. contract + integration)
- gofmt: normalized (LF) check on all 14 changed files CLEAN. Whole-repo
  `gofmt -l .` noise is a CRLF autocrlf working-copy artifact; committed tree is LF.

## Next steps

1. Push branch, create PR (base `main`, head `feat/106-F-formal-gate-foundations`).
2. Copilot review loop: reply then resolve threads via `gh api graphql`.
3. §1.9 pre-merge readiness gate.
4. STOP for operator merge approval (P-014) — do NOT merge autonomously.
5. Post-merge closure: shipment-reconcile (pre+post), `backlogit shipment ship 114-S`
   (moves 114-S queue->archive), compound-refresh, compact-context, push.

## Notes / guardrails

- 106-F stays `active` (F-series has F1/F4/F5+ remaining).
- Shipment 114-S stays `active` on the branch; queue->archive deferred to
  post-merge `ship_shipment` (per shipment convention).
- P-009 merge-commit-only verified allowed on repo.

## Post-merge closure (2026-07-31, completed)

- PR #324 MERGED via merge commit `f8870f864d596a1f3593405e54396d8129aa8871`
  (P-009 verified: merge commit has 2 parents 809e741d + d252007b). Branch
  `feat/106-F-formal-gate-foundations` deleted.
- All 6 CI checks green on HEAD `d252007b`; fresh Copilot review covered current
  HEAD; 3 threads fixed+replied+resolved; §1.9 gate passed.
- shipment-reconcile pre-mode: PROCEED (106.001-T/106.002-T pre-archived done;
  no orphans). post-mode: PROCEED (all archive files present; no P-007 deletions).
  Reports in `.backlogit/reconcile/`.
- `backlogit shipment ship 114-S --sha f8870f86...` shipped 114-S
  (queue->archive). archived_ids: [106.001-T, 106.002-T, 106-F, 114-S].
- HAZARD FOUND + CORRECTED: ship cascade-archived parent feature 106-F
  unconditionally (collectArchiveCandidateIDs appends every ancestor feature via
  featureScopeRoots with NO terminal-children gate). 106-F must stay active
  (F1/F4/F5/F6 deferred). Restored: `git restore .backlogit/queue/106-F.md` +
  removed untracked `.backlogit/archive/106-F.md` + `backlogit sync`. 106-F now
  active in queue.
- Compound learning captured:
  `docs/compound/2026-07-31-ship-shipment-unconditionally-archives-ancestor-features.md`.
- Bug stashed `D360C106` (high): gate feature archival on remaining non-terminal
  descendants in collectArchiveCandidateIDs.

## Final state

- 106.001-T (F2) done+archived; 106.002-T (F3) done+archived.
- 114-S shipped+archived. 106-F ACTIVE (spans F1-F6 across future cycles).
- Deliverables live on main: `internal/canonical/` (F2), `internal/core/status_taxonomy.go` (F3).
