<#
.SYNOPSIS
Terminal zero-warning global runner screening both supported GOOS surfaces.

.DESCRIPTION
Screens the `windows` and `linux` surfaces independently. Prints exactly
TERMINAL-LINT-OK:<FeatureId> only when both surfaces report zero residual
warnings. Both surfaces are always evaluated so a residual on either one fails.
Real golangci-lint is never run and the repository is never mutated.

Fixture injection: -FixtureRoot points at a directory containing windows.json
and linux.json, each:
  { "goos": "<surface>", "findings": [ { "path", "linter", "message", "line", "column" } ] }
A surface is clean iff its findings array is empty. Missing FixtureRoot or a
missing/malformed surface fixture fails closed.

Each fixture goos must equal its surface name (case-sensitive); otherwise fail closed.

Exit codes: 0 both clean; 1 residual warnings on at least one surface;
2 invalid or missing input / precondition.

.PARAMETER FeatureId
Feature id such as 175-F (required).
.PARAMETER FixtureRoot
Directory holding windows.json and linux.json.
#>
[CmdletBinding()]
param(
    [string]$FeatureId,
    [string]$FixtureRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Stop-Closed([string]$Message, [int]$Code = 2) {
    [Console]::Error.WriteLine("TERMINAL-LINT-FAIL: INVALID-INPUT: $Message")
    exit $Code
}

function Test-DupKey([System.Text.Json.JsonElement]$El) {
    if ($El.ValueKind -eq [System.Text.Json.JsonValueKind]::Object) {
        $seen = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
        foreach ($p in $El.EnumerateObject()) {
            if (-not $seen.Add($p.Name)) { return $true }
            if (Test-DupKey $p.Value) { return $true }
        }
    }
    elseif ($El.ValueKind -eq [System.Text.Json.JsonValueKind]::Array) {
        foreach ($e in $El.EnumerateArray()) { if (Test-DupKey $e) { return $true } }
    }
    return $false
}

function ConvertFrom-JsonElement([System.Text.Json.JsonElement]$El) {
    switch ($El.ValueKind) {
        ([System.Text.Json.JsonValueKind]::Object) {
            $h = [ordered]@{}
            foreach ($p in $El.EnumerateObject()) { $h[$p.Name] = ConvertFrom-JsonElement $p.Value }
            return [pscustomobject]$h
        }
        ([System.Text.Json.JsonValueKind]::Array) {
            $l = [System.Collections.Generic.List[object]]::new()
            foreach ($e in $El.EnumerateArray()) { $l.Add((ConvertFrom-JsonElement $e)) }
            return , $l.ToArray()
        }
        ([System.Text.Json.JsonValueKind]::String) { return $El.GetString() }
        ([System.Text.Json.JsonValueKind]::True) { return $true }
        ([System.Text.Json.JsonValueKind]::False) { return $false }
        ([System.Text.Json.JsonValueKind]::Number) {
            $raw = $El.GetRawText()
            if ($raw -cmatch '^-?[0-9]+\z') {
                $n = [System.Numerics.BigInteger]::Parse($raw, [System.Globalization.CultureInfo]::InvariantCulture)
                if ($n -ge [long]::MinValue -and $n -le [long]::MaxValue) { return [long]$n }
                return $n
            }
            return [double]::Parse($raw, [System.Globalization.NumberStyles]::Float, [System.Globalization.CultureInfo]::InvariantCulture)
        }
        default { return $null }
    }
}

function Read-StrictJson([string]$Path, [string]$Goos) {
    $text = Get-Content -LiteralPath $Path -Raw
    if ([string]::IsNullOrWhiteSpace($text)) { Stop-Closed "malformed $Goos fixture: empty" }
    try { $jd = [System.Text.Json.JsonDocument]::Parse($text) }
    catch { Stop-Closed "malformed $Goos fixture: $($_.Exception.Message)" }
    try {
        if ($jd.RootElement.ValueKind -ne [System.Text.Json.JsonValueKind]::Object) { Stop-Closed "malformed $Goos fixture: top level must be an object" }
        if (Test-DupKey $jd.RootElement) { Stop-Closed "malformed $Goos fixture: duplicate object keys" }
        return (ConvertFrom-JsonElement $jd.RootElement)
    }
    finally { $jd.Dispose() }
}

try {
    if ([string]::IsNullOrWhiteSpace($FeatureId)) { Stop-Closed 'FeatureId is required' }
    if ($FeatureId -cnotmatch '^[0-9]+-F\z') { Stop-Closed "invalid FeatureId '$FeatureId'" }
    if ([string]::IsNullOrWhiteSpace($FixtureRoot)) { Stop-Closed 'FixtureRoot is required (real lint is never run)' }

    $residual = @{}
    foreach ($goos in @('windows', 'linux')) {
        $path = Join-Path $FixtureRoot "$goos.json"
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { Stop-Closed "missing $goos surface fixture: $path" }
        $doc = Read-StrictJson $path $goos
        if ($null -eq $doc -or $doc -isnot [pscustomobject] -or $null -eq $doc.PSObject.Properties['findings']) {
            Stop-Closed "malformed $goos fixture: findings missing"
        }
        if ($null -eq $doc.PSObject.Properties['goos'] -or $doc.goos -isnot [string] -or $doc.goos -cne $goos) {
            Stop-Closed "malformed $goos fixture: goos must equal '$goos'"
        }
        $list = $doc.findings
        if ($list -isnot [System.Array]) { Stop-Closed "malformed $goos fixture: findings must be an array" }
        foreach ($m in $list) {
            if ($null -eq $m -or $m -isnot [pscustomobject]) { Stop-Closed "malformed $goos fixture: findings members must be objects" }
            foreach ($n in 'path', 'linter') {
                $v = $m.PSObject.Properties[$n]?.Value
                if ($v -isnot [string] -or [string]::IsNullOrWhiteSpace($v)) { Stop-Closed "malformed $goos fixture: finding $n must be a non-empty string" }
            }
            if ($m.PSObject.Properties['message']?.Value -isnot [string]) { Stop-Closed "malformed $goos fixture: finding message must be a string" }
        }
        $residual[$goos] = $list
    }

    $failed = $false
    foreach ($goos in @('windows', 'linux')) {
        foreach ($f in $residual[$goos]) {
            Write-Output ("SURFACE-RESIDUAL[{0}]: {1} {2} {3}" -f $goos, $f.path, $f.linter, $f.message)
            $failed = $true
        }
    }
    if ($failed) { exit 1 }

    Write-Output "TERMINAL-LINT-OK:$FeatureId"
    exit 0
}
catch {
    [Console]::Error.WriteLine("TERMINAL-LINT-FAIL: INVALID-INPUT: $($_.Exception.Message)")
    exit 2
}
