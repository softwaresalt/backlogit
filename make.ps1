#!/usr/bin/env pwsh
<#
.SYNOPSIS
    PowerShell equivalent of the project Makefile for Windows.

.PARAMETER Target
    Target to run. Defaults to 'all'.
    Valid targets: all, build, test, lint, vet, fmt, cover, clean, install, md-lint, verify-plugin

.EXAMPLE
    .\make.ps1              # runs 'all' (fmt + vet + lint + test + build)
    .\make.ps1 build        # compile binary to bin\backlogit.exe
    .\make.ps1 test         # run test suite with race detector
    .\make.ps1 lint         # golangci-lint
    .\make.ps1 vet          # go vet
    .\make.ps1 fmt          # gofmt check
    .\make.ps1 cover        # print coverage report (run 'test' first)
    .\make.ps1 clean        # remove bin/ and coverage.out
    .\make.ps1 install              # install to current location (or GOPATH\bin if not found)
    .\make.ps1 install -InstallPath C:\Tools  # install to specific path
    .\make.ps1 verify-plugin # validate plugin bundle structure
#>
param(
    [ValidateSet("all", "build", "test", "lint", "vet", "fmt", "cover", "clean", "install", "md-lint", "verify-plugin")]
    [string]$Target = "all",

    # Optional install directory for the 'install' target (e.g. C:\Tools).
    # Defaults to GOPATH\bin when not specified.
    [string]$InstallPath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Step([string]$Name, [scriptblock]$Cmd) {
    Write-Host "`n==> $Name" -ForegroundColor Cyan
    & $Cmd
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: $Name" -ForegroundColor Red
        exit $LASTEXITCODE
    }
}

switch ($Target) {

    "all" {
        Step "fmt"  { $bad = gofmt -l .; if ($bad) { Write-Host $bad; exit 1 } }
        Step "vet"  { go vet ./... }
        Step "lint" { golangci-lint run --timeout 5m }
        Step "test" { go test -race -coverprofile=coverage.out ./... }
        Step "build" {
            New-Item -ItemType Directory -Force -Path bin | Out-Null
            go build -o bin\backlogit.exe .\cmd\backlogit
        }
    }

    "build" {
        Step "build" {
            New-Item -ItemType Directory -Force -Path bin | Out-Null
            go build -o bin\backlogit.exe .\cmd\backlogit
        }
    }

    "test" {
        Step "test" { go test -race -coverprofile=coverage.out ./... }
    }

    "lint" {
        Step "lint" { golangci-lint run --timeout 5m }
    }

    "vet" {
        Step "vet" { go vet ./... }
    }

    "fmt" {
        Step "fmt" {
            $bad = gofmt -l .
            if ($bad) { Write-Host $bad -ForegroundColor Yellow; exit 1 }
            Write-Host "All files formatted." -ForegroundColor Green
        }
    }

    "cover" {
        Step "cover" { go tool cover -func=coverage.out }
    }

    "clean" {
        Step "clean" {
            Remove-Item -Recurse -Force bin      -ErrorAction SilentlyContinue
            Remove-Item -Force coverage.out       -ErrorAction SilentlyContinue
        }
    }

    "install" {
        if (-not $InstallPath) {
            $existing = Get-Command backlogit -ErrorAction SilentlyContinue
            if ($existing) {
                $InstallPath = Split-Path $existing.Source
            } else {
                $InstallPath = Join-Path $env:GOPATH "bin"
            }
        }
        Step "install" {
            New-Item -ItemType Directory -Force -Path $InstallPath | Out-Null
            $dest = Join-Path $InstallPath "backlogit.exe"
            go build -o $dest .\cmd\backlogit
            Write-Host "Installed: $dest" -ForegroundColor Green
        }
    }

    "md-lint" {
        Step "md-lint" {
            & (Join-Path $PSScriptRoot 'scripts/md-lint.ps1')
            if ($LASTEXITCODE -ne 0) {
                exit 1
            }
        }
    }

    "verify-plugin" {
        Step "verify-plugin" {
            go test ./tests/integration/ -run 'TestPluginBundleStructurallyValid' -count=1
            if ($LASTEXITCODE -ne 0) {
                exit 1
            }
        }
    }
}
