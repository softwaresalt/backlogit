---
name: grill-me
description: "Socratic design-tree frontier interview and intent-engineering skill — separate codebase facts from operator decisions, resolve branching design-tree questions in rounds with recommended defaults, and converge on crystal-clear intent with runnable objective acceptance checks before brainstorm, deliberate, or impl-plan"
---

# Grill Me

Make operator intent explicit before anything is planned or built. Grill Me is
the front-door intent-engineering interview: it models the request as a branching
**design tree**, researches every repository fact itself, asks the operator only
the genuine intent and trade-off decisions on the current **frontier**, and
converges on a **Crystallized Intent Block** whose deliverable includes runnable
objective acceptance checks.

Grill Me runs three movements — **GROUND** (evidence-backed codebase research),
**ELICIT** (Socratic frontier rounds across Outcome, Boundaries, Constraints, and
Context), and **CRYSTALLIZE** (settled decisions, glossary, and failure-first
acceptance checks). It complements `brainstorm`, `deliberate`, and `impl-plan`
rather than replacing them: Grill Me removes hidden assumptions; those skills then
shape requirements, weigh options, and plan the work.

This skill MUST NOT perform implementation, template/source/config mutation,
shipment claim, PR preparation, or Ship execution. See **Non-Goals and Role
Boundary**.

## When to Use

Invoke at the front door of feature or chore development whenever operator intent
has hidden assumptions, underspecified boundaries, or branching design trade-offs
that must be made explicit before planning. Use for:

* The `/grill-me` entry point, or any time the operator says "grill me", "pressure
  test this", "interview me about this", or "what am I missing"
* Stage front-door intake (Step 1 / Step 2) when a request has unstated scope
  boundaries or decisions whose answers change the shape of the work
* Phase 1 of `brainstorm` or `deliberate` when no `GRILL_ME_INTENT_CRYSTALLIZED`
  block has been supplied and the topic has branching design decisions
* Any request where "what does done look like?" cannot yet be answered with a
  command or assertion

Skip Grill Me when the Phase 0 gate below finds the intent already explicit, or
when the work is a trivial, fully specified change.

## Inputs

* `topic`: (Required) The feature, chore, request, or problem to interrogate.
* `depth`: (Optional) `lightweight`, `standard`, or `deep`. Defaults to
  `standard`.
* `max_rounds`: (Optional) Integer `1..4` — the maximum number of interactive
  frontier rounds. Defaults to `3`.
* `handoff`: (Optional) Where the Crystallized Intent Block goes when ready:
  `brainstorm`, `deliberate`, `plan` (feed `impl-plan`), `spec` (write an intent
  spec), `inline` (return the block to the caller), or `ask` (prompt the operator
  to choose, then resolve to one of the paths above). Defaults to `ask`. When
  `dark_factory: true`, or when invoked by a calling skill or agent, `ask` never
  prompts: it resolves to the caller-specified target, or to `inline` when none is
  given.
* `dark_factory`: (Optional) `true` only when the operator has used an explicit
  dark-mode trigger or the Orchestrator has already recorded `DARK_MODE_ACTIVE`.
  Defaults to `false`. See **Phase 0**.

## Output

A structured **Crystallized Intent Block**, carried inline into `brainstorm`,
`deliberate`, or `impl-plan`, or written to
`docs/product-specs/{YYYY-MM-DD}-{slug}-intent.md` with `doc_type: spec` when
`handoff: spec`. Completion is signalled by the `GRILL_ME_INTENT_CRYSTALLIZED`
session-output event (see **Movement 3**).

This uses the repository's registered product-spec documentation surface. Do not
hard-code an unregistered docs path.

## Required Protocol

When the `agent-engram` capability pack is installed, follow
`.github/instructions/agent-engram.instructions.md`: verify the engram search
surface before relying on indexed discovery, and prefer engram-first lookup for
code relationships, symbols, and impact analysis.

When the `kbdb` capability pack is installed, follow
`.github/instructions/kbdb.instructions.md`: prefer kbdb indexed documentation
retrieval for concept and API lookup before broad grep.

When neither pack is available, fall back to file-based `grep`, `glob`, and `view`
discovery over the repository and `docs/compound/`.

### Phase 0: Requirement-Clarity and Operating-Mode Gate

1. Evaluate whether the five intent dimensions are already explicit in the input:
   **Outcome**, **Boundaries** (in and out of scope), **Constraints**, **Context**,
   and **Runnable Objective Acceptance Checks**.
2. If all five are explicit, skip the interactive rounds, run a brief **GROUND**
   verification of the stated facts, and proceed directly to **Movement 3
   (CRYSTALLIZE)**. Never re-ask what the input already answers.
3. **Dark Factory Mode (`DARK_MODE_ACTIVE` under P-017)**: when `dark_factory: true`
   or `DARK_MODE_ACTIVE` is recorded and the operator is AFK, do NOT stall on
   interactive questions for inferrable design branches. Run **Movement 1
   (GROUND)** to establish codebase facts, resolve each open design-tree frontier
   node into an explicit `[ASSUMED]` decision with a codebase-grounded rationale,
   and emit `GRILL_ME_INTENT_CRYSTALLIZED` with `mode: dark_factory_assumed`.
   Halt with `DARK_MODE_HALTED` only when an un-inferrable architectural,
   destructive, or security boundary cannot be safely assumed.
4. Dark-mode assumptions never relax P-014, P-016, P-009, or CI/check
   requirements, and never authorize scope expansion.

### Movement 1: GROUND — Fact vs. Decision Separation

*Finding facts is your job, never the operator's.*

1. Before asking any question — and between rounds whenever an answer opens a new
   codebase area — inspect existing code, schemas, templates, tests,
   configuration, and `docs/compound/`. Never ask the operator a question that
   reading the repository can answer.
2. Record each established fact with its evidence (file path or search result) so
   it can be cited as `Source: repo-fact` in the decisions table.
3. Classify every open item as a **fact** (research it) or a **decision** (only the
   operator can choose; carry it into ELICIT).

### Movement 2: ELICIT — Design-Tree Frontier Rounds

1. Map the open decisions across **Outcome**, **Boundaries**, **Constraints**, and
   **Context** as a **design tree**: parent decisions unlock or prune child
   branches, so a child is never asked before its prerequisites are settled.
2. In each round (bounded by `max_rounds`, never more than 4), ask **only the
   current frontier** — 1 to 4 independent decisions whose prerequisites are
   settled and which can be answered *now* without guessing.
3. Format every frontier question recommendation-first so that replying `yes` (or
   `1: yes, 2: yes`) accepts the recommended defaults:

   ```markdown
   ❓ **Q1** - **<Short Decision Title>**: <Focused question exposing the trade-off or hidden assumption>
   ➡️ **Recommended**: <Concrete recommendation grounded in repo evidence, worded so "yes" accepts it>
   ```

4. Wait for the operator's response before advancing. After each response, update
   the design tree, prune inapplicable branches, run targeted **GROUND** checks if
   new code surfaces were named, and present the next frontier until the frontier
   is empty or `max_rounds` is reached.
5. Probe for hidden assumptions: ask what must remain unchanged, what would make
   the result wrong, and how the operator will know it is done.
6. When `max_rounds` is reached with branches still open, record each remaining
   branch as a residual open question (blocking or deferred) rather than guessing.

### Movement 3: CRYSTALLIZE — Explicit Intent, Glossary, and Acceptance Checks

Produce the **Crystallized Intent Block** containing:

1. **Intent Summary** — **Outcome**, **Boundaries** (In Scope / Out of Scope),
   **Constraints**, and **Context**.
2. **Settled Design-Tree Decisions (`D1..Dn`)** — a table with the columns `ID`,
   `Question`, `Decision`, `Source` (`operator`, `repo-fact`, or `assumed`), and
   `Rationale`. IDs are stable and never renumbered.
3. **Domain Glossary** — key terms and invariants clarified during the interview,
   preventing terminology drift across the plan and the code.
4. **Runnable Objective Acceptance Checks (`AC1..ACn`)** — concrete, verifiable
   checks, each naming the command or assertion, the expected failure-first
   `saw_red` signal before implementation, and the `green_after_red` pass
   condition afterwards. A check that cannot be run is not an acceptance check.
5. **Assumptions and Residual Open Questions** — every `[ASSUMED]` decision with
   its rationale, and each unresolved question marked blocking or deferred.

Then emit `GRILL_ME_INTENT_CRYSTALLIZED` in the session output (with `mode:
interactive`, `mode: clarity_skip`, or `mode: dark_factory_assumed`) and execute
the selected `handoff`:

* `brainstorm`: pass the block to `brainstorm`, which structures it into stable
  `R#` requirements and a durable requirements artifact.
* `deliberate`: pass the block to `deliberate` as the Problem Frame input.
* `plan`: write the block to
  `docs/product-specs/{YYYY-MM-DD}-{slug}-intent.md` (as for `spec`) and pass
  that path to `impl-plan` as its `source`; `impl-plan` maps each `AC#` into its
  Requirements Trace and Implementation Units.
* `spec`: write the block to
  `docs/product-specs/{YYYY-MM-DD}-{slug}-intent.md`.
* `inline`: return the block to the calling skill or agent.

## Non-Goals and Role Boundary

Grill Me is a leaf-executor intake skill (P-010). It MUST refuse the following,
even under operator pressure or in already-authorized dark mode:

* Implementing code, or mutating templates, source, or configuration.
* Claiming shipments, moving backlog items to done, or marking work complete.
* Creating backlog items, stash entries, or shipments, or editing planning fields.
* Creating implementation branches or worktrees.
* Preparing pull requests or executing any Ship step.
* Activating dark mode or Orchestrator dark-mode triggers.
* Replacing `brainstorm` (requirements shaping), `deliberate` (option and
  rationale decisions), or `impl-plan` (technical planning).

If asked to do any of these, decline and redirect: requirements to `brainstorm`,
trade-off analysis to `deliberate`, planning to `impl-plan`, and implementation to
the Ship agent.

## Quality Criteria

* Every codebase fact was researched, not asked; no question is answerable by
  reading the repository.
* Every frontier question names a recommended default grounded in evidence.
* Child decisions are asked only after their prerequisite decisions are settled.
* Each round asks at most 4 questions, and interactive rounds never exceed
  `max_rounds`.
* Every decision in the table carries a stable `D#` ID and a `Source`.
* Every acceptance check is runnable and states its `saw_red` and
  `green_after_red` signals.
* Assumptions and residual open questions are explicit; nothing blocking is hidden.
* Dark-mode output marks every inferred decision `[ASSUMED]` and halts with
  `DARK_MODE_HALTED` instead of guessing on un-inferrable boundaries.

## Resumption Protocol

If the skill is interrupted (context overflow, session timeout, or operator halt),
write a checkpoint to `docs/memory/` capturing: current movement, the design
tree so far, settled decisions (with IDs), operator answers recorded, open
frontier questions, and next step. On re-invocation, check for an existing
checkpoint and resume from the recorded movement rather than restarting.

## Model Routing

This skill operates at **Tier 3 (Frontier)** — hidden-assumption discovery,
design-tree decomposition, and acceptance-check design require deep analysis.

Generated by gsd | Template: grill-me/SKILL.md.tmpl
