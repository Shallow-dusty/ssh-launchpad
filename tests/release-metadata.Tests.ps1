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
        Test-Path -LiteralPath (Join-Path $script:Root '.github\release-notes-v0.2.4.md') |
            Should -BeTrue
    }
}
