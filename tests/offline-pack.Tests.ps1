#requires -Version 5.1

Describe 'Offline dependency pack' {
    It 'records source, license, redistribution and SHA-256 without bundling implicit files' {
        $root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
        $temp = Join-Path $root ('build/test-tmp/ssh-launchpad-pack-test-' + [guid]::NewGuid().ToString('N'))
        try {
            $payload = Join-Path $temp 'payload'
            New-Item -ItemType Directory -Path $payload -Force | Out-Null
            [IO.File]::WriteAllText((Join-Path $payload 'openssh.test'), 'not-an-installer', (New-Object Text.UTF8Encoding($false)))
            $metadata = @{
                schemaVersion = 1
                components = @(@{
                    file = 'openssh.test'
                    sourceUrl = 'https://example.invalid/openssh.test'
                    license = 'test-only'
                    redistributionAllowed = $false
                })
            } | ConvertTo-Json -Depth 5
            $metadataPath = Join-Path $temp 'metadata.json'
            [IO.File]::WriteAllText($metadataPath, $metadata, (New-Object Text.UTF8Encoding($false)))
            $output = Join-Path $temp 'pack.zip'
            Push-Location $root
            try {
                & (Join-Path $root 'scripts/new-offline-pack.ps1') -InputDirectory $payload -Metadata $metadataPath -Output $output
            }
            finally {
                Pop-Location
            }
            Add-Type -AssemblyName System.IO.Compression.FileSystem
            $archive = [IO.Compression.ZipFile]::OpenRead($output)
            try {
                $names = $archive.Entries | ForEach-Object { $_.FullName.Replace('\', '/') }
                $names | Should -Contain 'manifest.json'
                $names | Should -Contain 'bundle-checksums.txt'
                $names | Should -Contain 'payload/openssh.test'
                ($names | Where-Object { $_ -match 'PRIVATE|token|cookie' }) | Should -BeNullOrEmpty
            }
            finally {
                $archive.Dispose()
            }
        }
        finally {
            [GC]::Collect()
            [GC]::WaitForPendingFinalizers()
            for ($attempt = 0; $attempt -lt 3 -and (Test-Path -LiteralPath $temp); $attempt++) {
                try { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction Stop }
                catch {
                    if ($attempt -eq 2) { throw }
                    Start-Sleep -Milliseconds 100
                }
            }
        }
    }

    It 'works from a packaged directory with adjacent renamed help files' {
        $root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
        $temp = Join-Path $root ('build/test-tmp/ssh-launchpad-packaged-script-' + [guid]::NewGuid().ToString('N'))
        try {
            $payload = Join-Path $temp 'payload'
            $package = Join-Path $temp 'portable'
            New-Item -ItemType Directory -Path $payload, $package -Force | Out-Null
            [IO.File]::WriteAllText((Join-Path $payload 'component.bin'), 'payload', (New-Object Text.UTF8Encoding($false)))
            Copy-Item (Join-Path $root 'scripts/new-offline-pack.ps1') -Destination $package
            Copy-Item (Join-Path $root 'docs/offline-help.zh-CN.md') -Destination (Join-Path $package '离线帮助-中文.md')
            Copy-Item (Join-Path $root 'docs/offline-help.en.md') -Destination (Join-Path $package 'Offline Help - English.md')
            $metadataPath = Join-Path $temp 'metadata.json'
            @{
                schemaVersion = 1
                components = @(@{
                    file = 'component.bin'
                    sourceUrl = 'https://example.invalid/component.bin'
                    license = 'test-only'
                    redistributionAllowed = $false
                })
            } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $metadataPath -Encoding utf8
            $output = Join-Path $temp 'pack.zip'
            & (Join-Path $package 'new-offline-pack.ps1') -InputDirectory $payload -Metadata $metadataPath -Output $output
            $output | Should -Exist
        }
        finally {
            [GC]::Collect()
            [GC]::WaitForPendingFinalizers()
            if (Test-Path -LiteralPath $temp) { Remove-Item -LiteralPath $temp -Recurse -Force }
        }
    }
}
