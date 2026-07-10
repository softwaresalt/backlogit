# Ship session — 074-S doctor --target scope-vs-io classification

- **Date**: 2026-07-02
- **Agent**: Ship
- **Shipment**: `074-S` — "doctor --target scope-vs-io classification (071-S follow-up J)"
- **Feature**: `074-F` · **Task**: `074.001-T` (single, test-first, ~2h)
- **Branch**: `feat/074-doctor-target-scope-io-classification` (off `main` @ `9be6b1b`)
- **Plan**: `docs/exec-plans/2026-07-02-doctor-target-scope-io-classification-plan.md` (Plan Review gate = PASS)

## Items completed

- `074.001-T` → **done** (commit `70cc219`). Reclassify `PrepareDoctorTarget` branch (1):
  a `confineToStorageRoot` `err != nil` path-resolution fault now returns
  `kind=DoctorTargetIO` with the wrapped underlying error preserved in `res.Message`
  (`"confine target to storage root: %v"`), instead of `kind=DoctorTargetScope` with dropped text.
- `ok == false, err == nil` containment violations (incl. the 071-S `!pathContained`
  symlink-escape guard) **stay** `kind=DoctorTargetScope`.
- Added unexported package-level boundary seam `var confineFn = confineToStorageRoot` so the
  normally-unreachable IO branch is deterministically testable; `confineToStorageRoot`
  left **byte-for-byte unchanged** (verified in diff).

## TDD ordering executed (Constitution II)

1. Landed behavior-neutral seam (suite stayed green).
2. Added compiling assertion-red `TestPrepareDoctorTarget_ResolutionErrorIsIO` — observed
   `kind=scope`/empty `Message` (genuine assertion red, not a compile error). Paired with
   `TestPrepareDoctorTarget_LexicalOutOfScopeStaysScope` (locks the scope branch).
3. Applied the reclassification → green.

## Quality gates (full mandated set, in order)

- `go test -race ./...` → **PASS** (all packages, zero data races). Required installing a C
  toolchain (WinLibs mingw gcc 16.1.0 via winget) because `-race` needs cgo and none was present.
- `go vet ./...` → **PASS**.
- `golangci-lint run` → **PASS** (clean; no `.golangci.yml`, defaults).
- `gofmt -l .` → **PASS for changed files** (LF-normalized clean). Repo-wide flagging is the
  known Windows CRLF false positive (`core.autocrlf=true`); committed blobs are LF and green on CI.

## Review gate

- code-review agent, report-only → **PASS, 0 findings**. All four invariants verified:
  security (confineToStorageRoot unchanged), exit-code neutrality (scope & io → exit 3),
  seam correctness, TDD correctness. Advisory: `assert.NotNil` on a func value is a harmless
  no-op (intentional assert-restore hygiene per plan) — no action.

## Decisions / notes

- Operating in CLI-fallback mode (backlogit CLI v1.3.0; MCP tools not directly bound). Index synced.
- Operator's pre-existing in-flux files on `main` (`.backlogit/hooks_queue.jsonl`,
  `.github/agents/*`, `.cursor/`, `.github/copilot/`, `.gitignore`) were preserved and
  **excluded** from all commits. Every `git add` was path-scoped to 074-S artifacts.
- `backlogit comment` is MCP-only (no CLI subcommand) — task comment skipped in CLI mode.

## Branch state / next steps

- Feature commit `70cc219` (code) + pending `chore(backlog)` commit for 074 queue state + this memo.
- Next: push branch, create PR (merge-commit strategy per P-009), request Copilot review,
  run review-fix cycles, run §1.9 pre-merge readiness gate, present as **merge-ready**.
- **HALT at P-014 operator merge-approval gate — do NOT self-merge.**
