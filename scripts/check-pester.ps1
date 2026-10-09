#requires -Version 5.1
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Import-Module Pester -MinimumVersion 5.7.1 -ErrorAction Stop
$reportDirectory = Join-Path $root 'build/audit/pester'
New-Item -ItemType Directory -Force -Path $reportDirectory | Out-Null

$config = New-PesterConfiguration
$config.Run.Path = Join-Path $root 'tests'
$config.Run.Exit = $true
$config.TestResult.Enabled = $true
$config.TestResult.OutputPath = Join-Path $reportDirectory 'testResults.xml'
Invoke-Pester -Configuration $config
