#requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$InputDirectory,
    [Parameter(Mandatory)] [string]$Metadata,
    [Parameter(Mandatory)] [string]$Output
)

$ErrorActionPreference = 'Stop'
$source = (Resolve-Path -LiteralPath $InputDirectory).Path
$metadataPath = (Resolve-Path -LiteralPath $Metadata).Path
$outputPath = [IO.Path]::GetFullPath($Output)
$outputParent = Split-Path -Parent $outputPath
New-Item -ItemType Directory -Path $outputParent -Force | Out-Null
$definition = Get-Content -LiteralPath $metadataPath -Raw | ConvertFrom-Json
if ($definition.schemaVersion -ne 1 -or -not $definition.components) {
    throw 'metadata.json must use schemaVersion 1 and contain components.'
}

$stage = Join-Path $outputParent (".ssh-launchpad-offline-" + [guid]::NewGuid().ToString('N'))
try {
    New-Item -ItemType Directory -Path (Join-Path $stage 'payload') -Force | Out-Null
    $manifestComponents = @()
    foreach ($component in $definition.components) {
        if (-not $component.file -or -not $component.sourceUrl -or -not $component.license) {
            throw 'Every component needs file, sourceUrl, and license.'
        }
        if ($component.sourceUrl -notmatch '^https://') {
            throw "Component sourceUrl must use HTTPS: $($component.file)"
        }
        if ([IO.Path]::IsPathRooted($component.file) -or $component.file -match '(^|[\\/])\.\.([\\/]|$)') {
            throw "Component file must stay inside InputDirectory: $($component.file)"
        }
        $candidate = [IO.Path]::GetFullPath((Join-Path $source $component.file))
        if (-not $candidate.StartsWith($source + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Component file escapes InputDirectory: $($component.file)"
        }
        if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) {
            throw "Payload missing: $($component.file)"
        }
        $destination = Join-Path $stage (Join-Path 'payload' $component.file)
        New-Item -ItemType Directory -Path (Split-Path -Parent $destination) -Force | Out-Null
        Copy-Item -LiteralPath $candidate -Destination $destination
        $manifestComponents += [ordered]@{
            file = "payload/$($component.file -replace '\\','/')"
            sha256 = (Get-FileHash -LiteralPath $candidate -Algorithm SHA256).Hash.ToLowerInvariant()
            sourceUrl = $component.sourceUrl
            license = $component.license
            redistributionAllowed = [bool]$component.redistributionAllowed
        }
    }
    $manifest = [ordered]@{
        schemaVersion = 1
        format = 'ssh-launchpad-offline-pack'
        createdAt = [DateTime]::UtcNow.ToString('o')
        note = 'Local user-created dependency payload. Verify license before redistributing.'
        components = $manifestComponents
    }
    $json = $manifest | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText((Join-Path $stage 'manifest.json'), $json + "`n", (New-Object Text.UTF8Encoding($false)))
    $manifestComponents | ForEach-Object { "$($_.sha256)  $($_.file)" } |
        Set-Content -LiteralPath (Join-Path $stage 'bundle-checksums.txt') -Encoding ascii
    $repository = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
    Copy-Item (Join-Path $repository 'docs\offline-help.zh-CN.md'), (Join-Path $repository 'docs\offline-help.en.md') -Destination $stage
    if (Test-Path -LiteralPath $outputPath) { Remove-Item -LiteralPath $outputPath -Force }
    Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $outputPath
    Write-Host "Offline pack created: $outputPath"
}
finally {
    if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
}
