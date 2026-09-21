---
status: blocked
agent: ship
shipment_id: 155-S
feature_id: 174-F
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: a4c7110cf9c27e6830a623390596c1ef10ebcd4d
phase: wave-1-red-deliverable-quality-gate
---

# Shipment 155-S — Repository-Specific Quality-Gate Reassessment

## Findings that invalidate the original raw diagnosis

### Windows line endings

- `.gitattributes` declares `* text=auto`.
- Git reports `core.autocrlf=true`.
- Raw Windows `gofmt -l .` reports 518 files because checkout files are CRLF.
- Checking LF-normalized working-tree content or canonical Git blobs reduces the
  result to 9 files.
- The same 9 canonical findings exist at branch base
  `37a5cba4713953c855013fafd4fc05e479e5618c` and at current HEAD.
- The branch adds one Go file,
  `internal/core/shipment_blocked_shape_test.go`; its canonical blob is
  gofmt-clean. New canonical format findings: zero.

Therefore the raw 518-file count is not valid evidence of 518 formatting
violations. The actual repository baseline is 9 canonical findings, none
introduced by `155-S`.

### Linter version

- Local PATH initially resolved `golangci-lint v2.13.2`, which reported 56
  findings.
- `.github/workflows/ci.yml` pins `golangci-lint v1.64.8` and Go 1.24.
- Running the exact pinned linter under a temporary Go 1.24.0 toolchain passed
  with zero findings.
- `golangci-lint run --new-from-rev
  37a5cba4713953c855013fafd4fc05e479e5618c` also reported zero new findings.

Therefore the 56-finding result was unsupported local-tool-version drift, not a
current branch lint failure.

## Remaining policy-valid blocker

The installed execution contract does not define a no-new-format baseline:

- `.github/skills/build-feature/SKILL.md`, Step 0.5d, requires the
  red-deliverable task to run `gofmt -l .` unchanged and forbids a fix
  iteration.
- The same skill's Post-Loop Quality Gates require `gofmt -l .`.
- `.github/agents/_ship.agent.md`, Step 4.3 item 2, requires `gofmt -l .`.
- Workflow policy P-002.6 requires the repository-wide static gates, including
  `gofmt -l .`, at wave convergence.
- `Makefile`, `make.ps1`, `AGENTS.md`, the constitution, and Go instructions all
  repeat the unfiltered command. No documented changed-files, canonical-blob, or
  baseline-file alternative was found.
- CI does not run a formatting gate, so CI configuration cannot supply an
  implicit baseline mechanism for Ship.

The known RED selector is not the blocker. Step 0.5d expressly inverts the task
test gate and requires `TestUR1S_` to remain RED. The sole remaining blocker is
the unfiltered formatting contract encountering 9 pre-existing canonical
findings.

## Why shipment task scope cannot repair it

- `174.039-T` is a red-deliverable task whose zero-delta contract requires an
  empty production/test delta after its already-committed harness scaffold.
- Step 0.5d says not to iterate on a red-deliverable quality-gate failure.
- All later `155-S` members are blocked by dependency order until
  `174.039-T` completes.
- The 25-member shipment scope concerns resumable shipment blocked lifecycle and
  flat release-scope behavior. No member authorizes repository-wide formatting
  cleanup across the 9 unrelated files.

## Smallest governed next action

Stage must amend the governed execution contract before Ship can proceed. The
smallest safe options are:

1. add an explicit repository-recognized canonical-LF/no-new-format baseline
   command for per-task build-loop gates while retaining the final gate required
   by the amended policy; or
2. amend the current release plan/manifest with an authorized prerequisite that
   formats the 9 canonical baseline files before `174.039-T` is completed.

Ship cannot invent either mechanism, alter the plan, or expand this task's scope.
Shipment `155-S` and all current member statuses remain unchanged; `154-S`
remains queued and untouched.
