$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
$stage = Join-Path $dist "codex-dataproxy"
$zip = Join-Path $dist "codex-dataproxy.zip"

if (Test-Path $stage) {
    Remove-Item $stage -Recurse -Force
}
New-Item -ItemType Directory -Force $stage | Out-Null

Copy-Item (Join-Path $dist "codex-dataproxy.exe") $stage -Force
Copy-Item (Join-Path $root "portapp.json") $stage -Force
Copy-Item (Join-Path $root "README.md") $stage -Force
Copy-Item (Join-Path $root "USAGE.md") $stage -Force

$appDir = Join-Path $root "app"
if (!(Test-Path $appDir)) {
    New-Item -ItemType Directory -Force $appDir | Out-Null
    New-Item -ItemType File -Force (Join-Path $appDir ".put-codex-files-here") | Out-Null
}
Copy-Item $appDir (Join-Path $stage "app") -Recurse -Force

& (Join-Path $PSScriptRoot "patch-i18n.ps1") -AppDir (Join-Path $stage "app")
& (Join-Path $PSScriptRoot "patch-plugin-catalog.ps1") -AppDir (Join-Path $stage "app")
& (Join-Path $PSScriptRoot "patch-settings-search.ps1") -AppDir (Join-Path $stage "app")
& (Join-Path $PSScriptRoot "patch-model-list.ps1") -AppDir (Join-Path $stage "app")

New-Item -ItemType Directory -Force (Join-Path $stage "data\.codex") | Out-Null

if (Test-Path $zip) {
    Remove-Item $zip -Force
}
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip -Force

Write-Host "Packaged: $zip"
