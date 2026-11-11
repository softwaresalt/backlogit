---
title: U1h C1 bounded source GREEN handoff
date: 2026-11-07
timestamp_utc: 2026-11-07T23:37:08Z
task_id: 211.014-T
shipment_id: 212-S
source_head: 5cd22acfbc50841726bed37d5c88eeb9a2756173
status: source-green-awaiting-root-review
---

## Scope and accounting

Only the ignored private `bin\tmp\212-full-suite.py` source changed.
AST comparison identifies exactly `_bounded_summary` and `_main`.
The existing genuine assertion RED is recorded in
`212-S-U1h-C1-assertion-red.md`; no RED or attempt counter was reset.

Source attempt is now 2/3, with one remaining. The prior native failure
count remains one. Review-fix cycle remains 1/3; Root owns review completion.
Historical six failures and cycle-seven evidence are not changed or replayed.
Task status, parent checkpoints, authority receipts, and harness metadata
were not mutated.

ProposedAction: complete the two authorized private verification contracts.
ActionRisk: moderate, local contract edit. Approval is the bounded C1 request.
ActionResult: applied and synthetic GREEN verified, pending Root review.
Rollback reference is the exclusive, read-only current-driver backup below;
no rollback, overwrite of prior evidence, or cleanup was performed.

## Provenance

| Surface | SHA256 |
|---|---|
| Exclusive driver backup | `c334594ff3f120353e0a0e05bc1fb97bccb70109f100bbd95295cec299539bfc` |
| New driver | `9aead967085d5a9f45a2705ba4d2035e15b4cf71e160f8dacd4ecf1b500b957e` |
| Unchanged harness | `814f14a11086e2747eaf5a2a5d2e6ba1a716fca5ec3fc631ef35fe8db6d6d65a` |
| Unchanged observer | `00f512d9212975f2a144893dedaceda700c83c6d0f523ddd5439ff5676617e23` |
| Private source diff | `e0874968a4c065dd6cb992b88fc0abc8d6cd779a5b6c3a9badfa91cc844fc674` |

The new exclusive backup is
`bin\tmp\hcf1107\c1-fix\driver-before.py`.
`before.json` records the actual full HEAD, unrelated dirty status, and hashes
of every present tracked Go file before the source edit.
`validation.json` verifies those Go bytes remain unchanged.
All 60 original harness assertion calls, including arguments, remain
present among the current 68 assertions.

## GREEN and refusal evidence

The unchanged observer ran at `2026-11-07T23:36:40Z`:

```powershell
& .\scripts\verify-u1h-harness.ps1 -Phase Green -ProofDirectory (Join-Path $PWD 'bin\tmp\hcf1107\c1-fix\green')
```

Native unittest exit 0; observer exit 0; guard PASS.
All three original named groups passed: supervision, authorization, preflight.
Failures 0, errors 0, skips 0. Both new C1 cases and all original cases ran.
Exact LF RUN/PASS admits only the mocked full handler in the UNIT scenario.

Additional synthetic admission checks refused empty, wrong-package,
FAIL-after-PASS, SKIP-after-PASS, vacuous-output-after-PASS, PASS-before-RUN,
duplicate-RUN, and non-JSON LF streams.
Each refusal reports `ValueError: LF preflight failed; test child not launched`.
Native-zero preflight remains separate from null full-suite native evidence;
recorder exit is 2, prelaunch-only is true, and consumption is false.
The original harness additionally refuses missing named evidence, named SKIP,
PASS-only, and wrong-name evidence while preserving exact raw LF bytes.

A valid LF stream with a mocked `synthetic NoChild: creation refused` failure
reaches the mock full handler but has no actual child PID. It retains
prelaunch-only true, consumption false, null suite native, and recorder exit 2.
No full-child consumption is inferred from the planned command.
All output directories are exclusive; the original no-clobber checks pass.

## Power, poll, and budget preservation

The owned tiny Windows Python child proof is
`bin\tmp\hn798e7c\native-result.json`: PID 20612, native exit 0,
no supervisor or cleanup errors, job assigned and child resumed.
Hold and clear both ran on thread 19416, returning nonzero native results
2147483648 and 2147483649 respectively.
Maximum poll gap was 0.010532499756664038 seconds; over-five-second count 0.

The existing simulated six-second polling-gap case still reaches timeout
under the unchanged 2700-second wall budget without subtracting poll gaps.
The tiny real child uses the existing ten-second UNIT timeout.
Thread hold/clear implementation and explicit denial/cancellation handling,
authorization pairing and strict integers, ownership helpers, snapshot
helpers, and exclusive output logic are unchanged.
Ownership/resume fields now survive ordinary, oversized, and fixed fallback
projections with the existing native, hold/clear, and poll state.

## Persistent private source delta

The driver stays ignored and is never force-added. This diff applies only
to the verified current-driver backup above.

```diff
--- driver-before.py
+++ 212-full-suite.py
@@ -219,6 +219,7 @@
         "bytes_retained", "bytes_omitted", "line_characters_omitted",
         "lines_total", "lines_retained", "lines_omitted",
         "supervisor_error", "recorder_error", "cleanup_errors", "pid",
+        "job_assigned", "child_resumed",
         "execution_state_held", "execution_state_cleared",
         "execution_state_thread_id", "execution_state_clear_thread_id",
         "execution_state_hold_result", "execution_state_clear_result",
@@ -665,16 +666,52 @@
         recorder = _Recorder({line.strip() for line in listing if line.strip()})
         if not recorder.expected:
             raise ValueError("Go discovery returned no packages; test child not launched")
+        lf_counts = collections.Counter()
+        lf_invalid = False
+
+        def feed_lf(line):
+            nonlocal lf_invalid
+            try:
+                row = json.loads(line)
+                if not isinstance(row, dict):
+                    raise ValueError("non-object LF event")
+                package, test, action = (row.get("Package", ""), row.get("Test", ""),
+                                         row.get("Action", ""))
+                if (package != "github.com/softwaresalt/backlogit/tests"
+                        or not isinstance(test, str) or not isinstance(action, str)):
+                    lf_invalid = True
+                    return
+                if action in ("fail", "skip"):
+                    lf_invalid = True
+                if action == "output":
+                    output = row.get("Output", "")
+                    if (not isinstance(output, str)
+                            or re.search(r"no tests to run|no test files|\bSKIP\b|\bFAIL\b",
+                                         output, re.IGNORECASE)):
+                        lf_invalid = True
+                if test:
+                    if test != "TestU175_001_BaselineEol":
+                        lf_invalid = True
+                    elif action in ("run", "pass"):
+                        if (action == "run" and (lf_counts["run"] or lf_counts["pass"])
+                                or action == "pass" and (lf_counts["run"] != 1
+                                                        or lf_counts["pass"])):
+                            lf_invalid = True
+                        lf_counts[action] += 1
+            except (ValueError, TypeError):
+                lf_invalid = True
+
         preflight_eol = _supervise(
             [go, "test", "-count=1", "-timeout=2m", "-json", "-run",
-             "^TestU175_001_BaselineEol$", "./tests"], lambda _: None, timeout=120,
+             "^TestU175_001_BaselineEol$", "./tests"], feed_lf, timeout=120,
             raw_path=directory / "preflight-eol.raw",
             stderr_path=directory / "preflight-eol.stderr.raw")
         with (directory / "preflight-eol-result.json").open("x", encoding="utf-8", newline="\n") as stream:
             stream.write(json.dumps(preflight_eol) + "\n")
         if (preflight_eol["native_exit"] != 0 or preflight_eol["supervisor_error"]
                 or preflight_eol["timed_out"] or preflight_eol.get("cleanup_errors")
-                or preflight_eol.get("drain_truncated") or preflight_eol.get("line_characters_omitted")):
+                or preflight_eol.get("drain_truncated") or preflight_eol.get("line_characters_omitted")
+                or lf_invalid or lf_counts["run"] != 1 or lf_counts["pass"] != 1):
             raise ValueError("LF preflight failed; test child not launched")
         command = [go, "test", "-count=1", "-p=1", "-timeout=30m", "-json", "./..."]
         print(json.dumps({"event": "FULL_SUITE_BEGIN", "identity": identity, "args": command}),
```

## Root handoff

No actual Go list, LF test, full suite, package suite, token minting, global
OS configuration, permissions, network, unknown-process termination, push,
PR, merge, branch change, nested delegation, or backlog mutation occurred.
Go build/vet/lint/full-suite gates are outside this authorized synthetic scope.
No GoRoot, durable source, hibernation authority, producer, harness, or observer
change was made. Missing production learnings coverage remains knowledge-only
Root work, not an additional source defect.

Compact-context was invoked as a bounded assessment for this active U1h
memory scope: preserve active evidence and cursor; no candidate selected,
no archive or tracking mutation. This proof is the only eligible tracked
artifact for an explicit-path commit. Root must perform the current review;
this handoff does not declare review, task, or shipment completion.
