#requires -Version 5.1

Describe 'PowerShell bootstrap safety' {
    BeforeAll {
        $scriptPath = Join-Path $PSScriptRoot '..\scripts\bootstrap.ps1'
        $content = Get-Content -LiteralPath $scriptPath -Raw
    }

    It 'supports WhatIf' {
        if ($content -notmatch 'SupportsShouldProcess') { throw 'SupportsShouldProcess missing' }
        if ($content -notmatch 'ShouldProcess') { throw 'ShouldProcess missing' }
    }

    It 'implements Official rather than only accepting the value' {
        if ($content -notmatch "DownloadStrategy -eq 'Offline'") { throw 'Offline strategy missing' }
        if ($content -notmatch 'releaseBase') { throw 'Official release URL missing' }
        if ($content -notmatch 'Invoke-Download') { throw 'Downloader missing' }
    }

    It 'requires HTTPS and SHA-256' {
        if ($content -notmatch "Scheme -ne 'https'") { throw 'HTTPS gate missing' }
        if ($content -notmatch 'Get-FileHash -Algorithm SHA256') { throw 'SHA-256 verification missing' }
    }

    It 'does not disable TLS validation or execute downloaded script text' {
        if ($content -match 'CertificatePolicy') { throw 'Certificate policy override found' }
        if ($content -match 'SkipCertificateCheck') { throw 'TLS bypass found' }
        if ($content -match 'Invoke-Expression') { throw 'Downloaded text execution found' }
    }

    It 'never offers Apply as an automatic post-install stage' {
        if ($content -notmatch "ValidateSet\('Check', 'Plan', 'Verify', 'None'\)") { throw 'Safe Run set missing' }
        if ($content -match "ValidateSet\([^)]*'Apply'") { throw 'Apply offered as automatic Run stage' }
    }
}
