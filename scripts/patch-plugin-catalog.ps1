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
    Write-Host "No app.asar found, skipping plugin catalog patch: $asarPath"
    return
}

$asarPath = (Resolve-Path $asarPath).Path
$encoding = [Text.Encoding]::UTF8

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

function Apply-BytePatch {
    param(
        [byte[]]$Bytes,
        [string]$NeedleText,
        [string]$ReplacementText,
        [string]$Label,
        [string[]]$AlreadyText = @()
    )

    $needle = $encoding.GetBytes($NeedleText)
    $replacement = $encoding.GetBytes($ReplacementText)

    if ($needle.Length -ne $replacement.Length) {
        throw "$Label patch must be length-preserving"
    }

    $matches = Find-NeedleIndexes -Haystack $Bytes -Needle $needle
    if ($matches.Count -eq 0) {
        foreach ($text in @($ReplacementText) + $AlreadyText) {
            $alreadyPatched = Find-NeedleIndexes -Haystack $Bytes -Needle ($encoding.GetBytes($text))
            if ($alreadyPatched.Count -gt 0) {
                Write-Host "Codex plugin catalog patch already applied ($Label): $asarPath"
                return $false
            }
        }

        throw "Could not find Codex plugin catalog gate ($Label) in app.asar"
    }

    if ($matches.Count -ne 1) {
        throw "Expected one Codex plugin catalog gate ($Label), found $($matches.Count)"
    }

    $offset = $matches[0]
    [Array]::Copy($replacement, 0, $Bytes, $offset, $replacement.Length)
    return $true
}

$bytes = [IO.File]::ReadAllBytes($asarPath)
$changed = $false

$changed = (Apply-BytePatch `
    -Bytes $bytes `
    -NeedleText 'return t&&!n&&e.length===0?null:n?[`local`,`vertical`,...e]:[`local`,...e]' `
    -ReplacementText 'return 0&&!n&&e.length===0?null:n?[`local`,`vertical`,...e]:[`local`,...e]' `
    -Label "remote-default" `
    -AlreadyText @('return 0&&!n&&e.length===0?null:0?[`local`,`vertical`,...e]:[`local`,...e]')) -or $changed

$changed = (Apply-BytePatch `
    -Bytes $bytes `
    -NeedleText 'n?[`local`,`vertical`,...e]:[`local`,...e]' `
    -ReplacementText '0?[`local`,`vertical`,...e]:[`local`,...e]' `
    -Label "vertical-catalog") -or $changed

if ($changed) {
    [IO.File]::WriteAllBytes($asarPath, $bytes)
    Write-Host "Patched Codex plugin catalog gates: $asarPath"
} else {
    $remotePatched = $encoding.GetBytes('return 0&&!n&&e.length===0?null:0?[`local`,`vertical`,...e]:[`local`,...e]')
    $alreadyPatched = Find-NeedleIndexes -Haystack $bytes -Needle $remotePatched
    if ($alreadyPatched.Count -gt 0) {
        Write-Host "Codex plugin catalog patches already applied: $asarPath"
    } else {
        Write-Host "Codex plugin catalog patch state unchanged: $asarPath"
    }
}
