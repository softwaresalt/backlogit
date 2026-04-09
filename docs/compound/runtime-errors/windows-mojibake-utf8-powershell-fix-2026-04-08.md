---
title: "Windows-1252 Mojibake in UTF-8 JSONL — Detection and Fix with PowerShell"
problem_type: runtime_error
category: runtime_error
component: event_log
root_cause: encoding_error
resolution_type: code_fix
severity: medium
message: "Windows-1252 bytes written to a UTF-8 JSONL file produce mojibake characters that break JSON parsing."
file_path: ".backlogit/stash.jsonl"
resolved: true
tags: [encoding, utf-8, windows-1252, mojibake, powershell, jsonl, go, windows]
date: 2026-04-08
---

## Problem

A stash entry in `.backlogit/stash.jsonl` contained a Unicode em dash (U+2014, `—`)
that was written or copy-pasted through a Windows-1252 codepath. The three UTF-8
bytes for U+2014 (`0xE2 0x80 0x94`) were misinterpreted as three separate
Windows-1252 characters: `â` (U+00E2), `€` (U+20AC), `"` (U+201D). The JSONL
line was syntactically valid but contained the three-codepoint sequence
`â€"` wherever the em dash was intended.

The stash file is read by Go's `encoding/json` which requires valid UTF-8. The
mojibake sequence is valid UTF-8 (each codepoint is representable), so parsing
succeeded — but the `text` field contained garbled output that surfaced in MCP
tool responses and agent context.

## Symptoms

* Stash entry `text` field contains `â€"`, `â€œ`, or `â€` where em dashes or
  curly quotes were expected
* `backlogit_fetch_stash` returns readable JSON but with gibberish characters
* The pattern appears only in entries that were typed or pasted on Windows with
  a non-UTF-8 terminal or text editor default
* Hex dump of the file shows `C3 A2 E2 82 AC E2 80 9D` (the three separate
  Windows-1252 codepoints encoded as UTF-8) instead of `E2 80 94` (U+2014)

## What Did Not Work

Editing the file in VS Code and saving — VS Code respects the existing encoding
but does not automatically detect mojibake within a correctly-encoded UTF-8 file.
Using `Get-Content | Set-Content` in PowerShell without explicit encoding
parameters — PowerShell defaults to the system codepage on some versions,
re-encoding the file and compounding the corruption. Using `[System.Text.Encoding]::UTF8.GetString`
on file bytes read as `[byte[]]` — this reads the raw bytes correctly but the
replacement requires careful string-level substitution to avoid re-encoding.

## Solution

Read the file with `[System.IO.File]::ReadAllText` and write it back with
`[System.IO.File]::WriteAllText` using an explicit no-BOM UTF-8 encoding.
Replace the mojibake sequences with their intended characters using PowerShell
string `.Replace()`.

```powershell
$path = ".backlogit\stash.jsonl"

# Read as UTF-8 (works cross-platform; does not add BOM)
$text = [System.IO.File]::ReadAllText($path, [System.Text.Encoding]::UTF8)

# Em dash (U+2014): Windows-1252 bytes 0xE2 0x80 0x94 read as three codepoints
# U+00E2 (â), U+20AC (€), U+201D (")
$emDashMojibake = [char]0x00E2 + [char]0x20AC + [char]0x201D
$text = $text.Replace($emDashMojibake, "--")

# Left double curly quote (U+201C): bytes 0xE2 0x80 0x9C → â€œ
$leftQuoteMojibake = [char]0x00E2 + [char]0x20AC + [char]0x009C
$text = $text.Replace($leftQuoteMojibake, '"')

# En dash (U+2013): bytes 0xE2 0x80 0x93 → â€"
$enDashMojibake = [char]0x00E2 + [char]0x20AC + [char]0x009D
$text = $text.Replace($enDashMojibake, "--")

# Write back with explicit no-BOM UTF-8
[System.IO.File]::WriteAllText($path, $text, [System.Text.UTF8Encoding]::new($false))
Write-Host "Fixed. Re-validate with: Get-Content $path -Encoding UTF8 | ConvertFrom-Json"
```

### Verification

```powershell
# Verify no remaining mojibake patterns
$content = Get-Content ".backlogit\stash.jsonl" -Raw
if ($content -match 'â€') {
    Write-Warning "Possible remaining mojibake detected"
} else {
    Write-Host "Clean — no mojibake patterns found"
}

# Validate JSON structure
Get-Content ".backlogit\stash.jsonl" | ForEach-Object {
    if ($_.Trim()) { $_ | ConvertFrom-Json | Out-Null }
}
Write-Host "All lines parse as valid JSON"
```

## Why This Works

`[System.IO.File]::ReadAllText` with explicit `UTF8` encoding reads the file
bytes and interprets them as UTF-8, producing .NET strings with the actual
codepoints (U+00E2, U+20AC, U+201D for the em-dash mojibake triplet). The
`.Replace()` call substitutes the three-codepoint sequence with the intended
character. `WriteAllText` with `UTF8Encoding::new($false)` writes the result
as UTF-8 without a byte-order mark, which is the correct format for JSONL files
consumed by Go's `encoding/json`.

The key insight: the file bytes are valid UTF-8 throughout. The problem is that
the bytes represent the wrong Unicode codepoints — three Windows-1252 characters
encoded as UTF-8 instead of the single intended character encoded as UTF-8.
Fixing it requires string-level (not byte-level) substitution after decoding.

## Prevention

* Configure text editors and terminals on Windows to use UTF-8 by default.
  In Windows 11+: Settings → Time & Language → Language & Region → Administrative
  Language Settings → Change system locale → Beta: Use Unicode UTF-8.
* When accepting user-supplied text in Go, normalize it with
  `golang.org/x/text/encoding/charmap` if the source encoding is unknown.
* Validate JSONL files after any manual edit on Windows with a simple
  `Get-Content | ConvertFrom-Json` roundtrip in PowerShell.
* Add a CI check that validates `.backlogit/stash.jsonl` parses cleanly:
  ```bash
  while IFS= read -r line; do echo "$line" | python3 -c "import sys,json; json.load(sys.stdin)"; done < .backlogit/stash.jsonl
  ```
* Prefer ASCII alternatives (`--` for em dash, `"` for curly quotes) in
  machine-generated JSONL to eliminate the encoding surface entirely.

## Common Mojibake Substitution Table

| Intended char | Unicode | Windows-1252 mojibake | Replacement |
|---|---|---|---|
| Em dash `—` | U+2014 | `â€"` | `--` or `—` |
| Left curly quote `"` | U+201C | `â€œ` | `"` |
| Right curly quote `"` | U+201D | `â€` | `"` |
| En dash `–` | U+2013 | `â€"` | `--` or `–` |
| Ellipsis `…` | U+2026 | `â€¦` | `...` |
| Bullet `•` | U+2022 | `â€¢` | `*` |

## Related Solutions

* `go-patterns/f015-shipment-stash-patterns.md` — stash JSONL format and
  structure conventions
