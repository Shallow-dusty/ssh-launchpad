[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [Alias('Path')]
    [string]$Directory
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path -LiteralPath $Directory).Path
$releaseLayout = Test-Path -LiteralPath (Join-Path $root 'checksums.txt')

if ($releaseLayout) {
    $assets = Get-ChildItem -LiteralPath $root -File
    foreach ($pattern in @(
        'ssh-launchpad_*_windows_amd64.zip',
        'ssh-launchpad_*_linux_amd64.tar.gz',
        'ssh-launchpad_*_darwin_arm64.tar.gz',
        'ssh-launchpad_*_bootstrap.zip',
        'SSH-Launchpad_*_windows_amd64_setup.exe',
        'ssh-launchpad.spdx.json'
    )) {
        if (-not ($assets | Where-Object Name -Like $pattern)) {
            throw "Package smoke check failed: missing release asset $pattern"
        }
    }

    $bootstrap = $assets | Where-Object Name -Like 'ssh-launchpad_*_bootstrap.zip' | Select-Object -First 1
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [System.IO.Compression.ZipFile]::OpenRead($bootstrap.FullName)
    try {
        $entries = $archive.Entries | ForEach-Object FullName
        foreach ($required in @('bootstrap.ps1', 'bootstrap.sh', 'profiles/example.yaml', 'LICENSE', 'README.md')) {
            if (-not ($entries -match ([regex]::Escape($required) + '$'))) {
                throw "Package smoke check failed: bootstrap bundle missing $required"
            }
        }
    }
    finally {
        $archive.Dispose()
    }

    $manifest = Get-Content -LiteralPath (Join-Path $root 'checksums.txt')
    foreach ($asset in $assets | Where-Object Name -NotLike 'checksums.txt') {
        $line = $manifest | Where-Object { $_ -match ('^[a-fA-F0-9]{64}\s+\*?' + [regex]::Escape($asset.Name) + '$') }
        if (-not $line) { throw "Package smoke check failed: no checksum for $($asset.Name)" }
        $expected = (($line | Select-Object -First 1) -split '\s+')[0]
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $asset.FullName).Hash
        if ($actual -ne $expected) { throw "Package smoke check failed: checksum mismatch for $($asset.Name)" }
    }
}
else {
    foreach ($path in @(
        'README.md',
        'LICENSE',
        'CHANGELOG.md',
        'scripts\bootstrap.ps1',
        'scripts\bootstrap.sh',
        'profiles\example.yaml'
    )) {
        $candidate = Join-Path $root $path
        if (-not (Test-Path -LiteralPath $candidate)) {
            throw "Package smoke check failed: missing $path"
        }
    }
}

$privatePatterns = @(
    'BEGIN OPENSSH PRIVATE KEY',
    'BEGIN PRIVATE KEY',
    'authkey=',
    'tailscale auth',
    '100\.76\.50\.64',
    'KINDRED-REQUIEM',
    'kindr@'
)
$files = Get-ChildItem -LiteralPath $root -Recurse -File |
    Where-Object { $_.Length -lt 2MB }
foreach ($pattern in $privatePatterns) {
    $matches = $files | Select-String -Pattern $pattern -ErrorAction SilentlyContinue
    if ($matches) {
        throw "Package smoke check failed: private/device material matched '$pattern'."
    }
}

Write-Host "Package smoke check passed: $root"
