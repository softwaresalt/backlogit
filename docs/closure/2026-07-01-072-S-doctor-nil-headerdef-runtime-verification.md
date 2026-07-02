# Runtime Verification — 072-S doctor --target nil-HeaderDef hardening

- **Date**: 2026-07-01
- **Shipment**: `072-S` · Feature `072-F` · Task `072.001-T`
- **PR**: #158 (`feat/072-doctor-target-nil-headerdef`)
- **Surface**: `cli` (also structurally `mcp`) · **Mode**: manual
- **Verdict**: **PASS**

## Affected runtime surfaces

The change modifies the shared `core.ValidateDoctorTargetResolved`, which both surfaces route
through:
- CLI `backlogit doctor --target` — process exit code + JSON `kind`/`ok` fields
- MCP `backlogit_doctor` target payload (`handleDoctor → core.DoctorTarget`)

Only the (normally unreachable) nil-`HeaderDef` case changes: it moves from `kind=pass`/exit 0 to
`kind=io`/exit 3. The pass / validation / scope / busy / timeout paths are unchanged.

## Environment prechecks

- Global `C:\Tools\backlogit.exe` predates the `doctor --target` feature (071-S) — cannot exercise
  the surface. Built the binary from source (`go build ./cmd/backlogit`, exit 0) and ran the
  source binary for all CLI checks below.

## Scenarios executed (source build)

| Scenario | Command | Expected | Observed | Result |
|---|---|---|---|---|
| Pass path (non-regression) | `doctor --target .backlogit/queue/072-F.md --format json` | `kind=pass`, `ok=true`, exit 0 | `kind=pass`, `ok=true`, exit 0 | ✅ |
| IO path (bucket reused by new branch) | `doctor --target .backlogit/queue/does-not-exist.md` | `kind=io`, `ok=false`, exit 3 | `kind=io`, `ok=false`, exit 3 | ✅ |
| Scope path (non-regression) | `doctor --target README.md` | `kind=scope`, `ok=false`, exit 3 | `kind=scope`, `ok=false`, exit 3 | ✅ |

## New fail-closed branch (nil HeaderDef → io/exit 3)

- **Unreachable via normal CLI by design**: `config.WriteDefaults` (invoked at CLI init) always
  writes `header-def.yaml`, so `config.LoadHeaderDef` repopulates `ws.HeaderDef`; a real workspace
  never reaches the nil branch. Attempting to force it through the CLI is defeated by the same init
  path. This matches the plan's Reachability analysis.
- **Verified directly by unit test**: `TestDoctorTarget_NilHeaderDefFailsClosed` sets
  `ws.HeaderDef = nil` and asserts `kind=io` / `OK=false` / message "header definition not loaded"
  through the `DoctorTarget` wrapper (which also exercises the MCP path + lock lifecycle). Red
  before impl, green after. A loaded-`HeaderDef` sibling assertion pins classification precedence.

## Evidence

- `go test ./...` PASS (all packages incl. cli/mcp/contract/integration); `go vet` PASS;
  `golangci-lint run` PASS; changed files gofmt-clean (LF).
- CI on PR #158: `test (1.23)` pass, `test (1.24)` pass, `Docline frontmatter gate` pass,
  `CLI Reference Drift` pass.

## Follow-up recommendations

- No monitoring or rollback trigger required — zero-blast-radius defensive edge fix.
- One out-of-scope follow-up (stashed for Stage): create/update write paths in
  `internal/core/artifacts.go:224,514` share the same fail-open *shape*
  (`ValidateArtifactFields` behind `if ws.HeaderDef != nil`). Not doctor-path; tracked separately.

## Handoff to operational-closure

- Verification verdict: **PASS**
- Surfaces verified: CLI `doctor --target` exit codes + JSON kind/ok (pass/io/scope); MCP inherits
  structurally via the shared function
- BLOCKED prerequisites: none
- Risky action state: none (pure classification logic, reversible)
- Follow-up: `artifacts.go` write-path fail-open shape (stashed)
