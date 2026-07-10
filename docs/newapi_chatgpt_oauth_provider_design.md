# new-api 移植 ChatGPT OAuth Headless 登录并作为 Provider 提供 API 的方案

> 目标：把 OpenCode / Codex 类 ChatGPT OAuth headless 登录能力，以“new-api Provider / Channel”的方式接入 new-api，使用户仍然通过 new-api 的 API Key 调用 `/v1/responses`、`/v1/chat/completions`、`/v1/models`；后台通过多个 ChatGPT/Codex OAuth 凭据池转发请求，并按小时/周/月配额自动禁用与恢复。

---

## 1. 可行性结论

**可行，但建议做成独立 Provider，而不是把 OpenCode 代码硬复制进 new-api。**

推荐实现路径：

1. 在 new-api 新增 `ChatGPT OAuth / Codex OAuth` Provider 类型。
2. 支持管理员导入或 headless 登录 ChatGPT/Codex OAuth 凭据。
3. 后端维护 OAuth 凭据池：自动刷新、健康检查、配额检查、失败熔断。
4. 用户侧仍使用 new-api Token 调用统一 API。
5. Provider 内部优先实现 `/v1/responses`，再做 `/v1/chat/completions` 兼容转换。
6. 配额满时自动禁用对应 OAuth 凭据；全部凭据不可用时禁用渠道；配额恢复后自动启用。

**注意：ChatGPT OAuth 订阅转 API 可能受上游服务条款、账号风控、频率限制影响。建议只用于自有账号、私有部署、授权测试场景；对外商业转售或多用户公开服务应优先使用 OpenAI Platform API Key。**

---

## 2. 参考项目边界

### 2.1 new-api

new-api 已经是 AI API 网关，具备：

- 多 Provider / Channel 管理；
- 用户 Token 分发；
- 模型映射；
- 额度计费；
- 渠道自动禁用；
- OpenAI-compatible / Claude-compatible / Gemini-compatible 转换能力；
- Codex channel 相关改动基础。

本方案应复用 new-api 现有的渠道、Token、日志、计费、模型映射、自动禁用机制。

### 2.2 OpenCode / Codex OAuth headless 登录

OpenCode / Codex 类工具已经支持 ChatGPT Plus/Pro OAuth 登录，headless 场景通常使用：

- Device Code Flow；
- 手动复制回调 URL；
- 导入已有 `auth.json`；
- 读取 access token；
- 本地刷新 token。

迁移时不要把 OpenCode 作为运行时依赖，而是抽象出：

```text
OAuth 登录器 AuthFlow
OAuth 凭据存储 CredentialStore
Token 刷新器 TokenRefresher
上游请求客户端 UpstreamClient
配额采集器 QuotaFetcher
```

### 2.3 CLIProxyAPI / auth2api

这两个项目的核心思路可以参考：

```text
OAuth 登录态 / auth.json
        ↓
账号池 Account Pool
        ↓
模型路由 / 自动回退
        ↓
OpenAI-compatible API
        ↓
客户端 Codex / OpenCode / Cline / Roo Code / SDK
```

但 new-api 的定位更适合做成 Provider，因为 new-api 已经有用户、渠道、计费、模型、日志体系。

---

## 3. 总体架构

```text
用户客户端
  ├─ Codex CLI
  ├─ OpenCode
  ├─ Cline / Roo Code
  └─ OpenAI-compatible SDK
        │
        │ Authorization: Bearer sk-newapi-xxx
        ▼
new-api HTTP API
  ├─ /v1/responses
  ├─ /v1/chat/completions
  └─ /v1/models
        │
        ▼
new-api Relay Router
        │
        ▼
ChatGPT OAuth Provider
  ├─ Model Mapper
  ├─ Request Translator
  ├─ Credential Pool
  ├─ Token Refresher
  ├─ Quota Manager
  ├─ Health Checker
  ├─ Account Selector
  └─ Stream Translator
        │
        ▼
OpenAI / ChatGPT / Codex upstream
  ├─ OAuth access token
  ├─ Codex responses endpoint
  └─ quota / usage endpoint
```

---

## 4. 功能范围

### 4.1 必做功能

| 功能 | 说明 |
|---|---|
| Provider 类型 | 新增 `ChatGPTOAuth` 或 `CodexOAuth` 渠道类型 |
| OAuth 凭据管理 | 支持多个账号凭据，单独启用/禁用 |
| Headless 登录 | 支持 Device Code 或手动回调 URL 完成登录 |
| auth.json 导入 | 支持管理员导入 Codex/OpenCode 生成的凭据文件 |
| Token 加密存储 | access token / refresh token / auth.json 必须加密 |
| 自动刷新 | token 过期或 401 时自动刷新 |
| `/v1/responses` | 优先支持，作为主协议 |
| `/v1/chat/completions` | 转换为 responses 请求，兼容旧客户端 |
| `/v1/models` | 返回该 Provider 可用模型与别名 |
| 流式输出 | SSE stream 转发与格式转换 |
| 多账号池 | 轮询、权重、失败回退 |
| 小时/周/月配额 | 定期采集并缓存配额状态 |
| 配额自动禁用 | 配额满则禁用对应凭据或渠道 |
| 配额自动恢复 | reset 时间到后自动检查并恢复 |
| 管理界面 | 展示账号、状态、配额、刷新、测试、手动禁用 |
| 审计日志 | 登录、导入、刷新、禁用、启用、配额变化记录 |

### 4.2 暂不建议第一版实现

| 功能 | 原因 |
|---|---|
| 完整模拟 ChatGPT 网页所有能力 | 协议变化快，维护成本高 |
| 图片、语音、文件、Codex Cloud 全能力 | 第一版聚焦 coding text / tool call |
| 对外公开账号转售 | 合规和风控风险高 |
| 跨机器同时使用同一个 auth.json | 容易触发刷新冲突和风控 |
| 自动绕过风控 / 验证码 / MFA | 不应实现 |

---

## 5. Provider 命名建议

推荐命名：

```text
ProviderTypeChatGPTOAuth
ProviderTypeCodexOAuth
```

更推荐 UI 显示为：

```text
ChatGPT OAuth / Codex
```

原因：

- 技术上走的是 Codex/ChatGPT OAuth；
- 用户更容易理解“ChatGPT Plus/Pro 登录”；
- 避免误解为 OpenAI Platform API Key。

---

## 6. 数据库设计

### 6.1 `oauth_credentials`

用于保存每个 ChatGPT/Codex OAuth 账号。

```sql
CREATE TABLE oauth_credentials (
    id BIGINT PRIMARY KEY,
    channel_id BIGINT NOT NULL,
    provider VARCHAR(64) NOT NULL,
    account_label VARCHAR(128),
    account_email_hash VARCHAR(128),
    account_email_masked VARCHAR(128),

    credential_ciphertext TEXT NOT NULL,
    credential_version INT NOT NULL DEFAULT 1,
    auth_mode VARCHAR(32) NOT NULL DEFAULT 'chatgpt',

    access_expires_at DATETIME NULL,
    refresh_expires_at DATETIME NULL,
    last_refresh_at DATETIME NULL,
    last_quota_check_at DATETIME NULL,
    last_used_at DATETIME NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'active',
    disabled_reason VARCHAR(255) NULL,
    next_available_at DATETIME NULL,

    quota_state_json TEXT NULL,
    fail_count INT NOT NULL DEFAULT 0,
    priority INT NOT NULL DEFAULT 0,
    weight INT NOT NULL DEFAULT 1,

    manually_disabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_oauth_credentials_channel_status
    ON oauth_credentials(channel_id, status);

CREATE INDEX idx_oauth_credentials_next_available
    ON oauth_credentials(next_available_at);
```

字段说明：

| 字段 | 说明 |
|---|---|
| `channel_id` | 绑定 new-api 渠道 |
| `provider` | `chatgpt_oauth` / `codex_oauth` |
| `credential_ciphertext` | 加密后的 auth.json / token bundle |
| `account_email_hash` | 用于去重，不明文保存邮箱 |
| `account_email_masked` | 界面展示，如 `ju***@gmail.com` |
| `status` | `active` / `quota_exhausted` / `auth_failed` / `cooldown` / `manual_disabled` |
| `next_available_at` | 配额恢复或冷却结束时间 |
| `quota_state_json` | 最新配额快照 |
| `manually_disabled` | 管理员手动禁用时不自动恢复 |

---

### 6.2 `oauth_quota_windows`

用于保存小时/周/月窗口配额。

```sql
CREATE TABLE oauth_quota_windows (
    id BIGINT PRIMARY KEY,
    credential_id BIGINT NOT NULL,
    period_type VARCHAR(32) NOT NULL,
    window_start DATETIME NULL,
    window_end DATETIME NULL,
    limit_value DECIMAL(20, 8) NULL,
    used_value DECIMAL(20, 8) NULL,
    remaining_value DECIMAL(20, 8) NULL,
    reset_at DATETIME NULL,
    exhausted BOOLEAN NOT NULL DEFAULT FALSE,
    source VARCHAR(32) NOT NULL DEFAULT 'upstream',
    raw_json TEXT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,

    UNIQUE (credential_id, period_type)
);

CREATE INDEX idx_oauth_quota_windows_reset
    ON oauth_quota_windows(reset_at);
```

`period_type` 建议支持：

```text
hour
5hour
week
month
```

说明：

- 如果上游只返回 5 小时 / 周配额，则 `month` 可由 new-api 内部统计；
- 如果上游返回月配额，则直接使用上游数据；
- 如果无法获取某个窗口，则显示 `unknown`，不要误判为可用无限量。

---

### 6.3 `oauth_credential_events`

用于审计。

```sql
CREATE TABLE oauth_credential_events (
    id BIGINT PRIMARY KEY,
    credential_id BIGINT NOT NULL,
    channel_id BIGINT NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    old_status VARCHAR(32) NULL,
    new_status VARCHAR(32) NULL,
    message TEXT NULL,
    raw_json TEXT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_oauth_credential_events_credential
    ON oauth_credential_events(credential_id, created_at);
```

事件类型：

```text
login_started
login_success
import_success
refresh_success
refresh_failed
quota_updated
quota_exhausted
auto_disabled
auto_enabled
manual_disabled
manual_enabled
request_failed
health_check_failed
```

---

## 7. 凭据加密设计

### 7.1 环境变量

```bash
CHATGPT_OAUTH_CREDENTIAL_KEY="base64-32-bytes-key"
CHATGPT_OAUTH_ENABLE_HEADLESS_LOGIN=true
CHATGPT_OAUTH_ENABLE_AUTH_JSON_IMPORT=true
CHATGPT_OAUTH_QUOTA_CHECK_INTERVAL=300
CHATGPT_OAUTH_REFRESH_BEFORE_EXPIRE=600
CHATGPT_OAUTH_MAX_PARALLEL_PER_ACCOUNT=1
```

### 7.2 加密要求

必须满足：

1. access token / refresh token / auth.json 只加密存储；
2. 日志中永远不打印完整 token；
3. API 返回值永远不返回 token 明文；
4. 管理员导出功能默认关闭；
5. 如果支持导出，必须二次确认并记录审计日志；
6. 加密 key 丢失后无法解密，需要重新登录账号；
7. 支持 key rotation。

推荐：

```text
AES-256-GCM
nonce: 12 bytes random
associated data: provider + credential_id + channel_id
```

`credential_ciphertext` 保存结构：

```json
{
  "version": 1,
  "alg": "AES-256-GCM",
  "nonce": "base64...",
  "ciphertext": "base64...",
  "tag": "base64..."
}
```

---

## 8. 登录流程设计

### 8.1 管理员导入 `auth.json`

第一版最稳，建议先实现。

流程：

```text
管理员本地运行 Codex/OpenCode 登录
        ↓
生成 auth.json
        ↓
new-api 后台：ChatGPT OAuth 渠道 → 导入凭据
        ↓
后端校验 auth.json 结构
        ↓
尝试刷新 / 测试请求 / 拉取配额
        ↓
加密保存
        ↓
账号状态 active
```

优点：

- 实现简单；
- 避免 new-api 直接处理完整 OAuth 浏览器回调；
- 便于第一版快速落地。

缺点：

- 管理员操作稍麻烦；
- 需要明确提示 auth.json 是敏感凭据。

---

### 8.2 Device Code / Headless 登录

第二阶段实现。

流程：

```text
POST /api/admin/chatgpt-oauth/login/start
        ↓
后端请求 Device Code
        ↓
返回 verification_uri + user_code + expires_in
        ↓
管理员用任意浏览器登录 ChatGPT
        ↓
后端轮询 token
        ↓
拿到 token bundle
        ↓
测试 + 拉配额
        ↓
加密保存
```

管理端 UI：

```text
[开始 Headless 登录]

请在浏览器打开： https://...
输入验证码： ABCD-EFGH
剩余时间： 600 秒

状态：等待授权 / 已授权 / 登录失败 / 超时
```

注意：

- 不实现绕过 MFA、验证码、风控；
- 登录超时后立即删除临时 device code；
- 轮询频率遵守上游返回的 `interval`；
- 不把 OAuth Client Secret 写死在前端；
- 如果使用公开 client id，要确认授权边界。

---

### 8.3 手动回调 URL 粘贴

用于 SSH / 远程部署 / WSL 环境。

流程：

```text
管理员点击登录
        ↓
new-api 生成 OAuth 登录 URL
        ↓
管理员本地浏览器打开
        ↓
登录完成后复制 redirect URL
        ↓
粘贴回 new-api
        ↓
后端解析 code/state
        ↓
换取 token
        ↓
保存凭据
```

---

## 9. Provider 接口设计

### 9.1 Go 接口建议

```go
type OAuthProvider interface {
    Name() string
    StartLogin(ctx context.Context, req StartLoginRequest) (*StartLoginResult, error)
    CompleteLogin(ctx context.Context, req CompleteLoginRequest) (*CredentialBundle, error)
    ImportCredential(ctx context.Context, raw []byte) (*CredentialBundle, error)
    Refresh(ctx context.Context, cred *OAuthCredential) (*CredentialBundle, error)
    FetchQuota(ctx context.Context, cred *OAuthCredential) (*QuotaSnapshot, error)
    ListModels(ctx context.Context, cred *OAuthCredential) ([]ModelInfo, error)
    DoResponses(ctx context.Context, cred *OAuthCredential, req *ResponsesRequest) (*ResponsesResponse, error)
    StreamResponses(ctx context.Context, cred *OAuthCredential, req *ResponsesRequest) (<-chan SSEEvent, error)
}
```

### 9.2 Relay Handler

```go
type ChatGPTOAuthAdaptor struct {
    CredentialStore *CredentialStore
    TokenRefresher  *TokenRefresher
    QuotaManager    *QuotaManager
    Selector        *CredentialSelector
    Upstream        *CodexUpstreamClient
    Translator      *OpenAITranslator
}
```

请求流程：

```text
1. 校验 new-api 用户 Token
2. 根据模型选择 ChatGPT OAuth Channel
3. 从 CredentialPool 选 active 凭据
4. 预检查配额
5. 如果 token 快过期，刷新
6. 转换请求格式
7. 请求上游
8. 转换响应 / stream
9. 记录用量
10. 更新本地 quota 估算
11. 如果上游返回 quota exhausted，则禁用该凭据
12. 失败时尝试下一个凭据
```

---

## 10. API 接口设计

### 10.1 用户侧 API

用户仍然调用 new-api 原有地址。

#### `/v1/responses`

第一优先级支持。

```bash
curl https://newapi.example.com/v1/responses \
  -H "Authorization: Bearer sk-newapi-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5-codex",
    "input": "review this code"
  }'
```

#### `/v1/chat/completions`

兼容旧客户端。

```bash
curl https://newapi.example.com/v1/chat/completions \
  -H "Authorization: Bearer sk-newapi-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5-codex",
    "messages": [
      {"role": "user", "content": "hello"}
    ],
    "stream": true
  }'
```

内部转换：

```text
chat.completions.messages
        ↓
responses.input
        ↓
upstream codex responses
        ↓
responses stream
        ↓
chat.completions SSE chunks
```

#### `/v1/models`

返回模型别名：

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-5-codex",
      "object": "model",
      "owned_by": "chatgpt-oauth"
    }
  ]
}
```

---

### 10.2 管理端 API

#### 创建登录任务

```http
POST /api/admin/channels/{channel_id}/oauth/login/start
```

响应：

```json
{
  "login_id": "login_xxx",
  "method": "device_code",
  "verification_uri": "https://...",
  "user_code": "ABCD-EFGH",
  "expires_in": 600,
  "interval": 5
}
```

#### 查询登录状态

```http
GET /api/admin/oauth/login/{login_id}
```

响应：

```json
{
  "status": "pending"
}
```

或：

```json
{
  "status": "success",
  "credential_id": 123
}
```

#### 导入 auth.json

```http
POST /api/admin/channels/{channel_id}/oauth/credentials/import
Content-Type: multipart/form-data
```

响应：

```json
{
  "credential_id": 123,
  "status": "active",
  "account_email_masked": "ju***@gmail.com"
}
```

#### 查看凭据池

```http
GET /api/admin/channels/{channel_id}/oauth/credentials
```

响应：

```json
{
  "data": [
    {
      "id": 123,
      "account_label": "account-1",
      "status": "active",
      "disabled_reason": null,
      "next_available_at": null,
      "quota": {
        "hour": {"remaining": 80, "reset_at": "2026-07-09T11:00:00+08:00"},
        "week": {"remaining": 300, "reset_at": "2026-07-13T00:00:00+08:00"},
        "month": {"remaining": 1200, "reset_at": "2026-08-01T00:00:00+08:00"}
      },
      "last_refresh_at": "2026-07-09T09:00:00+08:00",
      "last_quota_check_at": "2026-07-09T09:05:00+08:00"
    }
  ]
}
```

#### 手动测试凭据

```http
POST /api/admin/oauth/credentials/{credential_id}/test
```

#### 手动刷新凭据

```http
POST /api/admin/oauth/credentials/{credential_id}/refresh
```

#### 手动检查配额

```http
POST /api/admin/oauth/credentials/{credential_id}/quota/check
```

#### 批量检查渠道所有凭据

```http
POST /api/admin/channels/{channel_id}/oauth/quota/check-all
```

#### 手动禁用 / 启用

```http
POST /api/admin/oauth/credentials/{credential_id}/disable
POST /api/admin/oauth/credentials/{credential_id}/enable
```

---

## 11. 配额管理方案

### 11.1 配额状态定义

```go
type QuotaSnapshot struct {
    CredentialID int64
    Windows      []QuotaWindow
    Raw          json.RawMessage
    CheckedAt    time.Time
}

type QuotaWindow struct {
    PeriodType     string // hour, 5hour, week, month
    WindowStart    *time.Time
    WindowEnd      *time.Time
    LimitValue     *decimal.Decimal
    UsedValue      *decimal.Decimal
    RemainingValue *decimal.Decimal
    ResetAt        *time.Time
    Exhausted      bool
    Source         string // upstream, local, estimated
}
```

### 11.2 是否可用判断

只要任意一个强约束窗口耗尽，就不可用。

```text
available = true

for each quota_window:
    if quota_window.exhausted == true:
        available = false
    if remaining_value != null && remaining_value <= 0:
        available = false
    if reset_at != null && now < reset_at && exhausted:
        available = false
```

如果配额未知：

```text
quota_unknown_policy = conservative | allow | deny
```

推荐默认：

```text
conservative
```

含义：

- 新导入账号允许低频测试；
- 正式请求前尽量先获取配额；
- 获取失败超过阈值后进入 `cooldown`，避免误打爆上游。

---

### 11.3 小时 / 周 / 月配额来源

| 配额类型 | 来源优先级 |
|---|---|
| 小时 / 5 小时 | 上游 quota API；如果没有则本地滑动窗口估算 |
| 周 | 上游 quota API；如果没有则本地自然周统计 |
| 月 | 上游 quota API；如果没有则 new-api 本地月统计 |

本地统计只能作为保护阈值，不应伪装成上游真实配额。

建议配置：

```json
{
  "quota_policy": {
    "windows": [
      {"type": "5hour", "source": "upstream", "required": true},
      {"type": "week", "source": "upstream", "required": true},
      {"type": "month", "source": "local", "required": false, "limit": 1000000}
    ],
    "disable_when_unknown_minutes": 30,
    "auto_enable": true
  }
}
```

---

### 11.4 自动禁用逻辑

触发条件：

1. 配额接口显示某个窗口 remaining <= 0；
2. 上游请求返回 quota/rate limit/exhausted 类错误；
3. 连续 N 次请求失败且错误属于账号级；
4. token 刷新失败；
5. 管理员手动禁用。

处理：

```text
credential.status = quota_exhausted
credential.disabled_reason = "week quota exhausted"
credential.next_available_at = quota.reset_at
```

如果某个 channel 下所有 credential 都不可用：

```text
channel.status = auto_disabled
channel.disabled_reason = "all oauth credentials unavailable"
```

但要区分：

- 凭据级禁用：单账号不可用；
- 渠道级禁用：所有账号不可用；
- 手动禁用：不能自动恢复；
- 自动禁用：满足条件后可以自动恢复。

---

### 11.5 自动恢复逻辑

定时任务：

```text
每 1 分钟：扫描 next_available_at <= now 的 quota_exhausted 凭据
每 5 分钟：检查 active 凭据配额
每 15 分钟：检查 cooldown 凭据
每 6 小时：刷新快过期 token
```

恢复流程：

```text
1. 查找 status=quota_exhausted 且 manually_disabled=false 的凭据
2. now >= next_available_at
3. 刷新 token
4. 拉取 quota
5. 如果所有 required window remaining > 0
6. status 改为 active
7. 写 oauth_credential_events:auto_enabled
8. 如果 channel 因 all credentials unavailable 被禁用，则重新检查并启用 channel
```

伪代码：

```go
func ReenableDueCredentials(ctx context.Context) {
    creds := store.FindDueDisabledCredentials(time.Now())
    for _, cred := range creds {
        if cred.ManuallyDisabled {
            continue
        }

        refreshed, err := refresher.RefreshIfNeeded(ctx, cred)
        if err != nil {
            markAuthFailedOrCooldown(cred, err)
            continue
        }

        quota, err := quotaManager.FetchAndSave(ctx, refreshed)
        if err != nil {
            markCooldown(cred, err)
            continue
        }

        if quotaManager.IsAvailable(quota) {
            store.MarkActive(cred.ID)
            eventLog.AutoEnabled(cred.ID)
        } else {
            store.MarkQuotaExhausted(cred.ID, quota.NextResetAt())
        }
    }
}
```

---

## 12. 账号选择策略

### 12.1 第一版策略

推荐：

```text
active credentials
  → 支持该 model
  → quota available
  → fail_count 最低
  → last_used_at 最早
  → priority 高
  → weight 加权轮询
```

### 12.2 并发锁

同一个 OAuth 账号建议限制并发：

```text
CHATGPT_OAUTH_MAX_PARALLEL_PER_ACCOUNT=1 或 2
```

原因：

- 避免同一账号高并发触发风控；
- 避免 auth.json 刷新竞争；
- 便于配额统计准确。

Redis 锁建议：

```text
lock:chatgpt_oauth:credential:{id}
TTL = request_timeout + 30s
```

如果没有 Redis，可先用数据库行锁或内存锁，但多实例部署必须用 Redis/DB 分布式锁。

---

## 13. 请求转换方案

### 13.1 `/v1/responses`

内部尽量保持原样转发。

需要处理：

- `model` 映射；
- `input` 类型校验；
- `stream`；
- `tools`；
- `reasoning`；
- `max_output_tokens`；
- `metadata`；
- `store:false` 默认策略。

建议默认追加：

```json
{
  "store": false
}
```

如果上游不支持某些字段，应按配置处理：

```text
unsupported_field_policy = drop | error | warn
```

推荐默认：

```text
drop + debug log
```

但安全相关字段不能静默丢弃。

---

### 13.2 `/v1/chat/completions`

转换规则：

```text
messages[] → responses.input[]
system message → instructions 或 input 中的 system role
user/assistant → input item
tools/functions → responses.tools
stream=true → SSE stream
temperature/top_p → 如上游支持则透传，不支持则丢弃并记录 debug
```

兼容输出：

```text
responses.output_text
        ↓
choices[0].message.content
```

流式输出：

```text
response.output_text.delta
        ↓
chat.completion.chunk choices[0].delta.content
```

### 13.3 错误映射

| 上游错误 | new-api 返回 |
|---|---|
| OAuth token expired | 刷新后重试一次 |
| 401 refresh failed | 401 / channel auth failed |
| quota exhausted | 429 + 禁用凭据 |
| rate limited | 429 + cooldown |
| upstream 5xx | 502 / fallback 下一个凭据 |
| malformed request | 400 |

---

## 14. 管理界面设计

### 14.1 渠道编辑页新增

Provider 类型：

```text
ChatGPT OAuth / Codex
```

配置项：

| 配置 | 说明 |
|---|---|
| 模型映射 | `gpt-5-codex:gpt-5-codex` 等 |
| 上游协议 | `responses` |
| 是否启用 Chat Completions 兼容 | 默认开启 |
| 配额未知策略 | conservative / allow / deny |
| 自动恢复 | 默认开启 |
| 每账号最大并发 | 默认 1 |
| 失败回退 | 默认开启 |
| 手动禁用不自动恢复 | 默认开启 |

---

### 14.2 凭据池表格

列：

```text
账号标签
邮箱掩码
状态
小时/5小时 remaining/reset
周 remaining/reset
月 remaining/reset
最后使用时间
最后刷新时间
最后配额检查时间
失败次数
操作
```

操作按钮：

```text
导入 auth.json
Headless 登录
刷新 Token
检查配额
测试请求
禁用
启用
删除
查看事件
```

### 14.3 状态颜色

| 状态 | 颜色 |
|---|---|
| active | 绿色 |
| quota_exhausted | 黄色 |
| cooldown | 橙色 |
| auth_failed | 红色 |
| manual_disabled | 灰色 |
| unknown | 蓝色 |

---

## 15. 后端任务设计

### 15.1 Scheduler

新增任务：

```text
OAuthCredentialRefreshTask
OAuthQuotaCheckTask
OAuthAutoDisableTask
OAuthAutoEnableTask
OAuthCredentialHealthCheckTask
```

建议频率：

| 任务 | 频率 |
|---|---|
| active 凭据配额检查 | 5 分钟 |
| quota_exhausted 到期检查 | 1 分钟 |
| token 快过期刷新 | 10 分钟 |
| auth_failed 重试 | 30 分钟 |
| 事件日志清理 | 1 天 |

---

### 15.2 请求前检查

```go
func BeforeRequest(cred *OAuthCredential) error {
    if cred.ManuallyDisabled {
        return ErrCredentialManualDisabled
    }
    if cred.Status != "active" {
        return ErrCredentialUnavailable
    }
    if quotaManager.IsExhausted(cred.QuotaState) {
        store.MarkQuotaExhausted(cred.ID, quota.NextResetAt())
        return ErrQuotaExhausted
    }
    if tokenRefresher.ShouldRefresh(cred) {
        return tokenRefresher.Refresh(cred)
    }
    return nil
}
```

### 15.3 请求后处理

```go
func AfterRequest(cred *OAuthCredential, resp *UpstreamResp, err error) {
    if err == nil {
        store.MarkUsed(cred.ID)
        localUsageEstimator.Add(cred.ID, resp.Usage)
        return
    }

    switch classifyError(err) {
    case ErrQuotaExhausted:
        store.MarkQuotaExhausted(cred.ID, err.ResetAt)
    case ErrUnauthorized:
        if refreshErr := refresher.Refresh(cred); refreshErr != nil {
            store.MarkAuthFailed(cred.ID, refreshErr)
        }
    case ErrRateLimited:
        store.MarkCooldown(cred.ID, err.RetryAfter)
    default:
        store.IncrementFailCount(cred.ID)
    }
}
```

---

## 16. 和 new-api 现有自动禁用机制的关系

建议分两层：

```text
Channel status
  └─ Credential status
```

### 16.1 凭据级禁用

单个 OAuth 账号不可用时，只禁用该凭据。

```text
oauth_credentials.status = quota_exhausted
```

### 16.2 渠道级禁用

当渠道下没有任何 active 凭据时，才禁用 channel。

```text
channels.status = disabled
disabled_reason = all_oauth_credentials_unavailable
```

### 16.3 自动恢复

当任意一个凭据恢复 active：

```text
if channel.disabled_reason == all_oauth_credentials_unavailable:
    enable channel
```

不要恢复管理员手动禁用的渠道。

---

## 17. 配置示例

### 17.1 渠道配置 JSON

```json
{
  "provider": "chatgpt_oauth",
  "upstream_protocol": "responses",
  "model_aliases": {
    "gpt-5-codex": "gpt-5-codex",
    "gpt-5-codex-high": "gpt-5-codex"
  },
  "default_request_options": {
    "store": false,
    "reasoning": {
      "effort": "medium"
    }
  },
  "quota_policy": {
    "unknown_policy": "conservative",
    "auto_disable": true,
    "auto_enable": true,
    "required_windows": ["5hour", "week"],
    "local_month_limit": null
  },
  "account_pool": {
    "strategy": "least_recently_used",
    "max_parallel_per_account": 1,
    "fallback_on_5xx": true,
    "fallback_on_quota": true
  }
}
```

### 17.2 模型价格配置

由于 ChatGPT OAuth 不是标准 API 计费，new-api 侧仍需要给用户扣费。建议：

```text
new-api 用户扣费 = 按模型配置的 token 价格扣费
上游 ChatGPT OAuth 配额 = 独立监控，不直接等于 new-api 用户余额
```

不建议把上游订阅配额直接暴露为用户余额。

---

## 18. 迁移 / Migration 方案

### 18.1 GORM Model

```go
type OAuthCredential struct {
    ID                   int64
    ChannelID            int64
    Provider             string
    AccountLabel         string
    AccountEmailHash     string
    AccountEmailMasked   string
    CredentialCiphertext string
    CredentialVersion    int
    AuthMode             string
    AccessExpiresAt      *time.Time
    RefreshExpiresAt     *time.Time
    LastRefreshAt        *time.Time
    LastQuotaCheckAt     *time.Time
    LastUsedAt           *time.Time
    Status               string
    DisabledReason       string
    NextAvailableAt      *time.Time
    QuotaStateJSON       string
    FailCount            int
    Priority             int
    Weight               int
    ManuallyDisabled     bool
    CreatedAt            time.Time
    UpdatedAt            time.Time
}
```

### 18.2 自动迁移

启动时执行：

```go
func MigrateOAuthProvider(db *gorm.DB) error {
    return db.AutoMigrate(
        &OAuthCredential{},
        &OAuthQuotaWindow{},
        &OAuthCredentialEvent{},
    )
}
```

注意：

- SQLite / MySQL / PostgreSQL 字段类型要兼容；
- 老数据库升级后不影响原有渠道；
- 新表不存在才创建；
- 字段新增要幂等；
- migration 失败时阻止启动，避免运行时数据损坏。

---

## 19. 测试计划

### 19.1 单元测试

| 模块 | 测试点 |
|---|---|
| Credential encryption | 加密、解密、key 错误、nonce 随机 |
| Auth import | auth.json 格式校验、缺字段、过期 token |
| Token refresh | 正常刷新、401、refresh token 失效 |
| Quota parser | hour/5hour/week/month 解析 |
| Quota state machine | exhausted → active，active → exhausted |
| Selector | 多账号轮询、失败跳过、手动禁用跳过 |
| Request translator | chat → responses，responses → chat |
| Stream translator | SSE delta 正确输出 |
| Error classifier | 401/429/5xx/quota/rate limit 分类 |

### 19.2 集成测试

使用 mock upstream，不直接依赖真实 ChatGPT：

```text
mock /responses
mock /quota
mock /token/refresh
mock 401
mock 429 quota exhausted
mock stream interrupted
```

### 19.3 并发测试

场景：

1. 同账号并发 10 个请求，只允许 1 个进入；
2. 多账号并发，能分散到不同凭据；
3. token 同时过期，只刷新一次；
4. quota 满时，不再选择该账号；
5. reset 后自动恢复。

### 19.4 回归测试

确保不影响：

- OpenAI API Key 渠道；
- Claude 渠道；
- Gemini 渠道；
- 普通用户 Token；
- 订阅额度；
- existing auto-disable logic；
- SQLite 老数据库升级。

---

## 20. 分阶段实施计划

### Phase 1：最小可用 Provider

目标：跑通 `auth.json 导入 → /v1/responses → 用户调用`。

任务：

1. 新增 Provider 类型；
2. 新增 oauth_credentials 表；
3. 实现 auth.json 导入；
4. 加密保存凭据；
5. 实现 token 读取；
6. 实现 `/v1/responses` 转发；
7. 实现基础模型映射；
8. 实现基础错误处理。

验收：

```text
用户用 new-api key 调 /v1/responses 成功返回结果。
```

---

### Phase 2：Token 刷新与健康检查

任务：

1. 实现 token 快过期刷新；
2. 401 后刷新并重试一次；
3. 失败时凭据进入 auth_failed；
4. 管理端支持手动刷新；
5. 管理端支持测试凭据。

验收：

```text
token 过期后无需重新导入 auth.json，系统可自动刷新。
```

---

### Phase 3：配额检查与自动禁用/恢复

任务：

1. 新增 oauth_quota_windows；
2. 实现 quota fetcher；
3. 实现 5hour/week/month 解析；
4. 实现本地 month fallback；
5. 请求前检查 quota；
6. 请求后处理 quota 错误；
7. Scheduler 自动恢复；
8. 管理界面展示配额。

验收：

```text
配额满：自动禁用凭据。
配额恢复：自动启用凭据。
全部凭据不可用：渠道自动禁用。
任意凭据恢复：渠道自动启用。
```

---

### Phase 4：Headless 登录

任务：

1. StartLogin API；
2. PollLogin API；
3. Device Code Flow；
4. 手动回调 URL 粘贴；
5. 登录事件审计；
6. 登录超时清理。

验收：

```text
服务器无浏览器环境下，管理员能完成 ChatGPT OAuth 登录并生成凭据。
```

---

### Phase 5：Chat Completions 兼容

任务：

1. chat messages → responses input；
2. tools/function_call 转换；
3. stream chunk 转换；
4. usage 映射；
5. 常见客户端测试：Codex / OpenCode / Cline / Roo Code。

验收：

```text
旧客户端调用 /v1/chat/completions 可正常使用。
```

---

### Phase 6：管理增强

任务：

1. 凭据池表格；
2. 批量检查所有凭据；
3. 批量刷新；
4. 事件日志页；
5. 配额趋势；
6. 模型别名管理；
7. 手动禁用/启用。

验收：

```text
管理员可以在 UI 中完成账号接入、检查、禁用、启用、排错。
```

---

## 21. 风险与规避

| 风险 | 说明 | 规避 |
|---|---|---|
| 上游协议变化 | ChatGPT/Codex OAuth 接口可能变化 | Provider 独立封装，便于快速修复 |
| 服务条款风险 | 订阅转 API 不一定适合商业分发 | 仅私有授权使用；公开服务使用 API Key |
| 账号风控 | 多账号/高并发可能触发限制 | 限制并发、低频健康检查、失败冷却 |
| token 泄露 | auth.json 等同敏感凭据 | 加密存储、日志脱敏、权限控制 |
| 配额接口不稳定 | quota API 可能拿不到 | local fallback + conservative policy |
| 并发刷新冲突 | 多实例同时刷新同账号 | 分布式锁 |
| 计费不一致 | new-api 用户扣费和上游订阅配额不同 | 两套体系分离展示 |
| 手动禁用被自动打开 | 误恢复管理员禁用 | manually_disabled 永远不自动恢复 |

---

## 22. 最小 PR 拆分建议

### PR 1：数据库与 Provider 骨架

- 新增 Provider 类型；
- 新增表；
- 新增模型；
- migration；
- 空实现 adaptor。

### PR 2：auth.json 导入与加密

- 导入 API；
- 加密存储；
- 管理界面导入按钮；
- 凭据列表。

### PR 3：responses 转发

- `/v1/responses` relay；
- stream；
- 模型映射；
- 错误映射。

### PR 4：刷新与健康检查

- token refresh；
- test credential；
- 401 retry；
- auth_failed 状态。

### PR 5：配额与自动禁用恢复

- quota_windows；
- quota fetcher；
- auto-disable；
- auto-enable；
- UI 配额展示。

### PR 6：Headless 登录

- Device Code；
- 手动回调；
- 登录状态轮询。

### PR 7：chat completions 兼容

- chat → responses；
- stream chunk；
- tools；
- 客户端兼容测试。

---

## 23. 验收标准

必须满足：

1. 老数据库可自动 migration；
2. 原有 OpenAI/Claude/Gemini 渠道不受影响；
3. 管理员可导入至少 2 个 ChatGPT OAuth 凭据；
4. 用户可用 new-api Token 调用 `/v1/responses`；
5. 凭据配额满后不会继续被选择；
6. 凭据配额恢复后可自动重新启用；
7. 手动禁用的凭据不会自动启用；
8. 全部凭据不可用时渠道自动禁用；
9. 任意凭据恢复后渠道自动启用；
10. token 不出现在日志、API 响应、前端页面源码中；
11. stream 输出兼容常见 OpenAI-compatible 客户端；
12. `/v1/models` 能返回可用模型；
13. 单账号并发可控；
14. 失败有明确事件日志。

---

## 24. 推荐默认配置

```text
第一版：
- 只开放管理员导入 auth.json
- 默认关闭 headless 登录
- 默认每账号并发 1
- 默认启用自动禁用
- 默认启用自动恢复
- 默认 quota_unknown_policy=conservative
- 默认优先 /v1/responses
- 默认开启 /v1/chat/completions 兼容，但标记为 beta
```

原因：

- 快速落地；
- 风险最小；
- 方便调试；
- 避免 OAuth 登录流程一次做太大；
- 后续再补 Device Code / 手动回调。

---

## 25. 参考资料

- new-api: https://github.com/QuantumNous/new-api
- new-api changelog: https://www.newapi.ai/en/docs/guide/wiki/changelog
- new-api Codex CLI guide: https://www.newapi.ai/en/docs/apps/codex-cli
- CLIProxyAPI: https://github.com/router-for-me/CLIProxyAPI
- auth2api: https://github.com/AmazingAng/auth2api
- OpenCode: https://opencode.ai/
- OpenCode headless device auth example: https://github.com/tumf/opencode-openai-device-auth
- OpenAI Codex auth docs: https://developers.openai.com/codex/auth
- OpenAI Codex CLI reference: https://developers.openai.com/codex/cli/reference
- OpenAI Codex CI/CD auth guidance: https://developers.openai.com/codex/auth/ci-cd-auth

---

## 26. 最终建议

如果你的目标是给 new-api 做 PR，建议不要一上来做完整 headless OAuth + 配额 + chat 兼容。

最合理的路线是：

```text
先实现 auth.json 导入 + /v1/responses + 凭据池
        ↓
再实现 token refresh
        ↓
再实现 quota 检查和自动禁用恢复
        ↓
最后实现 headless 登录和 chat completions 兼容
```

这样每个 PR 都可独立 review，也更容易被 new-api 上游接受。
