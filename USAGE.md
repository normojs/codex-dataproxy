# Codex DataProxy 简单使用教程

## 1. 解压文件

将 `codex-dataproxy.zip` 解压到短路径，例如：

```text
C:\CodexDP
```

## 2. 启动程序

双击：

```text
codex-dataproxy.exe
```

启动后不要关闭 cmd 窗口。

## 3. 打开 settings 页面

cmd 窗口会显示一个本地设置页面地址，例如：

```text
http://127.0.0.1:16666/settings
```

复制并打开这个地址。

## 4. 配置中转站和 Key

在 settings 页面中：

1. 查看默认的 DataProxy 中转站。
2. 填写 API Key。
3. 根据需要添加多个 Key。
4. 如果 Key 的 `models` 留空，会自动使用 `/v1/models` 获取模型。
5. 如果手动填写 `models`，会覆盖自动获取结果。

同一个 `base_url` 可以配置多个 Key。Codex DataProxy 会根据当前使用的模型自动选择对应 Key。

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
- 中转站是否支持 `/v1/models`
- 中转站是否支持 `/v1/responses`

配置交流与问题反馈：QQ 交流群 `891855578`
