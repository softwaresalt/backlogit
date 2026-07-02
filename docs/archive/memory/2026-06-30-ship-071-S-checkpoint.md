# Ship session checkpoint — 071-S Deterministic-gates slice

- **Branch**: `feat/deterministic-gates-slice` (created from dirty `main`; operator's
  unrelated in-flux files carried as uncommitted — commits scoped strictly to 071-S).
- **Shipment**: 071-S claimed → `active`. Feature 071-F + 9 tasks (071.001-T..071.009-T).
- **Mode**: CLI/file-backed (backlogit.exe v1.3.0 fallback for MCP ops). agent-intercom not
  in toolset → no broadcasts (degraded, non-blocking).
- **Plan**: docs/exec-plans/2026-06-30-backlogit-deterministic-gates-slice-plan.md (PASS attempt 2).

## Critical constraints
- `db.UpsertItem` = INSERT OR REPLACE full row → SetArtifactSize MUST reconstruct a
  fully-populated *models.Artifact via models.ArtifactFromFrontmatter before upsert.
- Body-preserving writes: mdfront.Decode/Encode + atomicfile.WriteFileAtomic (no reimpl).
- size stored under custom_fields.size (top-level dropped on reparse).
- doctor --target exit table: 0 pass / 1 validation / 2 timeout / 3 scope|io / 4 busy.
- Per-path lock: map[string]*sync.Mutex + O_EXCL sidecar + 60s stale TTL; busy non-blocking.
- Merge: merge-commit only (P-009); operator approval before merge (P-014).

## Execution order
U1 → U4 → U5 → U2 → U3 → U6 → U7 → U8 → U9 (U3/U9 MCP deferrable).

## Status
- [x] U1 core.DoctorTarget — GREEN (071.001-T → 98df0ec)
- [x] U4 task_lock — GREEN (071.002-T → 89dd162)
- [x] U5 lock in DoctorTarget — GREEN (071.006-T → 98df0ec)
- [x] U2 doctor --target CLI + exit codes — GREEN (071.004-T → e3b0efa)
- [x] U3 MCP doctor target param — GREEN (071.005-T → 057ed3d)
- [x] U6 header-def size enum — GREEN (071.003-T → 6c2a2f8)
- [x] U7 core.SetArtifactSize — GREEN (071.007-T → fad706a)
- [x] U8 update --size CLI — GREEN (071.008-T → 89cf751)
- [x] U9 MCP update_item size param — GREEN (071.009-T → 057ed3d)

## Quality gates (all pass)
- `go test ./...` → ok (all packages)
- `go vet ./...` → 0
- `golangci-lint run` → 0
- `gofmt` (EOL-normalized; local checkout is CRLF via autocrlf, blobs are LF) → clean

## Backlog state
- All 9 tasks + feature 071-F moved to `done` (relocated to `.backlogit/archive/`),
  each task's implementing commit SHA associated via `backlogit update --commit`.
- Shipment 071-S remains `active` in queue, to be shipped at post-merge with the merge SHA.

## Next steps
- Review gate → PR (feat/deterministic-gates-slice) → CI + Copilot review resolution. **DONE.**
- HALT at P-014 merge gate (present merge-ready PR; verify P-009 merge-commit-only; await approval). **← CURRENT.**
- Post-merge: shipment-reconcile pre/post, `backlogit shipment ship 071-S <merge-sha>`,
  archive-integrity check, doc/knowledge graduation, compact-context, index resync — on a
  `post-merge/071-deterministic-gates` branch with its own PR.

## PR + review resolution (session 2)
- **PR #156**: https://github.com/softwaresalt/backlogit/pull/156 (base `main`, head `1d7ee3a`).
- **CI on HEAD 1d7ee3a**: all 4 checks GREEN (CLI Reference Drift, Docline gate, test 1.23, test 1.24).
- **P-009 confirmed**: merge_commit=true, squash=false, rebase=false.
- **Copilot review-fix cycles (limit 3, all used)**:
  - Cycle 1 (`22134a8`, `08eace8`): 8 threads = A (lock IO-vs-busy), B (symlink scope guard), C (MCP ErrTaskBusy→conflict). Resolved.
  - Cycle 2 (`01655cf`): 2 threads = D (stat-failure not busy), E (own lock outside timeout goroutine → PrepareDoctorTarget/ValidateDoctorTargetResolved split). Resolved.
  - Cycle 3 (`1d7ee3a`): 4 threads = F (stale-removal IO), G (invalid size→validation_failed), H (closure exit-code table), I (0x08 control char in archive/071-F.md). Resolved.
  - **Final Copilot review on 1d7ee3a**: 2 NEW threads J + K — BOTH pre-existing (verified identical at 08eace8), non-regression, P2/P3. At cycle limit → deferred, replied+left UNRESOLVED for operator, documented in closure artifact. Will be stashed for Stage in post-merge Step 6.
    - J (doctor_target.go PrepareDoctorTarget): confineToStorageRoot resolution error → kind=scope loses error text (exit code identical, both →3). P3.
    - K (doctor_target.go ValidateDoctorTargetResolved): nil ws.HeaderDef → kind=pass (no validation). Defensive; HeaderDef always loaded via config.WriteDefaults. P2.
- **Artifacts**:
  - `docs/closure/2026-06-30-071-S-deterministic-gates-slice-runtime-verification.md` (committed edd4349, verdict PASS).
  - `docs/closure/2026-06-30-071-S-deterministic-gates-slice-closure.md` (pre-merge operational closure, READY WITH CONDITIONS; **uncommitted on disk** — to be committed on post-merge branch per prior-shipment pattern; docline valid).
- **Backlog**: all 10 items (071-F + 9 tasks) archived, status=done. Shipment 071-S still `active` (ship at post-merge with merge SHA).
- **Feature branch frozen at 1d7ee3a** to preserve the clean green-CI + reviewed HEAD for the operator's P-014 decision (avoid reopening the Copilot loop at the merge gate).
- **Uncommitted operator files (NEVER commit)**: .github/agents/*.agent.md (auto-mergeinstall, auto-tune, .ship, .stage, _orchestrator), .gitignore, .cursor/, .github/copilot/. Plus the on-disk closure artifact (071-S, commit on post-merge branch).

