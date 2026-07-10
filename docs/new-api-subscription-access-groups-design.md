# New API 多订阅组访问控制方案

## 一、目标与权限语义

新增独立的“订阅组”概念：

- 一个用户可以属于多个订阅组。
- 一个订阅套餐可以授权给多个订阅组。
- 用户只要属于套餐授权组中的任意一个，即可查看和购买该套餐。
- 系统存在一个内置的默认订阅组，代表“所有用户”。
- 套餐属于默认订阅组时，对全部用户开放。
- 不修改现有 `users.role` 和 `users.group`。
- 用户已购买的有效订阅，不因后来退出订阅组而立即失效。

当前 `SubscriptionPlan` 尚无访问组字段，用户套餐列表接口会直接返回全部启用套餐；余额购买和第三方支付也只是根据套餐 ID 查询并检查套餐是否启用。

因此，后端必须同时控制：

1. 套餐列表可见性；
2. 套餐详情读取；
3. 所有支付方式的下单入口；
4. 管理员手工绑定订阅；
5. 不能仅依赖前端隐藏。

---

## 二、数据库设计

不建议在 `users` 或 `subscription_plans` 中保存 JSON 数组。多用户、多套餐、多组之间是标准多对多关系，应使用关联表。

### 2.1 订阅组表

```go
type SubscriptionAccessGroup struct {
    Id          int    `json:"id" gorm:"primaryKey"`
    Key         string `json:"key" gorm:"type:varchar(64);uniqueIndex;not null"`
    Name        string `json:"name" gorm:"type:varchar(100);not null"`
    Description string `json:"description" gorm:"type:varchar(255);default:''"`

    Enabled   bool `json:"enabled" gorm:"not null;default:true"`
    IsDefault bool `json:"is_default" gorm:"not null;default:false"`

    SortOrder int   `json:"sort_order" gorm:"not null;default:0"`
    CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SubscriptionAccessGroup) TableName() string {
    return "subscription_access_groups"
}
```

内置默认组：

```text
key        = default
name       = 默认订阅组
is_default = true
enabled    = true
```

默认组应具有以下限制：

- 不允许删除；
- 不允许禁用；
- 不允许修改 `key`；
- 系统只能存在一个 `is_default=true` 的组；
- 所有用户都被视为默认组成员，不需要为每个用户插入关联记录。

### 2.2 用户与订阅组关联表

```go
type UserSubscriptionAccessGroup struct {
    UserId  int `json:"user_id" gorm:"primaryKey;index"`
    GroupId int `json:"group_id" gorm:"primaryKey;index"`

    CreatedBy int   `json:"created_by" gorm:"index"`
    CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
}

func (UserSubscriptionAccessGroup) TableName() string {
    return "user_subscription_access_groups"
}
```

该表只保存用户的额外订阅组，不保存默认组。

例如：

```text
用户 100：
- enterprise
- internal-test

默认组由系统自动附加。
```

用户的有效订阅组集合为：

```text
default + 数据库中已启用的显式订阅组
```

### 2.3 套餐与订阅组关联表

```go
type SubscriptionPlanAccessGroup struct {
    PlanId  int `json:"plan_id" gorm:"primaryKey;index"`
    GroupId int `json:"group_id" gorm:"primaryKey;index"`

    CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
}

func (SubscriptionPlanAccessGroup) TableName() string {
    return "subscription_plan_access_groups"
}
```

关联关系示例：

```text
基础套餐：
- default

企业套餐：
- enterprise
- partner

内部测试套餐：
- internal-test
```

---

## 三、兼容规则

建议采用以下规则：

```text
套餐没有任何订阅组关联
    → 对所有用户开放，兼容历史数据

套餐包含 default
    → 对所有用户开放

套餐只包含普通订阅组
    → 用户组与套餐组存在交集时开放
```

虽然新建套餐时前端会默认选择 `default`，后端仍应保留“无关联即公开”的兼容逻辑，避免旧数据库升级后所有套餐突然消失。

迁移时也可以将现有套餐全部关联到默认组，使数据更加明确：

```sql
INSERT INTO subscription_plan_access_groups(plan_id, group_id)
SELECT p.id, g.id
FROM subscription_plans p
JOIN subscription_access_groups g ON g.key = 'default'
WHERE NOT EXISTS (
    SELECT 1
    FROM subscription_plan_access_groups pg
    WHERE pg.plan_id = p.id
);
```

即便不执行该回填，兼容规则也能保证旧套餐继续公开。

---

## 四、后端模型扩展

### 4.1 返回给管理端的派生字段

不要让 GORM 直接将关联组保存进套餐表，可以增加非数据库字段：

```go
type SubscriptionPlan struct {
    // 原字段保持不变

    SubscriptionGroups []SubscriptionAccessGroup \
        `json:"subscription_groups,omitempty" gorm:"-"`
    SubscriptionGroupIds []int \
        `json:"subscription_group_ids,omitempty" gorm:"-"`
}
```

用户模型也可以增加：

```go
type User struct {
    // 原字段保持不变

    SubscriptionGroups []SubscriptionAccessGroup \
        `json:"subscription_groups,omitempty" gorm:"-"`
    SubscriptionGroupIds []int \
        `json:"subscription_group_ids,omitempty" gorm:"-"`
}
```

本方案只增加派生的订阅组字段，不改变现有 `role` 和 `group` 行为。

---

## 五、核心访问判断

统一实现一个服务，不允许每个支付 Controller 自己拼判断。

```go
func CanUserAccessSubscriptionPlan(
    userId int,
    planId int,
) (bool, error) {
    return CanUserAccessSubscriptionPlanTx(DB, userId, planId)
}

func CanUserAccessSubscriptionPlanTx(
    tx *gorm.DB,
    userId int,
    planId int,
) (bool, error) {
    if userId <= 0 || planId <= 0 {
        return false, errors.New("invalid user or plan id")
    }

    // 1. 套餐没有配置订阅组：兼容旧数据，公开访问
    var relationCount int64
    if err := tx.Model(&SubscriptionPlanAccessGroup{}).
        Where("plan_id = ?", planId).
        Count(&relationCount).Error; err != nil {
        return false, err
    }

    if relationCount == 0 {
        return true, nil
    }

    // 2. 套餐属于默认组：全部用户可访问
    var defaultCount int64
    if err := tx.Table("subscription_plan_access_groups AS pg").
        Joins("JOIN subscription_access_groups AS g ON g.id = pg.group_id").
        Where(
            "pg.plan_id = ? AND g.enabled = ? AND g.is_default = ?",
            planId, true, true,
        ).
        Count(&defaultCount).Error; err != nil {
        return false, err
    }

    if defaultCount > 0 {
        return true, nil
    }

    // 3. 用户显式订阅组与套餐组求交集
    var matchedCount int64
    err := tx.Table("subscription_plan_access_groups AS pg").
        Joins("JOIN subscription_access_groups AS g ON g.id = pg.group_id").
        Joins(
            "JOIN user_subscription_access_groups AS ug "+
                "ON ug.group_id = pg.group_id",
        ).
        Where(
            "pg.plan_id = ? AND ug.user_id = ? AND g.enabled = ?",
            planId, userId, true,
        ).
        Count(&matchedCount).Error

    return matchedCount > 0, err
}
```

推荐统一错误：

```go
var ErrSubscriptionPlanNotAccessible =
    errors.New("套餐不存在或当前用户无权访问")
```

不要返回“套餐存在但不属于你的组”，避免通过遍历套餐 ID 探测隐藏套餐。

---

## 六、套餐列表过滤

当前用户端接口直接查询所有 `enabled=true` 套餐，修改为：

```go
func GetAccessibleSubscriptionPlans(userId int) ([]SubscriptionPlan, error)
```

SQL 逻辑：

```sql
SELECT DISTINCT p.*
FROM subscription_plans p
WHERE p.enabled = 1
AND (
    NOT EXISTS (
        SELECT 1
        FROM subscription_plan_access_groups pg0
        WHERE pg0.plan_id = p.id
    )
    OR EXISTS (
        SELECT 1
        FROM subscription_plan_access_groups pg
        JOIN subscription_access_groups g
          ON g.id = pg.group_id
         AND g.enabled = 1
        LEFT JOIN user_subscription_access_groups ug
          ON ug.group_id = pg.group_id
         AND ug.user_id = ?
        WHERE pg.plan_id = p.id
          AND (
              g.is_default = 1
              OR ug.user_id IS NOT NULL
          )
    )
)
ORDER BY p.sort_order DESC, p.id DESC;
```

用户接口：

```go
func GetSubscriptionPlans(c *gin.Context) {
    userId := c.GetInt("id")

    plans, err := model.GetAccessibleSubscriptionPlans(userId)
    if err != nil {
        common.ApiError(c, err)
        return
    }

    // 返回用户有权访问的套餐
}
```

---

## 七、所有购买入口必须二次校验

即使套餐列表已经过滤，也必须防止用户直接构造：

```json
{
  "plan_id": 123
}
```

访问隐藏套餐。

必须在以下入口调用统一权限判断：

```text
PurchaseSubscriptionWithBalance
SubscriptionRequestEpay
SubscriptionRequestStripePay
SubscriptionRequestCreemPay
SubscriptionRequestWaffoPancakePay
其他后续新增支付入口
```

余额购买应在扣款事务内部检查：

```go
err := DB.Transaction(func(tx *gorm.DB) error {
    plan, err := getSubscriptionPlanByIdTx(tx, planId)
    if err != nil {
        return err
    }

    allowed, err := CanUserAccessSubscriptionPlanTx(tx, userId, plan.Id)
    if err != nil {
        return err
    }
    if !allowed {
        return ErrSubscriptionPlanNotAccessible
    }

    // 检查套餐状态、余额并创建订阅
})
```

其他支付入口在创建第三方支付链接和本地订单前检查：

```go
allowed, err := model.CanUserAccessSubscriptionPlan(userId, plan.Id)
if err != nil {
    common.ApiError(c, err)
    return
}
if !allowed {
    common.ApiError(c, model.ErrSubscriptionPlanNotAccessible)
    return
}
```

### 支付回调规则

支付订单创建成功后，不建议在支付回调阶段再次检查订阅组。

原因：

```text
用户符合条件并成功下单
→ 管理员在支付过程中调整用户组
→ 支付平台已经扣款
```

此时拒绝创建订阅会造成“已付款但无订阅”。

因此：

- 创建支付订单时检查组权限；
- 支付回调只验证订单、金额、支付平台和订单状态；
- 已成功创建的待支付订单视为已取得购买资格。

---

## 八、管理员手工绑定规则

管理员手工给用户绑定套餐时，建议默认也执行订阅组检查。

```go
func AdminBindSubscription(userId, planId int, force bool) error
```

普通管理员：

```text
用户必须属于套餐允许的订阅组。
```

Root 可选支持：

```json
{
  "plan_id": 123,
  "force": true
}
```

第一版可不实现强制绑定。管理员先给用户添加订阅组，再绑定订阅，更容易审计和理解。

---

## 九、已有订阅是否受组变更影响

建议组权限只控制：

- 用户能否看到套餐；
- 用户能否购买套餐；
- 管理员能否正常为该用户绑定套餐。

不影响已经创建的 `UserSubscription`：

```text
用户退出 enterprise 组
→ enterprise 套餐不再显示
→ 不允许续购 enterprise 套餐
→ 已购买且未过期的 enterprise 订阅继续使用
```

如果后续需要“退出组立即失效”，应单独增加：

```text
strict_group_enforcement
```

并在消费订阅额度时检查，不要与第一版语义混用。

---

## 十、管理员订阅组 API

建议增加独立路由：

```text
GET    /api/subscription/admin/access-groups
POST   /api/subscription/admin/access-groups
PUT    /api/subscription/admin/access-groups/:id
PATCH  /api/subscription/admin/access-groups/:id/status
DELETE /api/subscription/admin/access-groups/:id
```

创建请求：

```json
{
  "key": "enterprise",
  "name": "企业用户",
  "description": "企业合同客户可购买的套餐",
  "enabled": true,
  "sort_order": 100
}
```

删除普通组时建议：

- 组仍关联用户或套餐时，拒绝删除；
- 管理员先解除关联；
- 不建议默认级联删除，避免误开放或误关闭套餐。

删除前返回：

```json
{
  "user_count": 25,
  "plan_count": 3
}
```

默认组不提供删除、禁用操作。

---

## 十一、用户订阅组管理 API

可以提供独立接口：

```text
GET /api/subscription/admin/users/:id/access-groups
PUT /api/subscription/admin/users/:id/access-groups
```

更新请求：

```json
{
  "group_ids": [2, 5, 8]
}
```

更新时使用事务整体替换：

```go
func ReplaceUserSubscriptionAccessGroupsTx(
    tx *gorm.DB,
    userId int,
    groupIds []int,
    operatorId int,
) error
```

处理步骤：

1. 去重；
2. 验证组全部存在且启用；
3. 排除默认组；
4. 限制最多例如 64 个组；
5. 删除用户旧关联；
6. 批量插入新关联；
7. 清理用户订阅组缓存；
8. 写管理审计日志。

也可以把字段合并进现有用户编辑接口：

```json
{
  "id": 100,
  "username": "test",
  "group": "default",
  "subscription_group_ids": [2, 5]
}
```

推荐：

- 合并到现有用户编辑接口，方便后台操作；
- 同时保留独立 API，方便批量管理和自动化调用。

---

## 十二、套餐创建和更新 API

管理端套餐请求新增：

```go
type AdminUpsertSubscriptionPlanRequest struct {
    Plan                 model.SubscriptionPlan `json:"plan"`
    SubscriptionGroupIds *[]int                 `json:"subscription_group_ids"`
}
```

使用指针的语义：

```text
nil
    → 旧客户端未提交，不修改原关系

[]
    → 显式清空；后端转换为默认组

[2, 3]
    → 替换为指定组
```

创建套餐时：

- 未传组：自动关联默认组；
- 传空数组：自动关联默认组；
- 传普通组：保存指定组；
- 默认组和普通组同时出现：归一化为仅默认组。

更新套餐时必须在同一事务中：

1. 更新套餐字段；
2. 删除旧套餐组关联；
3. 批量插入新关联；
4. 清理套餐缓存；
5. 清理可见套餐列表缓存。

---

## 十三、前端订阅组管理页面

在订阅管理页面增加两个页签：

```text
套餐管理
订阅组管理
```

订阅组表格字段：

| 字段 | 说明 |
|---|---|
| 名称 | 企业用户、内部测试等 |
| 标识 | enterprise、internal-test |
| 状态 | 启用、禁用 |
| 类型 | 默认组、普通组 |
| 用户数 | 显式加入该组的用户数量 |
| 套餐数 | 使用该组的套餐数量 |
| 描述 | 管理员备注 |
| 操作 | 编辑、启禁用、删除 |

默认组显示锁定标记：

```text
默认订阅组 · 所有用户
```

不显示删除和禁用按钮。

---

## 十四、套餐编辑前端

类型增加：

```ts
export const subscriptionAccessGroupSchema = z.object({
  id: z.number(),
  key: z.string(),
  name: z.string(),
  enabled: z.boolean(),
  is_default: z.boolean(),
})

export const subscriptionPlanSchema = z.object({
  // 原字段
  subscription_groups: z.array(subscriptionAccessGroupSchema).optional(),
  subscription_group_ids: z.array(z.number()).optional(),
})
```

表单增加：

```ts
subscription_group_ids: z
  .array(z.number())
  .min(1, t('Please select at least one subscription group'))
```

默认值：

```ts
subscription_group_ids: [defaultGroupId]
```

套餐编辑抽屉增加“访问范围”区域：

```text
访问范围
[ 默认订阅组（所有用户） × ]

说明：
选择默认订阅组后，所有用户均可查看和购买此套餐。
选择普通订阅组后，只有属于任意选中组的用户可访问。
```

使用支持搜索和多选的 Combobox：

- 选中项使用 Badge 展示；
- 支持按名称和 key 搜索；
- 禁用组不允许新选择；
- 已关联但后来禁用的组显示“已禁用”；
- 选择默认组时清空其他组；
- 选择普通组时自动移除默认组。

默认组与其他组同时选择没有实际意义，前端应保持互斥。

---

## 十五、用户编辑前端

用户类型增加：

```ts
subscription_groups: z
  .array(subscriptionAccessGroupSchema)
  .optional(),

subscription_group_ids: z
  .array(z.number())
  .optional(),
```

用户编辑抽屉增加：

```text
订阅组

固定组：
[默认订阅组 · 所有用户]

额外订阅组：
[企业用户 ×] [内部测试 ×] [+ 添加]
```

规则：

- 默认组只显示，不加入提交数组；
- 普通订阅组支持多选；
- 创建用户时允许直接设置；
- 编辑用户时返回现有组；
- 用户列表增加“订阅组”列；
- 默认组不必在用户列表重复显示，只显示额外组；
- 组较多时显示前两个和 `+N`。

提交数据：

```ts
payload.subscription_group_ids =
  data.subscription_group_ids ?? []
```

---

## 十六、用户套餐页面

用户端不需要显示组选择器。

处理方式：

```text
后端只返回用户有权访问的套餐
前端正常渲染返回结果
```

可选增加标签：

```text
专属套餐
企业专享
内部套餐
```

但不要向用户返回其无权访问的组和套餐。

用户套餐接口可以返回：

```json
{
  "plan": {
    "id": 10,
    "title": "企业套餐",
    "restricted": true
  }
}
```

用户端不需要获得完整 `subscription_group_ids`，避免泄露内部组结构。

---

## 十七、缓存方案

第一版可以直接查询数据库，因为套餐和订阅组数量通常较少。

需要缓存时建议：

```text
subscription-plan-groups:{planId}
user-subscription-groups:{userId}
accessible-subscription-plans:{userId}:{version}
```

缓存失效时机：

- 修改套餐组：清理套餐缓存和相关用户可见套餐缓存；
- 修改用户组：清理该用户组缓存和可见套餐缓存；
- 禁用订阅组：清理所有相关套餐访问缓存；
- 删除订阅组：清理所有相关缓存。

为了降低批量清缓存复杂度，可以增加全局版本号：

```text
subscription-access-version
```

缓存键中包含该版本。任何组、成员或套餐关系变化时自增版本，旧缓存自然失效。

---

## 十八、数据库自动迁移

将三个新模型加入：

```go
DB.AutoMigrate(
    // 原模型
    &SubscriptionAccessGroup{},
    &UserSubscriptionAccessGroup{},
    &SubscriptionPlanAccessGroup{},
)
```

普通迁移和快速迁移列表都要增加。

启动后执行幂等初始化：

```go
func SeedDefaultSubscriptionAccessGroup() error
```

使用 Upsert 保证：

- 老数据库自动创建默认组；
- 多次启动不会重复；
- 多节点环境只由 Master 节点执行；
- 默认组被误改为禁用时自动恢复。

---

## 十九、审计日志

以下操作写管理审计：

```text
subscription_access_group.create
subscription_access_group.update
subscription_access_group.enable
subscription_access_group.disable
subscription_access_group.delete
subscription_access_group.user_replace
subscription_access_group.plan_replace
```

日志中记录：

```json
{
  "target_user_id": 100,
  "old_group_ids": [2],
  "new_group_ids": [2, 5],
  "operator_id": 1
}
```

不要只记录“修改成功”，否则后续无法追踪套餐为什么突然对某用户不可见。

---

## 二十、边界情况

### 20.1 订阅组被禁用

- 通过该组授权的套餐立即不再对该组用户显示；
- 已购买订阅继续有效；
- 套餐仍保留与该组的关联；
- 重新启用后自动恢复访问。

### 20.2 用户同时属于多个组

采用并集：

```text
用户组 ∩ 套餐组 ≠ 空集
    → 允许访问
```

### 20.3 套餐同时属于默认组和普通组

默认组代表所有用户，因此普通组没有额外作用。

前端应禁止这种组合，后端保存时也归一化为仅默认组。

### 20.4 删除用户

删除用户时清理：

```text
user_subscription_access_groups
```

### 20.5 删除套餐

如果未来支持删除套餐，需要同时清理：

```text
subscription_plan_access_groups
```

### 20.6 支付途中退出组

已经创建订单则允许支付完成；新订单不允许创建。

---

## 二十一、测试要求

后端至少覆盖：

1. 无组配置的历史套餐对所有用户可见；
2. 默认组套餐对所有用户可见；
3. 用户属于套餐任意一个组时可见；
4. 用户没有交集时不可见；
5. 禁用组不能授予访问；
6. 直接构造隐藏 `plan_id` 无法使用余额购买；
7. Stripe、Epay、Creem、Waffo 均无法创建隐藏套餐订单；
8. 管理员绑定时执行权限检查；
9. 用户退出组后已存在订阅继续有效；
10. 已创建支付订单在用户退出组后仍能正常回调；
11. 默认组不能删除和禁用；
12. 用户组整体替换事务失败时原关系不变；
13. SQLite、MySQL、PostgreSQL 自动迁移成功；
14. 多节点缓存失效后权限及时更新。

前端至少覆盖：

1. 新套餐默认选中默认组；
2. 默认组和普通组互斥；
3. 编辑套餐正确回填多个组；
4. 编辑用户正确回填多个订阅组；
5. 禁用组显示但不能新选择；
6. 用户页面不展示无权限套餐；
7. API 返回无权限后不能继续拉起支付。

---

## 二十二、推荐实施顺序

第一阶段：

```text
新增三张表
默认组初始化
订阅组 CRUD
用户组关系管理
套餐组关系管理
```

第二阶段：

```text
实现统一 CanUserAccessSubscriptionPlan
过滤套餐列表
覆盖余额和全部第三方支付入口
管理员绑定增加校验
```

第三阶段：

```text
套餐编辑多选
用户编辑多选
订阅组管理页面
用户列表订阅组展示
```

第四阶段：

```text
缓存
审计
迁移测试
接口测试
前端测试
```

---

## 二十三、最终数据关系与判定公式

数据关系：

```text
users
  └── user_subscription_access_groups
          └── subscription_access_groups
                  └── subscription_plan_access_groups
                          └── subscription_plans
```

权限公式：

```text
套餐无组配置
OR 套餐属于默认组
OR 用户订阅组与套餐订阅组存在交集
    → 用户可以查看和购买套餐

否则
    → 套餐列表不返回，直接提交 plan_id 也拒绝
```

该方案与现有系统角色、模型渠道分组、订阅升级分组完全隔离，能够兼容老数据库和旧套餐，同时支持企业组、代理商组、内部测试组等访问范围。
