param(
    [ValidateSet('Red', 'Green')][string]$Phase = 'Green',
    [string]$ProofDirectory
)

$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$baseline = '8058bd5e1958adc502669dd509811b986fabd4010e3768f429bd2e84b00dc250'
$runner = Join-Path $root 'bin\tmp\212-full-suite.py'
$harness = Join-Path $root 'bin\tmp\test_212_full_suite.py'
$names = @(
    'test_supervise_holds_and_clears_thread_execution_state_and_records_poll_gaps_without_subtracting_budget',
    'test_validate_authorization_accepts_matching_dynamic_ordinal_and_failed_count_and_rejects_mismatch',
    'test_preflight_eol_gate_blocks_test_child_on_crlf_and_preserves_exclusive_attempt_evidence'
)
if (!$ProofDirectory) {
    $ProofDirectory = Join-Path $root ('bin\tmp\u1h-' + [Guid]::NewGuid().ToString('N'))
}
$directory = [IO.Path]::GetFullPath($ProofDirectory)
if (!$directory.StartsWith($root + '\', [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Proof directory must stay inside the workspace'
}
if (Test-Path $directory) { throw 'Proof directory must be new; no overwrite' }
[IO.Directory]::CreateDirectory($directory) | Out-Null
$before = (Get-FileHash $runner -Algorithm SHA256).Hash.ToLowerInvariant()
if ($Phase -eq 'Red' -and $before -ne $baseline) { throw 'Red runner differs from immutable baseline' }
$report = Join-Path $directory 'unit-result.json'
$env:PYTHONDONTWRITEBYTECODE = '1'
& python -B $harness --report $report
$native = $LASTEXITCODE
if (!(Test-Path $report)) { throw 'Missing unit evidence; fail closed' }
$result = Get-Content $report -Raw | ConvertFrom-Json
$after = (Get-FileHash $runner -Algorithm SHA256).Hash.ToLowerInvariant()
$valid = $before -eq $after -and $result.tests_run -eq 3 -and $result.skips -eq 0 -and $result.errors -eq 0
foreach ($name in $names) {
    $events = @($result.events | Where-Object { $_.name -ceq $name })
    $valid = $valid -and $events.Count -eq 1
    if ($events.Count -eq 1) {
        if ($Phase -eq 'Red') {
            $valid = $valid -and $events[0].outcome -ceq 'FAIL' -and $events[0].type -ceq 'AssertionError'
        } else {
            $valid = $valid -and $events[0].outcome -ceq 'PASS'
        }
    }
}
$valid = $valid -and @($result.events).Count -eq 3
if ($Phase -eq 'Red') {
    $valid = $valid -and $native -eq 1 -and $result.failures -eq 3
} else {
    $valid = $valid -and $native -eq 0 -and $result.failures -eq 0
}
$proof = [ordered]@{
    timestamp_utc = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
    task = '211.014-T'
    phase = $Phase
    native_exit = $native
    observer_exit = $(if ($valid) { 0 } else { 2 })
    guard = $(if ($valid) { 'PASS' } else { 'FAIL' })
    baseline_sha256 = $baseline
    runner_sha256_before = $before
    runner_sha256_after = $after
    harness_sha256 = (Get-FileHash $harness -Algorithm SHA256).Hash.ToLowerInvariant()
    unit_result = $result
}
$utf8 = [Text.UTF8Encoding]::new($false)
[IO.File]::WriteAllText((Join-Path $directory 'observer-result.json'), (($proof | ConvertTo-Json -Depth 10) -replace "`r`n", "`n") + "`n", $utf8)
Write-Output ($proof | ConvertTo-Json -Depth 10 -Compress)
if (!$valid) { exit 2 }
exit 0
