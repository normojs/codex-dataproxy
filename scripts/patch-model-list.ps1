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
    Write-Host "No app.asar found, skipping dynamic model list patch: $asarPath"
    return
}

$asarPath = (Resolve-Path $asarPath).Path
$unpackedPath = "$asarPath.unpacked"
$backupPath = "$asarPath.dataproxy.bak"
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("codex-dataproxy-asar-" + [Guid]::NewGuid().ToString("N"))
$encoding = [Text.UTF8Encoding]::new($false)

function Invoke-Asar {
    param([string[]]$Arguments)

    & npx --yes @electron/asar @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "asar command failed: $($Arguments -join ' ')"
    }
}

try {
    New-Item -ItemType Directory -Force $tempRoot | Out-Null
    Invoke-Asar @("extract", $asarPath, $tempRoot)

    $assetsDir = Join-Path $tempRoot "webview\assets"
    $appMain = Get-ChildItem $assetsDir -Filter "app-main-*.js" | Select-Object -First 1
    if ($null -eq $appMain) {
        throw "Cannot find app-main bundle in app.asar"
    }

    $oldHandler = '"list-models-for-host":oU((e,t)=>e.sendRequest(`model/list`,t)),'
    $newHandler = '"list-models-for-host":aU(async(e,t,n)=>{try{let r=e.getHostId(),{codexHome:i}=await n.fetchFromHost(`codex-home`,{params:{hostId:r}});return JSON.parse((await n.fetchFromHost(`read-file`,{params:{hostId:r,path:La(i,`dataproxy-models.json`)}})).contents)}catch{return e.sendRequest(`model/list`,t)}}),'
    $marker = "dataproxy-models.json"

    $appMainText = [IO.File]::ReadAllText($appMain.FullName, $encoding)
    if ($appMainText.Contains($marker)) {
        Write-Host "Dynamic model RPC patch already present: $($appMain.Name)"
    } elseif ($appMainText.Contains($oldHandler)) {
        $appMainText = $appMainText.Replace($oldHandler, $newHandler)
        [IO.File]::WriteAllText($appMain.FullName, $appMainText, $encoding)
        Write-Host "Patched dynamic model RPC: $($appMain.Name)"
    } else {
        throw "Cannot find Codex list-models-for-host handler"
    }

    $modelFilter = Get-ChildItem $assetsDir -Filter "model-list-filter-*.js" | Select-Object -First 1
    if ($null -eq $modelFilter) {
        throw "Cannot find model-list-filter bundle in app.asar"
    }

    $filterText = 'function e({authMethod:e,availableModels:t,defaultModel:n,models:r,useHiddenModels:i}){let a=[],o=null,s=i&&e!==`amazonBedrock`;return r.forEach(r=>{if(s?t.has(r.model):!r.hidden){let i=e===`copilot`?[r.supportedReasoningEfforts.find(e=>e.reasoningEffort===`medium`)??{reasoningEffort:`medium`,description:`medium effort`}]:[...r.supportedReasoningEfforts];a.push({...r,supportedReasoningEfforts:i}),r.isDefault&&(o??=r)}}),o??=a.find(e=>e.model===n)??a[0]??null,{models:a,defaultModel:o}}export{e as t};' + [Environment]::NewLine + '//# sourceMappingURL=' + $modelFilter.Name + '.map'
    [IO.File]::WriteAllText($modelFilter.FullName, $filterText, $encoding)
    Write-Host "Patched model list filter: $($modelFilter.Name)"

    $modelQueries = Get-ChildItem $assetsDir -Filter "model-queries-*.js" | Select-Object -First 1
    if ($null -eq $modelQueries) {
        throw "Cannot find model-queries bundle in app.asar"
    }

    $modelQueriesText = [IO.File]::ReadAllText($modelQueries.FullName, $encoding)
    $hotLoadMarker = "refetchInterval:3000"
    if ($modelQueriesText.Contains($hotLoadMarker)) {
        Write-Host "Dynamic model hot-load patch already present: $($modelQueries.Name)"
    } else {
        $stalePattern = 'staleTime:[A-Za-z_$][A-Za-z0-9_$]*\.FIVE_MINUTES,queryFn:'
        $staleMatch = [regex]::Match($modelQueriesText, $stalePattern)
        if (!$staleMatch.Success) {
            throw "Cannot find Codex model query staleTime setting"
        }
        $replacement = 'staleTime:0,refetchOnMount:"always",refetchOnWindowFocus:"always",refetchInterval:3000,refetchIntervalInBackground:!1,queryFn:'
        $modelQueriesText = $modelQueriesText.Substring(0, $staleMatch.Index) + $replacement + $modelQueriesText.Substring($staleMatch.Index + $staleMatch.Length)
        [IO.File]::WriteAllText($modelQueries.FullName, $modelQueriesText, $encoding)
        Write-Host "Patched dynamic model hot-load query: $($modelQueries.Name)"
    }

    Copy-Item $asarPath $backupPath -Force
    if (Test-Path $unpackedPath) {
        Remove-Item $unpackedPath -Recurse -Force
    }

	Invoke-Asar @(
		"pack",
		"--unpack", "**/*.node",
		"--unpack-dir", "{node_modules/better-sqlite3,node_modules/node-pty/build,node_modules/node-pty/lib}",
		$tempRoot,
		$asarPath
	)

    Remove-Item $backupPath -Force
    Write-Host "Repacked Codex app.asar with dynamic model support: $asarPath"
} catch {
    if ((Test-Path $backupPath) -and !(Test-Path $asarPath)) {
        Move-Item $backupPath $asarPath -Force
    } elseif (Test-Path $backupPath) {
        Copy-Item $backupPath $asarPath -Force
    }
    throw
} finally {
    if (Test-Path $tempRoot) {
        Remove-Item $tempRoot -Recurse -Force
    }
}
