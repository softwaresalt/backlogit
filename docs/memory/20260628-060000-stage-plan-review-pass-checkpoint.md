# Stage checkpoint — plan-review PASS (cycle 2)

- **Phase:** Step 4 complete → entering Step 5 (Harvest)
- **Timestamp:** 2026-06-28T06:00:00Z
- **Session:** stash-to-backlog stage pass over backlogit active stash

## Selected work unit
- Stash `8863C6C8` (medium) — "Shared frontmatter codec extraction"
- Deliberation: `docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md` (Option B)
- Plan: `docs/exec-plans/2026-06-27-shared-frontmatter-codec-extraction-plan.md`

## Plan-review outcome
- **Cycle 1: FAIL** (3× P1: F1 nil-map panic/synthetic-frontmatter corruption; F2 corpus gap;
  F3 wrong runtime fixture) + 7 P2/P3 (F4 atomicfile split, F5 io.Writer seam, F6 reframe
  behavior-change, F7 out-of-scope, F8 alias-method legality, F9 mode-clamp, F10 doc.go ownership).
- Plan revised: two leaf packages (`internal/mdfront` codec + `internal/atomicfile` writer);
  U1 full corpus port + golden bytes; U2 io.Writer seam + mode-clamp `perm &^ 0o022`; U3 alias-inherited
  Encode/forward Decode only; U4 HasFrontmatter+non-nil-map guard + self-referential fixture +
  malformed-byte-unchanged assertion.
- **Cycle 2: PASS** (attempt 2, 1 re-entry cycle). Go Reviewer PASS (F1/F2/F3/F8 CLOSED, no new P1,
  mode-clamp + io.Writer seam confirmed). Learnings Researcher PASS (F3/F2 faithful to closure +
  compound prior art; no factual contradictions; 130 self-ref / 2 malformed counts match).
- Plan + deliberation both lint-clean (`backlogit docs lint` → 0 violations).

## Decomposition to harvest (4 units, TDD-first)
- **U1** — Create `internal/mdfront` codec (characterization-first; no deps)
- **U2** — Add hardened `WriteFileAtomic` to `internal/atomicfile` (test-first; no deps)
- **U3** — Migrate `internal/docline` to consume both leaf packages (characterization-first; deps U1+U2)
- **U4** — Migrate `internal/core/doctor.go` to consume both leaf packages (characterization-first; deps U1+U2)
- Dependency wiring: U3 dep {U1,U2}; U4 dep {U1,U2}. U1∥U2, U3∥U4.

## Next steps
1. Step 5 Harvest: create feature + 4 tasks (U1-U4) w/ acceptance criteria; `dep add` wiring; P-003 validate.
2. Step 5.5: assemble queued shipment (feature first, then tasks parent-first); record shipment_id.
3. Step 5.6: archive consumed stash `8863C6C8`; sync index.
4. Step 6: final memory + summary.
5. Landing: branch `chore/stage-<shipment-id>`, commit `.backlogit/`+`docs/`, PR to main, CI green
   (incl. docline gate), §1.9 Copilot readiness gate, HALT for operator merge (merge commit, no self-merge).

## Active stash after grooming (9 entries)
`21E17BFC`, `C55C5158`, `D6B44FF6`, `2797E9F8`, `D070FD3C`, `B349CBED` (docline L2 — valid),
`AE53BC5C` (docline L4 — valid), `8863C6C8` (selected — to be archived after harvest), `9685B1AA`.

## Archived this session (5)
`98C4F063`, `E4B7767C`, `71A2CB10` (operator-flagged stale, verified consumed/discharged),
`0615F487` (L1 — resolved by commit a366bd3d), `A2436E1E` (L3 — resolved by commit 887522ad).
