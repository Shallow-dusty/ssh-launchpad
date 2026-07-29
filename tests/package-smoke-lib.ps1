function Get-ArchiveEntryForManifestTarget {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [System.Collections.IEnumerable]$Entries,
        [Parameter(Mandatory)]
        [string]$ManifestFullName,
        [Parameter(Mandatory)]
        [string]$ManifestTarget
    )

    $manifestPath = $ManifestFullName.Replace('\', '/')
    $targetPath = $ManifestTarget.Replace('\', '/')
    if (
        $targetPath.StartsWith('/') -or
        $targetPath -match '^[A-Za-z]:' -or
        $targetPath -match '(^|/)\.\.(/|$)'
    ) {
        return $null
    }

    $separator = $manifestPath.LastIndexOf('/')
    $manifestDirectory = if ($separator -ge 0) {
        $manifestPath.Substring(0, $separator)
    }
    else {
        ''
    }
    $expectedPath = if ($manifestDirectory) {
        "$manifestDirectory/$targetPath"
    }
    else {
        $targetPath
    }

    return $Entries |
        Where-Object { $_.FullName.Replace('\', '/') -eq $expectedPath } |
        Select-Object -First 1
}
