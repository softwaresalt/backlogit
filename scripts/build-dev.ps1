# Builds the workspace-local debug backlogit binary used by .mcp.json (dogfooding).
# Windows locks a running executable, so an in-use bin/backlogit.exe (for example an
# active MCP server) is renamed aside first; restart the MCP server to pick up the build.
[CmdletBinding()]
param(
    [string]$Output = (Join-Path $PSScriptRoot '..\bin\backlogit.exe')
)
$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..')
Push-Location $root
try {
    $out = [IO.Path]::GetFullPath($Output)
    New-Item -ItemType Directory -Force -Path (Split-Path $out) | Out-Null
    Get-ChildItem -Path (Split-Path $out) -Filter ((Split-Path $out -Leaf) + '.old-*') -ErrorAction SilentlyContinue |
        ForEach-Object { Remove-Item -LiteralPath $_.FullName -ErrorAction SilentlyContinue }
    if (Test-Path -LiteralPath $out) {
        Move-Item -LiteralPath $out -Destination ($out + '.old-' + [DateTime]::UtcNow.ToString('yyyyMMddHHmmss'))
    }
    $version = (git describe --tags --always --dirty 2>$null)
    if (-not $version) { $version = 'dev' }
    $commit = (git rev-parse --short HEAD 2>$null)
    if (-not $commit) { $commit = 'unknown' }
    $date = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
    $pkg = 'github.com/softwaresalt/backlogit/internal/version'
    $ldflags = "-X $pkg.Version=$version-debug -X $pkg.Commit=$commit -X $pkg.BuildDate=$date"
    go build -gcflags 'all=-N -l' -ldflags $ldflags -o $out ./cmd/backlogit
    if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
    & $out version --format json
}
finally {
    Pop-Location
}
