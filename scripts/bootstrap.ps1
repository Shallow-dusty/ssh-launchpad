#requires -Version 5.1
[CmdletBinding(SupportsShouldProcess)]
param(
    [string]$Version = '0.1.0',
    [string]$InstallDirectory = (Join-Path $env:LOCALAPPDATA 'SSH-Launchpad\bin'),
    [ValidateSet('Official', 'Mirror', 'Proxy', 'Offline', 'Cache')]
    [string]$DownloadStrategy = 'Official',
    [string]$BaseUrl = 'https://github.com/Shallow-dusty/ssh-launchpad/releases/download',
    [string]$ProxyUrl,
    [string]$OfflineBundle,
    [string]$CacheDirectory = (Join-Path $env:LOCALAPPDATA 'SSH-Launchpad\cache'),
    [ValidateSet('Check', 'Plan', 'Verify', 'None')]
    [string]$Run = 'Check',
    [string]$Profile,
    [switch]$Desktop
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'Continue'

function Get-Architecture {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'ARM64' { return 'arm64' }
        'AMD64' { return 'amd64' }
        default { throw "Unsupported Windows architecture: $env:PROCESSOR_ARCHITECTURE" }
    }
}

function Invoke-Download {
    param(
        [Parameter(Mandatory)] [uri]$Uri,
        [Parameter(Mandatory)] [string]$Destination
    )

    if ($Uri.Scheme -ne 'https') {
        throw "Refusing non-HTTPS download: $Uri"
    }
    $attempts = 3
    for ($attempt = 1; $attempt -le $attempts; $attempt++) {
        try {
            $parameters = @{
                Uri = $Uri
                OutFile = $Destination
                UseBasicParsing = $true
                ErrorAction = 'Stop'
            }
            if ($ProxyUrl) {
                $parameters.Proxy = $ProxyUrl
            }
            Invoke-WebRequest @parameters
            return
        }
        catch {
            if ($attempt -eq $attempts) { throw }
            Write-Warning "Download attempt $attempt failed: $($_.Exception.Message)"
            Start-Sleep -Seconds ([Math]::Pow(2, $attempt))
        }
    }
}

function Get-ExpectedHash {
    param([string]$Manifest, [string]$AssetName)
    $line = Get-Content -LiteralPath $Manifest |
        Where-Object { $_ -match ('(?i)^[a-f0-9]{64}\s+\*?' + [regex]::Escape($AssetName) + '$') } |
        Select-Object -First 1
    if (-not $line) {
        throw "No SHA-256 entry for $AssetName in $Manifest"
    }
    return (($line -split '\s+')[0]).ToUpperInvariant()
}

function Assert-Hash {
    param([string]$Path, [string]$Expected)
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToUpperInvariant()
    if ($actual -ne $Expected.ToUpperInvariant()) {
        throw "SHA-256 mismatch for $Path. Expected $Expected, got $actual"
    }
}

$architecture = Get-Architecture
$tag = "v$Version"
$cliAsset = "ssh-launchpad_${Version}_windows_${architecture}.zip"
$desktopAsset = "SSH-Launchpad_${Version}_windows_${architecture}_setup.exe"
$assetName = if ($Desktop) { $desktopAsset } else { $cliAsset }
$archivePath = Join-Path $CacheDirectory $assetName
$manifestPath = Join-Path $CacheDirectory 'checksums.txt'

New-Item -ItemType Directory -Force -Path $CacheDirectory | Out-Null
New-Item -ItemType Directory -Force -Path $InstallDirectory | Out-Null

if ($DownloadStrategy -eq 'Offline') {
    if (-not $OfflineBundle -or -not (Test-Path -LiteralPath $OfflineBundle)) {
        throw 'Offline requires -OfflineBundle pointing to a local release asset.'
    }
    $archivePath = (Resolve-Path -LiteralPath $OfflineBundle).Path
    $adjacentManifest = Join-Path (Split-Path -Parent $archivePath) 'checksums.txt'
    if (-not (Test-Path -LiteralPath $adjacentManifest)) {
        throw "Offline bundle requires checksums.txt next to the asset."
    }
    $manifestPath = $adjacentManifest
}
elseif ($DownloadStrategy -eq 'Cache') {
    if (-not (Test-Path -LiteralPath $archivePath) -or -not (Test-Path -LiteralPath $manifestPath)) {
        throw "Verified cache is incomplete: $CacheDirectory"
    }
}
else {
    if ($DownloadStrategy -eq 'Mirror' -and $BaseUrl -notmatch '^https://') {
        throw 'Mirror BaseUrl must use HTTPS.'
    }
    if ($DownloadStrategy -eq 'Proxy' -and -not $ProxyUrl) {
        throw 'Proxy strategy requires -ProxyUrl.'
    }
    $releaseBase = "$($BaseUrl.TrimEnd('/'))/$tag"
    Invoke-Download -Uri "$releaseBase/checksums.txt" -Destination $manifestPath
    Invoke-Download -Uri "$releaseBase/$assetName" -Destination $archivePath
}

$expectedHash = Get-ExpectedHash -Manifest $manifestPath -AssetName $assetName
Assert-Hash -Path $archivePath -Expected $expectedHash
Write-Host "Verified $assetName ($expectedHash)" -ForegroundColor Green

if (-not $PSCmdlet.ShouldProcess($InstallDirectory, "Install $assetName")) {
    return
}

if ($Desktop) {
    $arguments = '/CURRENTUSER', '/S'
    $process = Start-Process -FilePath $archivePath -ArgumentList $arguments -Wait -PassThru
    if ($process.ExitCode -ne 0) {
        throw "Desktop installer failed with exit code $($process.ExitCode)"
    }
    Write-Host 'SSH Launchpad desktop installed.' -ForegroundColor Green
    return
}

$stage = Join-Path $CacheDirectory "extract-$Version-$architecture"
if (Test-Path -LiteralPath $stage) {
    Remove-Item -LiteralPath $stage -Recurse -Force
}
Expand-Archive -LiteralPath $archivePath -DestinationPath $stage -Force
$binary = Get-ChildItem -LiteralPath $stage -Filter 'ssh-launchpad.exe' -Recurse | Select-Object -First 1
if (-not $binary) {
    throw 'Release archive did not contain ssh-launchpad.exe.'
}
Copy-Item -LiteralPath $binary.FullName -Destination (Join-Path $InstallDirectory 'ssh-launchpad.exe') -Force
$installed = Join-Path $InstallDirectory 'ssh-launchpad.exe'

if ($Run -ne 'None') {
    $arguments = @($Run.ToLowerInvariant())
    if ($Profile) {
        $arguments += @('--profile', (Resolve-Path -LiteralPath $Profile).Path)
    }
    $arguments += @('--output', '-')
    & $installed @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "ssh-launchpad $Run failed with exit code $LASTEXITCODE"
    }
}

Write-Host "Installed: $installed" -ForegroundColor Green
Write-Host "Add this directory to PATH when desired: $InstallDirectory"
