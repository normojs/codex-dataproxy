$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force $dist | Out-Null

$goCommand = Get-Command go -ErrorAction SilentlyContinue
if ($goCommand) {
    $go = $goCommand.Source
}
elseif (Test-Path "C:\Program Files\Go\bin\go.exe") {
    $go = "C:\Program Files\Go\bin\go.exe"
}
else {
    throw "Go was not found. Install Go or add go.exe to PATH."
}

$pythonCommand = Get-Command python -ErrorAction SilentlyContinue
if (!$pythonCommand) {
    throw "Python was not found. Python with Pillow is required to generate the launcher icon."
}
$python = $pythonCommand.Source

Push-Location $root
try {
    & $python (Join-Path $PSScriptRoot "generate-icon.py")
    if ($LASTEXITCODE -ne 0) {
        throw "icon generation failed with exit code $LASTEXITCODE"
    }

    $goPath = (& $go env GOPATH).Trim()
    if ($LASTEXITCODE -ne 0 -or !$goPath) {
        throw "go env GOPATH failed with exit code $LASTEXITCODE"
    }

    $rsrc = Join-Path $goPath "bin\rsrc.exe"
    if (!(Test-Path $rsrc)) {
        $previousGoProxy = $env:GOPROXY
        $goProxies = @()
        if ($env:GOPROXY) {
            $goProxies += $env:GOPROXY
        }
        $goProxies += "https://goproxy.cn,direct"
        $goProxies += "direct"

        $installed = $false
        try {
            foreach ($proxy in ($goProxies | Select-Object -Unique)) {
                $env:GOPROXY = $proxy
                & $go install github.com/akavel/rsrc@latest
                if ($LASTEXITCODE -eq 0) {
                    $installed = $true
                    break
                }
            }
        }
        finally {
            $env:GOPROXY = $previousGoProxy
        }

        if (!$installed) {
            throw "go install github.com/akavel/rsrc@latest failed"
        }
    }

    & $rsrc -ico (Join-Path $root "assets\codex-dataproxy.ico") -o (Join-Path $root "rsrc_windows_amd64.syso")
    if ($LASTEXITCODE -ne 0) {
        throw "rsrc failed with exit code $LASTEXITCODE"
    }

    & $go mod tidy -go 1.26.0
    if ($LASTEXITCODE -ne 0) {
        throw "go mod tidy failed with exit code $LASTEXITCODE"
    }

    & $go build -trimpath -ldflags "-s -w" -o (Join-Path $dist "codex-dp.exe") .
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

Write-Host "Built: $dist\codex-dp.exe"
