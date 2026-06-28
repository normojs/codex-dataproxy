# Changelog

## v0.2.1 - 2026-06-29

- Added DataProxy browser device authorization and automatic dedicated-key storage.
- Added provider delete, reorder, provider test, and per-key test actions to the settings page.
- Added per-key model refresh diagnostics, including status, HTTP status, model count, latency, refresh time, and error text.
- Added duplicate model route diagnostics in the settings page.
- Added `model_aliases` and wildcard model matching for upstream-specific model names.
- Added local proxy request model rewriting for aliases while preserving streaming responses.
- Added local proxy authorization and base URL endpoint validation.
- Added duplicate key ID normalization.
- Added API key redaction in settings API responses and preservation of saved secrets when submitting masked keys.
- Added Origin/Referer and CSRF checks for non-GET local settings API requests.
- Added regression tests for settings API safety, redaction, route diagnostics, alias rewrites, streaming proxy behavior, refresh failure tracking, and DataProxy device flow.
- Fixed stale auto-fetched model routes after key refresh failures or key changes.
- Fixed DataProxy connected-app rebinding when the server requires token rotation, and switched Windows packaging to `tar.exe` for more reliable large Electron zips.

## v0.2.0 - 2026-06-26

- Redesigned the local settings page with a cleaner console-style UI.
- Added explicit effective default model display.
- Persisted the first merged model as `default_model` when no default model is configured.
- Added local Codex app-server startup with loopback WebSocket and token-file authentication.
- Added app-server status to the settings page.
- Added versioned Windows zip packaging: `codex-dataproxy-v0.2.0-windows.zip`.
- Added English `README.md` and Chinese `README-zh.md`.
- Added China fast download link for ModelScope.

## v0.1.0 - 2026-06-22

- Initial portable Codex DataProxy launcher.
- Added local proxy support for OpenAI-compatible API gateways.
- Added multi-key model routing and dynamic model list support.
