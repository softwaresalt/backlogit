<#
.SYNOPSIS
Per-task lint verification runner for the baseline-convergence feature.

.DESCRIPTION
Compares, for each group fingerprint listed by -TaskId (including zero-count
groups), the number of ALL remaining findings with that fingerprint (any owner)
against the task's expected_group_residual_after_task under the line-stable-group-multiset/v1
identity model: findings are keyed on (path, linter, normalized_message) with
multiplicity; line and column are never compared.

Fixture injection: -FixtureRoot points at a directory containing task-lint.json:
  { "tasks": { "<id>": { "expected_group_residual_after_task":
        [ { "path", "linter", "message", "count" } ] } },
    "findings": [ { "path", "linter", "message", "line", "column", "owner" (optional, ignored) } ] }
Groups not listed for the task are not compared.
Without -FixtureRoot the runner fails closed (it never runs real golangci-lint
and never mutates the repository).

Exit codes: 0 match (prints TASK-LINT-OK:<TaskId>); 1 residual mismatch;
2 invalid or missing input / unknown task / unreadable fixture.

.PARAMETER TaskId
Task id such as 175.001-T (required).
.PARAMETER FeatureId
Owning feature id such as 175-F (required).
.PARAMETER Phase
Schema (validate inputs and fixture only), PreCommit or PostCommit (default).
.PARAMETER FixtureRoot
Directory holding deterministic JSON fixtures.
#>
[CmdletBinding()]
param(
    [string]$TaskId,
    [string]$FeatureId,
    [string]$Phase = 'PostCommit',
    [string]$FixtureRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Stop-Closed([string]$Message, [int]$Code = 2) {
    [Console]::Error.WriteLine("TASK-LINT-FAIL: INVALID-INPUT: $Message")
    exit $Code
}

function Test-NonNegInt($Value) {
    if ($null -eq $Value -or $Value -is [bool] -or $Value -is [string]) { return $false }
    if ($Value -isnot [int] -and $Value -isnot [long] -and $Value -isnot [double] -and $Value -isnot [decimal] -and $Value -isnot [System.Numerics.BigInteger]) { return $false }
    if ($Value -is [double] -and ([double]::IsNaN($Value) -or [double]::IsInfinity($Value))) { return $false }
    if ($Value -lt 0 -or $Value -gt 2147483647) { return $false }
    return ($Value -eq [math]::Truncate([decimal]$Value))
}
function Get-Fingerprint($Item) {
    $msg = ([string]$Item.message).Normalize([System.Text.NormalizationForm]::FormC)
    $msg = ($msg -replace '\s+', ' ').Trim()
    return (ConvertTo-Json -InputObject @($Item.path, $Item.linter, $msg) -Compress)
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

function Read-StrictJson([string]$Path) {
    $text = Get-Content -LiteralPath $Path -Raw
    if ([string]::IsNullOrWhiteSpace($text)) { Stop-Closed 'fixture is empty' }
    try { $doc = [System.Text.Json.JsonDocument]::Parse($text) }
    catch { Stop-Closed "malformed fixture: $($_.Exception.Message)" }
    try {
        if ($doc.RootElement.ValueKind -ne [System.Text.Json.JsonValueKind]::Object) { Stop-Closed 'fixture top level must be an object' }
        if (Test-DupKey $doc.RootElement) { Stop-Closed 'fixture contains duplicate object keys' }
    }
    finally { $doc.Dispose() }
    $opts = @{}
    if ((Get-Command ConvertFrom-Json).Parameters.ContainsKey('DateKind')) { $opts['DateKind'] = 'String' }
    return (ConvertFrom-Json -InputObject $text @opts)
}

function Assert-Fingerprint($Item, [string]$Where) {
    foreach ($n in 'path', 'linter') {
        $v = $Item.PSObject.Properties[$n]?.Value
        if ($v -isnot [string] -or [string]::IsNullOrWhiteSpace($v)) { Stop-Closed "$Where $n must be a non-empty string" }
    }
    if ($Item.PSObject.Properties['message']?.Value -isnot [string]) { Stop-Closed "$Where message must be a string" }
}

try {
    if ([string]::IsNullOrWhiteSpace($TaskId)) { Stop-Closed 'TaskId is required' }
    if ($TaskId -cnotmatch '^[0-9]+(\.[0-9]+)*-(T|ST)\z') { Stop-Closed "invalid TaskId '$TaskId'" }
    if ([string]::IsNullOrWhiteSpace($FeatureId)) { Stop-Closed 'FeatureId is required' }
    if ($FeatureId -cnotmatch '^[0-9]+-F\z') { Stop-Closed "invalid FeatureId '$FeatureId'" }
    if ($Phase -cnotin 'Schema', 'PreCommit', 'PostCommit') { Stop-Closed "invalid Phase '$Phase'" }
    if ([string]::IsNullOrWhiteSpace($FixtureRoot)) { Stop-Closed 'FixtureRoot is required (real lint is never run)' }

    $fixturePath = Join-Path $FixtureRoot 'task-lint.json'
    if (-not (Test-Path -LiteralPath $fixturePath -PathType Leaf)) { Stop-Closed "fixture not found: $fixturePath" }
    $fixture = Read-StrictJson $fixturePath
    if ($null -eq $fixture.PSObject.Properties['tasks'] -or $null -eq $fixture.PSObject.Properties['findings']) { Stop-Closed 'fixture missing tasks/findings' }
    if ($fixture.tasks -isnot [pscustomobject]) { Stop-Closed 'fixture tasks must be an object' }

    $task = $fixture.tasks.PSObject.Properties[$TaskId]
    if ($null -eq $task) { Stop-Closed "unknown task id '$TaskId'" }

    foreach ($tp in $fixture.tasks.PSObject.Properties) {
        if ($tp.Name -cnotmatch '^[0-9]+(\.[0-9]+)*-(T|ST)\z') { Stop-Closed "invalid task key '$($tp.Name)'" }
        $tobj = $tp.Value
        if ($null -eq $tobj -or $tobj -isnot [pscustomobject] -or
            $null -eq $tobj.PSObject.Properties['expected_group_residual_after_task'] -or
            $tobj.expected_group_residual_after_task -isnot [System.Array]) {
            Stop-Closed "task '$($tp.Name)' must be an object with an expected_group_residual_after_task array"
        }
        $seenGroups = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
        foreach ($m in $tobj.expected_group_residual_after_task) {
            if ($null -eq $m -or $m -isnot [pscustomobject]) { Stop-Closed "task '$($tp.Name)' residual members must be objects" }
            Assert-Fingerprint $m "task '$($tp.Name)' residual"
            if (-not (Test-NonNegInt $m.PSObject.Properties['count']?.Value)) { Stop-Closed "task '$($tp.Name)' residual count must be a non-negative integer" }
            if (-not $seenGroups.Add((Get-Fingerprint $m))) { Stop-Closed "task '$($tp.Name)' has duplicate residual group fingerprint" }
        }
    }
    $taskObj = $task.Value

    if ($fixture.findings -isnot [System.Array]) { Stop-Closed 'fixture findings must be an array' }
    foreach ($f in $fixture.findings) {
        if ($null -eq $f -or $f -isnot [pscustomobject]) { Stop-Closed 'fixture findings members must be objects' }
        Assert-Fingerprint $f 'finding'
    }

    if ($Phase -eq 'Schema') {
        Write-Output "TASK-LINT-OK:$TaskId"
        exit 0
    }

    $expected = [System.Collections.Generic.Dictionary[string, int]]::new([StringComparer]::Ordinal)
    foreach ($e in $taskObj.expected_group_residual_after_task) {
        $fp = Get-Fingerprint $e
        $prev = 0
        [void]$expected.TryGetValue($fp, [ref]$prev)
        $expected[$fp] = $prev + [int]$e.count
    }
    $actual = [System.Collections.Generic.Dictionary[string, int]]::new([StringComparer]::Ordinal)
    foreach ($f in @($fixture.findings)) {
        if ($null -eq $f) { continue }
        $fp = Get-Fingerprint $f
        # Group residual counts every remaining occurrence regardless of owner.
        if (-not $expected.ContainsKey($fp)) { continue }
        $prev = 0
        [void]$actual.TryGetValue($fp, [ref]$prev)
        $actual[$fp] = $prev + 1
    }

    $sorted = [string[]]@($expected.Keys)
    [Array]::Sort($sorted, [StringComparer]::Ordinal)

    $mismatch = @()
    foreach ($fp in $sorted) {
        $e = $expected[$fp]
        $a = 0
        [void]$actual.TryGetValue($fp, [ref]$a)
        if ($a -ne $e) { $mismatch += "$fp expected=$e actual=$a" }
    }
    if ($mismatch.Count -gt 0) {
        $mismatch | ForEach-Object { Write-Output "RESIDUAL-MISMATCH: $_" }
        exit 1
    }
    Write-Output "TASK-LINT-OK:$TaskId"
    exit 0
}
catch {
    [Console]::Error.WriteLine("TASK-LINT-FAIL: INVALID-INPUT: $($_.Exception.Message)")
    exit 2
}
