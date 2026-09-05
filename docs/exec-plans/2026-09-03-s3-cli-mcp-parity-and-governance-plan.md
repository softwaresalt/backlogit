---
chunk_strategy: h1-h2-h3
description: "Execution plan for S3: CLI/MCP structured-error parity, docs classify tool, and create_checkpoint governance"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s3-cli-mcp-parity-and-governance-plan.md
title: "S3 Execution Plan — CLI/MCP Parity & Governance"
---

# S3 Execution Plan — CLI/MCP Parity & Governance

**Covering feature**: CLI/MCP surface parity — structured errors, classify tool, and create_checkpoint governance
**Deliberation**: docs/decisions/2026-09-03-dark-factory-grouping-ledger.md (5F4E0FC3 decision)
**Stash members**: 63E810D9, EB93E236, 5672D73E, 5F4E0FC3
**Tier**: composability/interoperability (shipment sequence S3)

## Problem Frame

The CLI and MCP surfaces diverge: unknown-key checkpoint rejections expose a
structured `unknown_fields` array on MCP but only a wrapped string on CLI; there
is no MCP equivalent of `backlogit docs classify`; and create_checkpoint is
ungoverned so registry-drift enforcement does not cover it. These are
interoperability/parity gaps staged ahead of feature work.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; errors wrapped |
| II. Test-First (P-002) | declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | n/a |
| IV. CLI Containment | n/a |
| V. Observability | Structured error envelope improves machine legibility |
| VI. Single Responsibility | Shared response builder factored once |
| VII. Destructive Approval | none |
| VIII. Safety Modes | additive |
| IX. Git-Friendly | registry YAML + fixture |
| X. Context Efficiency | parseable errors reduce agent guesswork |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1a — Transport-neutral bounded structured-error DTO/builder (owner: `internal/errors`) (EB93E236)
* Owner / dependency direction (grounded — corrected): the structured error DTO already lives in the neutral leaf `internal/errors` (`CheckpointUnknownFieldError{Fields}`), and the bounding routine already exists there (`CheckpointNonConformingError.BoundedFieldPaths()` → `BoundedFieldPathSet{Paths, Truncated, OmittedPaths, TruncatedPaths}` via `boundedFieldPathMaxCount` + `capFieldPathBytes`; the bounded HUMAN render is `FieldPathsForDisplay()`). Production `internal/cli` DOES import `internal/mcp` (`root.go:22`); there is no reverse edge. `internal/errors` is the correct owner as the only NEUTRAL leaf without presentation coupling — `internal/cli/format` is also stdlib-only/cycle-free, but it is CLI-owned presentation, so hosting the shared builder there would couple MCP → CLI presentation; `internal/mcp` would couple CLI → MCP.
* Scope:
  1. Extract the existing bounding routine into a package-level shared helper (`boundFieldPaths([]string) BoundedFieldPathSet`) in `internal/errors`, and add `CheckpointUnknownFieldError.BoundedFields() BoundedFieldPathSet` returning the SAME projection — converging the CREATE path onto the ONE bounded projection the disposition path already uses (no second bounding policy). The projection's machine field is `Paths` (JSON `paths`); the MCP/CLI layers map it to `unknown_fields`.
  2. Bound the HUMAN render too (closes the residual amplification vector): give `CheckpointUnknownFieldError` a `FieldPathsForDisplay()`-equivalent bounded render and route `Error()` (currently `strings.Join(e.Fields, ", ")`, UNBOUNDED) AND the MCP `checkpointUnknownFields` `Message` through it, so neither the `message` string nor the `unknown_fields` array echoes an unbounded raw offender list (mirrors the disposition type's 147-F/U1c "machine array and human message cannot drift" invariant).
  3. `internal/mcp/errors.go`: extend `checkpointUnknownFieldsResponse` with `unknown_fields_truncated` / `unknown_fields_omitted` / `unknown_fields_shortened` — all NON-omitempty (always-present, mirroring `checkpointDispositionErrorResponse`); populate `checkpointUnknownFields(...)` from `BoundedFields()` + the bounded `Message`. Keep the top-level `Error == "validation_failed"` and `IsError:true` (mirror ONLY the three scalars, NOT the disposition Error/Code/Retryable/Remediation fields); update the stale `checkpointUnknownFieldsResponse` doc comment to say the array is the BOUNDED projection.
* Bounding (deterministic, requirement 2): count cap (`boundedFieldPathMaxCount`); rune-safe per-name byte cap (`capFieldPathBytes` — first rune over cap yields `""`); total raw-path budget = count × per-name cap (a test asserts the raw-path ceiling; serialized JSON is larger due to escaping/envelope); stable `Truncated` / `OmittedPaths` / `TruncatedPaths` indicators; RAW field-NAME paths only (control-char safety comes from downstream `json.Marshal` escaping; NO caller VALUES echoed).
* Acceptance: `BoundedFields()` returns the bounded set; N > cap yields the cap with `Truncated=true` and `OmittedPaths=N-cap`; over-long path byte-capped with `TruncatedPaths` incremented; the bounded HUMAN render is asserted bounded when N > cap (`Error()` AND the MCP `Message`); MCP `backlogit_create_checkpoint` unknown-key rejection returns `Error=="validation_failed"`, `IsError:true`, `unknown_fields` + the three always-present scalars, and no value bytes.
* Files: `internal/errors/checkpoint_errors.go`, `internal/mcp/errors.go` (+ tests). Responsibility: converge the create path (array AND message) onto the single bounded projection across the neutral error type and its MCP consumer.

### U1b — CLI JSON-RPC `data` transport for structured domain errors (EB93E236)
* Scope: add an optional `Data any \`json:"data,omitempty"\`` field to `internal/cli/format.JSONRPCError` and a `WrapErrorData(id string, code int, msg string, data any)` constructor alongside `WrapError`. `WrapError` is unchanged; when `data` is nil the `data` key is ABSENT (omitempty), never `null`.
* Compatibility / error-code semantics (requirement 3): the `code` remains one of the existing `ErrCode*` constants and its meaning is unchanged; `data` is purely supplementary structured detail. All current `WrapError` callers emit byte-identical JSON to today (no `data` key). A byte-level test pins that the legacy path omits `data` and never emits `null`. Typed-nil guard: `omitempty` omits an interface-nil `data`, but a non-nil interface wrapping a TYPED-nil pointer/map marshals as `"data":null`; `WrapErrorData` normalizes via a `reflect.ValueOf(data).Kind()` + `IsNil()` check (a plain `data == nil` check does NOT catch this), and a test covers the typed-nil case so "never null" holds universally.
* Acceptance: `WrapErrorData` with a nil `data` is byte-identical to `WrapError`; with a non-nil `data` it embeds `data` as a structured object; the legacy `WrapError` golden test is unchanged. Single responsibility (CLI JSON-RPC transport).
* Files: `internal/cli/format/jsonrpc.go` (+ tests).

### U2 — CLI structured JSON error envelope for checkpoint unknown fields (63E810D9)
* Emission seam (grounded — corrected): the CLI JSON-RPC error envelope is serialized ONLY in `internal/cli/root.go` `Execute()` (≈ line 81: `format.WrapError(cmdPath, format.ErrCodeServerError, err.Error())`), NOT in `checkpoint*.go` (which returns a plain `error` up to `Execute`, and whose captured stdout is discarded on the error path). So the structured `data` envelope MUST be attached in `Execute()`.
* Scope: in `internal/cli/root.go` `Execute()`, before the generic `WrapError`, `errors.As` the returned error for `*CheckpointUnknownFieldError`; on match, map `BoundedFields()` into a CLI wire projection (`unknown_fields` + the three scalars, mirroring MCP) and emit via `WrapErrorData` (U1b) with the bounded `Message`. Use `errors.As` + local CLI-edge mapping in `Execute()` now — do NOT add a `JSONRPCData()` method to the neutral `internal/errors` type (that would leak CLI wire names into the leaf). If a second structured error later warrants a general interface, define it in `internal/cli/format` on a CLI-side wrapper, not in `internal/errors`. (Precondition: `%w`-wrap the typed error from checkpoint-create up to `Execute` so `errors.As` traverses — a `fmt.Errorf("%v", ...)` would sever it.)
* Acceptance: a CLI `backlogit checkpoint create` unknown-key rejection emits `{error, message, data:{unknown_fields, unknown_fields_truncated, unknown_fields_omitted, unknown_fields_shortened}}` with the SAME bounded shape as MCP; the envelope is verified on the WIRE (through `Execute`), not just constructed; a CLI↔MCP parity test asserts identical bounded path sets, scalars, and bounded message; no caller values echoed. Depends on U1a and U1b.
* Files: `internal/cli/root.go`, `internal/cli/checkpoint*.go` (surfaces the typed error) (+ parity test).

### U3 — Add backlogit_docs_classify MCP tool + registry/parity coupling (5672D73E) — reconfirmed
* Reconfirmation: `docline.Classify(relPath string) DocType` is a PURE classifier — it returns NO error and performs no filesystem resolution/containment, so it cannot itself emit `ErrPathEscapesWorkspace`. `backlogit_docs_classify` is not registered; `backlogit docs classify` is currently in the `cliOnlyIntentional` (Class-E) allow-list of `internal/cli/registry_parity_test.go`.
* Scope:
  1. Because `docline.Classify` is a PURE path-string classifier with NO containment, introduce a SHARED containment helper (`docline.ValidateClassifyPath(root, path)`) and call it from BOTH surfaces before `Classify`, so invalid inputs are rejected IDENTICALLY (no cross-surface asymmetry). The helper MUST enforce the full input contract itself — `core.SafeResolve` only validates the JOINED result and does NOT reject an empty, absolute, or volume-qualified RAW input (`internal/core/workspace.go:521-538`) — so the helper explicitly rejects (a) an empty/blank `path`, (b) absolute paths, and (c) volume/UNC-qualified forms (drive-letter `C:\`, leading `/` or `\`, `\\host`) on ALL platforms (platform-neutral), THEN calls `SafeResolve` for the traversal/escape check:
     - `internal/mcp/docs_tools.go`: register `backlogit_docs_classify` wrapping `docline.Classify`. Make `path` a REQUIRED schema argument (`mcplib.Required()`) plus a handler-side blank check — do NOT mirror `backlogit_docs_lint`'s OPTIONAL `path` (`docs_tools.go:26-34`), or an omitted `path` would be classified. Call the shared helper and map any failure to `ValidationFailed`. Do NOT claim `Classify` emits `ErrPathEscapesWorkspace`.
     - `internal/cli/docs.go` (`newDocsClassifyCommand`, which today passes `args[0]` straight to `Classify` with no validation): call the SAME shared helper and return the containment error, so the CLI rejects exactly the inputs the MCP tool does.
  2. `.autoharness/backlog-registry.yaml`: add an `operations.docs_classify` row (`mcp_tool: backlogit_docs_classify`, `cli_command: "backlogit docs classify {{path}}"`, `params.path: path`) mirroring `docs_lint`/`docs_migrate`/`docs_scope`, so the registry drift test does not fail on a registered-but-unmapped tool.
  3. `internal/cli/registry_parity_test.go`: remove `"backlogit docs classify"` from `cliOnlyIntentional` (it is now a paired surface).
  4. `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md`: add the `backlogit_docs_classify` row, update the tool count (56 → 57) and the row-total tally, and reclassify `docs classify` from CLI-only.
* Acceptance: MCP `backlogit_docs_classify` returns the same `DocType` as CLI `backlogit docs classify <path>`; an EMPTY, absolute, volume/UNC-qualified, or traversal/escaping `path` is REJECTED IDENTICALLY on BOTH CLI and MCP (invalid-input parity test covering all four classes) — never classified on one surface and rejected on the other — mapping to `validation_failed` on MCP; an OMITTED MCP `path` argument is rejected by the required-schema check; the registry drift/parity test passes with the new row and the allow-list removal; the parity matrix reflects 57 tools.

### U4 — Govern create_checkpoint (5F4E0FC3 decision) — reconfirmed against current registry
* Reconfirmation: `.autoharness/backlog-registry.yaml` `create_checkpoint` currently has `mcp_tool` / `cli_command` / `params` but NO `governed:` marker.
* Scope: add `governed: true` + `governed_name: checkpoint_create` to `create_checkpoint` and author the named behavioral fixture that dispatches the AUTHORITATIVE registry (the fixture selects `create_checkpoint` FROM the registry, asserts the operation key + `governed_name: checkpoint_create` + `mcp_tool` + `cli_command`, and invokes the REGISTERED handler in-process on BOTH surfaces — not `core.CreateCheckpoint` directly; compound: `2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md`). Also update `docs/design-docs/governed-operation-parity.md` (Registry Markers table + the "current governed set" sentence) to include `create_checkpoint`. No new result-shape / flag semantics.
* Acceptance: the registry drift test now covers `create_checkpoint`; the fixture dispatches the authoritative registry (registered handler, both surfaces) and passes; the governance design doc lists `create_checkpoint`.

## Dependency Graph

U1a (neutral bounded builder + MCP convergence) and U1b (CLI JSON-RPC `data`
transport) are independent of each other; U2 depends on BOTH. U3 and U4 are
independent. Order: U1a and U1b in parallel, then U2; U3 and U4 any time.
Backlog mapping: `155.001-T` (U1a), `155.005-T` (U1b), `155.002-T` (U2, depends
`155.001-T` + `155.005-T`), `155.003-T` (U3), `155.004-T` (U4).

## Runtime Verification and Closure

U1-U3 change CLI/MCP runtime surfaces (error envelopes, new tool); U4 changes
governance metadata. Verification: cross-surface parity tests + registry drift
test. Closure: parity fixtures are the durable closure artifact.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Own the shared structured-error DTO/builder in a transport-neutral package | **High** — placing it in `internal/cli/format` couples MCP → CLI presentation; placing it in `internal/mcp` couples CLI → MCP | Put the builder in the neutral leaf `internal/errors` (where `CheckpointUnknownFieldError` and the bounding routine already live); both surfaces consume it. A production CLI→MCP import exists (`root.go:22`), so `internal/errors` is the ONLY cycle-free leaf — which makes the neutral-leaf choice necessary, not optional |
| Bound attacker-controlled `unknown_fields` deterministically | **High** — the CREATE path currently joins ALL offender key names unbounded (output amplification; key-name exposure) | Reuse the EXISTING neutral bounding routine (`boundedFieldPathMaxCount` max count; `capFieldPathBytes` rune-safe per-name byte cap; total budget = count × cap; stable `Truncated` / `OmittedPaths` / `TruncatedPaths` indicators; raw control-character-safe field-NAME paths, never values). Converge the create path onto the same projection the disposition path already uses; test on BOTH CLI and MCP without echoing values |
| Add structured CLI JSON-RPC error `data` (`JSONRPCError.Data`) | Medium — agent-facing error envelopes become more structured | Add `Data any \`json:"data,omitempty"\`` + `WrapErrorData`; keep `WrapError` unchanged; legacy errors OMIT `data` (never `null`); `code` (`ErrCode*`) semantics unchanged; byte-level golden tests pin the legacy path |
| Add a new `backlogit_docs_classify` MCP tool | Medium — new MCP surface changes the public tool catalog | Keep the tool additive, mirror `backlogit_docs_lint` semantics + containment, and add a cross-surface parity test |

Rollback: disable the new MCP registration and keep the structured `data` field
behind the shared builder call path if compatibility issues appear. Compatibility:
all surfaces remain additive and backward-compatible when `data` is omitted for
legacy errors. Ownership: transport-neutral DTOs belong in a leaf package that can
consume `internal/errors`, not in an MCP- or CLI-owned presentation package.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — new MCP tool + CLI error envelope + JSONRPCError data field (all additive/backward-compatible).
* security/auth/permission/compliance-sensitive: PRESENT-minor — `unknown_fields` reflects attacker-controlled key names; mitigated by the reused neutral bounding routine (count/size/total bounds, stable truncation indicators, raw control-char-safe field-NAME paths, no value echo). Security Lens review required.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent — additive parity work.

Requires plan hardening: yes

## Prior Plan Review (invalidated)

dispatch_mode: multi-agent-dispatch
decision: INVALIDATED

The prior PASS record is retained only as invalidated history. It omitted mandatory personas and is superseded by the genuine multi-agent Plan Review below.

## Plan Review

<!-- plan-review-attempt: 3 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

personas:
* Constitution Reviewer (`claude-opus-4.8`)
* Go Reviewer, anchor (`gpt-5.6-sol`, effort high)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Correctness Reviewer (`claude-sonnet-4.6`)
* Architecture Strategist (`grok-4.6`)
* Learnings Researcher over `docs/compound/`
* MCP Protocol Reviewer (triggered by the MCP tool/error-contract changes) — MCP-PASS
* Agent-Native Parity Reviewer (triggered by CLI↔MCP structured-error parity) — PASS
* Security Lens Reviewer (triggered by attacker-controlled `unknown_fields`) — security-PASS
* Schema-CLI-Docs Coupling Reviewer (triggered by registry + docs coupling) — PASS after remediation

Review history (genuine multi-agent-dispatch, cross-model, 2 cycles):
* Attempt 1 (FAIL): two HIGH-confidence P1s — (C1) the create-path `Error()` / MCP `Message` still raw-joined attacker-controlled key names UNBOUNDED even though the `unknown_fields` array was bounded (re-opening the amplification vector through the message); (C2) U2 was wired to an unreachable seam (`checkpoint*.go`), so the `data` envelope could not reach the CLI wire (the seam is `internal/cli/root.go Execute()`). Plus Schema-CLI-Docs P0s: a missing `operations.docs_classify` registry row and a stale `cliOnlyIntentional` allow-list entry would fail the registry drift test.
* Attempt 2 (PASS): C1 resolved — U1a bounds the HUMAN render (`Error()` + MCP `Message`) via a `FieldPathsForDisplay()`-equivalent, converging on the single neutral projection (`BoundedFieldPathSet.Paths`); C2 resolved — U2 re-targeted to `Execute()` with `errors.As` → `BoundedFields()` → `WrapErrorData`, verified on the wire; U3 now also adds the registry row, removes the allow-list entry, and updates the parity matrix (56→57); grounding corrected (CLI→MCP exists; `internal/errors` is the neutral leaf without presentation coupling); typed-nil guard, always-present MCP scalars, and `docline.Classify` handler-side containment pre-check folded in. Ten-reviewer manifest — 6 mandatory personas (Constitution, Go anchor, Scope, Correctness, Architecture, Learnings) + 4 triggered lenses (MCP Protocol, Agent-Native Parity, Security Lens, Schema-CLI-Docs Coupling); zero residual P0/P1.

Residual (non-blocking P2/P3 advisories, implementation-time): use `errors.As` + CLI-edge mapping rather than a leaf-side `JSONRPCData()` interface; `reflect`-based typed-nil normalization mechanism; name the parity-matrix tally bucket to avoid an off-by-one.

Readiness: READY. Same-contract completion of the authorized S3 work; no scope creep. Ship may claim 137-S.
