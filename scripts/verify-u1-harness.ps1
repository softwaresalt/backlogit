# Verify shipment 212-S U1 without converting assertion RED into success.
[CmdletBinding()]
param([switch]$FeatureRoot)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
$root = Split-Path -Parent $PSScriptRoot
$env:GOROOT = Join-Path $root 'bin\_authoring_gomodcache\golang.org\toolchain@v0.0.1-go1.24.0.windows-amd64'
$env:PATH = "$env:GOROOT\bin;$env:PATH"
$env:GOTOOLCHAIN = 'local'
$env:GOFLAGS = '-mod=readonly'
$env:GOPROXY = 'off'
$env:GOSUMDB = 'off'
$env:GOCACHE = Join-Path $root 'bin\_authoring_gocache'
$env:GOMODCACHE = Join-Path $root 'bin\_authoring_gomodcache'
$env:GOPATH = Join-Path $root 'bin\_authoring_gopath'
$env:TEMP = Join-Path $root 'bin\_authoringtmp'
$env:TMP = $env:TEMP
$env:GOLANGCI_LINT_CACHE = Join-Path $root 'bin\_authoring_golangci_cache'

$selector = '^TestU1_'
$test = 'TestU1_MultiSessionCoordinatorSourceShape'
if ($FeatureRoot) {
    $selector = '^TestFeature211_'
    $test = 'TestFeature211_MultiSessionCoordinatorSourceShape'
}
$names = @(
    $test
    "$test/daemon_and_admission_sentinels"
    "$test/coordinator_types_and_busy_json_tags"
    "$test/exact_coordinator_signatures"
)

Push-Location -LiteralPath $root
try {
    $output = @(& "$env:GOROOT\bin\go.exe" test -timeout=30m -count=1 -v -run $selector ./internal/mcp 2>&1)
    $native = $LASTEXITCODE
    $text = ($output | ForEach-Object { "$_" }) -join "`n"
    Write-Output $text
    Write-Output "GO_TEST_NATIVE_EXIT=$native"

    if ($native -notin @(0, 1) -or
        $text -match '(?i)no tests to run|\[no test files\]|\[build failed\]|build failed|panic:|fatal error:' -or
        $text -notmatch '(?m)^(ok\s+|FAIL\s+)github\.com/softwaresalt/backlogit/internal/mcp(?:\s|$)') {
        Write-Output 'NAMED_TEST_GUARD=REJECTED (build, panic, vacuity, or unexpected exit/package)'
        exit 2
    }

    foreach ($name in $names) {
        $escaped = [regex]::Escape($name)
        if ($text -notmatch "(?m)^=== RUN\s+$escaped\s*$" -or
            $text -notmatch "(?m)^\s*--- (PASS|FAIL): $escaped \([^\r\n]+\)\s*$") {
            Write-Output "NAMED_TEST_GUARD=REJECTED (missing or skipped named test: $name)"
            exit 2
        }
        if ($native -eq 0 -and
            $text -notmatch "(?m)^\s*--- PASS: $escaped \([^\r\n]+\)\s*$") {
            Write-Output "NAMED_TEST_GUARD=REJECTED (exit 0 without named PASS: $name)"
            exit 2
        }
    }
    $top = [regex]::Escape($test)
    if ($native -eq 1 -and $text -notmatch "(?m)^--- FAIL: $top \([^\r\n]+\)\s*$") {
        Write-Output 'NAMED_TEST_GUARD=REJECTED (exit 1 without named assertion failure)'
        exit 2
    }
    Write-Output 'NAMED_TEST_GUARD=PASS'
    # Native 0 requires real named PASS; native 1 stays RED for downstream builds.
    exit $native
}
finally {
    Pop-Location
}
