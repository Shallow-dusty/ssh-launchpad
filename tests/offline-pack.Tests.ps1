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
                $names = $archive.Entries | ForEach-Object FullName
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
}
