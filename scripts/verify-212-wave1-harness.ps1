param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('211.009-T', '211.010-T', '211.011-T')]
    [string]$TaskID,
    [ValidateSet('Red', 'Green')]
    [string]$Phase = 'Green'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Split-Path $PSScriptRoot -Parent)
. ./bin/tmp/212-go-env.ps1

$core = 'github.com/softwaresalt/backlogit/internal/core'
$policy = 'github.com/softwaresalt/backlogit/internal/policyengine'
$integration = 'github.com/softwaresalt/backlogit/tests/integration'
$contracts = @{
    '211.009-T' = @{
        Packages = @('./internal/core')
        Tests = @(
            @($core, 'TestPlanArtifactMove_StrategyDetection', 'internal/core/archive_git_test.go'),
            @($core, 'TestInGitWorktreeBounded', 'internal/core/shipment_gate_ancestry_test.go'),
            @($core, 'TestIsAncestor', 'internal/core/shipment_gate_ancestry_test.go')
        )
        Witnesses = @('TestInGitWorktreeBounded')
        Marker = 'An error is expected but got nil'
        RequiredRuns = @(
            'TestInGitWorktreeBounded/no_repo_legacy_skip_with_isolated_git',
            'TestInGitWorktreeBounded/nested_and_intermediate_empty_git_dir_inside_outer_repo_fails_closed/direct',
            'TestInGitWorktreeBounded/nested_and_intermediate_empty_git_dir_inside_outer_repo_fails_closed/intermediate'
        )
    }
    '211.010-T' = @{
        Packages = @('./internal/core', './internal/policyengine')
        Tests = @(
            @($core, 'TestHeadSHABounded', 'internal/core/shipment_gate_headdrift_test.go'),
            @($core, 'TestRepairShipmentMemberEvidence_NoRepoHead_Errors', 'internal/core/shipment_repair_test.go'),
            @($policy, 'TestU163_002_ExecGitRequiresARepositoryDirectory', 'internal/policyengine/u163_002_record_test.go')
        )
        Witnesses = @('TestHeadSHABounded', 'TestRepairShipmentMemberEvidence_NoRepoHead_Errors', 'TestU163_002_ExecGitRequiresARepositoryDirectory')
        Marker = @{
            'TestHeadSHABounded' = 'Should be empty, but was'
            'TestRepairShipmentMemberEvidence_NoRepoHead_Errors' = 'does not contain "HEAD"'
            'TestU163_002_ExecGitRequiresARepositoryDirectory' = 'An error is expected but got nil'
        }
        RequiredRuns = @()
    }
    '211.011-T' = @{
        Packages = @('./internal/core', './tests/integration')
        Tests = @(
            @($core, 'TestDoctor_IgnoresIntentionalOrphans', 'internal/core/025_doctor_harness_test.go'),
            @($integration, 'TestShipmentWorkflow_ShipmentReleaseCleanup', 'tests/integration/shipment_workflow_test.go')
        )
        Witnesses = @('TestDoctor_IgnoresIntentionalOrphans', 'TestShipmentWorkflow_ShipmentReleaseCleanup')
        Marker = 'shipment refused: member 001.001-T is active (not completed through the gate)'
        RequiredRuns = @()
    }
}
$contract = $contracts[$TaskID]

# Compare assertion nodes, not file hashes: fixtures remain authorized mutable.
# Additional assertions are accepted; every original assertion must remain.
function Get-AssertionNodes([string]$Source, [string]$Name) {
    $match = [regex]::Match($Source, "(?ms)^func $([regex]::Escape($Name))\(.*?(?=^func |\z)")
    if (-not $match.Success) { throw "Missing canonical function $Name" }
    $node = [System.Collections.Generic.List[string]]::new()
    $depth = 0
    foreach ($line in ($match.Value -split '\r?\n')) {
        if ($node.Count -eq 0 -and $line -notmatch '^\s*(assert|require)\.\w+\(') { continue }
        $node.Add($line.Trim())
        $syntax = [regex]::Replace($line, '"(?:\\.|[^"\\])*"|`[^`]*`|//.*$', '')
        $depth += ([regex]::Matches($syntax, '\(')).Count - ([regex]::Matches($syntax, '\)')).Count
        if ($depth -eq 0) {
            $node -join "`n"
            $node.Clear()
        }
    }
    if ($node.Count -ne 0) { throw "Incomplete assertion node in $Name" }
}

foreach ($test in $contract.Tests) {
    $baseline = (git --no-pager show "bf6bf2f6:$($test[2])") -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "Cannot read Stage baseline for $($test[2])" }
    $current = Get-Content -Raw -LiteralPath $test[2]
    $oldNodes = @(Get-AssertionNodes $baseline $test[1])
    $newNodes = @(Get-AssertionNodes $current $test[1])
    if ($oldNodes.Count -eq 0) { throw "Vacuous assertion guard for $($test[1])" }
    foreach ($node in $oldNodes) {
        $oldCount = @($oldNodes | Where-Object { $_ -ceq $node }).Count
        $newCount = @($newNodes | Where-Object { $_ -ceq $node }).Count
        if ($newCount -lt $oldCount) { throw "Original assertion changed or removed in $($test[1]): $node" }
    }
    Write-Output "ASSERTION-NODES PASS $TaskID $($test[1]) original=$($oldNodes.Count)"
}

$names = @($contract.Tests | ForEach-Object { $_[1] })
$selector = '^(' + ($names -join '|') + ')$'
$goArgs = @('test', '-timeout=30m', '-count=1', '-race', '-json', '-run', $selector) + $contract.Packages
Write-Output "TASK $TaskID; closed siblings: 211.001-T,211.008-T; phase=$Phase"
Write-Output ("go " + ($goArgs -join ' '))
$lines = @(go @goArgs 2>&1)
$nativeExit = $LASTEXITCODE
$events = [System.Collections.Generic.List[object]]::new()
foreach ($line in $lines) {
    $text = "$line"
    Write-Output $text
    if (-not $text.StartsWith('{')) { throw "Non-JSON build/runtime output: $text" }
    $event = $text | ConvertFrom-Json
    $events.Add($event)
}
Write-Output "NATIVE-EXIT $TaskID $nativeExit"
$output = ($lines -join "`n")
if ($output -match 'panic:|build failed|no tests to run|DATA RACE') {
    throw "Not an assertion phase for $TaskID"
}
if (@($events | Where-Object { $_.Action -eq 'skip' }).Count -ne 0) { throw "Skipped tests in $TaskID" }
$packageEnds = @($events | Where-Object {
    -not $_.PSObject.Properties['Test'] -and $_.Action -in @('pass', 'fail')
})
if ($packageEnds.Count -ne $contract.Packages.Count) { throw "Incorrect package count for $TaskID" }
$expectedPackages = @($contract.Tests | ForEach-Object { $_[0] } | Select-Object -Unique)
foreach ($package in $expectedPackages) {
    if (@($packageEnds | Where-Object { $_.Package -ceq $package }).Count -ne 1) {
        throw "Missing/duplicate terminal package $package"
    }
}
foreach ($event in $events) {
    if ($event.PSObject.Properties['Test'] -and ($event.Test.Split('/')[0] -cnotin $names)) {
        throw "Undeclared test selected: $($event.Test)"
    }
}
foreach ($test in $contract.Tests) {
    $runs = @($events | Where-Object {
        $_.PSObject.Properties['Test'] -and $_.Test -ceq $test[1] -and $_.Package -ceq $test[0] -and $_.Action -eq 'run'
    })
    $ends = @($events | Where-Object {
        $_.PSObject.Properties['Test'] -and $_.Test -ceq $test[1] -and $_.Package -ceq $test[0] -and $_.Action -in @('pass', 'fail')
    })
    if ($runs.Count -ne 1 -or $ends.Count -ne 1) { throw "Missing/duplicate canonical test $($test[1])" }
    if ($Phase -eq 'Green' -and $ends[0].Action -ne 'pass') { throw "Not GREEN: $($test[1]); native=$nativeExit" }
    if ($Phase -eq 'Red' -and $test[1] -cin $contract.Witnesses) {
        $testOutput = (@($events | Where-Object {
            $_.PSObject.Properties['Test'] -and ($_.Test -ceq $test[1] -or $_.Test.StartsWith($test[1] + '/')) -and $_.Action -eq 'output'
        } | ForEach-Object { $_.Output })) -join ''
        $marker = $contract.Marker
        if ($marker -is [hashtable]) { $marker = $marker[$test[1]] }
        if ($ends[0].Action -ne 'fail' -or -not $testOutput.Contains($marker) -or -not $testOutput.Contains('Error Trace:')) {
            throw "Missing native assertion RED witness $($test[1])"
        }
    }
}
foreach ($name in $contract.RequiredRuns) {
    if (@($events | Where-Object {
        $_.PSObject.Properties['Test'] -and $_.Test -ceq $name -and $_.Action -eq 'run'
    }).Count -ne 1) { throw "Missing required subtest $name" }
    if ($Phase -eq 'Red' -and $name.EndsWith('isolated_git')) {
        if (@($events | Where-Object {
            $_.PSObject.Properties['Test'] -and $_.Test -ceq $name -and $_.Action -eq 'pass'
        }).Count -ne 1) { throw "Isolated fake git fixture did not PASS" }
    } elseif ($Phase -eq 'Red') {
        $subOutput = (@($events | Where-Object {
            $_.PSObject.Properties['Test'] -and $_.Test -ceq $name -and $_.Action -eq 'output'
        } | ForEach-Object { $_.Output })) -join ''
        foreach ($marker in @(
            'present-but-unresolved .git below outer worktree must fail closed',
            'artifact move must reject present-but-unresolved .git below outer worktree'
        )) {
            if (-not $subOutput.Contains($marker)) { throw "Missing both-API assertion RED: $name $marker" }
        }
    }
}
if ($Phase -eq 'Green') {
    if ($nativeExit -ne 0 -or @($events | Where-Object { $_.Action -eq 'fail' }).Count -ne 0) { throw "Not GREEN: native=$nativeExit" }
} elseif ($nativeExit -ne 1) {
    throw "RED must have native assertion exit 1, got $nativeExit"
}
Write-Output "VERIFIED $TaskID $Phase; packages=$($packageEnds.Count); named=$($names.Count); skips=0"
exit 0
