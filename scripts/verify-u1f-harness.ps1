param(
    [ValidateSet('Red', 'Green')]
    [string]$Phase = 'Green'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
try {
    . ./bin/tmp/212-go-env.ps1
    $env:TEMP = Join-Path $root 'bin/_h8a2f10c4/t'
    $env:TMP = $env:TEMP
    New-Item -ItemType Directory -Force $env:TEMP | Out-Null
    $version = & go version
    if ($LASTEXITCODE -ne 0 -or $version -notmatch '^go version go1\.24\.0 ') {
        throw "Pinned Go 1.24.0 required: $version"
    }
    $top = 'TestU1f_ExecuteReconstitutionPreliminaryPlanReadsUnderCanonicalLocks'
    $required = @(
        $top
        "$top/preliminary_plan_reads_wait_for_stash_global_and_membership_locks"
        "$top/concurrent_associate_commit_durable_replace_serializes_before_preliminary_plan_reads"
        "$top/pure_input_and_plan_validation_fails_fast_before_locks"
    )
    $lines = @(& go test -count=1 -timeout=30m -race -json -run '^TestU1f_' ./internal/core 2>&1)
    $nativeExit = $LASTEXITCODE
    $events = @()
    foreach ($line in $lines) {
        $text = "$line"
        Write-Output $text
        if ($text.StartsWith('{')) {
            $events += $text | ConvertFrom-Json
        }
    }
    Write-Output "U1f native exit: $nativeExit; phase: $Phase"
    $output = ($events | ForEach-Object { $_.Output }) -join ''
    if (($lines -join "`n") -match 'panic:|DATA RACE|build failed|no tests to run|test timed out') {
        throw 'Infrastructure failure is not harness evidence'
    }
    if (@($events | Where-Object Action -eq 'skip').Count -ne 0) {
        throw 'Skips are forbidden'
    }
    $runs = @($events | Where-Object Action -eq 'run')
    if ($runs.Count -ne $required.Count) { throw 'Unexpected or empty test selector' }
    foreach ($name in $required) {
        if (@($runs | Where-Object Test -eq $name).Count -ne 1) {
            throw "Required test must run exactly once: $name"
        }
        $terminal = @($events | Where-Object {
            $_.Test -eq $name -and $_.Action -in @('pass', 'fail')
        })
        if ($terminal.Count -ne 1) { throw "Missing unique terminal event: $name" }
        if ($Phase -eq 'Green' -and $terminal[0].Action -ne 'pass') {
            throw "GREEN requires all tests passing: $name (native exit $nativeExit)"
        }
    }
    if ($output -notmatch 'U1f subtest1 cleanup completed' -or
        $output -notmatch 'U1f subtest2 cleanup completed') {
        throw 'Cleanup completion evidence missing'
    }
    if ($Phase -eq 'Green') {
        if ($nativeExit -ne 0) { throw "GREEN requires native exit 0, got $nativeExit" }
        if (@($events | Where-Object Action -eq 'fail').Count -ne 0) {
            throw 'GREEN contains failures'
        }
    } else {
        if ($nativeExit -ne 1) { throw "RED requires native exit 1, got $nativeExit" }
        $failed = @($events | Where-Object Action -eq 'fail')
        if (@($failed | Where-Object Test -eq $required[1]).Count -ne 1 -or
            @($failed | Where-Object Test -eq $top).Count -ne 1 -or
            @($failed | Where-Object { $_.Test -and $_.Test -notin $required }).Count -ne 0 -or
            @($events | Where-Object { $_.Action -eq 'pass' -and $_.Test -eq $required[2] }).Count -ne 1 -or
            $output -notmatch 'U1f preliminary-read ordering assertion: returned before canonical locks:') {
            throw 'RED requires the genuine Subtest 1 ordering assertion and passing durable replacement guard'
        }
    }
    Write-Output "U1f $Phase observer PASS (native exit $nativeExit retained above)"
} finally {
    Pop-Location
}
