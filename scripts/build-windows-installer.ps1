#requires -Version 5.1
[CmdletBinding()]
param(
    [ValidatePattern('^\d+\.\d+\.\d+$')]
    [string]$Version = '0.2.4'
)

$ErrorActionPreference = 'Stop'
$repository = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$configPath = Join-Path $repository 'wails.json'
$config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
if ($config.info.productVersion -ne $Version) {
    throw "wails.json productVersion '$($config.info.productVersion)' does not match requested version '$Version'."
}

$template = Join-Path $repository 'packaging\windows\installer\project.nsi'
$infoTemplate = Join-Path $repository 'packaging\windows\info.json.in'
$generatedInstallerDirectory = Join-Path $repository 'build\windows\installer'
New-Item -ItemType Directory -Force -Path $generatedInstallerDirectory | Out-Null
Copy-Item -LiteralPath $template -Destination (Join-Path $generatedInstallerDirectory 'project.nsi') -Force

$parts = $Version.Split('.')
if ($parts.Count -ne 3 -or @($parts | Where-Object { $_ -notmatch '^\d+$' }).Count -ne 0) {
    throw "Windows ProductVersion must contain three numeric parts: $Version"
}
$infoPath = Join-Path $repository 'build\windows\info.json'
$infoExisted = Test-Path -LiteralPath $infoPath
$originalInfo = if ($infoExisted) { [IO.File]::ReadAllBytes($infoPath) } else { $null }
$originalFrontendVersion = $env:VITE_APP_VERSION

try {
    $env:VITE_APP_VERSION = $Version
    $infoSource = (Get-Content -LiteralPath $infoTemplate -Raw).Replace('@VERSION@', $Version)
    [IO.File]::WriteAllText($infoPath, $infoSource, (New-Object Text.UTF8Encoding($false)))

    $wails = Get-Command wails -ErrorAction Stop
    & $wails.Source build -clean -nsis -webview2 embed -installscope user -platform windows/amd64 `
        -ldflags "-s -w -X github.com/Shallow-dusty/ssh-launchpad/internal/launchpad.Version=$Version"
    if ($LASTEXITCODE -ne 0) {
        throw 'Wails NSIS build failed.'
    }
}
finally {
    if ($null -eq $originalFrontendVersion) {
        Remove-Item Env:\VITE_APP_VERSION -ErrorAction SilentlyContinue
    }
    else {
        $env:VITE_APP_VERSION = $originalFrontendVersion
    }
    if ($infoExisted) {
        [IO.File]::WriteAllBytes($infoPath, $originalInfo)
    }
    else {
        Remove-Item -LiteralPath $infoPath -Force -ErrorAction SilentlyContinue
    }
}

$installer = Get-ChildItem (Join-Path $repository 'build\bin') -Filter '*-installer.exe' |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1
if (-not $installer) {
    throw 'Wails did not produce an NSIS installer.'
}
$versionInfo = $installer.VersionInfo
if ($versionInfo.ProductVersion -notlike "$Version*") {
    throw "Installer ProductVersion '$($versionInfo.ProductVersion)' does not match '$Version'."
}
$application = Join-Path $repository 'build\bin\SSH-Launchpad.exe'
$applicationVersion = (Get-Item -LiteralPath $application).VersionInfo
if ($applicationVersion.ProductVersion -notlike "$Version*" -or $applicationVersion.FileVersion -notlike "$Version*") {
    throw "GUI version resources do not match '$Version': ProductVersion='$($applicationVersion.ProductVersion)', FileVersion='$($applicationVersion.FileVersion)'."
}

Write-Host "Windows upgrade installer ready: $($installer.FullName)"
