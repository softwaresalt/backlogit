$ErrorActionPreference = "Stop"
$workspaceRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
$binDirectory = Join-Path $workspaceRoot ".bin"
$bunExe = Join-Path $binDirectory "bun.exe"
if (-not (Test-Path -LiteralPath $bunExe -PathType Leaf)) {
    throw "Workspace-local Bun executable is missing: $bunExe"
}

Set-Location -LiteralPath $workspaceRoot
$env:BUN_INSTALL = Join-Path $binDirectory "bun"
$env:BUN_INSTALL_CACHE_DIR = Join-Path $binDirectory "bun-cache"
$env:BUN_RUNTIME_TRANSPILER_CACHE_PATH = "0"
$env:TEMP = Join-Path $binDirectory "tmp"
$env:TMP = $env:TEMP
$env:TMPDIR = $env:TEMP
New-Item -ItemType Directory -Path $env:BUN_INSTALL_CACHE_DIR, $env:TEMP -Force | Out-Null

& $bunExe install --cwd $PSScriptRoot --frozen-lockfile --ignore-scripts --backend copyfile --no-progress
exit $LASTEXITCODE
