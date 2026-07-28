---
chunk_strategy: h1-h2-h3
description: 'Rule-scoped markdownlint config resolves the frontmatter-title/body-H1 double count: retarget MD025 front_matter_title to a sentinel _title key while MD041 keeps its default, clearing 229 MD025 with zero file edits'
doc_type: learning
docline:
    date: 2026-07-26T00:00:00Z
    severity: high
    tags:
        - markdownlint
        - md025
        - md041
        - md001
        - frontmatter
        - ci-gate
        - p-008
        - configuration
        - doctor-to-compliance
        - github-actions
ingested_at: "2026-07-26T07:00:00Z"
schema_version: "1.0"
source: docs/compound/2026-07-26-markdownlint-frontmatter-title-double-count.md
title: 'markdownlint frontmatter-title vs body-H1 double count: scope MD025 to a sentinel key, keep MD041 default'
---

# markdownlint Frontmatter-Title / Body-H1 Double Count

## Context

Policy P-008 assumed a provisioned markdownlint gate (`.markdownlint.json`
enabling MD001/MD025/MD041, a `make md-lint` target, and a CI job) that the repo
never actually shipped — the precondition was aspirational. Shipment 106-S
(feature 126-F, PR #300, merge `59269785`) provisioned it **repo-wide** under an
operator-directed *doctor-to-compliance* direction, superseding an earlier
scoped-ignore approach (deliberation 054-DL, Option B).

The durable lesson is a markdownlint **configuration** technique that makes a
frontmatter-heavy corpus universally compliant with near-zero file edits.

## Problem

backlog and docline artifacts carry a YAML frontmatter `title:` **plus** a body
`# H1`. Two heading rules both consult markdownlint's `front_matter_title`
option, and with the **default** regex (`^\s*title\s*[:=]`) they interact badly:

1. **MD025 (single H1) fires on every such file.** The default regex makes MD025
   count the frontmatter `title:` as the document's first-level heading, so the
   body `# H1` becomes a *second* top-level heading and MD025 flags it. On this
   repo that was **229** of 250 total violations.
2. **MD041 (first line is a top-level heading) is simultaneously *satisfied*** by
   the same frontmatter `title:` — the default lets `title:` stand in for the
   required leading H1.

The trap: MD025 and MD041 want **opposite** treatment of the frontmatter title.
MD025 must *ignore* it (so the body `# H1` isn't a duplicate); MD041 must *honor*
it (so frontmatter-only-then-H1 files aren't reported headingless). A blanket
move — disabling `front_matter_title`, or pointing both rules at the same key —
fixes one rule and breaks the other. Empirically, retargeting **MD041** to a
non-existent key (or disabling it) makes the frontmatter `title:` stop crediting
MD041 and every frontmatter file fails: **~1,262** new violations.

## Solution (rule-scoped, asymmetric config)

Scope `front_matter_title` **per rule**, giving each rule the behavior it needs:

```json
{
  "default": false,
  "MD001": true,
  "MD025": { "front_matter_title": "^\\s*_title\\s*[:=]" },
  "MD041": true
}
```

* **MD025 → sentinel `_title`.** Point MD025's `front_matter_title` at a key
  (`_title`) that **no artifact uses**. MD025 now never treats the real
  frontmatter `title:` as a heading, so the single body `# H1` is the only
  top-level heading and MD025 passes — with **zero file edits** across all 229
  affected files.
* **MD041 → default (`true`).** Leave MD041 at default options so the frontmatter
  `title:` still credits the leading-heading requirement. Do **not** set
  `MD041.front_matter_title` to `_title` or `""`, and do not disable it.
* **`default: false`.** Enable *only* the three P-008 rules; every other rule
  stays off so the gate is exactly the declared contract, not markdownlint's
  broad default set.

The one-line takeaway: **when two heading rules share `front_matter_title` but
need opposite outcomes, scope the option on the rule that must ignore the
frontmatter title (MD025) and leave the rule that must honor it (MD041) at
default.**

## Empirical Verification (mandatory before trusting the gate)

Run the exact config repo-wide and confirm the counts move as predicted:

| Stage | MD001 | MD025 | MD041 | Total |
|---|---|---|---|---|
| default rule set | 1 | 229 | 20 | **250** |
| `_title` config only (zero edits) | 1 | 0 | 20 | **21** |
| config + 21 structural fixes | 0 | 0 | 0 | **0 / 1839** |

The 21 residuals were **genuine structural** violations, not config artifacts:
20 `SKILL.md` files missing a leading `# H1` (MD041) and 1 H2→H4 heading skip
(MD001). Those require real edits — see the auto-fix note below.

## Pitfalls

* **MD001/MD025/MD041 are not auto-fixable.** `markdownlint-cli2 --fix` does not
  remediate these heading rules. The config retarget is the *only* zero-edit path
  for the 229 MD025; the 21 structural residuals must be edited by hand (or by
  fixing the generator — see below).
* **Node runtime floor.** `markdownlint-cli2@0.23.1` declares
  `engines.node ">=22"`. A CI `actions/setup-node` pinned to Node 20 reintroduces
  an unsupported-runtime defect while a SHA-pin check still passes — pin
  `node-version: "22"` and assert it in a characterization test.
* **`gitignore: true` is not "the version-controlled corpus."** The cli2
  `{ "gitignore": true }` runner option lints all **non-ignored** Markdown. In a
  clean CI checkout that equals the tracked set, but locally it also covers
  untracked, non-ignored files (and excludes gitignored scratch such as
  `.autoharness/staging/`). State that distinction accurately in docs.
* **Quote frontmatter values containing `#`.** A `#` in a YAML scalar (e.g. a PR
  reference) is parsed as a comment and silently truncates the value. This bites
  closure/compound docs that cite PR numbers — the doc is now judged by the very
  gate it describes.

## Guard the Gate Against Fail-Open Regression

The gate is only trustworthy if a characterization test proves it stays
load-bearing. `tests/integration/markdownlint_gate_test.go` asserts, as
exact/prefix/length checks (never substrings):

* the config enables **exactly** MD001/MD025/MD041, MD025 has **exactly one** key
  (`front_matter_title`) whose value targets `_title`, and the cli2 runner config
  carries **no** rule- or scope-altering keys (only `gitignore`);
* the repo-wide job has no `md_touched` scoped classifier, no `needs`/`if`
  gating, `actions/setup-node@<40-hex-sha>` at `node-version: "22"`, and runs the
  exact line `make md-lint`;
* neither the job nor any step sets `continue-on-error` truthy — otherwise a
  violation would report as a non-blocking step and the hard-fail gate would
  silently degrade to advisory while every other assertion still passed.

The generalizable rule: **a gate-guard test must reject the fail-open forms
(substring look-alikes, `continue-on-error`, unguarded runtime versions), not
just assert the happy path.** A non-converging reviewer surfaced these
one-per-cycle; enumerate the whole fail-open class proactively.

## Evidence

* `.markdownlint.json` (the crux config) and `.markdownlint-cli2.jsonc`
  (`{ gitignore: true }` runner config, guard-tested to carry no rule keys).
* `scripts/md-lint.{sh,ps1}` pin `markdownlint-cli2@0.23.1`; `Makefile` /
  `make.ps1` `md-lint` targets delegate to them.
* `.github/workflows/ci.yml` — repo-wide SHA-pinned `md-lint` job, Node 22,
  `permissions: contents: read`, hard-fails CI on any violation.
* `tests/integration/markdownlint_gate_test.go` — the config + repo-wide-job
  characterization guards.
* Plan `docs/exec-plans/2026-07-25-markdownlint-p008-provisioning-plan.md`;
  decision `docs/decisions/2026-07-25-markdownlint-p008-provisioning-deliberation.md`.
* Shipment 106-S / feature 126-F / tasks 126.001-T…126.005-T — PR #300, merge
  `59269785`. cli2 auto-discovers and applies `.markdownlint.json` alongside
  `.markdownlint-cli2.jsonc` (verified: a 320-char line lints clean, proving
  `default:false` is active; a non-H1 first line fires MD041, proving the rules
  are live — the gate is real, not vacuous).

## Applicability

Reuse this whenever a Markdown corpus combines a YAML frontmatter `title:` with a
body `# H1` under a markdownlint gate (backlog artifacts, docline docs, ADRs,
generated docs). Prefer **rule-scoped configuration** over bulk file edits or
blanket rule disables: scope the rule that must *ignore* the frontmatter title to
a sentinel key and leave the rule that must *honor* it at default, then verify the
before/after counts empirically before making the gate blocking. Fix only the
residual **structural** violations by hand, and — when those live in generated
files (e.g. `SKILL.md` from an external template) — record a source-template
follow-up so the fix is not lost on regeneration, and rely on the repo-wide gate
to catch drift.
