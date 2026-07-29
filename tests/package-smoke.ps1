[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [Alias('Path')]
    [string]$Directory,
    [switch]$SkipSBOM
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'package-smoke-lib.ps1')

$root = (Resolve-Path -LiteralPath $Directory).Path
$releaseLayout = Test-Path -LiteralPath (Join-Path $root 'checksums.txt')

if ($releaseLayout) {
    $assets = Get-ChildItem -LiteralPath $root -File
    foreach ($pattern in @(
        'SSH-Launchpad_*_Windows_x64_Portable.zip',
        'SSH-Launchpad_*_Linux_x64_Portable.tar.gz',
        'SSH-Launchpad_*_macOS_ARM64_Portable.tar.gz',
        'ssh-launchpad_*_bootstrap.zip',
        'SSH-Launchpad_*_Windows_x64_Installer_UNSIGNED.exe'
    )) {
        if (-not ($assets | Where-Object Name -Like $pattern)) {
            throw "Package smoke check failed: missing release asset $pattern"
        }
    }
    if (-not $SkipSBOM -and -not ($assets | Where-Object Name -EQ 'ssh-launchpad.spdx.json')) {
        throw 'Package smoke check failed: missing release asset ssh-launchpad.spdx.json'
    }

    $windowsPortable = $assets | Where-Object Name -Like 'SSH-Launchpad_*_Windows_x64_Portable.zip' | Select-Object -First 1
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $portableArchive = [System.IO.Compression.ZipFile]::OpenRead($windowsPortable.FullName)
    try {
        $portableEntries = $portableArchive.Entries | ForEach-Object FullName
        foreach ($required in @(
            'ssh-launchpad.exe',
            '开始使用 SSH Launchpad.cmd',
            'Start SSH Launchpad.cmd',
            '离线帮助-中文.md',
            'Offline Help - English.md',
            'bundle-checksums.txt',
            'profiles/example.yaml'
        )) {
            if (-not ($portableEntries -match ([regex]::Escape($required) + '$'))) {
                throw "Package smoke check failed: Windows portable missing $required"
            }
        }
        $manifestEntry = $portableArchive.Entries |
            Where-Object FullName -Match '(^|/)bundle-checksums\.txt$' |
            Select-Object -First 1
        $reader = New-Object IO.StreamReader($manifestEntry.Open(), (New-Object Text.UTF8Encoding($false, $true)))
        try {
            $bundleManifest = $reader.ReadToEnd()
        }
        finally {
            $reader.Dispose()
        }
        if ($bundleManifest -notmatch '(?m)^[a-f0-9]{64}\s+离线帮助-中文\.md\r?$') {
            throw 'Package smoke check failed: Windows portable checksum manifest is not valid UTF-8 or omits the Chinese help filename'
        }
        foreach ($line in $bundleManifest -split "\r?\n" | Where-Object { $_ }) {
            if ($line -notmatch '^([a-f0-9]{64})\s+(.+)$') {
                throw "Package smoke check failed: invalid Windows portable checksum line '$line'"
            }
            $expected = $Matches[1]
            $entryName = $Matches[2].Replace('\', '/')
            $entry = Get-ArchiveEntryForManifestTarget `
                -Entries $portableArchive.Entries `
                -ManifestFullName $manifestEntry.FullName `
                -ManifestTarget $entryName
            if (-not $entry) {
                throw "Package smoke check failed: Windows portable checksum target missing: $entryName"
            }
            $stream = $entry.Open()
            $hasher = [Security.Cryptography.SHA256]::Create()
            try {
                $actual = ([BitConverter]::ToString($hasher.ComputeHash($stream))).Replace('-', '').ToLowerInvariant()
            }
            finally {
                $hasher.Dispose()
                $stream.Dispose()
            }
            if ($actual -ne $expected) {
                throw "Package smoke check failed: Windows portable checksum mismatch: $entryName"
            }
        }
    }
    finally {
        $portableArchive.Dispose()
    }

    foreach ($unixAsset in $assets | Where-Object Name -Match '_(Linux|macOS)_.*_Portable\.tar\.gz$') {
        $listing = & tar -tvzf $unixAsset.FullName
        if ($LASTEXITCODE -ne 0) {
            throw "Package smoke check failed: cannot list $($unixAsset.Name)"
        }
        if (-not (Test-TarListingExecutable -Listing $listing -RelativePath 'ssh-launchpad')) {
            throw "Package smoke check failed: $($unixAsset.Name) ssh-launchpad is not mode 0755"
        }
        if (
            $unixAsset.Name -match '_macOS_' -and
            -not (Test-TarListingExecutable -Listing $listing -RelativePath 'Start SSH Launchpad.command')
        ) {
            throw "Package smoke check failed: $($unixAsset.Name) .command launcher is not mode 0755"
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
