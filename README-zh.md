# Codex DataProxy 使用说明

Codex DataProxy 是一个 Windows 便携版 Codex Desktop 启动包。它会使用独立的便携数据目录启动 Codex，并通过本地代理接入一个或多个 OpenAI 兼容中转站。

中国快速下载链接：https://www.modelscope.cn/models/sunlab-uninstall/uninstall-app

英文 README：[README.md](README.md)

## 功能

- Windows 便携版 Codex Desktop 启动器。
- 本地 settings 页面：`http://127.0.0.1:<port>/settings`。
- 支持多个中转站配置，保存于 `config/*.yaml`。
- 支持同一个 `base_url` 下配置多个 API Key。
- 根据请求里的 `model` 自动选择可用 Key。
- 每个 Key 可自动请求 `/v1/models` 获取模型。
- 每个 Key 也可以手动填写模型列表。
- 启动本地 Codex app-server，供后续桌面和手机联动能力使用。
- Codex 运行时文件统一生成到 `data/.codex`。

## 下载

GitHub Releases 推荐下载带版本号的压缩包：

```text
codex-dataproxy-v0.2.0-windows.zip
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
codex-dataproxy.exe
```

3. cmd 窗口会显示本地 settings 页面地址，例如：

```text
http://127.0.0.1:16666/settings
```

4. 在 settings 页面添加中转站 `base_url` 和 API Key，然后保存。

5. 保存有效 Key 后，Codex Desktop 会继续启动，不需要重新打开程序。

## 默认模型

默认模型按以下顺序决定：

1. 当前中转站填写了 `default_model` 时，使用该模型。
2. `default_model` 为空时，使用模型合集里的第一个模型。
3. 模型合集来自 Key 手动填写的 `models`，或者该 Key 自动请求 `/v1/models` 得到的模型。

settings 页面会显示当前实际默认模型。

## 多 Key 模型路由

同一个中转站 `base_url` 可以配置多个 Key。不同 Key 支持的模型可能不同，Codex DataProxy 会根据请求中的 `model` 自动选择可用 Key。

规则：

- `models` 可以为空。
- 如果某个 Key 的 `models` 为空，则使用该 Key 的 `/v1/models` 自动获取模型。
- 如果某个 Key 的 `models` 不为空，则手动 `models` 覆盖自动获取结果。
- 多个 Key 的模型会合并去重。
- 如果多个 Key 都支持同一个模型，后面的 Key 覆盖前面的 Key。
- 没有精确匹配模型时，使用默认 Key 或第一个可用 Key。

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
```

这些运行时文件不要手动修改。

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
dist\codex-dataproxy.exe
dist\codex-dataproxy-v0.2.0-windows.zip
dist\codex-dataproxy.zip
```

配置交流与问题反馈：QQ 交流群 `891855578`
