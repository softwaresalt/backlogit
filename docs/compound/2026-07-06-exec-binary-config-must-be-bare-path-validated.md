---
chunk_strategy: h1-h2-h3
description: 'One durable security rule graduated from 082-S — when a config value names an external binary that the process will exec, validate it as a BARE command name resolved on PATH; reject any path-qualified value (absolute path, relative path, or `..` traversal). A path-qualified binary name in operator-writable config is an arbitrary-code-execution primitive: whoever can write the config chooses which executable runs. Pair bare-name validation with argv-array-only exec (never a shell string) and a MinimalEnv allowlist so PATH resolution itself is not attacker-steerable.'
doc_type: learning
docline:
    date: 2026-07-06T00:00:00Z
    severity: high
    tags:
        - security
        - rce
        - exec
        - config
        - path-validation
        - argv
        - minimalenv
        - gate
        - core
schema_version: "1.0"
source: docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md
title: 'A config-named binary that will be exec''d must be validated as a bare PATH-resolved command, never a caller-supplied path (082-S)'
---

# Exec'd Config Binaries Must Be Bare-PATH-Validated

One durable security rule graduated from shipment 082-S (feature 082-F,
"Pre-Task-Completion Gate Broker", PR #178, merge
`e47e1291c49f906a4b257c60f117a2cd05107db7`). Surfaced as a **P1 / 0.98-confidence**
finding in the mandatory pre-push adversarial multi-model review and remediated
before the branch was ever pushed.

## Rule — Reject path-qualified binary names in config; only accept a bare command resolved on PATH

### Problem

The gate broker reads `lifecycle.pre_task_completion_gate.autoharness_binary` from
workspace config and execs it (`autoharness gate check ...`). The first
implementation accepted the config value verbatim and passed it to
`exec.CommandContext(bin, args...)`. That turns an operator-writable (and,
in a shared/agent workspace, potentially attacker-writable) config string into an
**arbitrary-code-execution primitive**:

- `autoharness_binary: /tmp/evil` or `autoharness_binary: ./scripts/pwn.sh` makes
  backlogit exec an attacker-chosen file every time a task completes.
- No shell metacharacters are needed — the value *is* the executable path.
- The gate runs inline on the critical completion path while holding the workspace
  lock, so the malicious binary runs with the agent's privileges on every
  `move --status done`.

### Fix

`validateGateBinary` (`internal/config/schema.go`, ~:234) now rejects any
value that is path-qualified:

- reject absolute paths (`filepath.IsAbs`),
- reject values containing a path separator (`/` or `\`),
- reject `..` traversal segments,
- accept only a **bare command name** (e.g. `autoharness`) that is then resolved
  through normal `PATH` lookup at exec time.

This means the executable is chosen by the environment's `PATH` — an
administrative trust boundary — not by a free-form string inside a project file.

### Why bare-name-only is the correct boundary

A path-qualified binary name lets the *data* (config) pick the *code* (executable).
A bare name delegates that choice to `PATH`, which is controlled at the
environment/deployment layer. Config should be able to say *which named tool* to
use, never *which arbitrary file on disk* to execute.

## Defense-in-depth pairing (all three must hold together)

Bare-name validation is necessary but not sufficient. It is locked in with:

1. **argv-array-only exec** — `BuildArgs` produces a discrete `[]string`; every
   runner (`ExecRunner`, `ExecVersionRunner`, `ExecGitRunner`) calls
   `exec.CommandContext(bin, args...)`. **Never** build a shell string and never
   pass user/base-ref/path values through a shell, so metacharacter injection is
   structurally impossible.
2. **MinimalEnv allowlist** — the exec'd process inherits only an allowlisted
   environment (both Unix and Windows keys), so `PATH` resolution itself is not
   steerable by inherited env like `PATHEXT` tricks or injected `PATH` prefixes.
3. **No untrusted-path interpolation** — base-ref and other inputs are passed as
   discrete argv elements, never concatenated into a path or command string.

Drop any one of the three and the seam becomes exploitable again.

## Generalization

Whenever a config field, DB row, env var, or API payload names an executable that
your process will later `exec`:

- Validate it is a **bare command name**, not a path, unless you have an explicit,
  audited reason to accept a caller-supplied absolute path.
- Resolve via `PATH` (`exec.LookPath` semantics), not via the raw string.
- Always exec with an **argv array**, never a shell string.
- Constrain the child environment with an allowlist.

This is the "data must not choose the code" principle applied to process
execution. It is easy to regress during a refactor that "just passes the config
value through", so keep the validation unit-tested (path-qualified inputs must be
rejected) at the config boundary.

## Related

- `docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md` — locked
  security invariants (argv-only, MinimalEnv, stderr truncation, one-way
  core→gate boundary).
- `docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md`
  — the adversarial review that surfaced and verified this finding.
- `docs/compound/2026-07-06-external-process-timeout-before-probe.md` — the DoS
  sibling lesson on the same exec seam.
