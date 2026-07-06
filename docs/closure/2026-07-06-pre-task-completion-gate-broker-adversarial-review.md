---
chunk_strategy: h1-h2-h3
description: 'Adversarial multi-model review (3 reviewers, report-only) of the pre-task-completion gate broker (082-F) on branch feat/pre-task-completion-gate-broker before push. Reviewers: Gemini 3.1 Pro, GPT-5.4, Claude Opus 4.8, plus an independent Security Reviewer. No all-3-model HIGH-confidence blockers; locked security invariants confirmed holding (argv-only exec, no shell string, path-qualified binary rejection, MinimalEnv on gate+git runners, stderr-truncated/full-JSON-preserved, force_cli_only, no MCP force field, logs-only evidence, one-way core->gate boundary). Five findings remediated pre-push (binary-path RCE hardening P1 0.98, probe/base timeout coverage P2 0.95, version-probe MinimalEnv P2 0.9, forced-evidence audit-integrity refusal F2, explicit --gate-base audit completeness F3). Four LOW/by-design findings deferred and stashed (F1 base-ref precedence UX, F4 member-evidence ran=false fail-open, F5 shipment DecisionError class collapse, F7 move --json GateError payload). Gate decision: remediate-then-PROCEED.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md
title: 082-S pre-task-completion gate broker — Adversarial Multi-Model Review
---

# Adversarial Review — Pre-Task-Completion Gate Broker

- **Date:** 2026-07-06
- **Branch:** `feat/pre-task-completion-gate-broker` @ `248b24a`
- **Base:** `origin/main`
- **Scope:** `git diff origin/main..HEAD -- "internal/**/*.go"` — 37 changed Go files (24 source + 13 test), ~4.2k insertions in-scope.
- **Reviewers:** 3 independent models, dispatched in parallel with identical ruleset:
  - Reviewer-A — Gemini 3.1 Pro (Tier 1)
  - Reviewer-B — GPT-5.4 (Tier 2)
  - Reviewer-C — Claude Opus 4.8 (Tier 3)
- **Aggregation + verification:** performed by the orchestrator against source (not a blind merge). Every finding below was re-read at the cited line before classification.
- **Tests:** `go test ./internal/core/... ./internal/cli/... ./internal/mcp/... ./internal/config/... ./internal/errors/...` → **all 8 packages pass.**

---

## Verdict

**No HIGH-confidence (all-three-reviewer) findings were produced, therefore there are NO strictly auto-gate-blocking findings.**

However, three **MEDIUM-confidence findings (flagged by 2 of 3 reviewers and verified real)** and two **LOW-confidence-but-verified P1 findings** are strong enough that they should be **fixed or explicitly accepted before merge**. They are listed under *Recommended-Blocking* below. None is a silent gate-bypass; the locked security invariants (argv-only exec, no shell interpolation, MinimalEnv hardening on the exec/git seams, `force_cli_only:false` rejection, no-MCP-force, logs-only evidence, one-way `core → gate` boundary) all **hold as designed**.

### Constraints independently confirmed to HOLD
- **C1 (security exec):** `BuildArgs` produces a discrete argv slice; `ExecRunner`/`ExecVersionRunner`/`ExecGitRunner` all use `exec.CommandContext(bin, args...)` — never a shell string. `validateGateBinary` rejects absolute paths and `..` traversal (`config/schema.go:236`). Stderr truncated for human evidence (`truncateStderr`, 2 KiB) while full stderr/JSON is preserved on the typed error for machine callers.
- **C2/C3 (fail-open/closed + probe):** `Probe`/`failProbe` and `Broker.Evaluate` implement auto→fail-open, true→fail-closed correctly; `Decide` maps exit 0/1/2/other and binary-not-found/timeout per contract; **task-path** exit mapping (6/7/8) via `gateExitError` is correct.
- **C6 (force):** `force_cli_only:false` is rejected at validation (`config/schema.go:225`); `runGatedCompletion` rejects a non-CLI force source (`gate_transition.go:145`); the MCP move/update tools call `UpdateArtifactWithGate(..., TransitionOptions{})` with **no** force/gate_base fields in their schemas.
- **C7 (evidence):** `appendItemEventErr` writes to `WorkspaceLogsRoot` JSONL only, never frontmatter; the **pass** and **redirect** paths honor `evidence_required` and refuse on append failure.
- **C9 (boundary):** `internal/core/gate/*` imports only stdlib + `internal/errors`; no `internal/core` import → no cycle. `mcp/gate_errors.go` and `cli/gate_exit.go` couple only to the `internal/errors` leaf and route via `errors.As`.

---

## Confidence × Severity Matrix

| # | Finding | File:Line | Reviewers | Confidence | Severity | Priority | Action class |
|---|---------|-----------|-----------|-----------|----------|----------|--------------|
| 1 | `--gate-base` override silently ignored when config `base_ref` is non-auto | `core/gate/baseref.go:59` | A, B | **MEDIUM** | MAJOR¹ | 6 | gated_auto |
| 2 | Forced-evidence append failure only warns under `evidence_required` | `core/gate_transition.go:233` | A, B | **MEDIUM** | MAJOR | 6 | gated_auto |
| 3 | Base-override audit fires only on `NonDefault`, not on flag use | `core/gate_transition.go:198` | B, C | **MEDIUM** | MAJOR | 6 | gated_auto |
| 4 | Shipment member-evidence accepts `ran=false` fail-open pass | `core/shipment_gate.go:149` | B | LOW | MAJOR | 3 | gated_auto |
| 5 | Shipment `DecisionError` collapsed to blocked (exit 6), loses class/retryability | `core/shipment_gate.go:66` | B | LOW | MAJOR | 3 | manual |
| 6 | `ExecVersionRunner` probe not env-allowlisted / not workspace-confined | `core/gate_transition.go:70` | C | LOW | MINOR | 2 | advisory |
| 7 | `--json` emits no payload for `*GateError` class (config/setup/timeout) | `cli/gate_exit.go` / `cli/move.go:112` | C | LOW | MINOR | 2 | advisory |
| 8 | Shipment aggregate check runs before member-evidence scan | `core/shipment_gate.go:41` | A, B | LOW² | MINOR² | 2 | advisory |
| 9 | Pass evidence appended before durable write (orphan-evidence window) | `core/gate_transition.go:239` | A | LOW² | MINOR² | 2 | advisory |

¹ Reviewer-A rated CRITICAL, Reviewer-B rated MAJOR. Per the most-conservative rule the pair is CRITICAL, but verification shows a **narrow trigger** (requires a non-auto config `base_ref` *and* a simultaneous `--gate-base`), so the orchestrator adjudicates **MAJOR/P1**. Both ratings recorded.
² Reviewers rated these higher (MAJOR/CRITICAL); orchestrator verification **downgraded** them — see analysis. Preserved as advisory per protocol (never drop findings).

---

## Recommended-Blocking (fix or explicitly accept before merge)

### F1 — `--gate-base` override is silently overridden by config `base_ref` · MEDIUM · MAJOR (P1)
**`internal/core/gate/baseref.go:59-64`**
```go
if base := strings.TrimSpace(in.ConfigBaseRef); base != "" && base != "auto" {
    return resolveExplicit(ctx, git, base, "config", defaultRef)   // config wins
}
if base := strings.TrimSpace(in.GateBase); base != "" {            // never reached if config set
    return resolveExplicit(ctx, git, base, "gate_base", defaultRef)
}
```
**Constraint:** C1/C2 — `--gate-base` is documented as the *operator-only override*. When a workspace pins `lifecycle.pre_task_completion_gate.base_ref` to a non-auto value, the operator's explicit `--gate-base` flag is silently discarded and the gate runs against the *config* base, not the requested one. Because `autoharness gate check` degrades a wrong/unresolvable base toward an empty diff / pass, an operator intending to widen the diff base can be silently narrowed.
**Why it matters:** A privileged break-glass flag that is silently ignored is both a correctness bug (wrong diff base → wrong pass/fail) and an operator-trust hazard. The audit event (F3) compounds it: the ignored override also isn't audited.
**Remediation:** Give `GateBase` precedence over `ConfigBaseRef`. Check `in.GateBase` first; fall back to config `base_ref` only when no CLI override was supplied. Add a table test asserting CLI-wins when both are set. If config-wins is *intended*, document it and reject/warn when both are supplied so the flag is never silently dropped.

### F2 — Forced-evidence append failure only warns under `evidence_required` · MEDIUM · MAJOR (P1)
**`internal/core/gate_transition.go:233-237`**
```go
if opts.Force {
    if err := ws.appendGateEvidence(ctx, id, EventGateForced, outcome, &opts); err != nil {
        slog.WarnContext(ctx, "gate forced evidence append failed", ...)   // only warns
    }
}
artifact, err := updateArtifactUngated(ctx, ws, id, updates)                // proceeds anyway
```
**Constraint:** C6/C7 — the sibling `EventGatePassed` append (line 227-232) correctly refuses when `EvidenceRequiredValue()` is true, and the file's own invariant (lines 138-140) is *"no completion can persist without its audit record."* The dedicated `pre_task_completion_gate_forced` audit record does **not** honor that invariant: under `evidence_required:true`, a forced completion whose forced-event append fails still writes the terminal status, persisting a forced completion without its mandatory forced-audit record.
**Why it matters:** Force is the highest-privilege operation in the feature; its audit record is exactly what must never be silently lost.
**Mitigant (reduces but does not remove):** the preceding `EventGatePassed` append already carries `forced:true` + `force_reason` in its delta, so a forced completion is not wholly unaudited — but the discrete forced event that tooling keys on can still vanish.
**Remediation:** Mirror the pass-path guard: if `opts.Force` and the `EventGateForced` append fails and `EvidenceRequiredValue()` is true, return an error and refuse before `updateArtifactUngated`.

### F3 — Base-override audit fires only on `NonDefault`, not on flag use · MEDIUM · MAJOR (P2)
**`internal/core/gate_transition.go:197-202`** (with `baseref.go:84` `NonDefault: ref != defaultRef`)
```go
if ev.Base.NonDefault {                       // only when override != discovered default
    if oErr := ws.recordGateBaseOverride(...); oErr != nil { ... }
}
```
**Constraint:** C1 — *"base-ref override (`--gate-base`) audited via a `gate_base_override` event."* The audit keys on divergence from the default, not on override *usage*. If an operator supplies `--gate-base origin/HEAD` (or a config base) that resolves to the same ref auto-discovery would pick, `Source == "gate_base"` but `NonDefault == false`, so **no `pre_task_completion_gate_base_override` event is written** — a privileged flag use escapes the audit trail.
**Why it matters:** Audit completeness for a break-glass control. Functionally benign in the equal-to-default case, but the audit is about *intent/usage*, not effect.
**Remediation:** Trigger `recordGateBaseOverride` whenever `ev.Base.Source` is an explicit override (`"gate_base"`, or config non-auto), regardless of `NonDefault`; retain `NonDefault` as an additional divergence flag in the delta. Consider making the append failure transition-fatal under `evidence_required` (currently warn-only).

### F4 — Shipment member-evidence accepts a `ran=false` fail-open pass · LOW · MAJOR (P1)
**`internal/core/shipment_gate.go:146-155`**
```go
func latestGatePassEvidence(evs []events.Event) *events.Event {
    for i := range evs {
        if evs[i].EventType == EventGatePassed || evs[i].EventType == EventGateForced {
            latest = &e   // accepts even ran=false fail-open completions
        }
    }
}
```
**Constraint:** C2/C8 — a task completed while `enabled:auto` and autoharness was unresolvable logs `EventGatePassed` with `ran=false` (via `completeGatePass` → `newOutcome`, `ev.Ran=false`). If autoharness is later installed and the *shipment* gate enforces, `validateMemberGateEvidence` treats that no-run log entry as valid member evidence, so a member that **never actually ran the gate** satisfies the reconciliation guarantee described in the file header ("every member … carries a passing gate evidence event").
**Why it matters:** The member-evidence check is the shipment's stated guarantee that every member was gated; accepting `ran=false` undermines that intent.
**Mitigant:** the shipment-level aggregate `autoharness gate check` over the full diff (line 41/66) still runs and covers the union of changes, so genuinely-broken work is likely still caught — the gap is that per-member gating can be vacuous.
**Remediation:** In `latestGatePassEvidence` (or the caller), require `delta["ran"] == true` for a member to count as gated (the field is already recorded at `gate_transition.go:386`); or emit a distinct event type for fail-open no-run completions so they are never mistaken for a real pass.

### F5 — Shipment `DecisionError` collapses to blocked (exit 6), losing class/retryability · LOW · MAJOR (P2)
**`internal/core/shipment_gate.go:63-91`**
```go
if ev.Decision.Kind != gate.DecisionProceed {   // includes DecisionError (config/timeout)
    be := &blerrors.GateBlockedError{ ... Outcome: "blocked" ... }   // -> exit 6
    return fmt.Errorf("... blocked by shipment-level gate check: %w", be)
}
```
**Constraint:** C3 — at the *task* level `errorGate` (`gate_transition.go:280`) correctly routes `DecisionError` to a typed `*GateError` (config→exit 7, timeout→retryable exit 8). The *shipment* path lumps **every** non-proceed decision — including `DecisionError{config}` (autoharness exit 2) and `DecisionError{timeout}` — into a `GateBlockedError` with `Outcome:"blocked"` → exit 6. A retryable shipment-level timeout thus surfaces as a non-retryable block, and a config error surfaces as a policy block.
**Why it matters:** A caller/agent keying on exit 8 (retry) vs 6 (stop, needs repair) is misled; retryable timeouts won't be retried.
**Remediation:** Before the block branch, handle `ev.Decision.Kind == gate.DecisionError` separately: append `EventGateError` and return `gateErrorFromClass(dec.ErrorClass, shipmentID, ...)` — mirroring `errorGate` — so class and retryability are preserved.

---

## Advisory (LOW confidence — human judgment)

### F6 — Version probe not env-allowlisted / not workspace-confined · LOW · MINOR (P2)
**`internal/core/gate_transition.go:70`** — `buildGateBroker` hardens the gate-check runner (`b.env()`→`MinimalEnv`, `Dir=WorkspaceRoot`) and the git runner (`ExecGitRunner{Dir: root, Env: env}`), but constructs `gate.ExecVersionRunner{Binary: cfg.AutoharnessBinary}` with **neither `Env` nor `Dir`**. `ExecVersionRunner.Version` (`runner.go:113-114`) then leaves `cmd.Env == nil` (inherits the full ambient environment) and `cmd.Dir == ""` (process cwd, not the workspace). The version/contract probe — the seam that *decides whether to enforce* — is the one subprocess not covered by the trust-boundary hardening applied everywhere else.
**Remediation:** `gate.ExecVersionRunner{Binary: cfg.AutoharnessBinary, Dir: root, Env: env}`. Low risk (`autoharness version` is static), fix for consistency.

### F7 — `--json` emits no payload for the `*GateError` class · LOW · MINOR (P2)
**`internal/cli/move.go:110-119`** — under `--json`, `moveGateError` only renders a body for `*GateBlockedError` (`errors.As(err, &be)`); a `*GateError` (config/setup/timeout/in_progress) returns exit 7/8 with **empty stdout**. The `gateJSONPayload` struct already declares `Error`/`Retryable` fields (`gate_exit.go:59-60`) that are never populated on the CLI error path, and the MCP surface *does* emit a structured error for these classes — an inconsistent machine contract.
**Remediation:** Add a `renderGateErrorJSON` branch that marshals `{outcome:"error", error, retryable}` from the typed `*GateError`, mirroring the MCP `gateClassResult`.

### F8 — Shipment aggregate check runs before member scan (reviewers over-stated) · LOW · MINOR (P3)
**`internal/core/shipment_gate.go:41`** — Reviewers A+B flagged that `ws.GateBroker.Evaluate` runs before `validateMemberGateEvidence` and claimed it can "mask" a member-evidence refusal. **Verification correction:** the *refusal ordering* is member-first — `validateMemberGateEvidence` (line 59) returns before the shipment-diff block decision (line 66), so a missing/stale member refusal is **not** masked by a diff block. Only the aggregate subprocess *executes* early (the broker performs no writes and uses `NoCount`, so no state/counter effect). The residual is a minor efficiency point: an expensive aggregate `autoharness` run happens even when a member is trivially non-terminal.
**Note:** the one real early-return-before-member-scan cases are the `Evaluate` *error* (line 45) and fail-open `!ev.Enforced` (line 52) branches — both correct by design (you cannot validate through a gate that cannot run; fail-open intentionally skips).
**Remediation (optional):** reorder the cheap member scan ahead of the aggregate subprocess to fail faster and avoid a wasted process.

### F9 — Pass evidence appended before the durable write (reviewer rated CRITICAL; by design) · LOW · MINOR (P3)
**`internal/core/gate_transition.go:227-243`** — Reviewer-A rated this CRITICAL ("TOCTOU / partial completion"). **Verification correction:** this is an explicit, documented design choice (lines 138-140, 220-222): evidence-before-write guarantees *no completion persists without its audit record* — the **safe** direction. The characterization as "partial completion" is inverted: the only residual is the opposite (an *orphan* evidence entry if `updateArtifactUngated` fails after the append), and the transition returns an error so the caller knows it failed. Not a bypass, not gate-blocking.
**Remediation (optional):** if orphan-evidence precision matters, record a `pending` marker reconciled after the write, or emit a compensating `gate_write_failed` event. Low value; current behavior is defensible.

---

## Remediation Plan (ordered by priority = confidence × severity)

| Order | Finding | Priority | Class | Action |
|-------|---------|----------|-------|--------|
| 1 | F1 baseref override precedence | 6 | gated_auto | Make `--gate-base` win over config `base_ref`; add CLI-wins test. |
| 2 | F2 forced-evidence not fail-closed | 6 | gated_auto | Refuse forced completion when forced-event append fails under `evidence_required`. |
| 3 | F3 base-override audit gap | 6 | gated_auto | Audit on override *usage* (Source), not only `NonDefault`. |
| 4 | F4 shipment accepts `ran=false` | 3 | gated_auto | Require `ran==true` for member gate evidence (or distinct fail-open event). |
| 5 | F5 shipment `DecisionError`→blocked | 3 | manual | Route shipment `DecisionError` to typed `GateError` (exit 7/8). |
| 6 | F6 probe env/dir hardening | 2 | advisory | Pass `Dir`+`Env` to `ExecVersionRunner`. |
| 7 | F7 `--json` GateError payload | 2 | advisory | Emit structured JSON for `*GateError` class. |
| 8 | F8 shipment scan ordering | 2 | advisory | (Optional) member scan before aggregate subprocess. |
| 9 | F9 evidence-before-write | 2 | advisory | (Optional) orphan-evidence reconciliation; by-design. |

---

## Backlog Work Items (P0/P1)

```yaml
- type: bug
  title: "C1/C2: --gate-base override silently ignored when config base_ref is non-auto"
  description: "ResolveBaseRef checks ConfigBaseRef before GateBase, so a pinned non-auto base_ref discards the operator's --gate-base flag, running the gate against the wrong diff base with no audit."
  file: "internal/core/gate/baseref.go"
  line: 59
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Give GateBase precedence over ConfigBaseRef; fall back to config only when no CLI override is supplied. Add a CLI-wins table test. If config-wins is intended, reject/warn when both are supplied."
  linked_review: "docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md"

- type: bug
  title: "C6/C7: forced-gate evidence append failure not fail-closed under evidence_required"
  description: "completeGatePass only warns when the EventGateForced append fails, so under evidence_required:true a forced completion persists without its mandatory forced-audit record, unlike the pass-evidence path which refuses."
  file: "internal/core/gate_transition.go"
  line: 233
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "If opts.Force and the EventGateForced append fails and EvidenceRequiredValue() is true, return an error before updateArtifactUngated, mirroring the EventGatePassed guard."
  linked_review: "docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md"

- type: bug
  title: "C1: gate_base_override audit fires only on NonDefault, not on flag usage"
  description: "recordGateBaseOverride is gated on ev.Base.NonDefault, so a --gate-base value equal to the auto-discovered default is used without emitting a pre_task_completion_gate_base_override event — a privileged flag use escapes the audit trail."
  file: "internal/core/gate_transition.go"
  line: 198
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Trigger recordGateBaseOverride whenever ev.Base.Source is an explicit override (gate_base or config non-auto), regardless of NonDefault; keep NonDefault as an extra divergence flag. Consider fail-closed append under evidence_required."
  linked_review: "docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md"

- type: bug
  title: "C2/C8: shipment member-evidence accepts ran=false fail-open pass"
  description: "latestGatePassEvidence accepts any EventGatePassed/Forced regardless of the ran flag, so a member completed via auto fail-open (autoharness absent, ran=false) satisfies shipment member-evidence validation despite never actually running the gate."
  file: "internal/core/shipment_gate.go"
  line: 149
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Require delta[\"ran\"]==true for a member to count as gated (field already recorded), or emit a distinct event type for fail-open no-run completions. Note: mitigated by the shipment-level aggregate diff check."
  linked_review: "docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md"
```

---

## Reviewer Divergence Notes (adversarial value)

- **Consensus-of-two, verified real:** F1 (A,B), F2 (A,B), F3 (B,C). Independent agreement across *different model families* raised these to MEDIUM confidence; source verification confirmed all three.
- **Single-model catches, verified real:** F4, F5 (GPT-only) and F6, F7 (Claude-only) were each caught by exactly one model and confirmed genuine on read — the strongest argument for multi-model review, since a single-model pass would have missed them.
- **Consensus-of-two, verified over-stated → downgraded:** F8 (A,B claimed masking; refusal ordering is actually member-first) and F9 (A rated CRITICAL "partial completion"; it is a documented safe-direction design). Preserved as advisories per protocol — agreement is a signal, not proof, and orchestrator verification is the arbiter.

*Generated by Adversarial Review — 3 models (Gemini 3.1 Pro / GPT-5.4 / Claude Opus 4.8), orchestrator-verified.*
