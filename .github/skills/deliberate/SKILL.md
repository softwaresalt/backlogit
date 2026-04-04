---
name: deliberate
description: "Explore requirements and approaches through collaborative dialogue, capture the result in the backlogit stash, and create a linked deliberation artifact for later planning and harvest. Use when the user says 'deliberate', 'explore', 'think this through', 'what should we build', or 'help me think through'."
argument-hint: "[feature idea or problem to explore]"
---

# Deliberate on a Feature or Improvement

Deliberation answers **WHAT** to build through collaborative dialogue. It precedes deeper planning work. The durable output is backlogit-native: a stash entry in `.backlogit/queue/.stash.md` plus a linked `deliberation` artifact in `.backlogit/queue` so later harvest and planning flows can recover the full discussion.

## Agent-Intercom Communication (NON-NEGOTIABLE)

Call `ping` at session start. If agent-intercom is reachable, broadcast at every step. If unreachable, warn the user that operator visibility is degraded.

| Event | Level | Message prefix |
|---|---|---|
| Session start | info | `[DELIBERATE] Starting: {topic}` |
| Scope assessed | info | `[DELIBERATE] Scope: {lightweight\|standard\|deep}` |
| Learnings found | info | `[DELIBERATE] Learnings researcher found {count} relevant solutions` |
| Question asked | info | `[DELIBERATE] Asking user: {question_summary}` |
| Waiting for input | warning | `[WAIT] Blocked on user response` |
| Stash item created | success | `[DELIBERATE] Stash created: {stash_id}` |
| Deliberation created | success | `[DELIBERATE] Deliberation created: {deliberation_id}` |
| Session complete | success | `[DELIBERATE] Complete: {topic}` |

## Core Principles

1. **Assess scope first** -- Match ceremony to size and ambiguity of the work.
2. **Be a thinking partner** -- Suggest alternatives, challenge assumptions, explore what-ifs.
3. **Resolve product decisions here** -- User-facing behavior, scope boundaries, and success criteria belong in deliberation. Detailed implementation belongs in planning.
4. **Keep implementation out** -- Do not include libraries, schemas, endpoints, or code-level design unless the deliberation is inherently about a technical or architectural change.
5. **Right-size the artifact** -- Simple work gets a compact deliberation. Larger work gets a fuller one.
6. **Apply YAGNI to carrying cost** -- Prefer the simplest approach that delivers meaningful value.
7. **Use backlogit-native artifacts only** -- Capture the outcome through `backlogit stash add` and `backlogit deliberate`, not legacy `.backlog/...` files.
8. **Prefer durable lineage** -- Every completed session should leave behind a stash item and linked deliberation so planning and implementation can trace the origin.

## Feature Description

The user provides the feature idea or problem to explore as input when invoking this skill.

If no feature description is provided, ask: "What would you like to explore? Please describe the feature, problem, or improvement you are thinking about."

Do not proceed until you have a feature description.

## Workflow

### Phase 0: Resume, Assess, and Route

#### 0.1 Resume Existing Work

If the topic matches an existing stash-linked deliberation artifact in `.backlogit/queue`:

- Read the deliberation artifact
- Confirm with the user: "Found an existing deliberation for [topic]. Continue from this, or start fresh?"
- If resuming, summarize current state and continue from existing decisions

#### 0.2 Assess Whether Deliberation Is Needed

If the user provides specific acceptance criteria, exact expected behavior, well-defined scope, and referenced existing patterns:

- Keep the interaction brief
- Confirm understanding and present concise next-step options
- Still create a concise stash entry and deliberation when a durable handoff to planning is valuable
- Skip Phases 1.1 and 1.2; go directly to Phase 1.3 or Phase 3

#### 0.3 Assess Scope

Use the feature description plus a light scan to classify:

- **Lightweight** -- small, well-bounded, low ambiguity
- **Standard** -- normal feature or bounded refactor with some decisions
- **Deep** -- cross-cutting, strategic, or highly ambiguous

Broadcast the scope assessment.

### Phase 1: Understand the Idea

#### 1.1 Existing Context Scan

Search the codebase for relevant context, matching depth to scope:

**Codebase search**:

- Use grep/glob to search for the feature's key concepts across the codebase
- Identify affected modules and related code patterns
- Review existing implementations for consistency

**Learnings check**: Invoke `learnings-researcher` as a subagent to search `docs/compound/` for relevant past solutions. Broadcast the result count.

#### 1.2 Collaborative Dialogue

Ask one question at a time. Prefer single-select choices when natural options exist.

Cover these areas based on scope:

- **Lightweight**: 1-2 clarifying questions, then proceed
- **Standard**: Problem frame, intended behavior, scope boundaries, success criteria
- **Deep**: All standard areas plus: trade-offs, alternatives considered, risks, dependencies, migration concerns

Broadcast each question for operator visibility.

#### 1.3 Boundary Setting

Establish:

- What is in scope
- What is explicitly out of scope (non-goals)
- Success criteria (concrete, testable)
- Blocking assumptions

### Phase 2: Explore Approaches (Standard and Deep only)

For standard and deep scope:

1. Present 2-3 approaches with trade-offs
2. Get user preference
3. Document rationale for chosen approach

### Phase 3: Persist the Deliberation in backlogit

Persist the outcome through backlogit only:

1. Create a stash entry with `backlogit stash add "{topic}" --kind feature --priority {priority}` unless resuming an existing stash-linked deliberation.
2. Use the returned stash ID to create the linked deliberation with `backlogit deliberate <stash-id>`.
3. Populate the artifact sections with the discussion outcome.
4. Treat the stash entry and deliberation artifact as the durable handoff to later planning and harvest workflows.

```markdown
## Problem Frame

{1-2 paragraphs describing the problem and why it matters}

## Options

{Alternatives considered and their trade-offs}

## Chosen Direction

{The selected direction and rationale}

## Open Questions

{Questions that remain unresolved}

## Notes

{Supporting research, references, or follow-up notes}
```

When populating the sections:

* Use **Problem Frame** for the problem, goals, and scope framing
* Use **Options** for alternatives and trade-offs
* Use **Chosen Direction** for the selected path and rationale
* Use **Open Questions** for anything that must be resolved later
* Use **Notes** for references, research, and follow-up context

Broadcast the stash ID and resulting deliberation artifact ID when written.

### Phase 4: Next Steps

Present options:

1. "Harvest the stash entry into a backlogit work item and use the linked deliberation as planning context"
2. "Run the impl-plan skill standalone with the deliberation artifact as the planning source"
3. "Revise specific sections of the deliberation artifact"
4. "Park this for later"
