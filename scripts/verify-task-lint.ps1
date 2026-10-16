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
    "known_groups": [ { "path", "linter", "message" } ],
    "findings": [ { "path", "linter", "message", "line", "column", "owner" (optional, ignored) } ] }
known_groups is REQUIRED (may be empty): the baseline groups that are known and
intentionally allowed outside the selected task's expected residual. PreCommit and
PostCommit reject (exit 1, NEW-FINGERPRINT) any finding whose group is in neither the
selected task's expected residual groups nor known_groups, so a changed diagnostic
cannot pass even when the original group's expected residual is zero. Groups listed
in the selected task's residual need not be repeated in known_groups.
Known groups are not count-compared. Without -FixtureRoot the runner fails closed (it never runs real golangci-lint
and never mutates the repository).

Exit codes: 0 match (prints TASK-LINT-OK:<TaskId>); 1 residual mismatch or new fingerprint;
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
        $seen = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
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
            $h = [System.Collections.Specialized.OrderedDictionary]::new([StringComparer]::Ordinal)
            foreach ($p in $El.EnumerateObject()) {
                if ($h.Contains($p.Name)) { throw "duplicate JSON member: $($p.Name)" }
                $h[$p.Name] = ConvertFrom-JsonElement $p.Value
            }
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

function Read-StrictJson([string]$Path) {
    $text = Get-Content -LiteralPath $Path -Raw
    if ([string]::IsNullOrWhiteSpace($text)) { Stop-Closed 'fixture is empty' }
    try { $doc = [System.Text.Json.JsonDocument]::Parse($text) }
    catch { Stop-Closed "malformed fixture: $($_.Exception.Message)" }
    try {
        if ($doc.RootElement.ValueKind -ne [System.Text.Json.JsonValueKind]::Object) { Stop-Closed 'fixture top level must be an object' }
        if (Test-DupKey $doc.RootElement) { Stop-Closed 'fixture contains duplicate object keys' }
        return (ConvertFrom-JsonElement $doc.RootElement)
    }
    finally { $doc.Dispose() }
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

    if ($null -eq $fixture.PSObject.Properties['known_groups'] -or $fixture.known_groups -isnot [System.Array]) { Stop-Closed 'fixture known_groups must be an array' }
    $known = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($k in $fixture.known_groups) {
        if ($null -eq $k -or $k -isnot [pscustomobject]) { Stop-Closed 'known_groups members must be objects' }
        Assert-Fingerprint $k 'known_groups'
        if (-not $known.Add((Get-Fingerprint $k))) { Stop-Closed 'known_groups has duplicate group fingerprint' }
    }

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
    $newFp = [System.Collections.Generic.SortedSet[string]]::new([StringComparer]::Ordinal)
    foreach ($f in @($fixture.findings)) {
        if ($null -eq $f) { continue }
        $fp = Get-Fingerprint $f
        # Group residual counts every remaining occurrence regardless of owner.
        if (-not $expected.ContainsKey($fp)) {
            if (-not $known.Contains($fp)) { [void]$newFp.Add($fp) }
            continue
        }
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
    if ($mismatch.Count -gt 0 -or $newFp.Count -gt 0) {
        $mismatch | ForEach-Object { Write-Output "RESIDUAL-MISMATCH: $_" }
        foreach ($fp in $newFp) { Write-Output "NEW-FINGERPRINT: $fp" }
        exit 1
    }
    Write-Output "TASK-LINT-OK:$TaskId"
    exit 0
}
catch {
    [Console]::Error.WriteLine("TASK-LINT-FAIL: INVALID-INPUT: $($_.Exception.Message)")
    exit 2
}
