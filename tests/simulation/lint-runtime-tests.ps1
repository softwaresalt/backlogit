#!/usr/bin/env pwsh
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = (& git rev-parse --show-toplevel).Trim()
. (Join-Path $root 'scripts/lint-runtime.ps1')

$passed = 0
$failed = 0
function Assert-True {
    param([string]$Name, [bool]$Condition)
    if ($Condition) {
        $script:passed++
        Write-Output "PASS: $Name"
    }
    else {
        $script:failed++
        Write-Output "FAIL: $Name"
    }
}
function Assert-ThrowsLike {
    param([string]$Name, [scriptblock]$Action, [string]$Pattern)
    try {
        & $Action
        Assert-True -Name $Name -Condition $false
    }
    catch {
        Assert-True -Name $Name -Condition ([bool]($_.Exception.Message -match $Pattern))
    }
}
function Write-Artifact {
    param([string]$Path, [string]$Id, [string]$Status, [string]$Body)
    $content = @(
        '---'
        'artifact_type: task'
        "id: $Id"
        "status: $Status"
        '---'
        ''
        $Body
    ) -join "`n"
    [System.IO.File]::WriteAllText($Path, $content, [System.Text.UTF8Encoding]::new($false))
}

$temp = Join-Path $root (
    ".autoharness/staging/runtime-test-$([guid]::NewGuid().ToString('N'))")
try {
    [void](New-Item -ItemType Directory -Path (Join-Path $temp '.backlogit/queue') -Force)
    [void](New-Item -ItemType Directory -Path (Join-Path $temp '.backlogit/archive') -Force)
    $body = @'
<!-- BEGIN:task-lint-contract -->
```json
{"task_lint_cmd":"canonical","lint_scope":{"kind":"no-go-lint-surface"},"non_vacuity_evidence":{"success_marker":"ok"}}
```
<!-- END:task-lint-contract -->
'@
    $queue = Join-Path $temp '.backlogit/queue/900.001-T.md'
    $archive = Join-Path $temp '.backlogit/archive/900.001-T.md'
    Write-Artifact -Path $queue -Id '900.001-T' -Status 'queued' -Body $body
    $queued = Resolve-CanonicalArtifact -Root $temp -Id '900.001-T'
    $digestBefore = Get-TaskContractDigest -TaskPath $queued.Path
    Assert-True 'queued artifact resolves only from queue' (
        $queued.Location -ceq 'queue' -and $queued.Status -ceq 'queued')

    Remove-Item -LiteralPath $queue
    Write-Artifact -Path $archive -Id '900.001-T' -Status 'done' -Body $body
    $done = Resolve-CanonicalArtifact -Root $temp -Id '900.001-T'
    $digestAfter = Get-TaskContractDigest -TaskPath $done.Path
    Assert-True 'done artifact resolves only from archive' (
        $done.Location -ceq 'archive' -and $done.Status -ceq 'done')
    Assert-True 'contract digest is stable across lifecycle mutation and relocation' (
        $digestBefore -ceq $digestAfter)

    Write-Artifact -Path $queue -Id '900.001-T' -Status 'queued' -Body $body
    Assert-ThrowsLike 'duplicate queue/archive artifact fails' {
        Resolve-CanonicalArtifact -Root $temp -Id '900.001-T'
    } 'duplicate-artifact'
    Remove-Item -LiteralPath $queue
    Remove-Item -LiteralPath $archive

    Write-Artifact -Path $queue -Id 'wrong-id' -Status 'queued' -Body $body
    Assert-ThrowsLike 'frontmatter ID mismatch fails' {
        Resolve-CanonicalArtifact -Root $temp -Id '900.001-T'
    } 'artifact-id-mismatch'
    Remove-Item -LiteralPath $queue

    Write-Artifact -Path $queue -Id '900.001-T' -Status 'done' -Body $body
    Assert-ThrowsLike 'status/location mismatch fails' {
        Resolve-CanonicalArtifact -Root $temp -Id '900.001-T'
    } 'status-location-mismatch'
    Remove-Item -LiteralPath $queue

    Assert-True 'exact linter version parses' (
        (Get-GolangCILintVersionFromText 'golangci-lint has version 2.13.2 built with go1.26') -ceq
        '2.13.2')
    Assert-ThrowsLike 'unparseable linter version fails' {
        Get-GolangCILintVersionFromText 'not a version'
    } 'linter-version-unparseable'
    Assert-ThrowsLike 'wrong linter version fails closed' {
        Assert-GolangCILintVersionText 'golangci-lint has version 1.64.8'
    } 'linter-version-mismatch:1\.64\.8'

    $quoted = Join-Path $temp "path with spaces and ' quote.txt"
    [System.IO.File]::WriteAllText($quoted, 'payload')
    $probe = Join-Path $temp "probe with spaces and ' quote.ps1"
    [System.IO.File]::WriteAllText(
        $probe,
        'param([string]$Path); [Console]::Error.Write("diagnostic"); ' +
        '[Console]::Out.Write((ConvertTo-Json @{Issues=@();Path=$Path} -Compress))')
    $captured = Invoke-CapturedProcess -FileName 'pwsh' `
        -Arguments @('-NoProfile', '-File', $probe, '-Path', $quoted) -WorkingDirectory $temp
    $json = $captured.Stdout | ConvertFrom-Json
    Assert-True 'ArgumentList survives spaces and quotes' ($json.Path -ceq $quoted)
    Assert-True 'JSON stdout is isolated from diagnostics' (
        $captured.Stdout -notmatch 'diagnostic' -and $captured.Stderr -ceq 'diagnostic')
    Assert-True 'JSON schema accepts an Issues array' (
        (ConvertFrom-LintJson '{"Issues":[]}').Issues.Count -eq 0)
    foreach ($invalid in @(
            '', 'null', '{}', '{"Issues":null}', '{"Issues":"none"}', '{"Issues":{}}')) {
        Assert-ThrowsLike "JSON schema rejects [$invalid]" {
            ConvertFrom-LintJson $invalid
        } 'lint-json-(?:empty|schema)'
    }
    $ownedFiles = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    [void]$ownedFiles.Add('internal/split.go')
    $ownedKeys = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    [void]$ownedKeys.Add('internal/split.go|10|2|errcheck|first slice')
    $issues = @(
        [pscustomobject]@{
            Pos = [pscustomobject]@{ Filename = 'internal/split.go'; Line = 10; Column = 2 }
            FromLinter = 'errcheck'
            Text = 'first slice'
        },
        [pscustomobject]@{
            Pos = [pscustomobject]@{ Filename = 'internal/split.go'; Line = 20; Column = 2 }
            FromLinter = 'errcheck'
            Text = 'later slice'
        }
    )
    $ownedObserved = @(Get-OwnedObservedLintKeys -Issues $issues `
            -OwnedFiles $ownedFiles -OwnedKeys $ownedKeys)
    Assert-True 'same-file later slice does not block current owner' (
        $ownedObserved.Count -eq 1 -and
        $ownedObserved[0] -ceq 'internal/split.go|10|2|errcheck|first slice')
    $allowedOtherKeys = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    [void]$allowedOtherKeys.Add('internal/split.go|20|2|errcheck|later slice')
    $unexpected = @(Get-UnauthorizedLintKeys -Issues @($issues[1]) `
            -CurrentOwnerKeys $ownedKeys `
            -AllowedOtherOwnerKeys $allowedOtherKeys)
    Assert-True 'authorized unfinished later slice is permitted' ($unexpected.Count -eq 0)
    Assert-True 'missing unfinished later-slice finding fails exact residual comparison' (-not (
            Test-OrdinalSetEqual -Expected @(
                'internal/split.go|20|2|errcheck|later slice') -Actual @()))
    foreach ($mutation in @(
            [pscustomobject]@{ line = 21; linter = 'errcheck'; message = 'later slice' },
            [pscustomobject]@{ line = 20; linter = 'govet'; message = 'later slice' },
            [pscustomobject]@{ line = 20; linter = 'errcheck'; message = 'changed message' },
            [pscustomobject]@{ line = 30; linter = 'errcheck'; message = 'new warning' }
        )) {
        $mutated = @([pscustomobject]@{
                Pos = [pscustomobject]@{
                    Filename = 'internal/split.go'
                    Line = $mutation.line
                    Column = 2
                }
                FromLinter = $mutation.linter
                Text = $mutation.message
            })
        Assert-True "moved, changed, or new finding is rejected: $($mutation.message)" (
            @(Get-UnauthorizedLintKeys -Issues $mutated `
                    -CurrentOwnerKeys $ownedKeys `
                    -AllowedOtherOwnerKeys $allowedOtherKeys).Count -eq 1)
    }
    $outsideOwnedFile = @([pscustomobject]@{
            Pos = [pscustomobject]@{ Filename = 'internal/other.go'; Line = 9; Column = 1 }
            FromLinter = 'govet'
            Text = 'new warning outside owned file'
        })
    Assert-True 'new package finding outside owned files is rejected' (
        @(Get-UnauthorizedLintKeys -Issues $outsideOwnedFile `
                -CurrentOwnerKeys $ownedKeys `
                -AllowedOtherOwnerKeys $allowedOtherKeys).Count -eq 1)
    Assert-True 'same basename binds to its exact directory package' (
        (Resolve-OwnedFilePackage -File 'internal/a/shared.go' `
                -Packages @('./internal/a', './internal/b')) -ceq './internal/a')
    Assert-ThrowsLike 'same basename in another package cannot satisfy binding' {
        Resolve-OwnedFilePackage -File 'internal/a/shared.go' -Packages @('./internal/b')
    } 'file-package:internal/a/shared.go'
    Assert-True 'unowned extra package is not an exact package set' (-not (
            Test-OrdinalSetEqual -Expected @('./internal/a') `
                -Actual @('./internal/a', './internal/b')))

    Assert-True 'go-list membership recognizes selected platform file' (
        Test-GoListFileMembership -Output 'common.go|x_windows_amd64.go|' `
            -File 'internal/x_windows_amd64.go')
    Assert-True 'go-list membership rejects build-excluded file' (-not (
            Test-GoListFileMembership -Output 'common.go|' `
                -File 'internal/x_linux_amd64.go'))
    $platformInventory = [pscustomobject]@{
        primary_goos = 'windows'
        supported_goos = @('windows', 'linux')
        surface_exclusions = [pscustomobject]@{
            windows = @()
            linux = @('windows-only-key')
        }
        additional_findings = @([pscustomobject]@{ goos = 'linux' })
    }
    Assert-PlatformInventoryShape -Inventory $platformInventory
    Assert-True 'canonical platform inventory shape validates' $true
    $windowsAdditional = $platformInventory.PSObject.Copy()
    $windowsAdditional.additional_findings = @([pscustomobject]@{ goos = 'windows' })
    Assert-ThrowsLike 'Windows additional finding is rejected' {
        Assert-PlatformInventoryShape -Inventory $windowsAdditional
    } 'additional-finding-goos'
    $primaryExclusion = $platformInventory.PSObject.Copy()
    $primaryExclusion.surface_exclusions = [pscustomobject]@{
        windows = @('invalid-primary-exclusion')
        linux = @()
    }
    Assert-ThrowsLike 'primary Windows exclusion is rejected' {
        Assert-PlatformInventoryShape -Inventory $primaryExclusion
    } 'platform-exclusions'
    $requiredSurfaces = @{
        'internal/common.go' = @('windows', 'linux')
        'internal/linux.go' = @('linux')
    }
    Assert-RequiredGoosParticipation -RequiredByFile $requiredSurfaces `
        -ObservedByFileGoos @{
            'internal/common.go|windows' = $true
            'internal/common.go|linux' = $true
            'internal/linux.go|linux' = $true
        }
    Assert-True 'owned findings participate on every required inventory surface' $true
    Assert-ThrowsLike 'missing required Linux participation fails closed' {
        Assert-RequiredGoosParticipation -RequiredByFile $requiredSurfaces `
            -ObservedByFileGoos @{
                'internal/common.go|windows' = $true
                'internal/linux.go|linux' = $true
            }
    } 'not-in-required-package:linux:internal/common.go'

    Assert-True 'claim union exact match accepts exact paths' (
        Test-OrdinalSetEqual -Expected @('evidence.md', 'task.md', 'hooks.jsonl') `
            -Actual @('hooks.jsonl', 'evidence.md', 'task.md'))
    Assert-True 'claim union rejects extras' (-not (
            Test-OrdinalSetEqual -Expected @('evidence.md', 'task.md') `
                -Actual @('evidence.md', 'task.md', 'extra.md')))
    $ordinalPaths = @(Get-OrdinalDistinct -Values @(
            'task.md', 'task.md', 'Task.md', "ta$([char]0x00AD)sk.md"))
    Assert-True 'ordinal path collection preserves case and Unicode variants' (
        $ordinalPaths.Count -eq 3)
    Assert-CanonicalTaskPhase -Phase 'PostCommit'
    Assert-ThrowsLike 'case-varied task phase fails closed' {
        Assert-CanonicalTaskPhase -Phase 'postcommit'
    } 'task-phase:postcommit'
    $contractEnvelope = [pscustomobject][ordered]@{
        task_lint_cmd = 'canonical'
        lint_scope = [pscustomobject]@{ kind = 'no-go-lint-surface' }
        non_vacuity_evidence = [pscustomobject]@{
            success_marker = 'TASK-LINT-OK:900.001-T:terminal-convergence'
        }
    }
    Assert-TaskLintContractEnvelope -Contract $contractEnvelope `
        -TaskId '900.001-T' -Role 'terminal-convergence'
    Assert-True 'task contract envelope validates exact marker' $true
    $badEnvelope = $contractEnvelope.PSObject.Copy()
    $badEnvelope.non_vacuity_evidence = [pscustomobject]@{ success_marker = 'wrong' }
    Assert-ThrowsLike 'task contract marker mismatch fails closed' {
        Assert-TaskLintContractEnvelope -Contract $badEnvelope `
            -TaskId '900.001-T' -Role 'terminal-convergence'
    } 'non-vacuity-evidence:900.001-T'
    $extraEnvelope = [pscustomobject][ordered]@{
        task_lint_cmd = 'canonical'
        lint_scope = [pscustomobject]@{}
        non_vacuity_evidence = [pscustomobject]@{ success_marker = 'x' }
        extra = $true
    }
    Assert-ThrowsLike 'extra task contract envelope key fails closed' {
        Assert-TaskLintContractEnvelope -Contract $extraEnvelope `
            -TaskId '900.001-T' -Role 'terminal-convergence'
    } 'task-contract-schema:900.001-T'
    Assert-True 'post-commit claim union separates commit and governed working paths' (
        (Test-OrdinalSetEqual -Expected @('evidence.md') -Actual @('evidence.md')) -and
        (Test-OrdinalSetEqual -Expected @('task.md', 'hooks.jsonl') `
            -Actual @('hooks.jsonl', 'task.md')) -and
        (Test-OrdinalSetEqual -Expected @('evidence.md', 'task.md', 'hooks.jsonl') `
            -Actual @('evidence.md', 'hooks.jsonl', 'task.md')))
    Assert-True 'ordinal set equality rejects soft-hyphen identity drift' (-not (
            Test-OrdinalSetEqual -Expected @('message') -Actual @("mes$([char]0x00AD)sage")))
    $inventoryRows = [System.Collections.Generic.Dictionary[string,object]]::new(
        [System.StringComparer]::Ordinal)
    $inventoryRows.Add('windows-row', [pscustomobject]@{ goos = 'windows' })
    $inventoryRows.Add('linux-row', [pscustomobject]@{ goos = 'linux' })
    Assert-InventoryFindingCount -Declared 2 -RowsByKey $inventoryRows
    Assert-True 'nonempty platform inventory count validates after population' $true
    Assert-ThrowsLike 'platform inventory count mismatch fails' {
        Assert-InventoryFindingCount -Declared 1 -RowsByKey $inventoryRows
    } 'inventory-finding-count:1:2'
}
finally {
    if (Test-Path -LiteralPath $temp) {
        Remove-Item -LiteralPath $temp -Recurse -Force
    }
}

if ($failed -ne 0) {
    throw "LINT_RUNTIME_TESTS_FAILED:$failed"
}
Write-Output "LINT_RUNTIME_TESTS_OK:$passed"
