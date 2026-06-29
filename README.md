# Codex DataProxy

语言：[中文说明](README-zh.md)

Important: if a domestic model endpoint does not support the Responses API directly, and your gateway does not convert Responses requests to Chat Completions, use `ccs + codex` instead. You can also use the `dp.app.mbu.ltd` gateway; it supports GPT/OpenAI-compatible protocol conversion, but does not support Claude protocol conversion.

Codex DataProxy is a portable Windows launcher for Codex Desktop. It starts Codex with an isolated portable data directory and routes Codex API traffic through a local proxy so you can use one or more OpenAI-compatible API gateways.

China fast download: https://www.modelscope.cn/models/sunlab-uninstall/uninstall-app

## Features

- Portable Codex Desktop package for Windows.
- Local settings page at `http://127.0.0.1:<port>/settings`.
- Multiple gateway profiles in `config/*.yaml`.
- Multiple API keys under the same `base_url`.
- Model-to-key routing based on the requested `model`.
- Model aliases and wildcard model patterns for upstream-specific names.
- Automatic `base_url + /models` fetching per key.
- Manual model override per key.
- Settings page provider delete, test, reorder, and per-key test actions.
- Route diagnostics for duplicate models and per-key refresh status.
- Browser-based DataProxy binding so users do not have to copy a key manually.
- Local settings API CSRF checks and redacted API keys in responses.
- Local-only Codex app-server startup for future desktop/mobile integrations.
- Generated Codex runtime files under `data/.codex`.

## Download

Use the versioned archive from GitHub Releases:

```text
codex-dataproxy-v0.2.2-windows.zip
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
codex-dp.exe
```

3. Open the settings URL printed in the console:

```text
http://127.0.0.1:16666/settings
```

4. Click `绑定 DataProxy` to authorize in the browser, or add your gateway `base_url` and API key manually, then save.

5. Codex Desktop will continue launching after a valid key is saved.

## Model Routing

Codex Desktop talks only to the local proxy:

```text
http://127.0.0.1:<port>/v1
```

The real upstream API key is injected by Codex DataProxy. For each enabled key:

- `base_url` is the upstream API root. Include `/v1` only when your gateway requires it.
- If `models` is empty, Codex DataProxy fetches models from `base_url + /models`.
- If `models` is set, the manual list overrides the fetched list.
- Manual models may include wildcard patterns such as `qwen/*`.
- `model_aliases` maps the model Codex asks for to the upstream model name, for example `gpt-5.5=upstream-model`.
- Models from all enabled keys are merged.
- Duplicate model IDs are resolved by later keys overriding earlier keys.
- Duplicate route diagnostics are shown on the settings page.
- The default model is the provider `default_model`, or the first merged model.

Example provider fragment:

```yaml
model_aliases:
  gpt-5.5: upstream-model

keys:
  - id: main
    models: "upstream-model,qwen/*"
```

The local proxy rewrites aliased request bodies before forwarding them upstream.

## Protocol Compatibility

Codex DataProxy does not convert Responses requests to Chat Completions. Codex currently uses the Responses API only. For Chinese domestic models, the official model endpoint must support the Responses API directly, or the gateway must convert Responses requests to Chat Completions.

If the model endpoint and gateway you use do not support this conversion, use `ccs + codex`, or use the `dp.app.mbu.ltd` gateway. The `dp.app.mbu.ltd` conversion is for GPT/OpenAI-compatible protocols only; Claude protocol conversion is not supported.

## Generated Files

Do not edit these files manually:

```text
data\.codex\auth.json
data\.codex\config.toml
data\.codex\dataproxy-models.json
data\.codex\dataproxy-app-server.token
data\.codex\dataproxy-device-id
```

Gateway configuration is stored in:

```text
config\*.yaml
```

You can manage it from the settings page instead of editing YAML by hand.

The settings API is intended for the local settings page only. Non-GET `/api/*`
requests require the page CSRF token, and saved API keys are redacted in JSON
responses.

## Permissions

Do not run `codex-dp.exe` as Windows administrator. In Codex Desktop, use the in-app permission setting instead.

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
dist\codex-dp.exe
dist\codex-dataproxy-v0.2.2-windows.zip
dist\codex-dataproxy.zip
```

## GitHub Release CI

The repository does not commit the bundled `app/` directory. To let GitHub Actions
build a full Windows package, configure one repository variable or secret:

```text
CODEX_DATAPROXY_BASE_ZIP_URL=https://.../codex-dataproxy-v0.2.1-windows.zip
```

The URL must point to a previous full package zip that contains `app/Codex.exe`.
On `v*.*.*` tags, the workflow downloads that base package, reuses its bundled
Codex Desktop app, rebuilds `codex-dp.exe`, packages the zip, uploads the
artifact, and publishes a GitHub Release.

```powershell
git tag v0.2.2
git push origin v0.2.2
```
