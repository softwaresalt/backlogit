---
name: brainstorm
description: "Explore requirements and approaches through collaborative dialogue before writing a right-sized requirements document. Use for feature ideas, problem framing, or when the user says 'brainstorm', 'explore', 'what should we build', or 'help me think through'."
argument-hint: "[feature idea or problem to explore]"
---

# Brainstorm a Feature or Improvement

Brainstorming answers **WHAT** to build through collaborative dialogue. It precedes the `impl-plan` skill, which answers **HOW** to build it. The durable output is a requirements document in `.backlog/brainstorm/`.

## Agent-Intercom Communication (NON-NEGOTIABLE)

Call `ping` at session start. If agent-intercom is reachable, broadcast at every step. If unreachable, warn the user that operator visibility is degraded.

| Event | Level | Message prefix |
|---|---|---|
| Session start | info | `[BRAINSTORM] Starting: {topic}` |
| Scope assessed | info | `[BRAINSTORM] Scope: {lightweight\|standard\|deep}` |
| Learnings found | info | `[BRAINSTORM] Learnings researcher found {count} relevant solutions` |
| Question asked | info | `[BRAINSTORM] Asking user: {question_summary}` |
| Waiting for input | warning | `[WAIT] Blocked on user response` |
| Requirements doc written | success | `[BRAINSTORM] Requirements written: {file_path}` |
| Session complete | success | `[BRAINSTORM] Complete: {topic}` |

## Core Principles

1. **Assess scope first** -- Match ceremony to size and ambiguity of the work.
2. **Be a thinking partner** -- Suggest alternatives, challenge assumptions, explore what-ifs.
3. **Resolve product decisions here** -- User-facing behavior, scope boundaries, and success criteria belong in brainstorming. Detailed implementation belongs in planning.
4. **Keep implementation out** -- Do not include libraries, schemas, endpoints, or code-level design unless the brainstorm is inherently about a technical or architectural change.
5. **Right-size the artifact** -- Simple work gets a compact doc. Larger work gets a fuller document.
6. **Apply YAGNI to carrying cost** -- Prefer the simplest approach that delivers meaningful value.

## Feature Description

The user provides the feature idea or problem to explore as input when invoking this skill.

If no feature description is provided, ask: "What would you like to explore? Please describe the feature, problem, or improvement you are thinking about."

Do not proceed until you have a feature description.

## Workflow

### Phase 0: Resume, Assess, and Route

#### 0.1 Resume Existing Work

If the topic matches an existing `*-requirements.md` file in `.backlog/brainstorm/`:

- Read the document
- Confirm with the user: "Found an existing requirements doc for [topic]. Continue from this, or start fresh?"
- If resuming, summarize current state and continue from existing decisions

#### 0.2 Assess Whether Brainstorming Is Needed

If the user provides specific acceptance criteria, exact expected behavior, well-defined scope, and referenced existing patterns:

- Keep the interaction brief
- Confirm understanding and present concise next-step options
- Write a short requirements doc only if a durable handoff to planning is valuable
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

**Learnings check**: Invoke `learnings-researcher` as a subagent to search `.backlog/compound/` for relevant past solutions. Broadcast the result count.

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

### Phase 3: Produce Requirements Document

Write to `.backlog/brainstorm/{YYYY-MM-DD}-{slug}-requirements.md`

```markdown
---
title: "{Feature Title}"
date: YYYY-MM-DD
scope: lightweight|standard|deep
status: draft|approved
---

# {Feature Title}

## Problem Frame

{1-2 paragraphs describing the problem and why it matters}

## Requirements

{Numbered list of concrete requirements}

## Success Criteria

{Testable criteria that determine when the feature is complete}

## Scope Boundaries

### In Scope

{What this feature covers}

### Non-Goals

{What this feature explicitly does not cover}

## Key Decisions

{Decisions made during brainstorming with rationale}

## Outstanding Questions

### Resolve Before Planning

{Questions that must be answered before the impl-plan skill runs}

### Deferred to Implementation

{Questions that can be resolved during implementation}
```

Broadcast the file path when written.

### Phase 4: Next Steps

Present options:

1. "Run the backlog-harvester agent to plan, review, and decompose this requirements doc into backlog tasks" (Recommended — the harvester orchestrates impl-plan and plan-review automatically)
2. "Run the impl-plan skill standalone to create a plan without harvesting"
3. "Revise specific sections of the requirements"
4. "Park this for later"
