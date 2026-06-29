$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
$stage = Join-Path $dist "codex-dp"
$legacyStage = Join-Path $dist "codex-dataproxy"
$portapp = Get-Content (Join-Path $root "portapp.json") -Raw | ConvertFrom-Json
$version = $portapp.version
$zip = Join-Path $dist ("codex-dp-v{0}-windows.zip" -f $version)
$latestZip = Join-Path $dist "codex-dp.zip"

if (Test-Path $stage) {
    Remove-Item $stage -Recurse -Force
}
if (Test-Path $legacyStage) {
    Remove-Item $legacyStage -Recurse -Force
}
New-Item -ItemType Directory -Force $stage | Out-Null

Copy-Item (Join-Path $dist "codex-dp.exe") $stage -Force
Copy-Item (Join-Path $root "portapp.json") $stage -Force
Copy-Item (Join-Path $root "README.md") $stage -Force
Copy-Item (Join-Path $root "README-zh.md") $stage -Force
Copy-Item (Join-Path $root "CHANGELOG.md") $stage -Force
Copy-Item (Join-Path $root "USAGE.md") $stage -Force
@"
Codex DataProxy
Version: $version
Build date: $(Get-Date -Format "yyyy-MM-dd")
"@ | Set-Content (Join-Path $stage "VERSION.txt") -Encoding UTF8

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
if (Test-Path $latestZip) {
    Remove-Item $latestZip -Force
}
$legacyLatestZip = Join-Path $dist "codex-dataproxy.zip"
if (Test-Path $legacyLatestZip) {
    Remove-Item $legacyLatestZip -Force
}
Get-ChildItem -Path $dist -Filter "codex-dataproxy-v*-windows.zip" -File -ErrorAction SilentlyContinue |
    Remove-Item -Force

$tarCommand = Get-Command tar.exe -ErrorAction SilentlyContinue
if (!$tarCommand) {
    throw "tar.exe was not found. It is required to package the Electron app reliably."
}

Push-Location $stage
try {
    & $tarCommand.Source -a -cf $zip *
    if ($LASTEXITCODE -ne 0) {
        throw "tar.exe packaging failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

Copy-Item $zip $latestZip -Force

Write-Host "Packaged: $zip"
Write-Host "Latest copy: $latestZip"
