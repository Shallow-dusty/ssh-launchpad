$ErrorActionPreference = 'Stop'

Describe 'Release metadata contract' {
    BeforeAll {
        $script:Root = Split-Path -Parent $PSScriptRoot
        $script:WorkflowPath = Join-Path $script:Root '.github\workflows\release.yml'
        $script:Workflow = Get-Content -LiteralPath $script:WorkflowPath -Raw
    }

    It 'resolves release notes from the pushed tag' {
        $script:Workflow | Should -Match 'release-notes-\$\{GITHUB_REF_NAME\}\.md'
        $script:Workflow | Should -Not -Match '--notes-file\s+\.github/release-notes-v\d'
    }

    It 'fails instead of publishing without tag-matched notes' {
        $script:Workflow | Should -Match 'if \[\[ ! -f "\$notes_file" \]\]'
        $script:Workflow | Should -Match 'exit 1'
    }

    It 'contains notes for the current release candidate' {
        Test-Path -LiteralPath (Join-Path $script:Root '.github\release-notes-v0.2.3.md') |
            Should -BeTrue
    }

    It 'uses the validated Windows installer builder and runs an upgrade smoke' {
        $script:Workflow | Should -Match 'scripts/build-windows-installer\.ps1'
        $script:Workflow | Should -Match 'tests/installer-upgrade-smoke\.ps1'
        $script:Workflow | Should -Not -Match '(?m)^\s*run:\s*wails build '
    }

    It 'runs the documented release security and concurrency gates' {
        foreach ($gate in @('go test -race', 'staticcheck', 'govulncheck', 'gosec', 'pnpm audit')) {
            $script:Workflow | Should -Match ([regex]::Escape($gate))
        }
    }

    It 'isolates the previous installer from write-capable repository credentials' {
        $script:Workflow | Should -Match '(?m)^permissions:\r?\n\s+contents: read'
        $script:Workflow | Should -Match '(?ms)^  publish:.*?^    permissions:\r?\n      contents: write'
        $script:Workflow | Should -Match 'Checksum mismatch for \$baseAsset'
        $script:Workflow | Should -Match "(?ms)- name: Run verified upgrade smoke without repository credentials.*?GH_TOKEN: ''\r?\n\s+GITHUB_TOKEN: ''"
    }

    It 'creates an SBOM in local release packages before checksums' {
        $packageScript = Get-Content -LiteralPath (Join-Path $script:Root 'scripts\package-release.ps1') -Raw
        $packageScript | Should -Match 'syft'
        $packageScript | Should -Match 'ssh-launchpad\.spdx\.json'
        $packageScript.IndexOf('ssh-launchpad.spdx.json') | Should -BeLessThan $packageScript.LastIndexOf('checksums.txt')
    }

    It 'injects the requested release version into the frontend build' {
        $installerScript = Get-Content -LiteralPath (Join-Path $script:Root 'scripts\build-windows-installer.ps1') -Raw
        $viteConfig = Get-Content -LiteralPath (Join-Path $script:Root 'frontend\vite.config.ts') -Raw
        $installerScript | Should -Match 'VITE_APP_VERSION'
        $viteConfig | Should -Match 'VITE_APP_VERSION'
    }

    It 'pins every third-party workflow action to a full commit SHA' {
        foreach ($workflowName in @('release.yml', 'ci.yml')) {
            $workflow = Get-Content -LiteralPath (Join-Path $script:Root ".github\workflows\$workflowName")
            foreach ($line in $workflow | Where-Object { $_ -match '^\s*-?\s*uses:\s*[^./\s][^@\s]*@' }) {
                $line | Should -Match '@[a-f0-9]{40}(?:\s+#\s+\S+)?\s*$'
            }
        }
    }

    It 'does not persist checkout credentials into later workflow steps' {
        foreach ($workflowName in @('release.yml', 'ci.yml')) {
            $workflow = Get-Content -LiteralPath (Join-Path $script:Root ".github\workflows\$workflowName") -Raw
            ([regex]::Matches($workflow, 'uses:\s*actions/checkout@')).Count |
                Should -Be ([regex]::Matches($workflow, 'persist-credentials:\s*false')).Count
        }
    }

    It 'keeps release defaults aligned with the frontend package version' {
        $version = (Get-Content -LiteralPath (Join-Path $script:Root 'frontend\package.json') -Raw | ConvertFrom-Json).version
        (Get-Content -LiteralPath (Join-Path $script:Root 'wails.json') -Raw | ConvertFrom-Json).info.productVersion |
            Should -Be $version
        foreach ($path in @(
            'internal\launchpad\types.go',
            'scripts\bootstrap.ps1',
            'scripts\bootstrap.sh',
            'scripts\build-windows-installer.ps1',
            'scripts\package-release.ps1'
        )) {
            (Get-Content -LiteralPath (Join-Path $script:Root $path) -Raw) |
                Should -Match ([regex]::Escape($version))
        }
        Test-Path -LiteralPath (Join-Path $script:Root ".github\release-notes-v$version.md") |
            Should -BeTrue
    }
}
