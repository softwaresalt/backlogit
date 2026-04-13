<#
.SYNOPSIS
    Search installed skills by keyword.
.DESCRIPTION
    Scans all SKILL.md files under .github/skills/ and returns matches
    where the keyword appears in the skill name or its YAML frontmatter
    description field.
.PARAMETER Keyword
    The search term to match against skill names and descriptions.
.EXAMPLE
    scripts/search.ps1 review
#>
param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$Keyword
)

$skillsRoot = Join-Path $PSScriptRoot ".." ".github" "skills"
$skillsRoot = (Resolve-Path $skillsRoot -ErrorAction SilentlyContinue).Path

if (-not $skillsRoot -or -not (Test-Path $skillsRoot)) {
    Write-Error "Skills directory not found at .github/skills/"
    exit 1
}

$results = @()

Get-ChildItem -Path $skillsRoot -Directory | ForEach-Object {
    $skillName = $_.Name
    $skillFile = Join-Path $_.FullName "SKILL.md"

    if (-not (Test-Path $skillFile)) { return }

    $content = Get-Content $skillFile -Raw
    $description = ""

    # Extract description from YAML frontmatter
    if ($content -match '(?s)^---\s*\n(.*?)\n---') {
        $frontmatter = $Matches[1]
        if ($frontmatter -match 'description:\s*"([^"]*)"') {
            $description = $Matches[1]
        } elseif ($frontmatter -match "description:\s*'([^']*)'") {
            $description = $Matches[1]
        }
    }

    # Match keyword against skill name or description
    if ($skillName -match [regex]::Escape($Keyword) -or $description -match [regex]::Escape($Keyword)) {
        $relativePath = ".github/skills/$skillName/SKILL.md"
        $results += [PSCustomObject]@{
            Skill       = $skillName
            Description = if ($description.Length -gt 70) { $description.Substring(0, 67) + "..." } else { $description }
            Path        = $relativePath
        }
    }
}

if ($results.Count -eq 0) {
    Write-Host "No skills matching '$Keyword' found."
    exit 0
}

$results | Format-Table -AutoSize -Wrap
