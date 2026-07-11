# 管理工具与功能入口

本文档记录近期新增或调整的功能入口、使用条件与相关系统设置。除特别说明外，入口均要求管理员权限。

## 前端主题

- 新安装或未设置主题的实例默认使用新版前端（`default`）。
- 可在系统设置的站点信息中切换前端主题；已保存的主题选择优先于默认值。

## 用户与分组

### 用户多分组

- 入口：`Users` -> 创建或编辑用户。
- `Group` 为主分组；“Relay Groups”用于追加可用分组。
- API Key 调用会同时按主分组和追加分组验证渠道访问权限。

### 新用户默认分组与额度

- 入口：`System Settings` -> `Billing` -> `Quota Settings`。
- “Default Group for New Users”决定未显式指定分组的注册用户默认分组。
- “New User Quota”旁的“Convert USD to Quota”可按当前 `Quota Per Unit` 将美元金额换算为初始额度。

## 渠道管理

### 多密钥检测与恢复

- 入口：`Channels` -> 多密钥渠道行操作菜单 -> `Manage Keys`。
- 每个密钥可单独执行 `Test`；测试期间其他操作会被禁用。
- 多密钥管理窗口支持批量测试、自动恢复测试成功的自动禁用密钥，并保留手动禁用状态。
- 此入口仅在渠道启用多密钥模式时显示。

### Codex OAuth 渠道

- 入口：`Channels` -> 新建 Codex 渠道。
- 支持导入多种 OAuth JSON 导出格式，系统会规范化为 Codex 凭据格式。
- 新建时可使用 `ChatGPT Pro/Plus (headless)` 发起无头授权；完成授权后自动创建渠道。
- 编辑已有 Codex 渠道时可刷新凭据，并查看渠道使用量。
- Codex 默认模型会自动补充到新建渠道；客户端显式提供模型时仍以客户端请求为准。

### 查看渠道密钥的二次验证

- 入口：`System Settings` -> `Authentication` -> `Basic Authentication`。
- “Require two-factor authentication or passkey to view API keys”开启后，查看渠道密钥需要通过双因素认证或 Passkey 验证。

## 订阅管理

### 订阅计划与子额度

- 入口：管理员侧栏 `Subscriptions` -> `Plans`。
- 创建或编辑计划时可配置最多两个 `Sub Quota Limits`，用于小时、周或月等周期额度窗口。
- 用户钱包会显示有效订阅的子额度使用量、剩余额度和下次重置时间。

### 用户订阅控制

- 入口：用户钱包中的有效订阅卡片。
- 用户可停用、重新启用或删除自己的订阅；停用后订阅不参与扣费，但有效期仍继续流逝。

### 订阅可见权限

- 入口：管理员侧栏 `Subscriptions` -> `Subscription Visibility Permissions`。
- 可创建、重命名、启用或停用可见权限组，并关联订阅计划和用户。
- 用户创建/编辑抽屉可设置其订阅可见权限；用户表和订阅计划表会显示对应权限。
- 未分配专用权限时，用户按默认可见性规则查看订阅。

## 兑换码

- 入口：`Redemption Codes`，先勾选表格行。
- 批量工具栏支持复制所选兑换码和删除所选兑换码；删除前会要求确认。

## 协议兼容与稳定性

- Playground 支持可用的 ChatGPT/Codex 模型。
- `/v1/responses` 的流式请求、Codex Responses 请求默认值和 `prompt_cache_key` 兼容逻辑已补齐。
- Claude cache control 与 Zhipu V4 流式 usage 透传已修复。
- 这些能力没有独立管理入口，按对应 API 协议调用即可。

## 使用前检查

- 管理员侧栏模块由系统配置控制；`Subscriptions`、`Channels`、`Users` 与 `Redemption Codes` 被禁用时，对应入口不会显示。
- 订阅计划创建和修改要求先在支付网关设置中确认合规条款。
- 渠道敏感操作受渠道权限和二次验证设置限制。
