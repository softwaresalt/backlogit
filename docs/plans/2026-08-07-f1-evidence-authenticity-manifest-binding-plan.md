---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for formal-gate unit F1: an HMAC-authenticated gate-evidence proof over a domain-separated canonical envelope, with an externally anchored enforcement requirement, rollback detection, a dedicated formal-admission predicate, and fail-closed manifest binding verified at ship time.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-07-f1-evidence-authenticity-manifest-binding-plan.md
title: 'F1 — evidence authenticity primitive and manifest binding'
---

# F1 — evidence authenticity primitive and manifest binding

Source deliberation:
`docs/decisions/2026-08-07-f1-evidence-authenticity-mechanism-deliberation.md`.
Authoritative research input:
`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md` (Q1, Q2, F1).

<!-- plan-review-attempt: 2 -->

## Problem Frame

Gate evidence is structurally trusted, not cryptographically trusted. Events are
stamped `Actor: "backlogit"` for attribution only
(`internal/core/gate_evidence.go:35-65`); there is no signature, MAC, or
anti-replay state. A hand-authored or replayed JSONL record that fits the
`gateevidence.Latest` predicate is accepted as genuine
(`internal/gateevidence/gateevidence.go:66-120`).

Manifest binding is equally weak. Member-to-shipment binding is by terminal
status plus recorded `head_sha` plus git ancestry
(`internal/core/shipment_gate.go:581-632`); shipment-level passing evidence
records neither a report hash nor a head binding (`:490-497`). `EvidenceSHA`
cannot serve as the binding: it is copied verbatim from
`Delta["gate_report_hash"]` (`internal/gateevidence/gateevidence.go:114-120`),
which SHA-256s raw report bytes with no canonicalization, so distinct or replayed
events can share the same value.

### Success Criteria

* A gate-evidence event carries an HMAC proof over a **domain-separated**
  canonical envelope that a verifier without the key cannot produce.
* A proof is not transferable across purposes (task vs shipment), workspaces,
  items, or payload schema versions.
* A substituted, reordered, or member-dropped shipment manifest is refused even
  when its `head_sha` is still an ancestor of the shipment head.
* Enforcement cannot be silently lowered from inside the workspace.
* Absent, malformed, or non-verifying key material under enforcement **refuses**,
  on **every** path, including existing early returns.
* No secret value reaches the workspace, the index, the logs, any error string,
  or any **child process environment**.
* With formal admission disabled and unrequired, behavior is byte-identical to today.

### Guarantee statement (honest, load-bearing)

The counter below provides **rollback and duplicate detection within an intact
log**, not unconditional anti-replay. The mutating actor can delete or truncate
log records, so an actor who removes later block/requeue events can lower the
observed counter floor. Full anti-replay requires a high-water anchor **outside**
the actor's write set; this plan therefore supports an **optional verifier-side
high-water ledger** (path supplied by environment, e.g. held by CI) and enforces
it strictly when configured. When it is not configured, the guarantee is exactly
the narrower one stated in this paragraph and must be documented as such. No unit
in this plan may claim more.

### Scope Boundaries

**In scope:** the domain-separated proof envelope and its canonical
serialization; HMAC computation and verification; env-only key resolution,
validation, and child-process isolation; the externally anchored enforcement
requirement; the monotonic counter and the optional external high-water ledger;
the schema-validated formal report contract; the dedicated formal-admission
predicate and its wiring into **every** verification path; the canonical
manifest-to-evidence binding; structured MCP error contracts for every new
refusal; contract documentation.

**Out of scope:** machine waivers and ADVISORY admission (charter non-goal).
Asymmetric signing (deliberation Option B). Key rotation tooling beyond a
MAC-bound `key_id` and defined unknown-key behavior. Any change to
`gateevidence.Latest`, which keeps its existing consumers. Re-implementing
canonical serialization (`internal/canonical`, F2) or the status taxonomy
(`internal/core/status_taxonomy.go`, F3) — both already shipped. **No
speculative verifier interface**: the implementation uses concrete functions
until a second proof kind actually exists.

## Requirements Trace

| Requirement | Source | Unit |
|---|---|---|
| Env-only key custody; never config, flag, or workspace file | Deliberation; `internal/config/loader.go:188-196` | U1 |
| Key never inherited by a child process | Review A-03 | U2 |
| Enforcement anchored outside the workspace | Review A-02 | U1 |
| Domain-separated envelope (magic, purpose, schema, alg, key_id, workspace, item) | Review A-04 | U3 |
| Crypto hygiene: ≥32 decoded bytes, strict encoding, `hmac.Equal` | Review A-05 | U1, U3 |
| Single `error` return with sentinels, no dual-return enum | Review F1-01/F1-02 | U3 |
| All new sentinels in `internal/errors` | Review XP-01 | U1, U3 |
| Counter for rollback detection; optional external high-water ledger | Review A-01 | U4, U6 |
| Schema-validated formal report as the bound payload | Spike item 8; `internal/core/gate/decision.go:56-60` | U5 |
| Dedicated formal-admission predicate wired into every path | Spike item 8; Review A-02 | U6 |
| New binding digest, never reused `EvidenceSHA`; ordered manifest | Spike Q2 | U7 |
| Fail-closed on invalid **and** unverifiable | `docs/compound/2026-07-07-empty-head-fail-closed-repo-presence-probe.md` | U7 |
| Structured, actionable MCP refusal errors | Review F1-1 (parity) | U8 |
| Exact machine-consumed field names fixed at the producer | `docs/compound/2026-07-23-machine-readable-governance-field-contract.md` | U4, U9 |

## Implementation Units

### U1 — Key resolution, crypto hygiene, and the enforcement anchor (code)

Add `internal/config` support for a `formal_gate` block carrying only non-secret
values (`enabled`, `key_id`). Resolve the key **only** from
`BACKLOGIT_GATE_EVIDENCE_KEY`; require strict base64 or hex encoding decoding to
**at least 32 bytes**; reject any attempt to source it from config or a flag.

Enforcement is anchored **outside** the workspace: `BACKLOGIT_FORMAL_GATE_REQUIRED`
is authoritative. Workspace config may only **raise** strictness, never lower it —
if the environment requires formal admission, `formal_gate.enabled: false` in
config is ignored and the operation still refuses.

New sentinels live in `internal/errors/gate_errors.go`: `ErrGateKeyAbsent`,
`ErrGateKeyInvalid`, `ErrFormalGateRequired`.

Files: `internal/config/schema.go`, `internal/config/loader.go`,
`internal/errors/gate_errors.go`.
Scenarios (table-driven): key absent; key under 32 decoded bytes; key with
invalid encoding; config attempts to lower a required enforcement.
Posture: test-first.

### U2 — Key isolation from child processes and artifacts (code)

Resolve the key once into a private value and **strip
`BACKLOGIT_GATE_EVIDENCE_KEY` from every child-process environment**, not only
gate helpers. This covers **every** production `exec.Command`/`exec.CommandContext`
call site in the module, not just the gate subsystem: the git environment built
from `os.Environ()` in `internal/core/archive.go` (`gitCommandEnv`), the
unscrubbed `git log` calls in `internal/core/commits.go:132`
(`AutoLinkCommits`) and `internal/telemetry/branch_metrics.go:160`
(`ParseGitMergePRs`), which today leave `cmd.Env` nil and therefore inherit the
full process environment, and the gate runner's `MinimalEnv`. It also covers
`internal/core/gate/probe.go`'s `ExecVersionRunner.Version`, whose `cmd.Env`
stays nil (inheriting the full environment) whenever the `Env` field is unset —
default that runner to the scrubbed environment instead of nil-meaning-inherit.
Add canary assertions that the key value never appears in: a child environment,
an error string, a panic-recovery message, a structured log line, a JSONL event
delta, the SQLite index or FTS rows, or telemetry output.

Files: `internal/core/childenv.go`, `internal/core/archive.go`,
`internal/core/commits.go`, `internal/telemetry/branch_metrics.go`,
`internal/core/gate/probe.go`.
Scenarios: git child env excludes the key; gate runner child env excludes the
key; `AutoLinkCommits` and `ParseGitMergePRs` child envs exclude the key;
`ExecVersionRunner` with an unset `Env` field still excludes the key; canary
sweep over written artifacts finds no occurrence.
Posture: test-first.

### U3 — Domain-separated proof envelope and HMAC primitive (code)

New package `internal/gateproof`. The signed envelope is a fixed structure and
**everything below is inside the MAC**:

| Field | Purpose |
|---|---|
| `magic` | protocol constant, prevents cross-protocol reuse |
| `purpose` | `task` or `shipment` — prevents cross-purpose transfer |
| `schema` | payload schema version; unknown versions are rejected |
| `alg` | MAC algorithm identifier |
| `key_id` | bound under the MAC so it cannot be swapped |
| `workspace_id` | trusted workspace/repository identity |
| `item_id` | item or shipment identity |
| `event_type`, `ran`, `actor`, `timestamp_utc`, `head_sha` | event pinning |
| `report_digest` | canonical hash of the **validated** formal report |
| `counter` | monotonic per-item counter |
| `manifest_digest` | **required for `purpose: shipment`, forbidden for `task`** |

Only the MAC bytes themselves are stored outside the envelope.

API (single `error` return — **no** dual-return enum):
`Sign(env Envelope, key []byte) (string, error)` and
`Verify(env Envelope, mac string, key []byte) error`, returning
`errors.ErrProofInvalid` (definitively wrong) or `errors.ErrProofUnverifiable`
(cannot decide) from `internal/errors/gate_errors.go`. Comparison uses
`hmac.Equal`; the MAC is decoded to a fixed size before comparison.
Serialization is `internal/canonical.Canonicalize`. `timestamp_utc` is audit data
only and is never used for ordering.

Files: `internal/gateproof/gateproof.go`, `internal/gateproof/doc.go`.
Scenarios: round-trip; any tampered field → `ErrProofInvalid`; wrong key →
`ErrProofInvalid`; unknown `schema` → `ErrProofInvalid`; `task` envelope
carrying `manifest_digest` → rejected; shipment envelope missing it → rejected.
Posture: test-first.

### U4 — Producer wiring: emit proof and counter on evidence events (code)

`appendGateEvidence` (`internal/core/gate_transition.go:390-423`) computes the
next monotonic per-item counter, builds the envelope with `purpose: task`, signs
it, and writes `proof`, `key_id`, `proof_schema`, and `counter` as delta fields.
Preserve the existing append-before-status-write ordering and the
`evidence_required` refusal (`:223-249`). With formal admission neither enabled
nor required, emit exactly today's delta.

Counter allocation is **not** safe today: `appendItemEventWithActorErr`
(`internal/core/gate_evidence.go:48-70`) constructs a fresh
`NewWorkspaceEventWriter` per call with no shared lock across the
read-current-counter / increment / append sequence, so two concurrent
transitions on the same item can read the same floor and sign/append duplicate
counters. Wrap counter allocation plus append in a **combined in-process mutex
and cross-process sidecar-file lock**, mirroring the pattern
`internal/events/hook_events.go`'s `HookEventWriter` already uses
(`sync.Mutex` plus an `O_CREATE|O_EXCL` lock file with stale-lock recovery):
the critical section must span the read of the current counter, its increment,
signing, and the append itself, so no interleaving can produce two envelopes
sharing a counter value for the same item.

Files: `internal/core/gate_evidence.go`, `internal/core/gate_transition.go`.
Scenarios: enabled → delta carries proof and counter+1; disabled → delta
byte-identical to a golden fixture; sign failure under enforcement → refusal with
no status write; concurrent regression test — N goroutines appending gate
evidence for the same item concurrently produce N distinct, gapless counters
with no duplicates.
Posture: test-first.

### U5 — Schema-validated formal report contract (code)

Define the formal report schema and validator. A report that is empty, non-JSON,
or missing required attributed-review fields is **not** admissible even when the
broker returned exit 0 (`internal/core/gate/decision.go:56-60`). `report_digest`
is `internal/canonical.Hash` of the **validated** report, never of raw bytes.

Files: `internal/core/gate/formal_report.go`, `internal/core/gate/types.go`.
Scenarios: valid report → digest; empty stdout → rejected; non-JSON → rejected;
missing required field → rejected.
Posture: test-first.

### U6 — Formal-admission predicate wired into every verification path (code)

New predicate in `internal/gateevidence/formal.go`, separate from `Latest`.
Admits only: verified MAC; envelope context equal to verifier-supplied context;
`event_type == gate_passed`; `ran == true`; `report_digest` matching a validated
report; counter strictly greater than every previously admitted counter; and no
later block or requeue event. `EventGateForced` is never admissible.

When `BACKLOGIT_GATE_HIGHWATER_LEDGER` is configured, the highest admitted
counter is **read only** from that external ledger and enforced strictly
(reject if the incoming counter is not strictly greater than the ledger
value); when it is not configured, the counter provides only the narrower
rollback and duplicate detection stated in the guarantee statement.
**backlogit's own process never writes to the ledger** — updating it after a
successful admission is the responsibility of the external verifier/service
that owns the ledger (e.g., a CI step persists the new high-water value after
backlogit's check passes), not this codebase, so no out-of-workspace write
occurs from within backlogit and Principle IV (CLI Workspace Containment) is
not implicated by an operator-supplied path.

**Wiring is part of this unit, not an afterthought.** Enumerate and override every
existing early return that could bypass verification on the member-scan path —
nil broker, auto fail-open, missing report, ancestry timeout — so that under
enforcement each returns a refusal instead of proceeding.

Files: `internal/gateevidence/formal.go`, `internal/core/shipment_gate.go`.
Scenarios: forced event refused; replayed counter refused; later block after a
pass refused; nil broker under enforcement refused; auto fail-open under
enforcement refused; happy path admitted.
Posture: test-first.

### U7 — Manifest binding and ship-time verification (code)

Compute `manifest_digest` as `internal/canonical.Hash` over the **ordered**
manifest membership plus covering feature plus resolved shipment head, computed
from the authoritative projection. Include it in the `purpose: shipment` envelope
(`internal/core/shipment_gate.go:490-497`). At ship time recompute from live
state and verify, refusing on `ErrProofInvalid` **and** on
`ErrProofUnverifiable`. Additive to the existing `head_sha` ancestry and
head-drift guards, which are preserved unchanged. Any helper added carries its
own hard timeout cap.

Files: `internal/core/shipment_gate.go`.
Scenarios: member reordered → refused; member dropped → refused; covering feature
swapped → refused; unchanged manifest → admitted; verification error → refused,
not skipped.
Posture: test-first.

### U8 — Structured MCP error contract for refusals (code)

Map every new refusal to a structured, machine-readable MCP error rather than a
generic internal error: a stable code, the exact missing environment variable
name, the minimum key requirement, `retryable: false`, and the operator
remediation. Follow the existing precedent that maps durable-write outcomes to
machine-readable flags.

Files: `internal/mcp/errors.go`, `internal/mcp/tools.go`.
Scenarios: key absent → structured error naming `BACKLOGIT_GATE_EVIDENCE_KEY`;
proof invalid → non-retryable structured error; proof unverifiable → distinct
code.
Posture: test-first.

### U9 — Document the evidence-authenticity contract (docs)

Document the envelope field names and formats, the domain-separation rules, key
provisioning, the enforcement anchor, unknown-key behavior, the fail-closed
matrix, and — verbatim — the guarantee statement above, including the explicit
note that a symmetric key means **CI cannot be verify-only**.

Files: `docs/design-docs/formal-gate-evidence-authenticity.md`, plus a pointer
from `docs/ARCHITECTURE.md`.
Posture: documentation.

## Dependency Graph

```text
U1 ──> U2 ──> U3 ──> U4 ──┐
                     U5 ──┴──> U6 ──> U7 ──> U8 ──> U9
```

`U5` is independent of `U1`–`U4` but must land before `U6`. `U9` last.

## Decisions and Rationale

* **HMAC over asymmetric signing** — stdlib only, fits bounded units. Because the
  key is symmetric, a verify-only CI role is impossible; that is stated rather
  than wished away.
* **No verifier interface yet** — a single implementation does not justify an
  abstraction. The envelope carries `alg`, so a second kind can be added without
  redesign when it actually exists.
* **Single `error` return** — an `(Outcome, error)` pair invites callers to check
  one and miss the other. Sentinels plus `errors.Is` is the repository style.
* **Everything context-bearing inside the MAC** — `key_id`, `schema`, `alg`,
  `purpose`, and `workspace_id` outside the MAC would all be swappable.
* **Enforcement anchored in the environment** — a config flag inside the
  workspace is inside the actor's write set, so it cannot be the sole authority.
* **Honest counter guarantee** — the counter floor is derived from a mutable log.
  Claiming unconditional anti-replay would be false; the optional external
  ledger, read-only from backlogit's side and updated only by the external
  verifier that owns it, is the path to the stronger guarantee without
  backlogit performing an out-of-workspace write.

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| Log truncation lowers the counter floor | **high** | Guarantee narrowed explicitly; optional external high-water ledger enforced strictly when configured, read-only from backlogit's side so no out-of-workspace write occurs |
| Key leaks via child-process environment | **high** | U2 strips it centrally and asserts absence in git and gate-runner child environments |
| Enforcement disabled from inside the workspace | **high** | Environment anchor is authoritative; config may only raise strictness |
| Verification skipped by an existing early return | **high** | U6 enumerates and overrides every early return with end-to-end tests |
| Cross-workspace or cross-purpose proof transfer | high | Domain-separated envelope with `magic`, `purpose`, `workspace_id`, `item_id`, `schema` |
| Symmetric key: a verifier can forge | accepted | Documented; verify-only CI explicitly declared impossible |
| Scope drift back toward PR #239 | high | Waivers, reservations, session handles named out of scope |
| Behavior change for existing workspaces | medium | Disabled path asserted byte-identical against a golden fixture |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Safety-First Go | No `unsafe`. Sentinels in `internal/errors`; errors wrapped with `%w`; `hmac.Equal` for comparison. |
| II. Test-First | Every code unit is test-first with an explicit red phase. |
| III. Workspace Isolation | No new workspace paths. Key is env-only, never written, stripped from child environments. |
| IV. CLI Containment | No writes outside the workspace by backlogit itself. The optional high-water ledger path, when configured, is read-only from backlogit's perspective — enforcement compares against it, but only an external verifier/service ever writes to update it, so backlogit performs no out-of-workspace write. |
| V. Structured Observability | U8 gives every refusal a structured, actionable machine-readable error. |
| VI. Single Responsibility | Stdlib only; reuses shipped `internal/canonical`; no speculative interface. |
| IX. Git-Friendly Persistence | Evidence remains JSONL; no new persistent workspace format. |
| X. Context Efficiency | Verification is a targeted predicate, not a bulk scan. |

No violations.

## Plan Hardening Signals

* security-sensitive: cryptographic material and a trust boundary — **yes**
* contract change: new machine-consumed delta fields on a governed event — **yes**
* fail-closed refusal path on a release-critical gate — **yes**
* secrets management — **yes**

Requires plan hardening: yes

## Runtime Verification and Closure

* **Verification surface:** the gate completion path and the shipment ship path,
  each with enforcement on and off, exercised through **both** CLI and MCP.
* **Scenarios:** happy path; absent key; short key; bad encoding; tampered proof;
  replayed counter; truncated log; reordered manifest; dropped member; swapped
  covering feature; forced event; empty-stdout report; nil broker; auto fail-open.
* **Rollback:** unset `BACKLOGIT_FORMAL_GATE_REQUIRED` and disable
  `formal_gate.enabled`; no data migration to reverse.
* **Closure artifact:** must record the guarantee statement verbatim, the
  verify-only-CI impossibility, and the unknown-key behavior.

## Plan Hardening

Hardening was required (four signals).

### Protected Invariants (must not regress)

1. Evidence append happens **before** the durable status write, and
   `evidence_required` still refuses on append failure.
2. `gateevidence.Latest` is unchanged for its existing consumers.
3. The existing `head_sha` ancestry check, empty-head refusal, malformed-SHA
   refusal, and head-drift guard remain in force.
4. With enforcement neither enabled nor required, the emitted delta is
   byte-identical to today.
5. No secret value is serialized, logged, indexed, or inherited by a child process.
6. Evidence stays in item logs; it never moves into frontmatter.
7. Workspace config can only **raise** strictness relative to the environment anchor.

### Learnings and Instructions Consulted

* `docs/compound/2026-07-07-empty-head-fail-closed-repo-presence-probe.md`
* `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md`
* `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md`
* `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`
* `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md`
* `docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md`
* `.github/instructions/constitution.instructions.md` (III, VI, VII),
  `.github/instructions/strict-safety.instructions.md`,
  `.github/instructions/go.instructions.md`

### Risky Actions (carry forward to Ship)

| # | ProposedAction | Targets | change_kind | ActionRisk | rollback | approval_required |
|---|---|---|---|---|---|---|
| A1 | Introduce cryptographic key handling into the gate path | `internal/config/*`, `internal/gateproof/*` | contract + security | **high** | Disable enforcement | **yes** |
| A2 | Add fail-closed refusal to the shipment ship path | `internal/core/shipment_gate.go` | behavior change on a release-critical path | **high** | Disable enforcement | **yes** |
| A3 | Add new machine-consumed delta fields to a governed event | `internal/core/gate_evidence.go` | contract | moderate | Additive; ignored when disabled | no |
| A4 | Provision `BACKLOGIT_GATE_EVIDENCE_KEY` / `BACKLOGIT_FORMAL_GATE_REQUIRED` | operator environment | config change | **high** | Unset the variables | **yes — operator only.** The agent MUST NOT generate, echo, store, or commit key material. |
| A5 | Strip an environment variable from child processes | `internal/core/childenv.go`, `internal/core/archive.go` | behavior change to subprocess env | moderate | Plain revert | no |

`ActionResult` for every entry starts `planned`.

### Deepened Verification and Rollback (for Ship)

* **Negative-path first.** Land and observe failing tests for absent key, short
  key, bad encoding, tampered proof, replayed counter, truncated log, and every
  manifest-substitution variant before any happy path is implemented.
* **Disabled-path characterization.** Capture today's delta as a golden fixture
  before U4 and assert byte-identity with enforcement off afterwards.
* **Secret-leak canary sweep.** Assert absence across child environments, error
  strings, panic recovery, structured logs, JSONL deltas, SQLite and FTS rows,
  telemetry output, and test fixtures.
* **Unverifiable is a refusal.** Test with an injected verifier error and an
  injected timeout, not only an invalid MAC.
* **Early-return audit is a deliverable.** U6 must produce an enumerated list of
  every bypass path with a test per path.
* **Rollback trigger:** any refusal of a legitimately passing gate, or any secret
  observed outside process memory, in the first validation window → unset the
  enforcement anchor and open a defect.
* **Validation window:** one full shipment cycle with enforcement enabled, owned
  by the operator.

### Unresolved Operator Decisions

* Whether to provision an external high-water ledger. Without it the guarantee is
  the narrower rollback-detection statement; this is an operations choice, not a
  code blocker. The ledger, when provisioned, is owned and written by the
  external verifier/service — backlogit only reads it for comparison, so no
  workspace-containment exception is required.
* Key rotation ergonomics beyond a MAC-bound `key_id` and defined unknown-key
  behavior. Deferred until a second key exists.

## Plan Review

* **dispatch_mode: multi-agent-dispatch** (Constitution Reviewer, Scope Boundary
  Auditor, Security Lens Reviewer, Architecture Strategist, Go Reviewer,
  Agent-Native Parity Reviewer, Learnings Researcher — cross-model).
* **Cycle 1 decision: FAIL.** P1: log-derived counter floor defeats the
  anti-replay claim (A-01); workspace-config enforcement flag is inside the
  actor's write set and existing early returns can skip verification (A-02);
  child-process environment inheritance leaks the key (A-03); no domain
  separation or workspace/purpose binding, and schema/version placement relative
  to the MAC was contradictory (A-04); speculative verifier interface (Arch);
  dual-return `(Outcome, error)` smell and unnamed `Outcome` type (F1-01/F1-02);
  sentinels scattered across packages (XP-01); no structured MCP error contract
  for refusals (parity F1-1). P2: crypto hygiene unspecified (A-05); untraced
  `require_formal_report` knob (scope).
* **Resolutions:** guarantee statement narrowed and an optional external
  high-water ledger added (U6); `BACKLOGIT_FORMAL_GATE_REQUIRED` anchors
  enforcement outside the workspace and config may only raise strictness (U1);
  early-return enumeration made an explicit deliverable of U6; key stripped from
  every child process with a canary sweep (U2); envelope redesigned with `magic`,
  `purpose`, `schema`, `alg`, `key_id`, `workspace_id`, `item_id` all **inside**
  the MAC (U3); verifier interface removed; API collapsed to a single `error`
  return with `ErrProofInvalid` / `ErrProofUnverifiable`; all sentinels moved to
  `internal/errors/gate_errors.go`; structured MCP refusal contract added as U8;
  crypto hygiene fixed (≥32 decoded bytes, strict encoding, `hmac.Equal`,
  fixed-size decode, unknown-key behavior); `require_formal_report` removed and
  report validation made mandatory under enforcement; A1/A2/A4 upgraded to
  `approval_required: yes`.

### Cycle 2 Decision

decision: PASS

* dispatch_mode: multi-agent-dispatch
* P0: 0 — P1: 0 — remaining P2/P3 accepted as advisory follow-ups below.
* Accepted advisory follow-ups (not blocking): external high-water ledger
  provisioning is an operations decision; key rotation ergonomics deferred until
  a second key exists.
