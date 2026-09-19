Set-StrictMode -Version Latest

$script:RequiredGolangCILintVersion = '2.13.2'
$script:ExecutableStatuses = @('queued', 'active', 'blocked')
$script:TerminalStatuses = @('done', 'archived')

function Get-RepositoryRoot {
    $root = (& git rev-parse --show-toplevel 2>$null)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($root)) {
        throw 'git-root'
    }
    return $root.Trim()
}

function Get-FrontmatterScalar {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][string]$Name
    )

    $inFrontmatter = $false
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -ceq '---') {
            if (-not $inFrontmatter) {
                $inFrontmatter = $true
                continue
            }
            break
        }
        if ($inFrontmatter -and
            $line -match "^$([regex]::Escape($Name)):\s*(\S+)\s*$") {
            return $Matches[1]
        }
    }
    return ''
}

function Resolve-ContainedPath {
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][string]$Relative,
        [switch]$Directory
    )

    if ([System.IO.Path]::IsPathRooted($Relative) -or
        $Relative -match '(^|[\\/])\.\.([\\/]|$)') {
        throw "uncontained-path:$Relative"
    }

    $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath((Join-Path $rootFull $Relative))
    if (-not $full.StartsWith(
            "$rootFull$([System.IO.Path]::DirectorySeparatorChar)",
            [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "uncontained-path:$Relative"
    }

    $cursor = $rootFull
    foreach ($segment in ($Relative -split '[\\/]')) {
        $cursor = Join-Path $cursor $segment
        try { $item = Get-Item -LiteralPath $cursor -Force -ErrorAction Stop }
        catch { throw "missing-path:$Relative" }
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "reparse-point:$Relative"
        }
    }
    $pathType = if ($Directory) { 'Container' } else { 'Leaf' }
    if (-not (Test-Path -LiteralPath $full -PathType $pathType)) {
        throw "wrong-path-type:$Relative"
    }
    return $full
}

function Resolve-CanonicalArtifact {
    <#
    Resolves an artifact by lifecycle, never by "first file found". Executable
    artifacts live in queue; successful terminal artifacts live in archive.
    Presence in both locations is always corruption, even if one copy looks
    valid.
    #>
    param(
        [Parameter(Mandatory)][string]$Root,
        [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9.-]+$')][string]$Id,
        [string]$QueueDir = '.backlogit/queue',
        [string]$ArchiveDir = '.backlogit/archive'
    )

    $queueRelative = ((Join-Path $QueueDir "$Id.md") -replace '\\', '/')
    $archiveRelative = ((Join-Path $ArchiveDir "$Id.md") -replace '\\', '/')
    $queuePath = $null
    $archivePath = $null
    try { $queuePath = Resolve-ContainedPath -Root $Root -Relative $queueRelative }
    catch {
        if ($_.Exception.Message -notmatch '^missing-path:') { throw }
    }
    try { $archivePath = Resolve-ContainedPath -Root $Root -Relative $archiveRelative }
    catch {
        if ($_.Exception.Message -notmatch '^missing-path:') { throw }
    }
    $queueExists = $null -ne $queuePath
    $archiveExists = $null -ne $archivePath

    if ($queueExists -and $archiveExists) {
        throw "duplicate-artifact:$Id"
    }
    if (-not $queueExists -and -not $archiveExists) {
        throw "missing-artifact:$Id"
    }

    $relative = if ($queueExists) { $queueRelative } else { $archiveRelative }
    $path = if ($queueExists) { $queuePath } else { $archivePath }
    $resolvedId = Get-FrontmatterScalar -Path $path -Name id
    if ($resolvedId -cne $Id) {
        throw "artifact-id-mismatch:${Id}:$resolvedId"
    }
    $status = Get-FrontmatterScalar -Path $path -Name status
    if ([string]::IsNullOrWhiteSpace($status)) {
        throw "artifact-status-missing:$Id"
    }

    if ($queueExists -and $status -cnotin $script:ExecutableStatuses) {
        throw "status-location-mismatch:${Id}:${status}:queue"
    }
    if ($archiveExists -and $status -cnotin $script:TerminalStatuses) {
        throw "status-location-mismatch:${Id}:${status}:archive"
    }

    [pscustomobject]@{
        Id = $Id
        Path = $path
        RelativePath = $relative
        Status = $status
        Location = if ($queueExists) { 'queue' } else { 'archive' }
    }
}

function Get-JsonContractBlock {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][ValidatePattern('^[a-z0-9-]+$')][string]$Name
    )

    $text = Get-Content -LiteralPath $Path -Raw
    $pattern = '(?s)<!-- BEGIN:' + [regex]::Escape($Name) +
        ' -->\s*```json\s*(.*?)\s*```\s*<!-- END:' +
        [regex]::Escape($Name) + ' -->'
    $matches = [regex]::Matches($text, $pattern)
    if ($matches.Count -ne 1) {
        throw "contract-block-count:${Name}:$($matches.Count)"
    }
    try {
        $json = $matches[0].Groups[1].Value |
            ConvertFrom-Json -NoEnumerate -DateKind String -ErrorAction Stop
    }
    catch {
        throw "contract-json:$Name"
    }
    if ($null -eq $json -or
        $json -isnot [System.Management.Automation.PSCustomObject]) {
        throw "contract-object:$Name"
    }
    [pscustomobject]@{
        Object = $json
        CanonicalJson = ($json | ConvertTo-Json -Depth 100 -Compress)
    }
}

function Get-Sha256Text {
    param([Parameter(Mandatory)][string]$Text)
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($Text)
    $hash = [System.Security.Cryptography.SHA256]::HashData($bytes)
    return [Convert]::ToHexString($hash).ToLowerInvariant()
}

function Get-TaskContractDigest {
    param([Parameter(Mandatory)][string]$TaskPath)
    $block = Get-JsonContractBlock -Path $TaskPath -Name 'task-lint-contract'
    return Get-Sha256Text -Text $block.CanonicalJson
}

function Invoke-CapturedProcess {
    param(
        [Parameter(Mandatory)][string]$FileName,
        [Parameter(Mandatory)][string[]]$Arguments,
        [Parameter(Mandatory)][string]$WorkingDirectory,
        [hashtable]$Environment = @{}
    )

    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $FileName
    foreach ($argument in $Arguments) {
        [void]$psi.ArgumentList.Add($argument)
    }
    $psi.WorkingDirectory = $WorkingDirectory
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    foreach ($entry in $Environment.GetEnumerator()) {
        $psi.Environment[$entry.Key] = [string]$entry.Value
    }

    try {
        $process = [System.Diagnostics.Process]::Start($psi)
    }
    catch {
        throw "native-launch:$FileName"
    }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    return [pscustomobject]@{
        ExitCode = $process.ExitCode
        Stdout = $stdoutTask.GetAwaiter().GetResult()
        Stderr = $stderrTask.GetAwaiter().GetResult()
    }
}

function Assert-GolangCILintVersion {
    param(
        [Parameter(Mandatory)][string]$Root,
        [string]$Executable = 'golangci-lint'
    )
    $result = Invoke-CapturedProcess -FileName $Executable -Arguments @('version') `
        -WorkingDirectory $Root
    if ($result.ExitCode -ne 0) {
        throw "linter-version-exit:$($result.ExitCode)"
    }
    $combined = "$($result.Stdout)`n$($result.Stderr)"
    $version = Assert-GolangCILintVersionText -Text $combined
    return $version
}

function Assert-GolangCILintVersionText {
    param([Parameter(Mandatory)][string]$Text)
    $version = Get-GolangCILintVersionFromText -Text $Text
    if ($version -cne $script:RequiredGolangCILintVersion) {
        throw "linter-version-mismatch:$version"
    }
    return $version
}

function Get-GolangCILintVersionFromText {
    param([Parameter(Mandatory)][string]$Text)
    $match = [regex]::Match($Text, '(?m)\bversion\s+v?(\d+\.\d+\.\d+)\b')
    if (-not $match.Success) {
        throw 'linter-version-unparseable'
    }
    return $match.Groups[1].Value
}

function Invoke-StructuredGolangCILint {
    param(
        [Parameter(Mandatory)][string]$Root,
        [string[]]$Packages = @(),
        [Parameter(Mandatory)][ValidateSet('windows', 'linux')][string]$Goos,
        [string]$Executable = 'golangci-lint'
    )

    $arguments = @(
        'run',
        '--output.json.path', 'stdout',
        '--output.text.path', 'stderr',
        '--show-stats=false',
        '--max-issues-per-linter', '0',
        '--max-same-issues', '0',
        '--uniq-by-line=false',
        '--issues-exit-code', '0'
    ) + @('--') + @($Packages)
    $result = Invoke-CapturedProcess -FileName $Executable -Arguments $arguments `
        -WorkingDirectory $Root -Environment @{ GOOS = $Goos }
    if ($result.ExitCode -ne 0) {
        throw "linter-exit:$($result.ExitCode):$($result.Stderr.Trim())"
    }
    $json = ConvertFrom-LintJson -Text $result.Stdout
    [pscustomobject]@{
        Issues = @($json.Issues)
        DiagnosticText = $result.Stderr
    }
}

function ConvertFrom-LintJson {
    param([AllowEmptyString()][string]$Text)
    if ([string]::IsNullOrWhiteSpace($Text)) {
        throw 'lint-json-empty'
    }
    try {
        $json = $Text | ConvertFrom-Json -NoEnumerate -ErrorAction Stop
    }
    catch {
        throw 'lint-json-malformed'
    }
    $hasIssues = [bool](
        $null -ne $json -and
        $json -is [System.Management.Automation.PSCustomObject] -and
        $null -ne $json.PSObject.Properties['Issues'])
    if (-not $hasIssues -or $json.Issues -isnot [System.Array]) {
        throw 'lint-json-schema'
    }
    return $json
}

function Get-LintFindingKey {
    param([Parameter(Mandatory)]$Finding, [switch]$Native)
    if ($Native) {
        $path = ('' + $Finding.Pos.Filename) -replace '\\', '/'
        $line = [int]$Finding.Pos.Line
        $column = [int]$Finding.Pos.Column
        $linter = '' + $Finding.FromLinter
        $message = '' + $Finding.Text
    }
    else {
        $path = ('' + $Finding.path) -replace '\\', '/'
        $line = [int]$Finding.line
        $column = [int]$Finding.column
        $linter = '' + $Finding.linter
        $message = '' + $Finding.message
    }
    return @($path, "$line", "$column", $linter, $message) -join '|'
}

function Get-OwnedObservedLintKeys {
    param(
        [Parameter(Mandatory)][object[]]$Issues,
        [Parameter(Mandatory)]$OwnedFiles,
        [Parameter(Mandatory)]$OwnedKeys
    )
    return @(
        foreach ($issue in $Issues) {
            $path = ('' + $issue.Pos.Filename) -replace '\\', '/'
            if ($OwnedFiles.Contains($path)) {
                $key = Get-LintFindingKey -Finding $issue -Native
                if ($OwnedKeys.Contains($key)) { $key }
            }
        }
    )
}

function Test-OrdinalSetEqual {
    param([object[]]$Expected, [object[]]$Actual)
    [string[]]$e = @($Expected | ForEach-Object { "$_" })
    [string[]]$a = @($Actual | ForEach-Object { "$_" })
    [System.Array]::Sort($e, [System.StringComparer]::Ordinal)
    [System.Array]::Sort($a, [System.StringComparer]::Ordinal)
    if ($e.Count -ne $a.Count) { return $false }
    for ($i = 0; $i -lt $e.Count; $i++) {
        if (-not [System.String]::Equals(
                $e[$i], $a[$i], [System.StringComparison]::Ordinal)) {
            return $false
        }
    }
    return $true
}

function Get-OrdinalDistinct {
    param([object[]]$Values)
    $seen = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::Ordinal)
    foreach ($value in $Values) {
        $text = "$value"
        if (-not [string]::IsNullOrWhiteSpace($text) -and $seen.Add($text)) {
            $text
        }
    }
}

function Assert-CanonicalTaskPhase {
    param([AllowEmptyString()][string]$Phase)
    if (-not [string]::IsNullOrEmpty($Phase) -and
        $Phase -cnotin @('Schema', 'PreCommit', 'PostCommit')) {
        throw "task-phase:$Phase"
    }
}

function Assert-TaskLintContractEnvelope {
    param(
        [Parameter(Mandatory)] [object]$Contract,
        [Parameter(Mandatory)] [string]$TaskId,
        [Parameter(Mandatory)] [string]$Role
    )
    if (-not (Test-OrdinalSetEqual `
            -Expected @('task_lint_cmd', 'lint_scope', 'non_vacuity_evidence') `
            -Actual @($Contract.PSObject.Properties.Name))) {
        throw "task-contract-schema:$TaskId"
    }
    if ($Contract.non_vacuity_evidence -isnot
            [System.Management.Automation.PSCustomObject] -or
        -not (Test-OrdinalSetEqual -Expected @('success_marker') `
            -Actual @($Contract.non_vacuity_evidence.PSObject.Properties.Name)) -or
        ('' + $Contract.non_vacuity_evidence.success_marker) -cne
            "TASK-LINT-OK:${TaskId}:${Role}") {
        throw "non-vacuity-evidence:$TaskId"
    }
}

function Get-UnauthorizedLintKeys {
    param(
        [Parameter(Mandatory)] [object[]]$Issues,
        [Parameter(Mandatory)]
        [System.Collections.Generic.HashSet[string]]$CurrentOwnerKeys,
        [Parameter(Mandatory)]
        [System.Collections.Generic.HashSet[string]]$AllowedOtherOwnerKeys
    )
    $result = @()
    foreach ($issue in $Issues) {
        $key = Get-LintFindingKey -Finding $issue -Native
        if ($CurrentOwnerKeys.Contains($key) -or
            -not $AllowedOtherOwnerKeys.Contains($key)) {
            $result += $key
        }
    }
    return @($result)
}

function Test-GoListFileMembership {
    param(
        [Parameter(Mandatory)] [string]$Output,
        [Parameter(Mandatory)] [string]$File
    )
    $base = [System.IO.Path]::GetFileName($File)
    return @($Output -split '\|' | Where-Object { $_ -ceq $base }).Count -eq 1
}

function Assert-PlatformInventoryShape {
    param(
        [Parameter(Mandatory)] [object]$Inventory,
        [object[]]$FeatureSupportedGoos = @('windows', 'linux')
    )
    if (('' + $Inventory.primary_goos) -cne 'windows' -or
        -not (Test-OrdinalSetEqual -Expected @('windows', 'linux') `
            -Actual @($Inventory.supported_goos)) -or
        -not (Test-OrdinalSetEqual -Expected @('windows', 'linux') `
            -Actual $FeatureSupportedGoos)) {
        throw 'platform-policy'
    }
    if ($Inventory.surface_exclusions -isnot [System.Management.Automation.PSCustomObject] -or
        -not (Test-OrdinalSetEqual -Expected @('windows', 'linux') `
            -Actual @($Inventory.surface_exclusions.PSObject.Properties.Name)) -or
        @($Inventory.surface_exclusions.windows).Count -ne 0) {
        throw 'platform-exclusions'
    }
    foreach ($row in @($Inventory.additional_findings)) {
        if ($row.PSObject.Properties.Name -cnotcontains 'goos' -or
            ('' + $row.goos) -cne 'linux') {
            throw 'additional-finding-goos'
        }
    }
}

function Assert-RequiredGoosParticipation {
    param(
        [Parameter(Mandatory)] [hashtable]$RequiredByFile,
        [Parameter(Mandatory)] [hashtable]$ObservedByFileGoos
    )
    foreach ($file in $RequiredByFile.Keys) {
        foreach ($goos in @($RequiredByFile[$file])) {
            if (-not $ObservedByFileGoos["$file|$goos"]) {
                throw "not-in-required-package:${goos}:$file"
            }
        }
    }
}

function Resolve-OwnedFilePackage {
    param(
        [Parameter(Mandatory)] [string]$File,
        [Parameter(Mandatory)] [object[]]$Packages
    )
    $normalized = $File -replace '\\', '/'
    $directory = [System.IO.Path]::GetDirectoryName($normalized) -replace '\\', '/'
    $expected = "./$directory"
    $matches = @($Packages | Where-Object { "$_" -ceq $expected })
    if ($matches.Count -ne 1) { throw "file-package:$normalized" }
    return $expected
}

function Assert-InventoryFindingCount {
    param(
        [Parameter(Mandatory)][int]$Declared,
        [Parameter(Mandatory)]$RowsByKey
    )
    if ($Declared -lt 1 -or $Declared -ne $RowsByKey.Count) {
        throw "inventory-finding-count:${Declared}:$($RowsByKey.Count)"
    }
}
