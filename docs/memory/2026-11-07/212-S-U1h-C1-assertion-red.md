---
title: U1h C1 assertion RED evidence
date: 2026-11-07
timestamp_utc: 2026-11-07T23:31:49Z
task_id: 211.014-T
shipment_id: 212-S
source_head: 25421faa74b5b5e8546e99bd7544db61f12eb8de
status: assertion-red-awaiting-root-fix
---

## Scope and outcome

Only the private `bin\tmp\test_212_full_suite.py` harness changed.
The driver and strict observer are unchanged. No Go command ran: discovery,
LF test, and full-child calls used the existing mocked `_supervise` seam;
real `Popen` remained forbidden there. The ownership group also ran its
existing short, owned Windows Python child.

Both defects complete the exact U1h evidence contract, not a separate
Stage expansion. Native-zero LF output is not named RUN/PASS evidence.
Root must retain exact package/test identity, RUN followed by PASS, and
no skips when implementing the fix.

All three original scenario group names remain unchanged. AST comparison
verified all 60 original assertion calls remain present, including their
arguments; the updated harness has 68 assertion calls. Existing four
preflight scenarios still execute before the new cases.

## Assertion RED receipts

| Capture | Native unittest exit | Observer exit | Groups | Failures | Errors | Skips |
|---|---|---|---|---|---|---|
| Strict GREEN observer at 23:31:14Z | 1 | 2 | 3 | 2 | 0 | 0 |
| Targeted ownership at 23:31:49Z | 1 | Not invoked | 1 | 1 | 0 | 0 |
| Targeted LF at 23:31:49Z | 1 | Not invoked | 1 | 1 | 0 | 0 |

The strict observer invocation was:

```powershell
& .\scripts\verify-u1h-harness.ps1 -Phase Green -ProofDirectory (Join-Path $PWD 'bin\tmp\hcf1107\red')
```

Its native unittest result is FAIL/PASS/FAIL in the unchanged group order:
supervision, authorization, preflight. Both failures are actual
`AssertionError`, not `TypeError` or `AttributeError`. The observer correctly
rejects this partial GREEN; its all-groups/no-skips GREEN criteria were not
weakened. `-Phase Red` was not used: that mode requires the immutable
8058 baseline and three failed groups, neither of which describes this
current-driver C1 RED.

### Ownership and resume evidence

The ordinary projection retains all checked native fields. The oversized
and forced fixed-shape fallback projections each lose `job_assigned` and
`child_resumed`, producing four explicit missing-field observations.
Byte bounds, fallback selection, actor-detail string bounds, PID, native
exit, thread IDs, hold/clear native results, and held/cleared flags passed
their checks before the final failing assertion.

### LF admission evidence

Each native-zero negative case incorrectly launches the mocked full handler:

* Package PASS with no named test match
* Exact named RUN/SKIP, followed by package PASS
* Exact named PASS without RUN
* RUN/PASS for a different named test

For each negative case, both result documents incorrectly report
`prelaunch_only: false` and `test_cycle_consumed: true`, and `result.json`
reports `test_child_launched: true`. The grouped assertion captures all
24 violations, not just the first one.

Exact LF RUN/PASS permits the deliberately failing mocked full handler.
All five cases retain their exact LF JSON output bytes and native-zero
preflight result. Output retention and the valid exact case passed; they
were not skipped or treated as standalone full-suite success.

## Provenance and private captures

Exclusive, read-only backups were created before editing in the new ignored
`bin\tmp\hcf1107` directory. No old backup or 8058 baseline was modified.

| Surface | Before SHA256 | Current SHA256 |
|---|---|---|
| Driver | `c334594ff3f120353e0a0e05bc1fb97bccb70109f100bbd95295cec299539bfc` | `c334594ff3f120353e0a0e05bc1fb97bccb70109f100bbd95295cec299539bfc` |
| Harness | `18a08cafab14b45c45af95d5d369f38dd380c593fa175187b0e9cc32f3b8c1b7` | `814f14a11086e2747eaf5a2a5d2e6ba1a716fca5ec3fc631ef35fe8db6d6d65a` |
| Observer | `00f512d9212975f2a144893dedaceda700c83c6d0f523ddd5439ff5676617e23` | `00f512d9212975f2a144893dedaceda700c83c6d0f523ddd5439ff5676617e23` |

Private evidence paths, intentionally not force-added:

* `bin\tmp\hcf1107\before.json`
* `bin\tmp\hcf1107\driver-before.py`
* `bin\tmp\hcf1107\harness-before.py`
* `bin\tmp\hcf1107\observer-before.ps1`
* `bin\tmp\hcf1107\observer-console.txt`
* `bin\tmp\hcf1107\red\unit-result.json`
* `bin\tmp\hcf1107\red\observer-result.json`
* `bin\tmp\hcf1107\red\driver-before.py` and `driver-after.py`
* `bin\tmp\hcf1107\red\harness-before.py` and `harness-after.py`
* `bin\tmp\hcf1107\ownership-targeted.json`
* `bin\tmp\hcf1107\lf-targeted.json`
* `bin\tmp\hcf1107\targeted-receipt.json`
* `bin\tmp\hcf1107\harness-change.patch`

The strict observer's real Python ownership proof is
`bin\tmp\hn517e30\native-result.json`. Targeted ownership uses the same
fixture; its separate child proof path is recorded in its targeted JSON.
Mocked `_main` output directories are new private `bin\_rr<8hex>` children
and remain available as evidence, not execution authority.

## Persistent harness delta

This delta records the ignored harness change without force-adding it.
Apply only to the verified harness-before bytes above.

```diff
--- harness-before.py
+++ test_212_full_suite.py
@@ -165,6 +165,36 @@
         with (native_directory / "native-result.json").open("x", encoding="utf-8", newline="\n") as stream:
             stream.write(json.dumps({"supervision": native, "calls": native_trace}) + "\n")
         self.__class__.native_proof = str(native_directory.relative_to(RUNNER._ROOT))
+
+        evidence = dict(native, recorder_error="actor-detail-" * 1000)
+        preserved = (
+            "job_assigned", "child_resumed", "pid", "native_exit",
+            "execution_state_held", "execution_state_cleared",
+            "execution_state_thread_id", "execution_state_clear_thread_id",
+            "execution_state_hold_result", "execution_state_clear_result",
+        )
+        violations = []
+        for projection in ("ordinary", "oversized", "fallback"):
+            summary = {"event": "synthetic-ownership", "supervision": dict(evidence)}
+            if projection != "ordinary":
+                summary["all_reported_failures"] = ["x" * 70000]
+            if projection == "fallback":
+                summary["identity"] = {
+                    str(i): {str(j): "x" * 1000 for j in range(32)}
+                    for i in range(32)
+                }
+            encoded = RUNNER._bounded_summary(summary)
+            bounded = json.loads(encoded)
+            self.assertLessEqual(len(encoded.encode("utf-8")), RUNNER._LIMIT)
+            self.assertEqual(bool(bounded.get("essential_detail_omitted")),
+                             projection == "fallback")
+            for key in preserved:
+                if bounded["supervision"].get(key) != evidence[key]:
+                    violations.append(projection + " lost " + key)
+            if projection != "ordinary":
+                self.assertLessEqual(len(bounded["supervision"]["recorder_error"]), 512)
+        self.assertEqual(violations, [],
+                         "C1 ownership/resume evidence must survive every projection")
 
     def test_validate_authorization_accepts_matching_dynamic_ordinal_and_failed_count_and_rejects_mismatch(self):
         def documents(ordinal, failed):
@@ -224,7 +254,7 @@
         core = "github.com/softwaresalt/backlogit/internal/core"
         tests = "github.com/softwaresalt/backlogit/tests"
 
-        def scenario(name, discovery_exit=0, eol_exit=1, full_pid=999):
+        def scenario(name, discovery_exit=0, eol_exit=1, full_pid=999, eol_events=None):
             directory = parent / name
             calls = []
             identity = {"operation": "synthetic-unit-only", "scope": "212-S", "head": HEAD,
@@ -241,14 +271,23 @@
                     info["native_exit"] = discovery_exit
                 elif "^TestU175_001_BaselineEol$" in command:
                     info["native_exit"] = eol_exit
-                    feed(json.dumps({"Package": tests, "Test": "TestU175_001_BaselineEol",
-                                     "Action": "fail" if eol_exit else "pass"}) + "\n")
+                    events = eol_events if eol_events is not None else [
+                        {"Package": tests, "Test": "TestU175_001_BaselineEol", "Action": "run"},
+                        {"Package": tests, "Test": "TestU175_001_BaselineEol",
+                         "Action": "fail" if eol_exit else "pass"},
+                    ]
+                    eol_output = "".join(json.dumps(event) + "\n" for event in events)
+                    for line in eol_output.splitlines(keepends=True):
+                        feed(line)
                 else:
                     # Synthetic suite results only; Popen is separately forbidden.
                     info["native_exit"], info["pid"] = 1, full_pid
                 for key in ("raw_path", "stderr_path"):
                     if kwargs.get(key):
-                        Path(kwargs[key]).write_bytes(b"synthetic unit evidence only\n")
+                        content = (eol_output.encode("utf-8")
+                                   if "^TestU175_001_BaselineEol$" in command and key == "raw_path"
+                                   else b"synthetic unit evidence only\n")
+                        Path(kwargs[key]).write_bytes(content)
                 return info
 
             argv = ["--authorization-token-file", "synthetic-token",
@@ -302,6 +341,44 @@
             result = json.loads((directory / "result.json").read_text(encoding="utf-8"))
             self.assertIs(result["test_child_launched"], launched)
 
+        exact = {"Package": tests, "Test": "TestU175_001_BaselineEol"}
+        run = dict(exact, Action="run")
+        passed = dict(exact, Action="pass")
+        violations = []
+        for name, events, allowed in (
+                ("lf-no-match", [{"Package": tests, "Action": "output",
+                                  "Output": "testing: warning: no tests to run\n"},
+                                 {"Package": tests, "Action": "pass"}], False),
+                ("lf-skipped", [run, dict(exact, Action="skip"),
+                                {"Package": tests, "Action": "pass"}], False),
+                ("lf-pass-only", [passed], False),
+                ("lf-wrong-name", [dict(run, Test="TestOther"),
+                                   dict(passed, Test="TestOther")], False),
+                ("lf-valid-exact", [run, passed, {"Package": tests, "Action": "pass"}], True)):
+            directory, calls, native = scenario(name, eol_exit=0, eol_events=events)
+            expected_output = "".join(json.dumps(event) + "\n" for event in events).encode("utf-8")
+            self.assertEqual((directory / "preflight-eol.raw").read_bytes(), expected_output,
+                             name + " must retain exact LF JSON output")
+            self.assertEqual(json.loads((directory / "preflight-eol-result.json").read_text(
+                encoding="utf-8"))["native_exit"], 0)
+            self.assertNotEqual(native, 0, "mock full handler deliberately fails")
+            full_calls = [command for command, _ in calls
+                          if command[1] == "test" and command[-1] == "./..."]
+            if bool(full_calls) != allowed:
+                violations.append(name + " full child launch=" + str(bool(full_calls)))
+            for filename in ("result.json", "recorder-result.json"):
+                result = json.loads((directory / filename).read_text(encoding="utf-8"))
+                for key, expected in (("prelaunch_only", not allowed),
+                                      ("test_cycle_consumed", allowed)):
+                    if result.get(key) is not expected:
+                        violations.append(name + " " + filename + " " + key
+                                          + "=" + str(result.get(key)))
+            result = json.loads((directory / "result.json").read_text(encoding="utf-8"))
+            if result["test_child_launched"] is not allowed:
+                violations.append(name + " test_child_launched=" + str(result["test_child_launched"]))
+        self.assertEqual(violations, [],
+                         "C1 native zero requires exact LF RUN/PASS and no skips")
+
 
 class ProofResult(unittest.TextTestResult):
     def __init__(self, *args, **kwargs):
```

## Root handoff

Root owns the driver fix and subsequent strict GREEN validation. No native
Go acceptance case, full suite, real list/discovery, token operation,
backlog mutation, checkpoint, PR, push, or merge was performed. All primary
Go source remains untouched. No nested agents were used.

These are RED evidence captures, not replacement GREEN attempts or a reset
of the existing three-attempt/one-native-failure budget and review cycle.
Root owns remaining-cycle accounting.

Compact-context was invoked for this task's memory scope as a bounded
assessment: active U1h recovery/evidence cannot be compacted; no candidate
was selected, no archive or tracking mutation was performed.
The sole eligible tracked change is this proof; the private harness and
backups stay ignored. Commit type is `test(tests)`; no push is authorized.
