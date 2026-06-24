# Codex DataProxy 使用说明

Codex DataProxy 是一个 Windows 便携版启动包，用来启动 Codex Desktop，并通过本地代理接入一个或多个 OpenAI 兼容中转站。

## 快速使用

1. 将 `codex-dataproxy.zip` 解压到一个较短的目录，例如：

```text
C:\CodexDP
```

2. 双击启动：

```text
codex-dataproxy.exe
```

3. cmd 窗口会显示本地设置页面地址，例如：

```text
http://127.0.0.1:16666/settings
```

打开这个地址，在 settings 页面里配置中转站和 API Key。

4. 首次启动会自动创建默认配置：

```text
config\dataproxy.yaml
```

用户不需要手动编辑配置文件，平时通过 settings 页面添加、编辑、排序和切换中转站。

## 多 Key 模型路由

同一个中转站 `base_url` 可以配置多个 Key。不同 Key 支持的模型可能不同，Codex DataProxy 会根据请求中的 `model` 自动选择可用 Key。

规则：

- `models` 可以为空。
- 如果某个 Key 的 `models` 为空，则使用该 Key 的 `/v1/models` 自动获取模型。
- 如果某个 Key 的 `models` 不为空，则手动 `models` 覆盖自动获取结果。
- 多个 Key 的模型会合并去重。
- 如果多个 Key 都支持同一个模型，后面的 Key 覆盖前面的 Key。

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
```

这些文件不要手动修改。

## 权限与沙盒

进入 Codex 后，建议在：

```text
设置 -> 常规 -> 开启完全访问权限
```

如果 Codex 提示设置智能体权限，请选择“完全访问”。

不要右键选择“以管理员身份运行”。Codex 里的“完全访问”是应用内权限，不是 Windows 管理员权限。

## 重新打包

```powershell
.\scripts\build.ps1
.\scripts\package.ps1
```

输出文件：

```text
dist\codex-dataproxy.exe
dist\codex-dataproxy.zip
```

配置交流与问题反馈：QQ 交流群 `891855578`
