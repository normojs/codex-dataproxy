param(
    [string]$AppDir = ""
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
if ($AppDir -eq "") {
    $AppDir = Join-Path $root "app"
}

$asarPath = Join-Path $AppDir "resources\app.asar"
if (!(Test-Path $asarPath)) {
    Write-Host "No app.asar found, skipping settings search patch: $asarPath"
    return
}

$asarPath = (Resolve-Path $asarPath).Path
$encoding = [Text.Encoding]::UTF8
$needle = $encoding.GetBytes('C=te&&g&&!S')
$replacement = $encoding.GetBytes('C=!0&&g&&!S')

if ($needle.Length -ne $replacement.Length) {
    throw "settings search patch must be length-preserving"
}

function Find-NeedleIndexes {
    param(
        [byte[]]$Haystack,
        [byte[]]$Needle
    )

    $indexes = New-Object System.Collections.Generic.List[int]
    $start = 0
    while ($start -lt $Haystack.Length) {
        $idx = [Array]::IndexOf($Haystack, $Needle[0], $start)
        if ($idx -lt 0) {
            break
        }

        if ($idx + $Needle.Length -le $Haystack.Length) {
            $matched = $true
            for ($i = 1; $i -lt $Needle.Length; $i++) {
                if ($Haystack[$idx + $i] -ne $Needle[$i]) {
                    $matched = $false
                    break
                }
            }

            if ($matched) {
                $indexes.Add($idx)
                $start = $idx + $Needle.Length
                continue
            }
        }

        $start = $idx + 1
    }

    return $indexes
}

$bytes = [IO.File]::ReadAllBytes($asarPath)
$matches = Find-NeedleIndexes -Haystack $bytes -Needle $needle
if ($matches.Count -eq 0) {
    $alreadyPatched = Find-NeedleIndexes -Haystack $bytes -Needle $replacement
    if ($alreadyPatched.Count -gt 0) {
        Write-Host "Codex settings search patch already applied: $asarPath"
        return
    }

    throw "Could not find Codex settings search gate in app.asar"
}

if ($matches.Count -ne 1) {
    throw "Expected one Codex settings search gate, found $($matches.Count)"
}

$offset = $matches[0]
[Array]::Copy($replacement, 0, $bytes, $offset, $replacement.Length)
[IO.File]::WriteAllBytes($asarPath, $bytes)

Write-Host "Patched Codex settings search gate: $asarPath"
