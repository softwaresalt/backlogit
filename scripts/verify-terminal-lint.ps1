#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidatePattern('^\d+-F$')][string]$FeatureId
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'lint-runtime.ps1')

try {
    $root = Get-RepositoryRoot
    $feature = Resolve-CanonicalArtifact -Root $root -Id $FeatureId
    $contract = (Get-JsonContractBlock -Path $feature.Path `
        -Name 'baseline-lint-convergence-contract').Object
    if ($contract.PSObject.Properties.Name -cnotcontains 'supported_goos' -or
        -not (Test-OrdinalSetEqual -Expected @('windows', 'linux') `
            -Actual @($contract.supported_goos))) {
        throw 'supported-goos-contract'
    }
    [void](Assert-GolangCILintVersion -Root $root)
    foreach ($goos in @($contract.supported_goos)) {
        $lint = Invoke-StructuredGolangCILint -Root $root -Goos "$goos"
        if ($lint.Issues.Count -ne 0) {
            throw "nonzero-findings:${goos}:$($lint.Issues.Count)"
        }
    }
    Write-Output "TERMINAL-LINT-OK:${FeatureId}:windows,linux"
    exit 0
}
catch {
    [Console]::Error.WriteLine("TERMINAL-LINT-FAIL:${FeatureId}:$($_.Exception.Message)")
    exit 81
}
