#requires -Version 5.1
[CmdletBinding()]
param(
    [string]$Version = '0.2.0',
    [switch]$IncludeWindowsDesktop
)

$ErrorActionPreference = 'Stop'
$repository = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$releaseRoot = Join-Path $repository "dist\v$Version"
if (Test-Path -LiteralPath $releaseRoot) {
    throw "Release directory already exists: $releaseRoot"
}
New-Item -ItemType Directory -Path $releaseRoot | Out-Null

$originalGOOS = $env:GOOS
$originalGOARCH = $env:GOARCH
$originalCGO = $env:CGO_ENABLED
try {
    foreach ($target in @(
        @{ OS = 'windows'; Arch = 'amd64'; Extension = '.exe'; Archive = 'zip' },
        @{ OS = 'windows'; Arch = 'arm64'; Extension = '.exe'; Archive = 'zip' },
        @{ OS = 'linux'; Arch = 'amd64'; Extension = ''; Archive = 'tar' },
        @{ OS = 'linux'; Arch = 'arm64'; Extension = ''; Archive = 'tar' },
        @{ OS = 'darwin'; Arch = 'amd64'; Extension = ''; Archive = 'tar' },
        @{ OS = 'darwin'; Arch = 'arm64'; Extension = ''; Archive = 'tar' }
    )) {
        $env:GOOS = $target.OS
        $env:GOARCH = $target.Arch
        $env:CGO_ENABLED = '0'
        $name = "ssh-launchpad_${Version}_$($target.OS)_$($target.Arch)"
        $displayOS = switch ($target.OS) { 'windows' { 'Windows' }; 'linux' { 'Linux' }; 'darwin' { 'macOS' } }
        $displayArch = if ($target.Arch -eq 'amd64') { 'x64' } else { 'ARM64' }
        $assetBase = "SSH-Launchpad_${Version}_${displayOS}_${displayArch}_Portable"
        $stage = Join-Path $releaseRoot $assetBase
        New-Item -ItemType Directory -Path $stage | Out-Null
        $binary = Join-Path $stage ("ssh-launchpad" + $target.Extension)
        & go build -trimpath -ldflags "-s -w -X github.com/Shallow-dusty/ssh-launchpad/internal/launchpad.Version=$Version" -o $binary ./cmd/ssh-launchpad
        if ($LASTEXITCODE -ne 0) { throw "Go build failed for $name" }
        Copy-Item LICENSE, README.md, CHANGELOG.md -Destination $stage
        Copy-Item profiles -Destination $stage -Recurse
        Copy-Item docs\offline-help.zh-CN.md -Destination (Join-Path $stage '离线帮助-中文.md')
        Copy-Item docs\offline-help.en.md -Destination (Join-Path $stage 'Offline Help - English.md')
        Copy-Item docs\offline-pack.md -Destination $stage
        Copy-Item scripts\new-offline-pack.ps1, scripts\new-offline-pack.sh -Destination $stage
        if ($target.OS -eq 'windows') {
            Copy-Item 'packaging\launchers\开始使用 SSH Launchpad.cmd', 'packaging\launchers\Start SSH Launchpad.cmd' -Destination $stage
        }
        elseif ($target.OS -eq 'darwin') {
            Copy-Item 'packaging\launchers\Start SSH Launchpad.command' -Destination $stage
        }
        else {
            Copy-Item 'packaging\launchers\ssh-launchpad.desktop' -Destination $stage
        }
        $bundleHashLines = Get-ChildItem -LiteralPath $stage -Recurse -File | Sort-Object FullName | ForEach-Object {
            $relative = $_.FullName.Substring($stage.Length + 1).Replace('\', '/')
            "$((Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant())  $relative"
        }
        $bundleHashLines | Set-Content -LiteralPath (Join-Path $stage 'bundle-checksums.txt') -Encoding ascii
        if ($target.Archive -eq 'zip') {
            Compress-Archive -Path (Join-Path $stage '*') -DestinationPath (Join-Path $releaseRoot "$assetBase.zip")
        }
        else {
            & tar -czf (Join-Path $releaseRoot "$assetBase.tar.gz") -C $stage .
            if ($LASTEXITCODE -ne 0) { throw "tar failed for $name" }
        }
    }
}
finally {
    $env:GOOS = $originalGOOS
    $env:GOARCH = $originalGOARCH
    $env:CGO_ENABLED = $originalCGO
}

$bootstrapStage = Join-Path $releaseRoot "ssh-launchpad_${Version}_bootstrap"
New-Item -ItemType Directory -Path $bootstrapStage | Out-Null
Copy-Item scripts\bootstrap.ps1, scripts\bootstrap.sh, scripts\new-offline-pack.ps1, scripts\new-offline-pack.sh, LICENSE, README.md -Destination $bootstrapStage
Copy-Item docs\offline-help.zh-CN.md, docs\offline-help.en.md, docs\offline-pack.md -Destination $bootstrapStage
Copy-Item profiles -Destination $bootstrapStage -Recurse
Compress-Archive -Path (Join-Path $bootstrapStage '*') -DestinationPath (Join-Path $releaseRoot "ssh-launchpad_${Version}_bootstrap.zip")

if ($IncludeWindowsDesktop) {
    $wails = Get-Command wails -ErrorAction Stop
    & $wails.Source build -clean -nsis -webview2 embed -installscope user -platform windows/amd64
    if ($LASTEXITCODE -ne 0) { throw 'Wails NSIS build failed' }
    $installer = Get-ChildItem build\bin -Filter '*installer.exe' | Select-Object -First 1
    if (-not $installer) {
        $installer = Get-ChildItem build\bin -Filter '*.exe' |
            Where-Object Name -ne 'SSH-Launchpad.exe' |
            Select-Object -First 1
    }
    if (-not $installer) { throw 'Wails did not produce an NSIS installer' }
    Copy-Item $installer.FullName (Join-Path $releaseRoot "SSH-Launchpad_${Version}_Windows_x64_Installer_UNSIGNED.exe")
}

$stagingDirectories = Get-ChildItem -LiteralPath $releaseRoot -Directory
foreach ($directory in $stagingDirectories) {
    Remove-Item -LiteralPath $directory.FullName -Recurse -Force
}
$hashLines = Get-ChildItem -LiteralPath $releaseRoot -File |
    Sort-Object Name |
    ForEach-Object {
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
        "$hash  $($_.Name)"
    }
$hashLines | Set-Content -LiteralPath (Join-Path $releaseRoot 'checksums.txt') -Encoding ascii
Write-Host "Release staging ready: $releaseRoot"
