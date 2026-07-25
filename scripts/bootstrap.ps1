#requires -Version 5.1
[CmdletBinding(SupportsShouldProcess)]
param(
    [ValidatePattern('^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$')]
    [string]$Version = '0.2.4',
    [string]$InstallDirectory = (Join-Path $env:LOCALAPPDATA 'SSH-Launchpad\bin'),
    [ValidateSet('Official', 'Mirror', 'Proxy', 'Offline', 'Cache')]
    [string]$DownloadStrategy = 'Official',
    [string]$BaseUrl = 'https://github.com/Shallow-dusty/ssh-launchpad/releases/download',
    [string]$ProxyUrl,
    [string]$OfflineBundle,
    [string]$CacheDirectory = (Join-Path $env:LOCALAPPDATA 'SSH-Launchpad\cache'),
    [ValidateSet('Check', 'Plan', 'Verify', 'None')]
    [string]$Run = 'Check',
    [string]$Profile,
    [switch]$Desktop,
    [ValidateSet('Auto', 'zh-CN', 'en')]
    [string]$Language = 'Auto'
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'Continue'
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[Console]::InputEncoding = $utf8NoBom
[Console]::OutputEncoding = $utf8NoBom
$OutputEncoding = $utf8NoBom
if ($Language -eq 'Auto') {
    $Language = if ([Globalization.CultureInfo]::CurrentUICulture.Name -like 'zh*') { 'zh-CN' } else { 'en' }
}
function Get-Text {
    param([string]$Zh, [string]$En)
    if ($Language -eq 'zh-CN') { return $Zh }
    return $En
}

function Get-Architecture {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'ARM64' { return 'arm64' }
        'AMD64' { return 'amd64' }
        default { throw (Get-Text "不支持的 Windows 架构：$env:PROCESSOR_ARCHITECTURE" "Unsupported Windows architecture: $env:PROCESSOR_ARCHITECTURE") }
    }
}

function Invoke-HttpsDownloadOnce {
    param(
        [Parameter(Mandatory)] [uri]$Uri,
        [Parameter(Mandatory)] [string]$Destination
    )

    Add-Type -AssemblyName System.Net.Http
    $handler = [System.Net.Http.HttpClientHandler]::new()
    $handler.AllowAutoRedirect = $false
    if ($ProxyUrl) {
        $handler.Proxy = [System.Net.WebProxy]::new($ProxyUrl)
        $handler.UseProxy = $true
    }
    $client = [System.Net.Http.HttpClient]::new($handler)
    try {
        $current = $Uri
        for ($redirect = 0; $redirect -le 10; $redirect++) {
            if ($current.Scheme -ne 'https') {
                throw (Get-Text "已拒绝非 HTTPS 下载或重定向：$current" "Refusing non-HTTPS download or redirect: $current")
            }
            $response = $client.GetAsync($current, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
            try {
                $status = [int]$response.StatusCode
                if ($status -ge 300 -and $status -lt 400) {
                    if (-not $response.Headers.Location) { throw "Redirect response omitted Location: $current" }
                    $current = [uri]::new($current, $response.Headers.Location)
                    continue
                }
                $response.EnsureSuccessStatusCode() | Out-Null
                $input = $response.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
                try {
                    $output = [IO.File]::Open($Destination, [IO.FileMode]::Create, [IO.FileAccess]::Write, [IO.FileShare]::None)
                    try { $input.CopyTo($output) }
                    finally { $output.Dispose() }
                }
                finally { $input.Dispose() }
                return
            }
            finally { $response.Dispose() }
        }
        throw "Too many HTTPS redirects: $Uri"
    }
    finally {
        $client.Dispose()
        $handler.Dispose()
    }
}

function Invoke-Download {
    param(
        [Parameter(Mandatory)] [uri]$Uri,
        [Parameter(Mandatory)] [string]$Destination
    )

    if ($Uri.Scheme -ne 'https') {
        throw (Get-Text "已拒绝非 HTTPS 下载：$Uri" "Refusing non-HTTPS download: $Uri")
    }
    $attempts = 3
    for ($attempt = 1; $attempt -le $attempts; $attempt++) {
        try {
            Invoke-HttpsDownloadOnce -Uri $Uri -Destination $Destination
            return
        }
        catch {
            if ($attempt -eq $attempts) { throw }
            Write-Warning (Get-Text "第 $attempt 次下载失败，将自动重试：$($_.Exception.Message)" "Download attempt $attempt failed; retrying: $($_.Exception.Message)")
            Start-Sleep -Seconds ([Math]::Pow(2, $attempt))
        }
    }
}

function Get-ExpectedHash {
    param([string]$Manifest, [string]$AssetName)
    $line = Get-Content -LiteralPath $Manifest |
        Where-Object { $_ -match ('(?i)^[a-f0-9]{64}\s+\*?' + [regex]::Escape($AssetName) + '$') } |
        Select-Object -First 1
    if (-not $line) {
        throw (Get-Text "checksums.txt 中没有 $AssetName 的 SHA-256。" "No SHA-256 entry for $AssetName in $Manifest")
    }
    return (($line -split '\s+')[0]).ToUpperInvariant()
}

function Assert-Hash {
    param([string]$Path, [string]$Expected)
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToUpperInvariant()
    if ($actual -ne $Expected.ToUpperInvariant()) {
        throw (Get-Text "下载文件校验失败，已拒绝使用：$Path" "SHA-256 mismatch for $Path. Expected $Expected, got $actual")
    }
}

function Assert-ZipEntriesSafe {
    param([string]$Archive, [string]$Destination)
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $destinationRoot = [IO.Path]::GetFullPath($Destination).TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
    $zip = [IO.Compression.ZipFile]::OpenRead($Archive)
    try {
        foreach ($entry in $zip.Entries) {
            $entryName = $entry.FullName.Replace('/', [IO.Path]::DirectorySeparatorChar)
            if ([IO.Path]::IsPathRooted($entryName) -or $entryName -match '(^|[\\/])\.\.([\\/]|$)' -or $entryName -match ':') {
                throw (Get-Text "压缩包包含不安全路径：$($entry.FullName)" "Archive contains an unsafe path: $($entry.FullName)")
            }
            $target = [IO.Path]::GetFullPath((Join-Path $destinationRoot $entryName))
            if (-not $target.StartsWith($destinationRoot, [StringComparison]::OrdinalIgnoreCase)) {
                throw (Get-Text "压缩包条目越过安装暂存目录：$($entry.FullName)" "Archive entry escapes the staging directory: $($entry.FullName)")
            }
        }
    }
    finally {
        $zip.Dispose()
    }
}

$architecture = Get-Architecture
$assetArchitecture = if ($architecture -eq 'amd64') { 'x64' } else { 'ARM64' }
$tag = "v$Version"
$cliAsset = "SSH-Launchpad_${Version}_Windows_${assetArchitecture}_Portable.zip"
$desktopAsset = "SSH-Launchpad_${Version}_Windows_x64_Installer_UNSIGNED.exe"
if ($Desktop -and $architecture -ne 'amd64') {
    throw (Get-Text 'v0.2.4 暂未提供 Windows ARM64 GUI 安装器，请使用 portable CLI。' 'v0.2.4 does not provide a Windows ARM64 GUI installer; use the portable CLI.')
}
$assetName = if ($Desktop) { $desktopAsset } else { $cliAsset }
$archivePath = Join-Path $CacheDirectory $assetName
$manifestPath = Join-Path $CacheDirectory 'checksums.txt'

New-Item -ItemType Directory -Force -Path $CacheDirectory | Out-Null

if ($DownloadStrategy -eq 'Offline') {
    if (-not $OfflineBundle -or -not (Test-Path -LiteralPath $OfflineBundle)) {
        throw (Get-Text '离线模式需要 -OfflineBundle 指向本地 Release 文件。' 'Offline requires -OfflineBundle pointing to a local release asset.')
    }
    $archivePath = (Resolve-Path -LiteralPath $OfflineBundle).Path
    $adjacentManifest = Join-Path (Split-Path -Parent $archivePath) 'checksums.txt'
    if (-not (Test-Path -LiteralPath $adjacentManifest)) {
        throw (Get-Text '离线文件旁必须有 checksums.txt。' 'Offline bundle requires checksums.txt next to the asset.')
    }
    $manifestPath = $adjacentManifest
}
elseif ($DownloadStrategy -eq 'Cache') {
    if (-not (Test-Path -LiteralPath $archivePath) -or -not (Test-Path -LiteralPath $manifestPath)) {
        throw (Get-Text "已校验缓存不完整：$CacheDirectory" "Verified cache is incomplete: $CacheDirectory")
    }
}
else {
    if ($DownloadStrategy -eq 'Mirror' -and $BaseUrl -notmatch '^https://') {
        throw (Get-Text '镜像地址必须使用 HTTPS。' 'Mirror BaseUrl must use HTTPS.')
    }
    if ($DownloadStrategy -eq 'Proxy' -and -not $ProxyUrl) {
        throw (Get-Text '代理模式需要 -ProxyUrl。' 'Proxy strategy requires -ProxyUrl.')
    }
    $releaseBase = "$($BaseUrl.TrimEnd('/'))/$tag"
    Invoke-Download -Uri "$releaseBase/checksums.txt" -Destination $manifestPath
    Invoke-Download -Uri "$releaseBase/$assetName" -Destination $archivePath
}

$expectedHash = Get-ExpectedHash -Manifest $manifestPath -AssetName $assetName
Assert-Hash -Path $archivePath -Expected $expectedHash
Write-Host (Get-Text "已验证 $assetName（$expectedHash）" "Verified $assetName ($expectedHash)") -ForegroundColor Green

if (-not $PSCmdlet.ShouldProcess($InstallDirectory, "Install $assetName")) {
    return
}

if ($Desktop) {
    $arguments = '/CURRENTUSER', '/S'
    $process = Start-Process -FilePath $archivePath -ArgumentList $arguments -Wait -PassThru
    if ($process.ExitCode -ne 0) {
        throw (Get-Text "桌面安装器失败，退出码 $($process.ExitCode)。" "Desktop installer failed with exit code $($process.ExitCode)")
    }
    Write-Host (Get-Text 'SSH Launchpad 桌面版已安装。' 'SSH Launchpad desktop installed.') -ForegroundColor Green
    return
}

New-Item -ItemType Directory -Force -Path $InstallDirectory | Out-Null
$stage = Join-Path $CacheDirectory "extract-$Version-$architecture"
if (Test-Path -LiteralPath $stage) {
    Remove-Item -LiteralPath $stage -Recurse -Force
}
Assert-ZipEntriesSafe -Archive $archivePath -Destination $stage
Expand-Archive -LiteralPath $archivePath -DestinationPath $stage -Force
$binary = Get-ChildItem -LiteralPath $stage -Filter 'ssh-launchpad.exe' -Recurse | Select-Object -First 1
if (-not $binary) {
    throw (Get-Text 'Release 压缩包中没有 ssh-launchpad.exe。' 'Release archive did not contain ssh-launchpad.exe.')
}
Copy-Item -LiteralPath $binary.FullName -Destination (Join-Path $InstallDirectory 'ssh-launchpad.exe') -Force
$installed = Join-Path $InstallDirectory 'ssh-launchpad.exe'

if ($Run -ne 'None') {
    $arguments = @('--lang', $Language, $Run.ToLowerInvariant())
    if ($Profile) {
        $arguments += @('--profile', (Resolve-Path -LiteralPath $Profile).Path)
    }
    $arguments += @('--output', '-')
    & $installed @arguments
    if ($LASTEXITCODE -ne 0) {
        throw (Get-Text "ssh-launchpad $Run 未完成，退出码 $LASTEXITCODE。" "ssh-launchpad $Run failed with exit code $LASTEXITCODE")
    }
}

Write-Host (Get-Text "已安装：$installed" "Installed: $installed") -ForegroundColor Green
Write-Host (Get-Text "需要从终端调用时，可把此目录加入 PATH：$InstallDirectory" "Add this directory to PATH when desired: $InstallDirectory")
