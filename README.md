# Codex DataProxy

Codex DataProxy is a portable Windows launcher for Codex Desktop. It starts Codex with an isolated portable data directory and routes Codex API traffic through a local proxy so you can use one or more OpenAI-compatible API gateways.

China fast download: https://www.modelscope.cn/models/sunlab-uninstall/uninstall-app

Chinese README: [README-zh.md](README-zh.md)

## Features

- Portable Codex Desktop package for Windows.
- Local settings page at `http://127.0.0.1:<port>/settings`.
- Multiple gateway profiles in `config/*.yaml`.
- Multiple API keys under the same `base_url`.
- Model-to-key routing based on the requested `model`.
- Automatic `/v1/models` fetching per key.
- Manual model override per key.
- Local-only Codex app-server startup for future desktop/mobile integrations.
- Generated Codex runtime files under `data/.codex`.

## Download

Use the versioned archive from GitHub Releases:

```text
codex-dataproxy-v0.2.0-windows.zip
```

For users in China, use the faster mirror:

```text
https://www.modelscope.cn/models/sunlab-uninstall/uninstall-app
```

## Quick Start

1. Download and extract the zip to a short path, for example:

```text
C:\CodexDataProxy
```

2. Run:

```text
codex-dataproxy.exe
```

3. Open the settings URL printed in the console:

```text
http://127.0.0.1:16666/settings
```

4. Add your gateway `base_url` and API key, then save.

5. Codex Desktop will continue launching after a valid key is saved.

## Model Routing

Codex Desktop talks only to the local proxy:

```text
http://127.0.0.1:<port>/v1
```

The real upstream API key is injected by Codex DataProxy. For each enabled key:

- If `models` is empty, Codex DataProxy fetches models from `/v1/models`.
- If `models` is set, the manual list overrides the fetched list.
- Models from all enabled keys are merged.
- Duplicate model IDs are resolved by later keys overriding earlier keys.
- The default model is the provider `default_model`, or the first merged model.

## Generated Files

Do not edit these files manually:

```text
data\.codex\auth.json
data\.codex\config.toml
data\.codex\dataproxy-models.json
data\.codex\dataproxy-app-server.token
```

Gateway configuration is stored in:

```text
config\*.yaml
```

You can manage it from the settings page instead of editing YAML by hand.

## Permissions

Do not run `codex-dataproxy.exe` as Windows administrator. In Codex Desktop, use the in-app permission setting instead.

Recommended Codex setting:

```text
Settings -> General -> Enable full access
```

## Build

```powershell
.\scripts\build.ps1
.\scripts\package.ps1
```

Outputs:

```text
dist\codex-dataproxy.exe
dist\codex-dataproxy-v0.2.0-windows.zip
dist\codex-dataproxy.zip
```
