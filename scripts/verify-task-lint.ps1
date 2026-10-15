<#
.SYNOPSIS
Per-task lint verification runner for the baseline-convergence feature.

.DESCRIPTION
Compares the actual residual multiset owned by -TaskId against the task's
expected_group_residual_after_task under the line-stable-group-multiset/v1
identity model: findings are keyed on (path, linter, normalized_message) with
multiplicity; line and column are never compared.

Fixture injection: -FixtureRoot points at a directory containing task-lint.json:
  { "tasks": { "<id>": { "expected_group_residual_after_task":
        [ { "path", "linter", "message", "count" } ] } },
    "findings": [ { "path", "linter", "message", "line", "column", "owner" } ] }
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
    [ValidateSet('Schema', 'PreCommit', 'PostCommit')]
    [string]$Phase = 'PostCommit',
    [string]$FixtureRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Stop-Closed([string]$Message, [int]$Code = 2) {
    [Console]::Error.WriteLine("TASK-LINT-FAIL: $Message")
    exit $Code
}

function Get-Fingerprint($Item) {
    $msg = ([string]$Item.message -replace '\s+', ' ').Trim()
    return ('{0}|{1}|{2}' -f $Item.path, $Item.linter, $msg)
}

try {
    if ([string]::IsNullOrWhiteSpace($TaskId)) { Stop-Closed 'TaskId is required' }
    if ($TaskId -cnotmatch '^[0-9]+(\.[0-9]+)*-[A-Z]{1,2}$') { Stop-Closed "invalid TaskId '$TaskId'" }
    if ([string]::IsNullOrWhiteSpace($FeatureId)) { Stop-Closed 'FeatureId is required' }
    if ($FeatureId -cnotmatch '^[0-9]+(\.[0-9]+)*-[A-Z]{1,2}$') { Stop-Closed "invalid FeatureId '$FeatureId'" }
    if ([string]::IsNullOrWhiteSpace($FixtureRoot)) { Stop-Closed 'FixtureRoot is required (real lint is never run)' }

    $fixturePath = Join-Path $FixtureRoot 'task-lint.json'
    if (-not (Test-Path -LiteralPath $fixturePath -PathType Leaf)) { Stop-Closed "fixture not found: $fixturePath" }
    $fixture = Get-Content -LiteralPath $fixturePath -Raw | ConvertFrom-Json
    if ($null -eq $fixture.tasks -or $null -eq $fixture.findings) { Stop-Closed 'fixture missing tasks/findings' }

    $task = $fixture.tasks.PSObject.Properties[$TaskId]
    if ($null -eq $task) { Stop-Closed "unknown task id '$TaskId'" }

    if ($Phase -eq 'Schema') {
        Write-Output "TASK-LINT-OK:$TaskId"
        exit 0
    }

    $expected = @{}
    foreach ($e in @($task.Value.expected_group_residual_after_task)) {
        if ($null -eq $e) { continue }
        $fp = Get-Fingerprint $e
        $expected[$fp] = [int]$expected[$fp] + [int]$e.count
    }
    $actual = @{}
    foreach ($f in @($fixture.findings)) {
        if ([string]$f.owner -ne $TaskId) { continue }
        $fp = Get-Fingerprint $f
        $actual[$fp] = [int]$actual[$fp] + 1
    }

    $mismatch = @()
    foreach ($fp in (@($expected.Keys) + @($actual.Keys) | Sort-Object -Unique)) {
        $e = [int]$expected[$fp]
        $a = [int]$actual[$fp]
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
    [Console]::Error.WriteLine("TASK-LINT-FAIL: $($_.Exception.Message)")
    exit 2
}
