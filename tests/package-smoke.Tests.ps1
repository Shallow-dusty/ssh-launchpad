BeforeAll {
    . (Join-Path $PSScriptRoot 'package-smoke-lib.ps1')
}

Describe 'Portable package manifest layout' {
    It 'resolves manifest targets relative to a bundle root directory' {
        $entries = @(
            [pscustomobject]@{ FullName = 'SSH-Launchpad_0.2.3_Windows_x64_Portable/bundle-checksums.txt' }
            [pscustomobject]@{ FullName = 'SSH-Launchpad_0.2.3_Windows_x64_Portable/CHANGELOG.md' }
        )

        $entry = Get-ArchiveEntryForManifestTarget `
            -Entries $entries `
            -ManifestFullName $entries[0].FullName `
            -ManifestTarget 'CHANGELOG.md'

        $entry.FullName | Should -Be 'SSH-Launchpad_0.2.3_Windows_x64_Portable/CHANGELOG.md'
    }

    It 'continues to support a flat archive layout' {
        $entries = @(
            [pscustomobject]@{ FullName = 'bundle-checksums.txt' }
            [pscustomobject]@{ FullName = 'CHANGELOG.md' }
        )

        $entry = Get-ArchiveEntryForManifestTarget `
            -Entries $entries `
            -ManifestFullName $entries[0].FullName `
            -ManifestTarget 'CHANGELOG.md'

        $entry.FullName | Should -Be 'CHANGELOG.md'
    }

    It 'rejects manifest targets that escape the bundle root' {
        $entries = @(
            [pscustomobject]@{ FullName = 'bundle/bundle-checksums.txt' }
            [pscustomobject]@{ FullName = 'outside.txt' }
        )

        Get-ArchiveEntryForManifestTarget `
            -Entries $entries `
            -ManifestFullName $entries[0].FullName `
            -ManifestTarget '../outside.txt' |
            Should -BeNullOrEmpty
    }
}
