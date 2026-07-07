---
doc_type: closure
schema_version: "1.0"
title: "085-S Adversarial Multi-Model Security Review — Shipment-gate empty-head fail-closed"
date: 2026-07-07
shipment: "085-S"
branch: "feat/085-shipment-gate-empty-head-fail-closed"
commits:
  - "4844e45"
  - "9eb5a2f"
  - "bf80557"
  - "b00a5e6"
review_mode: "adversarial-multi-model (report-only)"
reviewers: 3
verdict: "BLOCK"
tags:
  - "security"
  - "gate"
  - "fail-closed"
  - "adversarial-review"
---

# 085-S Adversarial Multi-Model Security Review

**Scope:** Security-hardening change to the ENFORCED shipment-completion gate
(`internal/core/shipment_gate.go`) closing two empty-head fail-open holes
(1AEA2B0E empty shipment head; B85DAEE8 empty member head_sha), plus the new
bounded repo-presence discriminator `inGitWorktreeBounded`.

**Mode:** Report-only. No code was modified. HIGH-confidence P0/P1 consensus
findings are gate-blocking.

**Files reviewed (diff `main..HEAD`):**
`internal/core/shipment_gate.go`, `shipment_gate_test.go`,
`shipment_gate_ancestry_test.go`, `shipment_gate_headdrift_test.go`.

---

## VERDICT: BLOCK

Blocking finding:

- **F1 — Present-but-broken-repo fail-open in `inGitWorktreeBounded`
  (shipment_gate.go:192–194).** The `"not a git repository"` substring
  discriminator cannot distinguish a genuine no-repo directory from a
  present-but-broken `.git` pointer. A `.git` gitfile pointing at a missing
  gitdir exits 128 with stderr `fatal: not a git repository: (NULL)`, which
  **matches the whitelisted substring** and is misclassified as `(false, nil)`
  → legacy skip → the shipment ships with member-lineage and drift enforcement
  bypassed. This directly contradicts the function's own documented contract
  ("any OTHER 128 ... corrupt .git ... MUST fail closed", shipment_gate.go:189–191)
  and leaves 1AEA2B0E **partially open** for the broken-worktree sub-case.

**SEC-1 (fail-open holes closed under enforcement+real-repo):** NOT-CONFIRMED —
the common paths (normal worktree, unborn branch) correctly fail closed, but a
present-but-broken `.git` pointer whose stderr contains "not a git repository"
still fails open (empirically reproduced on `git version 2.55.0.windows.2`),
bypassing the member-lineage scan and drift guard.

**SEC-2 (legitimate empty-head cases preserved, no completion breakage):**
CONFIRMED — genuine no-repo ships, no-repo empty member head stays skipped
(`shipmentHead==""` short-circuit), non-enforcement returns early, bare-repo /
inside-`.git` correctly skip (`--is-inside-work-tree != "true"`), and the 084
ancestor / equality-fast-path / malformed-guard semantics are unchanged by the
re-indentation. The single "forced member without head_sha, shipped in a real
repo → refused" edge is an **intended design decision** (deliberation Option D,
lines 217–227 / 278–279: "no forced exception"), not a defect.

---

## Per-Reviewer Independent Findings

### Reviewer A — Tier 1 (model: gemini-3.1-pro-preview)

| # | Sev | Rule | Loc | Finding |
|---|-----|------|-----|---------|
| A1 | MINOR | localization-over-refusal | :193 | If git stderr is localized (LANG=fr_FR etc.) it won't contain the English "not a git repository", causing a **safe** fail-closed over-refusal in a legitimate no-repo env. Fix: pin `LC_ALL=C`/`LANG=C` for these probes. |
| A2 | MINOR | sec-1-verdict | — | **CONFIRMED** — repo-presence discriminator reliably identifies a real work tree and fails closed on unresolvable lineage. *(Note: A did not consider the broken-`.git` vector.)* |
| A3 | MINOR | sec-2-verdict | — | **CONFIRMED** — genuine no-repo gets `(false,nil)` and cleanly bypasses the fail-closed blocks, preserving the legacy skip. |

### Reviewer B — Tier 2 (model: gpt-5.4)

| # | Sev | Rule | Loc | Finding |
|---|-----|------|-----|---------|
| B1 | MAJOR | git-probe-no-repo-classification | :193 | Any exit-128 stderr containing "not a git repository" is treated as no-repo skip; git emits that substring for broken/mispointed metadata too (a `.git` file pointing at a missing gitdir), so an enforced workspace can hit `shipmentHead==""` and fall back to the legacy skip instead of failing closed. Fix: narrow to the canonical outside-any-repo diagnostic, or stat for a `.git` entry and fail closed; add a broken-`.git` regression test. |
| B2 | MINOR | sec-1-verdict | :193 | **NOT-CONFIRMED** — main flows closed, but the probe has a fail-open classification path for some broken-repo exit-128 diagnostics that contain the no-repo substring. |
| B3 | MINOR | sec-2-verdict | :491 | **CONFIRMED** — non-enforced flows return early, `shipmentHead==""` keeps the no-repo skip, and the empty-member-head refusal executes only inside `shipmentHead != ""`; no over-refusal of no-repo / break-glass in real callers. |

### Reviewer C — Tier 3 / frontier (model: claude-opus-4.8, high effort)

| # | Sev | Rule | Loc | Finding |
|---|-----|------|-----|---------|
| C1 | MAJOR | sec-fail-open-substring | :193 | **Empirically verified (git 2.55):** a work tree whose `.git` gitfile points at a missing gitdir emits `fatal: not a git repository: (NULL)` on exit 128 → matched by the lowercased substring → skipped → `shipmentHead==""` bypasses the entire member-lineage scan **and** drift guard. Reachable via a moved repo, pruned worktree, unmounted submodule gitdir, or dangling `.git` pointer — no forged evidence needed. Fix: disambiguate genuine no-repo (`not a git repository (or any of the parent directories)`) from present-but-broken (`not a git repository: <path>`), or stat for `.git` and fail closed when present-but-broken. |
| C2 | MINOR | over-refusal-break-glass | :493 | Flipped empty-member-head refusal does not exempt `EventGateForced`; a break-glass member without head_sha (forced in a no-repo authoring context) is now refused when shipped in a real repo. *(Aggregator note: this is **intended** per deliberation Option D — see disposition below.)* |
| C3 | MINOR | stdout-parse-robustness | :173 | Exact `bytes.Equal(TrimSpace(stdout),"true")`; any stray stdout byte collapses to `(false,nil)` → skip → fail-open direction. Practically unreachable (git writes only `true\n`), but the wrong default direction for a security guard. Fix: match `HasPrefix("true")` boundary, or treat exit-0 stdout that is neither `true` nor `false` as fail-closed. |
| C4 | MINOR | test-coverage-gap | ancestry_test:103 | `TestInGitWorktreeBounded` covers real-worktree/no-repo/expired-ctx but omits the exit-128 corrupt/broken cases — precisely the branch harboring F1. Add: (a) dangling `.git` gitfile → expect `(false, err)`; (b) corrupt-128 without the phrase → expect error. |
| C5 | MINOR | sec-1-verdict | :333 | **NOT-CONFIRMED** — TOCTOU only ever over-refuses (never bypasses) because head is read pre-Evaluate and the probe runs post-Evaluate; but closure is INCOMPLETE due to the broken-worktree substring fail-open (C1). |
| C6 | MINOR | sec-2-verdict | :467 | **CONFIRMED** — the only production caller (gateShipmentCompletion:359) passes `shipmentHead` solely from `headSHABounded`; the invariant "non-empty shipmentHead ⟺ resolved real HEAD" holds; 084 fast-paths semantically unchanged. One over-refusal edge = the intended forced-no-head case. |

---

## Aggregator Independent Verification

The consensus line (:193) was independently reproduced by the assembling agent
on the exact toolchain (`git version 2.55.0.windows.2`):

| Scenario | stderr (verbatim) | exit | contains "not a git repository" | current classification |
|---|---|---|---|---|
| Genuine no-repo temp dir | `fatal: not a git repository (or any of the parent directories): .git` | 128 | yes | `(false,nil)` skip — **correct** |
| `.git` file → missing gitdir | `fatal: not a git repository: (NULL)` | 128 | **yes** | `(false,nil)` skip — **WRONG (fail-open)** |
| Empty/corrupt `.git` dir | `fatal: not a git repository (or any of the parent directories): .git` | 128 | yes | `(false,nil)` skip — arguable |

`gate.MinimalEnv()` (runner.go:103-127) forwards `LANG`/`LC_ALL` **if present in
the parent env** but does **not pin** them to `C` — so A1's localization concern
is real and interacts with any tightened English-phrase match.

**Distinctive discriminator:** genuine no-repo carries the parenthetical
`(or any of the parent directories)`; a present-but-broken pointer does not.
Matching that longer phrase (or a `.git` stat) closes F1 cleanly.

---

## Consensus Findings (confidence-weighted)

Confidence: HIGH = all 3 reviewers; MEDIUM = majority (≥2); LOW = single reviewer.
F1 confidence is elevated to **HIGH** because the fail-open was independently
flagged by 2 reviewers (B, C), the line was flagged by all 3 (A flagged the same
line for the inverse over-refusal risk), **and** the aggregator empirically
reproduced the exploit on the target toolchain.

| ID | Finding | File:Line | Reviewers | Severity | Confidence | Priority (P) | Action Class |
|----|---------|-----------|-----------|----------|------------|--------------|--------------|
| F1 | Substring `"not a git repository"` misclassifies a present-but-broken `.git` (empirically: `not a git repository: (NULL)`) as no-repo → fail-open bypass of member-lineage + drift under enforcement; contradicts the function's documented "corrupt .git MUST fail closed" contract | shipment_gate.go:192-194 | B, C (+ A same line; + aggregator repro) | MAJOR | **HIGH** | **P1** | gated_auto / manual |
| F2 | `TestInGitWorktreeBounded` omits the exit-128 broken-`.git` cases — the exact branch of F1; adding them would have caught it | ancestry_test.go:103 | C | MINOR | LOW | P3 | manual (ships with F1) |
| F3 | `MinimalEnv` does not pin `LC_ALL=C`/`LANG=C`; a localized git stderr defeats the English substring match → safe-direction over-refusal, but reduces robustness of the F1 fix | runner.go:103-127 / shipment_gate.go:193 | A | MINOR | LOW | P3 | gated_auto |
| F4 | Exact-`true` stdout match collapses any unexpected stdout to a `(false,nil)` skip (fail-open direction); practically unreachable but wrong default for a security guard | shipment_gate.go:170-177 | C | MINOR | LOW | P3 | advisory |
| F5 | Flipped empty-member-head refusal does not exempt `EventGateForced` (forced-no-head shipped in a real repo → refused) | shipment_gate.go:491-518 | C | MINOR | LOW | P3 | **advisory — BY DESIGN** |

### Disposition notes

- **F5 is not a defect.** The deliberation explicitly evaluated and rejected a
  forced-evidence exception (Option D, lines 217–227): "forced-in-real-repo
  records a head, and forced-in-no-repo shipped-in-real-repo is a genuine
  inconsistency that should refuse." The behavior is intended under strict mode's
  "refuse if unverifiable" contract, with the documented escape valve (lower to
  `enabled:auto`/`false`, or record a head). Recommend a one-line code comment
  pointing at the deliberation so future readers don't re-litigate it, and
  (optional) a real-repo `shipmentHead != ""` test pinning the intended refusal.

- **F4** is real-direction-wrong but effectively unreachable: `git rev-parse
  --is-inside-work-tree` emits exactly `true\n` or `false\n`; warnings go to
  stderr (further suppressed by MinimalEnv). Track alongside F1's hardening.

- **TOCTOU:** confirmed benign (C5). `shipmentHead` is read pre-Evaluate; the
  probe runs post-Evaluate only when `shipmentHead==""`. A repo appearing between
  the two reads can only make the probe return `true` → fail closed
  (over-refusal), never a bypass. No exploitable ordering hazard.

- **084 ancestor path integrity:** confirmed intact by all reviewers. The diff's
  re-indentation of the `if h != shipmentHead {` block preserves nesting; the
  equality fast-path, `isGitObjectName` malformed-guard, and `isAncestor`
  fail-closed exit-code matrix are semantically unchanged.

- **Evidence emission (Principle V):** confirmed correct and non-masking. Both
  `EventGateBlocked` append sites are best-effort (a failed append logs a warning
  but does not swallow the subsequent typed refusal). Regression tests assert the
  `empty-shipment-head` and `empty-member-head` reasons are recorded.

- **Test quality:** RED/GREEN tests are substantive (not vacuous), gated on
  `git` availability via `t.Skip`. `TestShipmentGate_EmptyShipmentHeadInRepo_Refused`
  exercises the real unborn-branch worktree; `..._NoRepo_Skips` and
  `..._EmptyMemberHeadNoRepoSkipped` pin the preserved legacy skips; R7 correctly
  flips accept→refuse with a typed `*GateBlockedError` assertion. The single gap
  is F2 (no broken-`.git` exit-128 case).

---

## Remediation Queue (ordered by confidence × severity)

1. **[P1 · HIGH · F1] Close the present-but-broken-repo fail-open.** In
   `inGitWorktreeBounded`, stop treating the bare `"not a git repository"`
   substring as a no-repo skip. Preferred: match the genuine-no-repo phrase
   `not a git repository (or any of the parent directories)` (the parenthetical is
   git's stable marker for "outside any repo"); any other exit-128 (including
   `not a git repository: <path>` / `(NULL)`) → fail closed. Alternatively, stat
   `ws.RootPath` for a `.git` entry and fail closed when a `.git` is present but
   git rejects the repo. `gated_auto` (deterministic, localized-string sensitive).

2. **[P3 · LOW · F3] Pin `LC_ALL=C`** for the git metadata probes (or at least
   `inGitWorktreeBounded`) so the F1 English-phrase match is locale-stable.
   `gated_auto`.

3. **[P3 · LOW · F2] Add exit-128 broken-repo probe tests:** (a) `.git` gitfile →
   missing gitdir must yield `(false, err)` fail-closed; (b) a corrupt 128 whose
   stderr lacks the no-repo phrase must yield an error. Ships with F1. `manual`.

4. **[P3 · LOW · F4] Harden stdout parsing:** treat exit-0 stdout that is neither
   `true` nor `false` as fail-closed rather than a silent skip. `advisory`.

5. **[P3 · LOW · F5] Document (do not change behavior):** add a code comment at
   the empty-member-head refusal referencing deliberation Option D; optionally a
   real-repo test pinning the intended forced-no-head refusal. `advisory`.

---

## Backlog Work Items (P0/P1)

```yaml
- type: bug
  title: "inGitWorktreeBounded: broken .git pointer misclassified as no-repo (fail-open)"
  description: >
    The exit-128 `"not a git repository"` substring match in inGitWorktreeBounded
    cannot distinguish a genuine no-repo directory from a present-but-broken .git
    pointer (a .git gitfile pointing at a missing gitdir emits
    `fatal: not a git repository: (NULL)` on git 2.55.0.windows.2, which matches
    the substring). The broken worktree is skipped as no-repo, so under enforcement
    an empty shipment head bypasses the member-lineage scan and drift guard and the
    shipment ships — the exact 1AEA2B0E fail-open the change claims to close, and a
    contradiction of the function's own doc-comment ("corrupt .git ... MUST fail closed").
  file: "internal/core/shipment_gate.go"
  line: 193
  severity: "MAJOR"
  confidence: "HIGH"
  fix: >
    Match the genuine-no-repo phrase `not a git repository (or any of the parent
    directories)` (or stat RootPath for a .git entry and fail closed when
    present-but-broken); pin LC_ALL=C for locale stability; add exit-128
    broken-.git regression tests.
  linked_review: "docs/closure/2026-07-07-085-S-adversarial-review.md"
```

---

## Reviewer Roster

| Reviewer | Tier | Model | Effort |
|----------|------|-------|--------|
| A | 1 (fast) | gemini-3.1-pro-preview | default |
| B | 2 (standard) | gpt-5.4 | default |
| C | 3 (frontier) | claude-opus-4.8 | high |

All three reviewer instances returned results; consensus quorum satisfied
(≥2 required). Aggregation and empirical verification performed by the assembling
agent (no further delegation).

---

## Final Verdict Lines

```
VERDICT: BLOCK — F1 (shipment_gate.go:192-194) present-but-broken-repo fail-open,
                 HIGH confidence (2 reviewers + aggregator empirical repro),
                 MAJOR/P1, contradicts documented fail-closed contract.

SEC-1 (fail-open holes closed under enforcement+real-repo): NOT-CONFIRMED —
       common worktree/unborn-branch paths fail closed, but a broken .git pointer
       emitting "not a git repository" is misclassified as no-repo and ships,
       bypassing member-lineage + drift enforcement (empirically reproduced).

SEC-2 (legitimate empty-head cases preserved, no completion breakage): CONFIRMED —
       genuine no-repo ships, no-repo empty member head stays skipped,
       non-enforcement/bare-repo/inside-.git preserved, 084 fast-paths unchanged;
       the single forced-no-head over-refusal is an intended design decision
       (deliberation Option D), not a regression.
```

---

## Remediation — F1 (applied, commit 586993f)

**Finding:** `inGitWorktreeBounded` matched the loose substring `not a git repository`, so a present-but-broken `.git` pointer (gitfile → missing gitdir) emitting `fatal: not a git repository: (NULL)` (exit 128) was misclassified as a genuine no-repo skip → empty-head fail-open re-opened.

**Empirical confirmation (this toolchain, git 2.55.0.windows.2):**
- Genuine no-repo → `fatal: not a git repository (or any of the parent directories): .git`
- Broken `.git` pointer → `fatal: not a git repository: (NULL)`  ← lacks the parenthetical marker
- Real worktree → `true` (exit 0)

**Fix (commit 586993f):**
1. Discriminator tightened to git's stable genuine-no-repo marker `not a git repository (or any of the parent directories)`. A broken/corrupt repo (any other exit-128 stderr) now falls through to **fail closed** — matching the function's documented contract.
2. `withCLocale` forces `LC_ALL=C` / `LANG=C` for the probe (stripping inherited `LANG`/`LC_ALL` from `MinimalEnv`, since duplicate env keys are not reliably last-wins), so the English diagnostic is matched regardless of host locale (addresses advisory F3).
3. Added broken-`.git`-pointer regression case (d) to `TestInGitWorktreeBounded` — **confirmed RED before the fix** (`An error is expected but got nil`), GREEN after.

**Post-fix verification:**
- `TestInGitWorktreeBounded` (a real / b no-repo / c expired-ctx / d broken-pointer): PASS
- All gate tests (EmptyShipmentHeadInRepo_Refused, EmptyShipmentHeadNoRepo_Skips, EmptyMemberHeadNoRepoSkipped, StaleRefused/R7, AllMembersHaveEvidence_Ships): PASS — legitimate no-repo skips preserved.
- `go vet`, `golangci-lint`, `gofmt` (structural), full `go test ./...`: clean.

**Advisory dispositions:** F2 (broken-repo test gap) — closed by case (d). F3 (LC_ALL=C) — applied. F4 (exact-`true` match collapses fail-open) — unreachable in practice; left as-is (exit-0-non-true is a legitimate not-a-worktree skip). F5 (forced-no-head refusal) — intended (deliberation Option D); documented in the code invariant comment.

## Re-review verdict

See "Re-review (post-586993f)" section below (fresh multi-model pass after 586993f).

---

## Re-review (post-586993f)

**Scope:** Fresh 3-reviewer adversarial pass verifying commit `586993f` closes F1
(present-but-broken `.git` pointer fail-open) and introduces no new holes. Files
re-read: `internal/core/shipment_gate.go` (`inGitWorktreeBounded`, `withCLocale`,
`hasEnvKey`, exit-semantics doc-comment) and `internal/core/shipment_gate_ancestry_test.go`
(`TestInGitWorktreeBounded` case (d)). `gate.MinimalEnv()` allowlist independently
re-confirmed. Mode: report-only. HIGH-confidence P0/P1 consensus = gate-blocking.

### Reviewer roster (re-review)

| Reviewer | Tier | Model | Effort |
|----------|------|-------|--------|
| A | 1 (fast) | gemini-3.1-pro-preview | default |
| B | 2 (standard) | gpt-5.4 | default |
| C | 3 (frontier) | claude-opus-4.8 | high |

All three returned; quorum satisfied (≥2).

### What the fix does (verified against current source)

1. **Discriminator tightened (shipment_gate.go:206-210).** No-repo skip now requires
   `exit == 128` **AND** lowercased stderr `bytes.Contains` `not a git repository (or
   any of the parent directories)` — git's stable "outside any repository" marker. Any
   OTHER exit-128 stderr (including the F1 gitfile→missing-gitdir form
   `fatal: not a git repository: (NULL)`, which lacks the parenthetical) now falls
   through to `return false, err` → **fail closed**. Caller fails closed on
   `err != nil || inRepo`.
2. **`withCLocale` (shipment_gate.go:221-230)** strips every inherited `LANG`/`LC_ALL`
   entry from `MinimalEnv()` then appends `LC_ALL=C`, `LANG=C` — one each, no dup — so
   the English diagnostic is matched regardless of host locale.
3. **`hasEnvKey` (shipment_gate.go:235-237)** exact key match with `=` boundary.
4. **Regression case (d)** in `TestInGitWorktreeBounded`: a `.git` gitfile pointing at a
   missing gitdir must `require.Error` (fail closed).

### Consensus on F1

**F1 IS CLOSED (HIGH confidence — 3/3).** The exact F1 vector — a `.git` gitfile
pointing at a missing gitdir, emitting `fatal: not a git repository: (NULL)` (exit 128,
no parenthetical) — now falls through the tightened discriminator to the fail-closed
`return false, err` path. All three reviewers (and the committed RED→GREEN test (d))
confirm this. Reviewer B's `f1_closed:false` was a scope conflation — B's own rationale
states "586993f fixes the specific broken-gitfile `(NULL)` case and test (d) passes";
B's objection is the adjacent empty-`.git`-**directory** shape (finding N1 below), not
the F1 vector itself.

### withCLocale / hasEnvKey / localization (question 3) — RESOLVED, no bypass

All three reviewers agree the locale hardening is sound:

- **`hasEnvKey` exact-match is correct.** The `=` boundary + full-slice compare means
  `LANG` does not strip `LANGUAGE` (index 4 is `U`, not `=`) and `LC_ALL` does not strip
  `LC_ALLOTHER` (index 6 is `O`, not `=`). No false strips.
- **No duplicate-key hazard.** Every `LANG`/`LC_ALL` entry is removed, then exactly one
  `LC_ALL=C` and one `LANG=C` appended.
- **No `LANGUAGE` localization bypass.** Two independent layers: (a) `gate.MinimalEnv()`
  allowlist is `{PATH, HOME, USERPROFILE, SYSTEMROOT, SystemRoot, TEMP, TMP, TMPDIR,
  LANG, LC_ALL, GIT_DIR, GIT_CONFIG_GLOBAL, GIT_CONFIG_NOSYSTEM, HOMEDRIVE, HOMEPATH,
  APPDATA, LOCALAPPDATA, PATHEXT, COMSPEC}` — it does **not** forward `LANGUAGE` or
  `LC_MESSAGES`, so a host-translated locale never reaches the child; and (b) per the
  gettext spec, `LC_ALL=C` forces `LC_MESSAGES=C`, which makes gettext **ignore**
  `LANGUAGE` entirely even if it were present. `cmd.Env` fully replaces the ambient env
  (no inheritance), and gitconfig cannot localize `die()` strings. **A translated
  diagnostic cannot defeat the English substring match.**

### Direction-of-failure (question 2, robustness)

**Genuine no-repo still skips (SEC-2 preserved, 3/3).** A real no-repo temp dir emits
`fatal: not a git repository (or any of the parent directories): .git`, which matches
the tightened marker → `(false, nil)` skip. Test case (b) pins this. The parenthetical
is git's long-stable `setup.c` source-English string; under `LC_ALL=C` it is emitted
verbatim on every platform git targets, including Windows git. **Failure direction on
any future wording drift is SAFE:** a genuine no-repo whose message changed would no
longer match → `return false, err` → caller **fails closed (over-refuse)**, never fail
open. No new over-refusal of a currently-legitimate no-repo ship was found.

### NEW findings (post-fix)

Confidence: HIGH = all 3; MEDIUM = majority (≥2); LOW = single reviewer. Severity
conflicts resolved to the most conservative; real-world impact annotated.

| ID | Finding | File:Line | Reviewers | Sev | Conf | Priority | Action |
|----|---------|-----------|-----------|-----|------|----------|--------|
| N1 | Empty/corrupt `.git` **directory** (no HEAD/objects/refs) fails git's `is_git_directory()`, git walks up and emits the genuine-no-repo parenthetical → `(false,nil)` **skip**, not fail-closed. This is a present-but-broken `.git` state that the function's doc-contract ("corrupt .git … MUST fail closed") says should refuse. Reviewer B rated MAJOR fail-open; Reviewer C (frontier, empirical) rated MINOR doc-accuracy and demonstrated it is **not an exploitable lineage bypass** — an empty `.git` dir has zero readable HEAD/objects/refs, git itself classifies it as no-repo, semantically identical to no `.git` at all, so there is no lineage to bypass. Contract-vs-behavior inconsistency, negligible security impact. | shipment_gate.go:206-210 (+ :151-153 doc) | B, C | MAJOR (contested → effective MINOR impact) | **MEDIUM** | P2 | gated_auto |
| N2 | The no-repo marker is matched with `bytes.Contains` rather than an anchored/prefix match. If a git version echoes the unresolved gitdir path in the exit-128 message (`not a git repository: '<path>'`) and an attacker with workspace filesystem write can name a directory containing the literal substring `not a git repository (or any of the parent directories)` and point `.git` at it, `Contains` would match → skip → fail-open. Not reproducible on the target toolchain (git 2.55 emits `(NULL)`, not the path), requires attacker `.git`+path control, version-dependent. Fail-open **direction**, so worth hardening for a security guard. | shipment_gate.go:206-208 | A | MINOR | **LOW** | P3 | advisory / gated_auto |

**Both N1 and N2 are closed by a single message-independent hardening** (recommended,
not blocking): before accepting the parenthetical marker as genuine no-repo, `os.Stat`
`filepath.Join(ws.RootPath, ".git")` — if a `.git` entry (file or directory) is present
but git could not resolve a work tree, **fail closed**; only skip when no `.git` entry
exists at `RootPath`. This enforces the documented "present-but-broken `.git` MUST fail
closed" contract for **all** shapes (N1), and makes classification independent of git's
message wording — closing the substring-injection and version-drift concerns (N2).
Optionally also anchor the match with `HasPrefix` on the trimmed stderr. Add regression
cases: empty `.git` dir → fail closed; corrupt `.git` dir → fail closed.

### SEC re-confirmation

- **SEC-1 (fail-open holes closed under enforcement + real-repo): CONFIRMED (majority
  2/3; F1 vector 3/3).** The three original holes fail closed: empty shipment head
  1AEA2B0E (unborn real worktree → `--is-inside-work-tree` exit 0/`true` → refuse);
  empty member head B85DAEE8 (member-evidence path runs only when `shipmentHead != ""`,
  i.e. a resolved real HEAD); and the F1 broken-`.git`-pointer (`(NULL)` → no
  parenthetical → fail closed, test (d)). **Residual, non-blocking:** the empty/corrupt
  `.git`-**directory** shape (N1) still skips — a contract inconsistency assessed by the
  frontier reviewer as non-exploitable (no readable lineage to bypass), tracked as
  follow-up hardening, not a confirmed exploitable fail-open. Reviewer B's
  NOT-CONFIRMED is attributable entirely to N1.
- **SEC-2 (legitimate empty-head cases preserved, no completion breakage): CONFIRMED
  (3/3).** Genuine no-repo still ships (test (b)); no-repo empty member head stays
  skipped; non-enforcement returns early; bare-repo / inside-`.git` remain exit-0
  non-`true` skips; the tightened match adds **no** new over-refusal of a legitimate
  no-repo (drift is safe-direction fail-closed only). `isAncestor` and the 084
  ancestor/equality/malformed-guard paths are untouched — no regression. F5
  (forced-no-head refusal) not re-litigated — intended design (deliberation Option D).

### Final verdict lines (re-review)

```
RE-REVIEW VERDICT: PASS — F1 (present-but-broken .git pointer) is CLOSED with 3/3
                   consensus; committed RED→GREEN test (d) pins it. No HIGH-confidence
                   P0/P1 consensus finding remains. The locale hardening (withCLocale/
                   hasEnvKey) is correct with no LANGUAGE localization bypass. Two
                   non-blocking residuals recorded: N1 (MEDIUM — empty/corrupt .git
                   DIRECTORY still skips; contract-inconsistent but non-exploitable per
                   frontier empirical analysis) and N2 (LOW — unanchored Contains match,
                   version-dependent fail-open direction). Both close with one optional
                   message-independent hardening (stat RootPath/.git → fail closed if
                   present-but-unresolved).

SEC-1: CONFIRMED — the three enforcement+real-repo fail-open holes (1AEA2B0E empty
       shipment head, B85DAEE8 empty member head, and the F1 broken-.git pointer) all
       fail closed. Residual empty-.git-DIRECTORY skip (N1) is a doc-contract
       inconsistency with negligible security impact (no readable lineage to bypass),
       tracked as follow-up hardening — not a confirmed exploitable bypass.

SEC-2: CONFIRMED — genuine no-repo still ships, no-repo member skip preserved,
       non-enforcement / bare-repo / inside-.git unchanged, message-drift fails in the
       safe (over-refuse) direction, and the 084 ancestry path is untouched. The
       tightened discriminator introduces no new legitimate-completion breakage.
```

### Backlog work items (re-review residuals — non-blocking)

```yaml
- type: bug
  title: "inGitWorktreeBounded: empty/corrupt .git directory skipped instead of fail-closed"
  description: >
    An empty or structurally-invalid .git DIRECTORY (missing HEAD/objects/refs) fails
    git's is_git_directory() check; git walks up and emits the genuine-no-repo
    parenthetical `not a git repository (or any of the parent directories)`, so
    inGitWorktreeBounded returns (false,nil) skip instead of failing closed. This
    contradicts the function's doc-contract ("corrupt .git ... MUST fail closed").
    Frontier-reviewer empirical analysis rates real-world impact negligible (an empty
    .git dir exposes no readable lineage to bypass), but the contract inconsistency
    should be closed for defense-in-depth / anti-tamper consistency.
  file: "internal/core/shipment_gate.go"
  line: 206
  severity: "MINOR"
  confidence: "MEDIUM"
  fix: >
    Before accepting the parenthetical marker as genuine no-repo, os.Stat
    filepath.Join(ws.RootPath, ".git"); if a .git entry (file or dir) exists but git
    did not resolve a work tree, return (false, err) fail closed. Only skip when no
    .git entry exists at RootPath. Add regression tests: empty .git dir -> fail closed,
    corrupt .git dir -> fail closed. This also anchors classification against git's
    message wording (closes N2).
  linked_review: "docs/closure/2026-07-07-085-S-adversarial-review.md"

- type: bug
  title: "inGitWorktreeBounded: unanchored Contains match on no-repo marker (fail-open direction)"
  description: >
    The exit-128 no-repo discriminator uses bytes.Contains on the parenthetical marker.
    If a git version echoes the unresolved gitdir path in its message and an attacker
    with workspace write control names a directory containing the literal marker string
    and points .git at it, Contains would match -> skip -> fail-open. Not reproducible
    on git 2.55 (emits `(NULL)`, not the path); version-dependent, requires attacker
    .git+path control. Fail-open direction, low likelihood.
  file: "internal/core/shipment_gate.go"
  line: 206
  severity: "MINOR"
  confidence: "LOW"
  fix: >
    Replace/augment Contains with the message-independent .git-stat guard above, or
    anchor with HasPrefix on the trimmed, lowercased stderr
    (`fatal: not a git repository (or any of the parent directories)`).
  linked_review: "docs/closure/2026-07-07-085-S-adversarial-review.md"
```

---

## Remediation — N1 + N2 (applied, commit 203a4b1)

Although the re-review VERDICT was **PASS** (N1 MEDIUM / N2 LOW are non-blocking and N1 was empirically non-exploitable), this is a security-hardening PR whose purpose is closing present-but-broken-repo fail-opens, so the single recommended remediation was applied.

**Empirical grounding (git 2.55.0.windows.2):**
- Empty `.git` dir → `fatal: not a git repository (or any of the parent directories): .git` (exit 128) — message COLLIDES with genuine no-repo.
- Garbage `.git` dir → same marker.
- Genuine no-repo → NO `.git` entry at RootPath.

**Fix (commit 203a4b1):** Added a message-INDEPENDENT primary discriminator: `os.Stat(RootPath/.git)` in the exit-!=0 branch. Reaching that branch means git did not resolve a work tree, so a present `.git` entry is a present-but-broken repo → **fail closed** regardless of diagnostic wording (closes N1's empty/corrupt `.git`-dir case and N2's version/locale-drift concern). A genuine outside-any-repo has no `.git` entry there (an ancestor repo would have returned exit 0 `true`). The stderr marker now only distinguishes genuine no-repo (no `.git`) from other fatals. Added empty-`.git`-dir regression case (e) to `TestInGitWorktreeBounded`.

**Post-fix verification:** `TestInGitWorktreeBounded` (a–e) PASS; all gate tests PASS (no-repo skips preserved); `go vet`, `golangci-lint`, `gofmt` (structural), full `go test ./...` clean.

**Net finding disposition:** F1 (P1) fixed (586993f) · N1 (MEDIUM) fixed (203a4b1) · N2 (LOW) fixed (203a4b1) · F2/F3/F4 addressed or unreachable · F5 intended-by-design. **No open findings remain.**
