#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Build script for backlogit (PowerShell equivalent of Makefile).

.PARAMETER Target
    The build target to run. Defaults to 'build'.
    Valid targets: build, test, lint, vet, fmt, cover, clean, install, all

.EXAMPLE
    .\build.ps1
    .\build.ps1 build
    .\build.ps1 test
    .\build.ps1 all
#>
param(
    [ValidateSet("build", "test", "lint", "vet", "fmt", "cover", "clean", "install", "all")]
    [string]$Target = "build"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Step {
    param([string]$Name, [scriptblock]$Action)
    Write-Host "`n==> $Name" -ForegroundColor Cyan
    & $Action
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: $Name (exit $LASTEXITCODE)" -ForegroundColor Red
        exit $LASTEXITCODE
    }
}

switch ($Target) {
    "build" {
        Invoke-Step "go build" {
            New-Item -ItemType Directory -Force -Path bin | Out-Null
            go build -o bin\backlogit.exe .\cmd\backlogit
        }
        Write-Host "`nBinary: bin\backlogit.exe" -ForegroundColor Green
    }

    "test" {
        Invoke-Step "go test" {
            go test -race -coverprofile=coverage.out ./...
        }
    }

    "lint" {
        Invoke-Step "golangci-lint" {
            golangci-lint run --timeout 5m
        }
    }

    "vet" {
        Invoke-Step "go vet" {
            go vet ./...
        }
    }

    "fmt" {
        Invoke-Step "gofmt check" {
            $unformatted = gofmt -l .
            if ($unformatted) {
                Write-Host "Unformatted files:" -ForegroundColor Yellow
                $unformatted
                exit 1
            }
            Write-Host "All files formatted." -ForegroundColor Green
        }
    }

    "cover" {
        Invoke-Step "coverage report" {
            go tool cover -func=coverage.out
        }
    }

    "clean" {
        Invoke-Step "clean" {
            Remove-Item -Recurse -Force bin -ErrorAction SilentlyContinue
            Remove-Item -Force coverage.out -ErrorAction SilentlyContinue
            Write-Host "Cleaned bin/ and coverage.out" -ForegroundColor Green
        }
    }

    "install" {
        Invoke-Step "go install" {
            go install .\cmd\backlogit
        }
        Write-Host "Installed to $env:GOPATH\bin\backlogit.exe" -ForegroundColor Green
    }

    "all" {
        Invoke-Step "gofmt check" {
            $unformatted = gofmt -l .
            if ($unformatted) { Write-Host $unformatted; exit 1 }
        }
        Invoke-Step "go vet"      { go vet ./... }
        Invoke-Step "golangci-lint" { golangci-lint run --timeout 5m }
        Invoke-Step "go test"     { go test -race -coverprofile=coverage.out ./... }
        Invoke-Step "go build"    {
            New-Item -ItemType Directory -Force -Path bin | Out-Null
            go build -o bin\backlogit.exe .\cmd\backlogit
        }
        Write-Host "`nAll targets passed. Binary: bin\backlogit.exe" -ForegroundColor Green
    }
}
