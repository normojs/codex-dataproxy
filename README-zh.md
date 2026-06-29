# Codex DataProxy 使用说明

语言：[英文说明](README.md)

重要提示：如果你使用的国产模型官方接口不支持 Responses 协议，并且当前中转站也不支持把 Responses 协议转换为 Chat Completions 协议，请使用 `ccs + codex`；也可以使用 `dp.app.mbu.ltd` 中转站。`dp.app.mbu.ltd` 支持的是 GPT/OpenAI 兼容协议转换，不支持 Claude 协议转换。

Codex DataProxy 是一个 Windows 便携版 Codex Desktop 启动包。它会使用独立的便携数据目录启动 Codex，并通过本地代理接入一个或多个 OpenAI 兼容中转站。

中国快速下载链接：https://www.modelscope.cn/models/sunlab-uninstall/uninstall-app

## 功能

- Windows 便携版 Codex Desktop 启动器。
- 本地 settings 页面：`http://127.0.0.1:<port>/settings`。
- 支持多个中转站配置，保存于 `config/*.yaml`。
- 支持同一个 `base_url` 下配置多个 API Key。
- 根据请求里的 `model` 自动选择可用 Key。
- 支持模型别名和通配符模型规则，适配上游不同命名。
- 每个 Key 可自动请求 `base_url + /models` 获取模型。
- 每个 Key 也可以手动填写模型列表。
- settings 页面支持删除、测试、排序中转站，也支持单独测试 Key。
- settings 页面显示重复模型路由和每个 Key 的刷新状态、错误、延迟。
- 支持浏览器授权绑定 DataProxy，避免手动复制 API Key。
- 本地 settings API 增加 CSRF 校验，接口返回的 API Key 会脱敏。
- 启动本地 Codex app-server，供后续桌面和手机联动能力使用。
- Codex 运行时文件统一生成到 `data/.codex`。

## 下载

GitHub Releases 推荐下载带版本号的压缩包：

```text
codex-dataproxy-v0.2.2-windows.zip
```

国内用户可使用中国快速下载链接：

```text
https://www.modelscope.cn/models/sunlab-uninstall/uninstall-app
```

## 快速使用

1. 下载并解压压缩包，建议放到较短目录，例如：

```text
C:\CodexDataProxy
```

2. 双击启动：

```text
codex-dp.exe
```

3. cmd 窗口会显示本地 settings 页面地址，例如：

```text
http://127.0.0.1:16666/settings
```

4. 在 settings 页面点击 `绑定 DataProxy` 通过浏览器授权，或手动添加中转站 `base_url` 和 API Key，然后保存。

5. 保存有效 Key 后，Codex Desktop 会继续启动，不需要重新打开程序。

## 默认模型

默认模型按以下顺序决定：

1. 当前中转站填写了 `default_model` 时，使用该模型。
2. `default_model` 为空时，使用模型合集里的第一个模型。
3. 模型合集来自 Key 手动填写的 `models`，或者该 Key 自动请求 `base_url + /models` 得到的模型。

settings 页面会显示当前实际默认模型。

## 多 Key 模型路由

同一个中转站 `base_url` 可以配置多个 Key。不同 Key 支持的模型可能不同，Codex DataProxy 会根据请求中的 `model` 自动选择可用 Key。

规则：

- `models` 可以为空。
- `base_url` 是上游 API 根路径；中转站需要 `/v1` 时才填写 `/v1`，不需要时不要补。
- 如果某个 Key 的 `models` 为空，则使用该 Key 的 `base_url + /models` 自动获取模型。
- 如果某个 Key 的 `models` 不为空，则手动 `models` 覆盖自动获取结果。
- 手动 `models` 支持通配符，例如 `qwen/*`。
- `model_aliases` 可以把 Codex 请求的模型名映射到上游模型名，例如 `gpt-5.5=upstream-model`。
- 多个 Key 的模型会合并去重。
- 如果多个 Key 都支持同一个模型，后面的 Key 覆盖前面的 Key。
- settings 页面会显示重复模型路由，方便判断最终走哪个 Key。
- 没有精确匹配模型时，使用默认 Key 或第一个可用 Key。

配置示例：

```yaml
model_aliases:
  gpt-5.5: upstream-model

keys:
  - id: main
    models: "upstream-model,qwen/*"
```

本地代理转发请求前会把别名模型改写成上游模型名。

## 协议兼容

Codex DataProxy 不做 Responses 到 Chat Completions 的协议转换。Codex 目前只支持 Responses 协议，因此使用国产模型时，需要模型官方接口直接支持 Responses 协议，或者由中转站把 Responses 协议转换为 Chat Completions 协议。

如果你使用的模型和中转站都不支持这个转换，请使用 `ccs + codex`；也可以使用 `dp.app.mbu.ltd` 中转站。`dp.app.mbu.ltd` 支持的是 GPT/OpenAI 兼容协议转换，不支持 Claude 协议转换。

## 本地代理

Codex Desktop 不直接访问真实中转站，而是固定访问本地代理：

```text
http://127.0.0.1:<port>/v1
```

真实中转站地址和 Key 保存在：

```text
config\*.yaml
```

启动器会自动生成 Codex 运行时文件：

```text
data\.codex\auth.json
data\.codex\config.toml
data\.codex\dataproxy-models.json
data\.codex\dataproxy-app-server.token
data\.codex\dataproxy-device-id
```

这些运行时文件不要手动修改。

settings 页面使用的本地 `/api/*` 接口只面向本机页面。非 GET 请求需要页面内置的 CSRF Token，接口返回配置时不会暴露完整 API Key。

## 权限与沙盒

不要右键选择“以管理员身份运行”。Codex 里的“完全访问”是应用内权限，不是 Windows 管理员权限。

进入 Codex 后，建议在：

```text
设置 -> 常规 -> 开启完全访问权限
```

如果 Codex 提示设置智能体权限，请选择“完全访问”。

## 重新打包

```powershell
.\scripts\build.ps1
.\scripts\package.ps1
```

输出文件：

```text
dist\codex-dp.exe
dist\codex-dataproxy-v0.2.2-windows.zip
dist\codex-dataproxy.zip
```

## GitHub 自动发布

仓库不会提交大体积的 `app/` 目录。要让 GitHub Actions 自动打完整
Windows 包，请先在仓库的 Actions variables 或 secrets 里配置：

```text
CODEX_DATAPROXY_BASE_ZIP_URL=https://.../codex-dataproxy-v0.2.1-windows.zip
```

这个 URL 必须指向一个包含 `app/Codex.exe` 的旧版完整 zip。推送
`v*.*.*` 标签后，workflow 会下载这个基底包，复用其中的 Codex Desktop
本体，重新构建 `codex-dp.exe`，打包 zip，上传 artifact，并发布 GitHub
Release。

```powershell
git tag v0.2.2
git push origin v0.2.2
```

配置交流与问题反馈：QQ 交流群 `891855578`
