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

Describe 'Portable tar executable layout' {
    It 'accepts executable files in a flat archive' {
        $listing = @(
            '-rwxr-xr-x  0 root root 123 2026-07-29 10:00 ssh-launchpad'
            '-rwxr-xr-x  0 root root 456 2026-07-29 10:00 Start SSH Launchpad.command'
        )

        Test-TarListingExecutable -Listing $listing -RelativePath 'ssh-launchpad' |
            Should -BeTrue
        Test-TarListingExecutable -Listing $listing -RelativePath 'Start SSH Launchpad.command' |
            Should -BeTrue
    }

    It 'accepts executable files beneath a bundle root directory' {
        $listing = @(
            '-rwxr-xr-x  0 runner runner 123 2026-07-29 10:00 SSH-Launchpad_0.2.3_Linux_x64_Portable/ssh-launchpad'
            '-rwxr-xr-x  0 runner runner 456 2026-07-29 10:00 SSH-Launchpad_0.2.3_macOS_x64_Portable/Start SSH Launchpad.command'
        )

        Test-TarListingExecutable -Listing $listing -RelativePath 'ssh-launchpad' |
            Should -BeTrue
        Test-TarListingExecutable -Listing $listing -RelativePath 'Start SSH Launchpad.command' |
            Should -BeTrue
    }

    It 'rejects a non-executable target' {
        $listing = @(
            '-rw-r--r--  0 runner runner 123 2026-07-29 10:00 bundle/ssh-launchpad'
        )

        Test-TarListingExecutable -Listing $listing -RelativePath 'ssh-launchpad' |
            Should -BeFalse
    }
}
