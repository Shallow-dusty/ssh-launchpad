#requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$BaseInstaller,
    [Parameter(Mandatory)]
    [string]$UpgradeInstaller,
    [string]$BaseVersion = '0.2.3',
    [string]$UpgradeVersion = '0.2.4'
)

$ErrorActionPreference = 'Stop'
$base = (Resolve-Path -LiteralPath $BaseInstaller).Path
$upgrade = (Resolve-Path -LiteralPath $UpgradeInstaller).Path
$uninstallRoot = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall'
$displayName = 'SSH Launchpad'

function Get-LaunchpadEntries {
    @(Get-ChildItem -LiteralPath $uninstallRoot -ErrorAction SilentlyContinue |
        ForEach-Object { Get-ItemProperty -LiteralPath $_.PSPath } |
        Where-Object DisplayName -EQ $displayName)
}

function Invoke-SilentInstaller([string]$Path) {
    $process = Start-Process -FilePath $Path -ArgumentList '/S' -PassThru -Wait -WindowStyle Hidden
    if ($process.ExitCode -ne 0) {
        throw "Installer failed with exit code $($process.ExitCode): $Path"
    }
}

$existing = Get-LaunchpadEntries
if ($existing.Count -ne 0) {
    throw 'Upgrade smoke requires a clean per-user SSH Launchpad installation state.'
}

$installed = $false
try {
    Invoke-SilentInstaller $base
    $installed = $true
    $baseEntries = Get-LaunchpadEntries
    if ($baseEntries.Count -ne 1) {
        throw "Expected one v$BaseVersion uninstall entry, found $($baseEntries.Count)."
    }
    if ($baseEntries[0].DisplayVersion -ne $BaseVersion) {
        throw "Expected base DisplayVersion $BaseVersion, got $($baseEntries[0].DisplayVersion)."
    }
    $installDirectory = Split-Path -Parent $baseEntries[0].DisplayIcon
    $sentinel = Join-Path $installDirectory 'upgrade-preservation-sentinel.txt'
    [IO.File]::WriteAllText($sentinel, 'preserve across in-place upgrade')

    Invoke-SilentInstaller $upgrade
    $upgradeEntries = Get-LaunchpadEntries
    if ($upgradeEntries.Count -ne 1) {
        throw "Expected one v$UpgradeVersion uninstall entry, found $($upgradeEntries.Count)."
    }
    if ($upgradeEntries[0].DisplayVersion -ne $UpgradeVersion) {
        throw "Expected upgraded DisplayVersion $UpgradeVersion, got $($upgradeEntries[0].DisplayVersion)."
    }
    $upgradedDirectory = Split-Path -Parent $upgradeEntries[0].DisplayIcon
    if ($upgradedDirectory -ne $installDirectory) {
        throw "Install directory changed during upgrade: '$installDirectory' -> '$upgradedDirectory'."
    }
    if (-not (Test-Path -LiteralPath $sentinel)) {
        throw 'Upgrade removed an unrelated file, indicating uninstall/reinstall instead of in-place replacement.'
    }
    if ($upgradeEntries[0].InstallLocation -ne $installDirectory) {
        throw "InstallLocation was not recorded as '$installDirectory'."
    }

    $programsShortcut = Join-Path ([Environment]::GetFolderPath('Programs')) 'SSH Launchpad.lnk'
    $desktopShortcut = Join-Path ([Environment]::GetFolderPath('Desktop')) 'SSH Launchpad.lnk'
    foreach ($shortcut in @($programsShortcut, $desktopShortcut)) {
        if (-not (Test-Path -LiteralPath $shortcut)) {
            throw "Expected shortcut is missing: $shortcut"
        }
    }

    $executable = $upgradeEntries[0].DisplayIcon
    $productVersion = (Get-Item -LiteralPath $executable).VersionInfo.ProductVersion
    if ($productVersion -notlike "$UpgradeVersion*") {
        throw "Installed executable ProductVersion '$productVersion' does not match '$UpgradeVersion'."
    }

    Write-Host "Installer upgrade smoke passed: $BaseVersion -> $UpgradeVersion in $installDirectory"
}
finally {
    if ($installed) {
        $entry = Get-LaunchpadEntries | Select-Object -First 1
        $uninstaller = if ($entry) { $entry.UninstallString.Trim('"') } else { '' }
        if ($uninstaller -and (Test-Path -LiteralPath $uninstaller)) {
            Invoke-SilentInstaller $uninstaller
        }
    }
    if ((Get-LaunchpadEntries).Count -ne 0) {
        throw 'Upgrade smoke cleanup failed: uninstall entry remains.'
    }
}
