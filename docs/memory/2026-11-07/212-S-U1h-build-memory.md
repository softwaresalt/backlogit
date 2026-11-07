---
title: U1h committed GREEN handoff
date: 2026-11-07
task_id: 211.014-T
shipment_id: 212-S
source_head: 5f91f25f15bb94531fcd97cf28f0c14b53446c39
status: awaiting-root-review
---

## Outcome

* Implemented only the four approved private driver functions for U1h.
* `& .\scripts\verify-u1h-harness.ps1 -Phase Green` passed all three
  named groups without skips, errors, or failures; native and observer exits 0.
* The exact authorized LF target passed with offline pinned Go 1.24.0:
  `go test -count=1 -timeout=2m -run '^TestU175_001_BaselineEol$' ./tests`.
* No real Go discovery or full repository suite ran. All `_main` Go children
  were mocked; the LF acceptance command was the sole actual Go test child.
* Task remains active. Root owns standard/adversarial review, completion,
  archival, receipt issuance, checkpoint updates, and the final full gate.

## Preservation and provenance

* Before editing, exclusively created ignored `bin\tmp\h76dda9` and made its
  exact baseline copies read-only.
* Driver baseline SHA256:
  `8058bd5e1958adc502669dd509811b986fabd4010e3768f429bd2e84b00dc250`
* Initial harness SHA256:
  `d3624f020514225f556eac9d22d4159c420a165e46305cd48c424a12bf8dccfd`
* Initial observer SHA256:
  `6e8457a05f3366b89538de2f69f4a179d501202b899c5a505a9a43f5fa4efd77`
* Tracked evidence is in `212-S-U1h-green`: exact raw-byte base64 snapshots,
  readable driver/harness diffs, attempt receipts, native proof, LF proof,
  and SHA256 provenance. Base64 preserves original CRLF bytes without relying
  on Git newline conversion; evidence metadata is UTF-8 LF.
* Final driver SHA256:
  `c334594ff3f120353e0a0e05bc1fb97bccb70109f100bbd95295cec299539bfc`
* Final harness SHA256:
  `18a08cafab14b45c45af95d5d369f38dd380c593fa175187b0e9cc32f3b8c1b7`
* Final observer SHA256:
  `00f512d9212975f2a144893dedaceda700c83c6d0f523ddd5439ff5676617e23`

## Native evidence and constraints

* Final owned Python child PID 37692 exited 0. Hold and clear executed on
  polling thread 24192, returning 2147483648 and 2147483649 respectively;
  held/cleared flags were true and cleanup/supervisor errors were absent.
* The LF child PID 30892 exited 0, with hold/clear on thread 50352 and no
  supervisor or cleanup errors.
* Synthetic controls cover denied hold with no child, denied clear with
  explicit cleanup error, launch failure, cancellation/native exit preservation,
  six-second polling gaps without deadline extension, and bounded fallback.
* Authorization tests cover paired dynamic integers, boolean/float rejection,
  scope/head/raw-token hash/issuer/receipt ID/budget mismatches.
* Mocked discovery/LF failures preserve exclusive outputs and block the full
  child. Mocked failed/no-PID full launch remains unconsumed; an actual mocked
  full-child PID is consumed regardless of native failure.
* Automatic idle sleep is mitigated only while the polling thread holds the
  request. Manual sleep, hibernation, and power loss remain possible.
* Outer budget stays 2700 seconds; Go package timeout stays 1800 seconds.
  No global power configuration changed.

## Attempts and handoff

* Reconfirmed assertion RED: three groups, native 1, observer 0.
* Three GREEN invocations: native exits 1, 0, 0; one native failure.
  Attempt 1 exposed Windows text stdout CRLF in the new short-child assertion.
  The child now writes explicit LF bytes; no original assertion was weakened.
* Wave inputs stayed `green_regression_cmds=[]`, `sibling_red_selectors=[]`,
  `open_red_selectors=[]`, `red_deliverable=false`. Tests-only is not exempt.
* No full-gate waiver or execution receipt was issued. Six historical full
  failures, prior raw/token files, U1g, U3-U7, Stage artifacts, checkpoint
  210317, and unrelated dirty backlog/archive state were not modified.
* Invoked compact-context for this task's memory scope: no eligible candidates;
  active recovery and existing tracking state remain preserved.
* No push, PR, merge, status completion, archival, or commit association.
  Next: Root reviews the committed evidence and private exact-byte driver.
