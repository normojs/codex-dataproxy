# Codex DataProxy Settings 开发方案

## 目标

Codex DataProxy 后续不再要求用户手动编辑 `codex-dataproxy.yml`。用户通过本地 `/settings` 页面添加、编辑、排序和切换中转站。Codex Desktop 始终连接本地代理地址，由启动器负责把请求转发到当前选中的中转站。

核心目标：

- 支持多个中转站配置。
- 支持同一个 `base_url` 下配置多个 Key，并按请求模型自动选择 Key。
- 支持 settings 页面新增、编辑、删除、排序和切换中转站。
- Codex Desktop 不直接保存真实中转站 Key。
- 切换中转站后，新请求无需重启 Codex Desktop 即可生效。
- UI 风格参考 OpenAI 官网的浅色、干净、现代方向，同时保留 Codex Desktop 的工具型信息密度。

## 架构

```text
Codex Desktop
  |
  | http://127.0.0.1:<port>/v1/responses
  v
Codex DataProxy local server
  |-- /v1/*        转发到当前 active provider
  |-- /settings    本地设置页面
  |-- /api/*       设置页面使用的 JSON API
  v
Active upstream endpoint + matched key
```

Codex 生成配置时只写本地代理：

```toml
model_provider = "dataproxy-local"

[model_providers.dataproxy-local]
name = "Codex DataProxy"
base_url = "http://127.0.0.1:16666/v1"
requires_openai_auth = true
wire_api = "responses"
```

`auth.json` 只写本地占位 Key：

```json
{
  "OPENAI_API_KEY": "codex-dataproxy-local"
}
```

真实中转站 Key 只保存在 `config/*.yaml`，由本地代理根据请求中的 `model` 选择合适 Key 并注入到上游请求。

## 配置目录

推荐目录：

```text
config\
  dataproxy.yaml
  backup.yaml
```

每个中转站一个 YAML 文件。文件名只用于存储，真实身份以 `id` 为准。

```yaml
id: "dataproxy"
name: "DataProxy"
active: true
enabled: true
sort: 10

base_url: "https://dp.app.mbu.ltd/v1"

default_model: "gpt-5.5"

keys:
  - id: "gpt-main"
    name: "GPT 主 Key"
    api_key: "sk-xx"
    enabled: true
    sort: 10
    default: true
    models: "gpt-5.5,gpt-5.1"

  - id: "deepseek"
    name: "DeepSeek Key"
    api_key: "sk-xx"
    enabled: true
    sort: 20
    models: "deepseek-ai/DeepSeek-V4-Flash,deepseek-chat"
```

字段说明：

- `id`：稳定 ID，设置页生成，不能重复。
- `name`：页面显示名，可中文。
- `active`：当前使用的中转站，只允许一个为 `true`。
- `enabled`：是否启用。
- `sort`：设置页排序值。
- `base_url`：OpenAI 兼容地址。
- `default_model`：该中转站默认模型。
- `keys`：同一个中转站地址下的多个 Key。
- `keys[].api_key`：真实 Key，页面展示时默认脱敏。
- `keys[].models`：手动模型列表，英文逗号分隔；可以为空。
- `keys[].default`：没有精确匹配模型时的兜底 Key。

首次启动时，如果 `config/` 不存在，启动器自动创建：

```text
config\dataproxy.yaml
```

默认内容使用 DataProxy 地址、一个名为 `GPT 主 Key` 的默认 Key、`api_key: "sk-xx"`、`default_model: "gpt-5.5"`。

## 模型到 Key 的路由

同一个 `base_url` 下，不同 Key 可能拥有不同模型权限。因此本地代理不能只按中转站切换，还要按请求模型选择 Key。

请求示例：

```json
{
  "model": "deepseek-ai/DeepSeek-V4-Flash",
  "input": "..."
}
```

选择规则：

1. 读取请求 JSON 顶层 `model`。
2. 先计算 active provider 下每个 Key 的有效模型列表：手动 `models` 优先，否则使用该 Key 的 `/v1/models` 自动结果。
3. 在启用状态的 Key 中查找有效模型列表包含该模型的 Key。
4. 如果多个 Key 都支持该模型，按排序后的合并结果选择最后覆盖生效的 Key，并在 settings 页面显示重复模型提示。
5. 如果没有精确匹配，使用 `default: true` 的 Key。
6. 如果没有默认 Key，返回本地错误，提示该模型没有可用 Key。

模型来源：

- `keys[].models` 是手动配置，可以为空。
- 如果 `keys[].models` 为空，启动器使用该 Key 请求上游 `/v1/models` 自动获取模型。
- 如果 `keys[].models` 不为空，手动配置覆盖该 Key 自动获取到的模型。

模型合集处理：

- 对 active provider 下每个启用 Key 分别请求上游 `/v1/models`。
- 先得到每个 Key 的有效模型列表：手动 `models` 优先，否则使用 `/v1/models` 自动结果。
- 合并所有 Key 的有效模型列表并去重。
- 如果多个 Key 都支持同一个模型，后面的 Key 覆盖前面的 Key。
- 保存模型到本地 catalog 时，同时记录 `model -> key_id` 的推荐路由。
- settings 页面显示每个模型来自哪个 Key。

初版模型匹配使用精确 ID。后续可以扩展：

```yaml
models: "deepseek-ai/*,qwen/*"
```

用于通配模型系列。

## 启动流程

1. 启动器创建或读取 `config/*.yaml`。
2. 启动本地 HTTP server，只监听 `127.0.0.1`。
3. settings 页面仅监听本机 localhost，不使用页面 token。
4. 生成 Codex 的 `data\.codex\auth.json` 和 `config.toml`，base_url 指向本地代理。
5. 如果没有可用 active provider，cmd 显示设置页面地址，不启动 Codex Desktop。
6. 如果 active provider 至少有一个真实 Key，启动 Codex Desktop。
7. cmd 持续显示本地设置页地址。

## 热切换行为

切换中转站时：

- 设置页调用 `POST /api/providers/{id}/activate`。
- 服务端把该 provider 设为 active，其他 provider 设为 inactive。
- 内存状态立即更新。
- 新的 Codex 请求马上走新的 active provider。
- 新请求会根据 `model` 自动选择 active provider 下匹配的 Key。
- 正在进行的流式请求不打断，等下一次请求生效。
- 同步刷新 `data\.codex\dataproxy-models.json`。

边界：

- 如果 Codex 当前选中的模型在新 provider 的任何 Key 中都不存在，本地代理返回明确错误。
- 后续可以加 `model_aliases`，把 Codex 传入的模型自动映射到 active provider 的默认模型。
- 初版不做 Responses 到 Chat Completions 协议转换，仍由中转站支持。

## 本地 HTTP API

所有 `/api/*` 请求仅接受本机 settings 页面访问，不再要求页面 token：

```text
GET /api/providers
```

页面入口：

```text
GET /settings
```

API：

```text
GET    /api/providers
POST   /api/providers
GET    /api/providers/{id}
PUT    /api/providers/{id}
DELETE /api/providers/{id}
POST   /api/providers/{id}/activate
POST   /api/providers/{id}/test
POST   /api/providers/{id}/keys
PUT    /api/providers/{id}/keys/{key_id}
DELETE /api/providers/{id}/keys/{key_id}
POST   /api/providers/{id}/keys/{key_id}/test
POST   /api/providers/reorder
POST   /api/models/refresh
```

`POST /api/providers/{id}/keys/{key_id}/test` 行为：

- 检查 `base_url` 是否可解析。
- 使用该 Key 请求 `/v1/models`。
- 返回延迟、HTTP 状态、模型数量和错误摘要。
- 不把完整 Key 写入日志。

代理转发：

```text
ANY /v1/*
```

转发规则：

- 目标地址为 active provider 的 `base_url + 原始 /v1 后缀`。
- 从请求 JSON 中读取 `model`。
- 根据 `model` 选择 active provider 下匹配的 Key。
- 覆盖 `Authorization: Bearer <matched.api_key>`。
- 复制 Content-Type、Accept、OpenAI 相关 header。
- 过滤 hop-by-hop header。
- 流式响应直接透传。

## Settings UI 设计

### 视觉方向

风格关键词：

- 参考 OpenAI 官网浅色、留白、清晰卡片和克制排版
- Codex Desktop 工具型一致感
- 浅色中性背景、细边框、低饱和强调色
- 局部使用青绿色和蓝色表达代理、路由和在线状态
- 工具型、紧凑、清晰，不做营销页
- 不使用大面积渐变、装饰图形或夸张卡片

建议色板：

```text
background: #f7f7f4
surface:    #ffffff
surface-2:  #f1f3f5
border:     #d9dde3
text:       #111318
muted:      #667085
accent:     #10a37f
accent-2:   #2563eb
success:    #0f9f6e
warning:    #b7791f
danger:     #d92d20
```

### 页面结构

```text
┌──────────────────────────────────────────────────────────────┐
│ Codex DataProxy Settings              当前：DataProxy / GPT主Key │
├───────────────┬──────────────────────────────────────────────┤
│ Provider List │ Provider Detail                              │
│               │                                              │
│ + 新增        │ 名称             [ DataProxy              ]   │
│               │ Base URL         [ https://.../v1         ]   │
│ ● DataProxy   │ 默认模型         [ gpt-5.5               ]   │
│   2 keys      │                                              │
│   38 models   │ Key 路由表                                      │
│               │ GPT 主 Key     gpt-5.5,gpt-5.1       [编辑] │
│               │ DeepSeek Key   deepseek-ai/...       [编辑] │
│               │                                              │
│ ○ Backup      │ [测试连接] [拉取模型] [保存] [设为当前]       │
│   未配置 Key  │                                              │
├───────────────┴──────────────────────────────────────────────┤
│ 状态栏：本地代理运行中 127.0.0.1:16666 | /v1/responses        │
└──────────────────────────────────────────────────────────────┘
```

### 左侧中转站列表

列表项内容：

- active 状态点。
- 中转站名称。
- 默认模型。
- Key 数量。
- 模型数量。
- Key 状态：全部可用 / 部分未配置 / 未配置。
- 连接状态：未测试 / 正常 / 失败。

操作：

- 新增按钮。
- 上移 / 下移排序按钮。
- 删除按钮放入更多菜单。
- 点击列表项切换右侧详情，不立即切 active。

### 右侧详情表单

字段：

- 名称
- Base URL
- 默认模型
- Enabled 开关
- Key 路由表

控件：

- Base URL 旁边显示 `/v1/models` 测试结果。
- Key 路由表支持新增、编辑、删除、排序。
- API Key 默认密码框，右侧眼睛按钮显示/隐藏。
- 每个 Key 的模型列表用单行输入，支持英文逗号。
- 拉取模型后显示每个 Key 的模型数量和前几个模型名。

按钮：

- `测试连接`
- `拉取模型`
- `保存`
- `设为当前`
- `新增 Key`
- `删除`

状态：

- 保存中
- 测试中
- 成功
- 失败，显示简短错误
- 未保存更改

### 顶部栏

内容：

- 标题：Codex DataProxy
- 副标题：本地代理与中转站设置
- 当前 active provider
- 设置页安全状态：Local only
- 打开配置目录按钮

### 底部状态栏

显示：

- 本地代理地址
- 当前 active provider
- 当前匹配 Key
- 当前默认模型
- `/v1/models` 最近刷新时间
- Codex 是否正在运行

## UI 交互细节

- 新增 provider 时打开右侧空表单，不弹大模态。
- 新增 Key 时在 Key 路由表内展开行内表单。
- 删除 provider 用确认弹窗。
- 删除 Key 用确认弹窗，至少保留一个 Key。
- 切 active 前，如果有未保存更改，先提示保存。
- 切 active 成功后，顶部和状态栏立即更新。
- Provider 测试失败不阻止保存，但没有可用 Key 时不能设为 active。
- `api_key` 为空或 `sk-xx` 时，该 Key 显示“未配置”。
- 同一个模型被多个 Key 声明时，在模型路由区域显示“重复路由”提示；最终以后面的 Key 为准。
- 拖拽排序可以后续做，初版用上移/下移按钮更稳。

## 空状态

没有 provider 时：

```text
还没有中转站
添加一个中转站后，Codex DataProxy 就可以代理 Codex Desktop 请求。
[添加中转站]
```

没有有效 active provider 时：

```text
需要先配置中转站
请添加 API Key，并点击“设为当前”。
```

cmd 同步显示 settings URL，不启动 Codex Desktop。

## 实现拆分

### 第一阶段：配置目录

- 新增 provider YAML 结构。
- 自动创建默认 `config/dataproxy.yaml`。
- 读取、排序、校验、保存 provider。
- 废弃用户手动配置 `codex-dataproxy.yml`。

### 第二阶段：本地代理

- 启动 `127.0.0.1:<port>`。
- Codex 配置改为本地代理地址。
- `/v1/*` 转发到 active provider。
- 真实 Key 只由代理注入。

### 第三阶段：Settings API

- `/api/providers` CRUD。
- activate、reorder、test、refresh models。
- localhost-only 访问限制。

### 第四阶段：Settings UI

- 单页 HTML/CSS/JS，嵌入 Go binary 或运行时生成。
- 左侧 provider 列表，右侧详情表单。
- 现代 Codex 风格视觉。
- 响应式：宽屏两栏，小屏上下布局。

### 第五阶段：VM 测试

- 新系统无配置启动，自动创建默认 provider。
- 无 Key 时只打开 settings，不启动 Codex。
- 填 Key 后保存并设为当前。
- 启动 Codex，确认请求经过本地代理。
- 切换 provider 后，新请求走新 provider。
