# macOS Support Plan

目标：让 Codex DataProxy 的核心能力保持跨平台，平台差异只留在启动器和打包层。

## 跨平台核心

这些逻辑应继续保持纯 Go、无 Windows 依赖：

- 本地 settings HTTP server
- `/v1/*` 本地代理
- `config/*.yaml` 多中转站、多 Key、模型路由
- `/v1/models` 自动拉取与合并
- `data/.codex/auth.json`
- `data/.codex/config.toml`
- `data/.codex/dataproxy-models.json`

settings 页面不再使用 token，Windows 和 macOS 都使用：

```text
http://127.0.0.1:<port>/settings
```

## 平台层

后续 macOS 支持建议拆成这些平台接口：

- `platform_windows.go`
  - 启动 `app/Codex.exe`
  - 设置 `APPDATA`、`LOCALAPPDATA`、`CODEX_HOME`
  - 使用 Windows Job Object 关闭子进程
  - 管理员权限降级逻辑

- `platform_darwin.go`
  - 启动 `app/Codex.app/Contents/MacOS/Codex`
  - 设置 `CODEX_HOME`
  - 必要时设置独立 `HOME`，避免污染用户真实 `~/.codex`
  - 使用 process group 关闭 Codex 子进程
  - 使用 `open <settings-url>` 打开设置页

- `platform_other.go`
  - Linux/未知平台的保守 fallback

## 打包约定

Windows：

```text
codex-dataproxy.exe
app/Codex.exe
data/
config/
```

macOS 建议：

```text
codex-dataproxy
app/Codex.app/
data/
config/
```

## 需要在 macOS 上实测确认

- Codex Desktop macOS bundle 的真实可执行文件路径
- Codex Desktop 是否尊重 `CODEX_HOME`
- 是否需要额外处理 `HOME`、`XDG_*` 或 Electron 用户数据目录
- Codex Desktop 的模型列表 bundle 文件名是否与 Windows 包一致
- `app.asar` 补丁在 macOS 包内的实际路径

结论：设置页、代理、模型路由这些可以直接复用；macOS 主要工作是新增平台启动层和 macOS 包内 Codex.app 的补丁/复制流程。
