# Data Proxy 服务端接口需求

## 目标

让 Codex DataProxy 客户端可以通过浏览器授权绑定用户的 Data Proxy 账号，然后由服务端为当前设备创建或复用一个 `codex-dp` 专用 API Key。客户端拿到该专用 Key 后，才能继续通过本地代理调用 Data Proxy 的 OpenAI 兼容模型接口。

核心原则：

- 客户端不收集 Data Proxy 密码。
- 登录、注册、2FA、风控和授权确认都在 Data Proxy 网页完成。
- 客户端不默认导出用户已有普通 Token 的明文。
- 服务端为当前设备创建或复用 `codex-dp` 专用 Token，并把完整 `sk-...` 只返回给客户端一次。
- 客户端后续用该 `sk-...` 调用 `/v1/models`、`/v1/responses` 等 OpenAI 兼容接口。
- 专用 Token 可以独立切换分组、轮换和撤销，不影响用户已有普通 Token。

## 名词

| 名称 | 说明 |
| --- | --- |
| Connected App | Data Proxy 中授权给第三方/桌面客户端的应用，这里是 `codex-dp` |
| Device Flow | 桌面客户端发起设备授权，浏览器完成登录和确认，客户端轮询结果 |
| Management Token | 客户端调用 Data Proxy 管理接口的凭证，建议前缀 `cdpat_` |
| API Key | 调用 OpenAI 兼容模型接口的 `sk-...` |
| Dedicated Token | Data Proxy 为当前设备创建的 `codex-dp` 专用 API Key |
| Group | Data Proxy 分组，用于渠道路由、倍率、资源隔离 |

## 最小闭环

```text
Codex DataProxy
  |
  | POST /device/start
  v
Data Proxy Server
  |
  | 打开 verification_uri_complete
  v
Data Proxy Web 登录/注册/授权
  |
  | POST /device/poll
  v
Codex DataProxy 获取 management_token
  |
  | GET /config
  | GET /groups
  | POST /tokens/ensure
  v
Data Proxy Server 返回当前设备专用 sk-...
  |
  | 本地代理注入 Authorization: Bearer sk-...
  v
Data Proxy /v1/models /v1/responses
```

Phase 1 只需要实现：

1. 设备授权。
2. 获取绑定账号和服务配置。
3. 获取分组列表。
4. 创建或复用当前设备专用 Token。
5. 轮换、撤销、切换专用 Token 分组。
6. OpenAI 兼容 `/v1/models` 和 `/v1/responses` 可用。

用户已有普通 Token 的列表和选择可以放到 Phase 2。

## 认证方式

管理接口使用：

```http
Authorization: Bearer cdpat_xxx
```

模型接口使用：

```http
Authorization: Bearer sk_xxx
```

所有接口必须使用 HTTPS。

## 接口清单

### 1. 发起设备授权

```http
POST /api/connected-apps/codex-dp/device/start
Content-Type: application/json
```

请求：

```json
{
  "device_id": "uuid-v4-generated-by-client",
  "device_name": "Windows PC",
  "platform": "windows",
  "app_version": "0.2.1",
  "client": "codex-dp",
  "locale": "zh-CN"
}
```

响应：

```json
{
  "device_code": "dev_xxx",
  "user_code": "CDP-2026",
  "verification_uri": "https://dp.app.mbu.ltd/connected-apps/codex-dp/activate",
  "verification_uri_complete": "https://dp.app.mbu.ltd/connected-apps/codex-dp/activate?user_code=CDP-2026",
  "expires_in": 600,
  "interval": 3
}
```

服务端要求：

- `device_code` 必须高熵、短期有效。
- `user_code` 适合用户手动核对。
- `interval` 是客户端轮询最小间隔。
- 同一设备重复 start 时，可以创建新 session，也可以让旧 session 失效。

### 2. 轮询设备授权

```http
POST /api/connected-apps/codex-dp/device/poll
Content-Type: application/json
```

请求：

```json
{
  "device_code": "dev_xxx"
}
```

等待授权：

```json
{
  "status": "authorization_pending",
  "interval": 3
}
```

授权成功：

```json
{
  "status": "authorized",
  "management_token": "cdpat_xxx",
  "management_token_expires_at": 1790000000,
  "server_url": "https://dp.app.mbu.ltd",
  "base_url": "https://dp.app.mbu.ltd/v1",
  "user": {
    "id": 123,
    "username": "alice",
    "display_name": "Alice"
  },
  "capabilities": {
    "groups": true,
    "dedicated_tokens": true,
    "token_rotate": true,
    "token_revoke": true,
    "token_group_update": true
  }
}
```

其他状态：

```json
{ "status": "slow_down", "interval": 8 }
{ "status": "denied" }
{ "status": "expired" }
{ "status": "invalid_device_code" }
```

服务端要求：

- 客户端轮询过快时返回 `slow_down`。
- `management_token` 只在 `authorized` 状态返回。
- `management_token` 应该可撤销、可过期。
- 建议 `management_token` 权限只限 connected app 管理范围，不能调用模型接口。

### 3. 获取客户端配置

```http
GET /api/connected-apps/codex-dp/config
Authorization: Bearer cdpat_xxx
```

响应：

```json
{
  "server_url": "https://dp.app.mbu.ltd",
  "base_url": "https://dp.app.mbu.ltd/v1",
  "user": {
    "id": 123,
    "username": "alice",
    "display_name": "Alice"
  },
  "selected_token": {
    "id": 456,
    "name": "codex-dp - Windows PC",
    "masked_key": "sk-1122********8899",
    "group": "fast",
    "effective_group": "fast",
    "owned_by_connected_app": true
  },
  "capabilities": {
    "groups": true,
    "dedicated_tokens": true,
    "token_rotate": true,
    "token_revoke": true,
    "token_group_update": true
  }
}
```

服务端要求：

- 不返回完整 `sk-...`。
- 返回当前用户、base_url、能力开关和当前选中的 Token 摘要。

### 4. 获取分组列表

```http
GET /api/connected-apps/codex-dp/groups
Authorization: Bearer cdpat_xxx
```

响应：

```json
{
  "data": [
    {
      "id": "fast",
      "name": "fast",
      "display_name": "高速分组",
      "available": true,
      "is_default": true
    },
    {
      "id": "cheap",
      "name": "cheap",
      "display_name": "低价分组",
      "available": false,
      "unavailable_reason": "当前账号暂不可用"
    }
  ]
}
```

服务端要求：

- 不可用分组可以返回，但要说明原因。
- `id/name` 需要稳定，客户端会用于创建或切换专用 Token。

### 5. 创建或复用当前设备专用 Token

这是最关键的接口。客户端需要从这里拿到可调用模型接口的 `sk-...`。

```http
POST /api/connected-apps/codex-dp/tokens/ensure
Authorization: Bearer cdpat_xxx
Content-Type: application/json
```

请求：

```json
{
  "device_id": "uuid-v4-generated-by-client",
  "device_name": "Windows PC",
  "platform": "windows",
  "app_version": "0.2.1",
  "group": "fast",
  "rotate": false
}
```

响应：

```json
{
  "selected": true,
  "created": true,
  "rotated": false,
  "api_key_once": true,
  "api_key": "sk_xxx",
  "base_url": "https://dp.app.mbu.ltd/v1",
  "token": {
    "id": 456,
    "name": "codex-dp - Windows PC",
    "masked_key": "sk-1122********8899",
    "status": "enabled",
    "group": "fast",
    "effective_group": "fast",
    "group_available": true,
    "owned_by_connected_app": true,
    "connected_app_slug": "codex-dp",
    "device_id": "uuid-v4-generated-by-client",
    "expired_at": null,
    "last_used_at": null
  }
}
```

服务端要求：

- 如果当前设备已有有效专用 Token，默认复用。
- 如果没有，创建一个新的专用 Token。
- 如果 `rotate = true`，轮换该设备专用 Token，并返回新的完整 `api_key`。
- `api_key` 只在创建或轮换时返回一次。
- 如果复用旧 Token 但服务端无法再次拿到明文，应该返回明确错误，要求客户端调用 rotate。
- 专用 Token 必须打标：
  - `owned_by_connected_app = true`
  - `connected_app_slug = "codex-dp"`
  - `device_id = 当前设备 ID`
- 专用 Token 的分组可以由客户端管理。

建议：为了保证客户端首次绑定一定可用，`ensure` 在首次创建时必须返回完整 `api_key`。

### 6. 修改专用 Token 分组

```http
PUT /api/connected-apps/codex-dp/tokens/:id/group
Authorization: Bearer cdpat_xxx
Content-Type: application/json
```

请求：

```json
{
  "group": "cheap"
}
```

响应：

```json
{
  "token": {
    "id": 456,
    "name": "codex-dp - Windows PC",
    "masked_key": "sk-1122********8899",
    "status": "enabled",
    "group": "cheap",
    "effective_group": "cheap",
    "group_available": true,
    "owned_by_connected_app": true
  }
}
```

服务端要求：

- 只允许修改 `owned_by_connected_app = true` 且属于当前 connected app 的 Token。
- 不允许修改用户已有普通 Token 的分组。
- 不返回完整 `api_key`。

### 7. 轮换专用 Token

```http
POST /api/connected-apps/codex-dp/tokens/:id/rotate
Authorization: Bearer cdpat_xxx
```

响应：

```json
{
  "rotated": true,
  "api_key_once": true,
  "api_key": "sk_new_xxx",
  "base_url": "https://dp.app.mbu.ltd/v1",
  "token": {
    "id": 456,
    "name": "codex-dp - Windows PC",
    "masked_key": "sk-5566********7788",
    "status": "enabled",
    "group": "fast",
    "effective_group": "fast",
    "owned_by_connected_app": true
  }
}
```

服务端要求：

- 只允许轮换专用 Token。
- 返回新完整 `api_key` 一次。
- 旧 key 应立即失效，或按服务端策略短暂宽限。

### 8. 撤销专用 Token

```http
POST /api/connected-apps/codex-dp/tokens/:id/revoke
Authorization: Bearer cdpat_xxx
```

响应：

```json
{
  "revoked": true
}
```

服务端要求：

- 只允许撤销专用 Token。
- 不默认删除或撤销用户普通 Token。

### 9. 撤销设备授权

```http
POST /api/connected-apps/codex-dp/device/revoke
Authorization: Bearer cdpat_xxx
Content-Type: application/json
```

请求：

```json
{
  "device_id": "uuid-v4-generated-by-client",
  "revoke_dedicated_token": false
}
```

响应：

```json
{
  "revoked": true,
  "dedicated_token_revoked": false
}
```

服务端要求：

- 撤销 management token。
- 可选撤销该设备的专用 Token。

## Phase 2 可选接口：查看用户 Token

如果希望客户端展示已有普通 Token，可增加：

```http
GET /api/connected-apps/codex-dp/tokens
Authorization: Bearer cdpat_xxx
```

响应：

```json
{
  "data": [
    {
      "id": 123,
      "name": "My existing token",
      "masked_key": "sk-1111********2222",
      "status": "enabled",
      "group": "fast",
      "effective_group": "fast",
      "group_available": true,
      "owned_by_connected_app": false,
      "expired_at": null,
      "last_used_at": 1790000000
    }
  ]
}
```

重要：

- 该接口不应该返回用户已有普通 Token 的完整 `api_key`。
- 普通 Token 可用于展示、筛选和参考分组。
- 如果未来要允许客户端使用已有普通 Token，建议服务端单独做高风险授权确认，而不是默认开放。

## OpenAI 兼容接口要求

客户端最终会用专用 `sk-...` 调用：

```http
GET /v1/models
POST /v1/responses
```

建议继续兼容：

```http
POST /v1/chat/completions
```

但不要只支持 chat completions。当前 Codex Desktop 更需要 `/v1/responses`。

注意：Codex DataProxy 客户端本身不做 Responses 到 Chat Completions 的协议转换。如果服务端接入的是只支持 Chat Completions 的国产模型，需要模型官方接口直接支持 Responses 协议，或者由中转站/服务端完成 Responses 到 Chat Completions 的协议转换。如果模型和中转站都不支持这个转换，请使用 `ccs + codex`；也可以使用 `dp.app.mbu.ltd` 中转站。`dp.app.mbu.ltd` 支持的是 GPT/OpenAI 兼容协议转换，不支持 Claude 协议转换。

## 错误码约定

错误响应建议统一格式：

```json
{
  "error": {
    "code": "token_group_unavailable",
    "message": "当前账号不能使用该分组",
    "request_id": "req_xxx"
  }
}
```

建议错误码：

| HTTP | code | 场景 |
| --- | --- | --- |
| 400 | invalid_request | 请求字段缺失或格式错误 |
| 401 | invalid_management_token | management token 无效 |
| 403 | app_not_authorized | 用户未授权 codex-dp |
| 403 | insufficient_scope | management token 权限不足 |
| 404 | token_not_found | Token 不存在或不属于当前用户 |
| 409 | token_not_owned_by_connected_app | 尝试管理普通 Token |
| 409 | token_group_unavailable | 分组不可用 |
| 409 | token_disabled | Token 已禁用 |
| 410 | device_code_expired | device_code 已过期 |
| 429 | rate_limited | 请求过快 |
| 500 | internal_error | 服务端异常 |

## 安全要求

1. `management_token` 不能调用模型接口。
2. `management_token` 需要可撤销、可过期。
3. `device_code` 短期有效，且只能使用一次。
4. `api_key` 只在专用 Token 创建或轮换时返回一次。
5. 服务端日志不得记录完整 `management_token`、完整 `api_key`、完整 Authorization header。
6. 所有返回给客户端的普通 Token 只允许展示 `masked_key`。
7. 修改分组、轮换、撤销只允许作用于 `codex-dp` 专用 Token。
8. 解绑设备时应撤销 management token。
9. 可提供用户控制台页面，用于查看和撤销已授权设备。

## 客户端需要保存的数据

普通配置可以保存：

```json
{
  "server_url": "https://dp.app.mbu.ltd",
  "base_url": "https://dp.app.mbu.ltd/v1",
  "user": {
    "id": 123,
    "username": "alice",
    "display_name": "Alice"
  },
  "selected_token": {
    "id": 456,
    "name": "codex-dp - Windows PC",
    "masked_key": "sk-1122********8899",
    "group": "fast",
    "owned_by_connected_app": true
  },
  "last_sync_at": 1790000000
}
```

系统安全存储保存：

```json
{
  "management_token": "cdpat_xxx",
  "management_token_expires_at": 1790000000,
  "api_key": "sk_xxx",
  "api_key_token_id": 456
}
```

服务端只需要保证 `tokens/ensure` 或 `tokens/:id/rotate` 能在必要时重新返回一个可用的专用 `sk-...`，客户端即可恢复模型调用能力。
