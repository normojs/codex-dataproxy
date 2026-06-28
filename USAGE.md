# Codex DataProxy 简单使用教程

语言：[完整中文说明](README-zh.md) | [英文说明](README.md)

重要提示：如果你使用的国产模型官方接口不支持 Responses 协议，并且当前中转站也不支持把 Responses 协议转换为 Chat Completions 协议，请使用 `ccs + codex`；也可以使用 `dp.app.mbu.ltd` 中转站。`dp.app.mbu.ltd` 支持的是 GPT/OpenAI 兼容协议转换，不支持 Claude 协议转换。

## 1. 解压文件

将 `codex-dataproxy.zip` 解压到短路径，例如：

```text
C:\CodexDP
```

## 2. 启动程序

双击：

```text
codex-dp.exe
```

启动后不要关闭 cmd 窗口。

## 3. 打开 settings 页面

cmd 窗口会显示一个本地设置页面地址，例如：

```text
http://127.0.0.1:16666/settings
```

复制并打开这个地址。

## 4. 绑定 DataProxy 或配置中转站

在 settings 页面中：

1. 优先点击 `绑定 DataProxy`，在浏览器里登录并确认授权。
2. 授权成功后，程序会自动保存当前设备专用 Key。
3. 也可以手动查看默认的 DataProxy 中转站并填写 API Key。
4. 根据需要添加多个 Key。
5. 如果 Key 的 `models` 留空，会自动使用 `base_url + /models` 获取模型。
6. 如果手动填写 `models`，会覆盖自动获取结果。
7. 可用 `测试连接` 或每行 Key 的测试按钮检查配置。

同一个 `base_url` 可以配置多个 Key。`base_url` 是上游 API 根路径，是否包含 `/v1` 由中转站决定。Codex DataProxy 会根据当前使用的模型自动选择对应 Key。

手动模型支持通配符，例如 `qwen/*`。如果 Codex 里的模型名和上游模型名不同，可以在“模型别名”里按 `Codex模型名=上游模型名` 填写映射。

协议兼容说明：Codex DataProxy 不做 Responses 到 Chat Completions 的协议转换。Codex 目前只支持 Responses 协议；如果使用国产模型，需要模型官方接口直接支持 Responses 协议，或者中转站支持把 Responses 协议转换为 Chat Completions 协议。如果你使用的模型和中转站都不支持这个转换，请使用 `ccs + codex`；也可以使用 `dp.app.mbu.ltd` 中转站。`dp.app.mbu.ltd` 支持的是 GPT/OpenAI 兼容协议转换，不支持 Claude 协议转换。

## 5. 开启完全访问权限

进入 Codex 后，按下面路径设置：

```text
设置 -> 常规 -> 开启完全访问权限
```

如果 Codex 提示设置智能体权限，请选择“完全访问”。

不要右键选择“以管理员身份运行”。Codex 里的“完全访问”是应用内权限，不是 Windows 管理员权限。

## 6. 开始使用

回到 Codex 主界面，选择模型后即可开始使用。

如果模型列表为空，请检查：

- settings 页面里的 Key 是否填写正确
- 中转站是否支持 `base_url + /models`
- 中转站是否支持 `base_url + /responses`
- Key 行上的刷新状态和错误提示
- 模型路由区域是否提示重复路由或别名映射不正确

配置交流与问题反馈：QQ 交流群 `891855578`
