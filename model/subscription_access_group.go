package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const DefaultSubscriptionAccessGroupKey = "default"

var ErrSubscriptionPlanNotAccessible = errors.New("套餐不存在或当前用户无权访问")

type SubscriptionAccessGroup struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	Key         string `json:"key" gorm:"type:varchar(64);uniqueIndex;not null"`
	Name        string `json:"name" gorm:"type:varchar(100);not null"`
	Description string `json:"description" gorm:"type:varchar(255);default:''"`
	Enabled     bool   `json:"enabled" gorm:"not null"`
	IsDefault   bool   `json:"is_default" gorm:"not null"`
	SortOrder   int    `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`

	UserCount int64 `json:"user_count" gorm:"-"`
	PlanCount int64 `json:"plan_count" gorm:"-"`
}

func (SubscriptionAccessGroup) TableName() string { return "subscription_access_groups" }

type UserSubscriptionAccessGroup struct {
	UserId    int   `json:"user_id" gorm:"primaryKey;index"`
	GroupId   int   `json:"group_id" gorm:"primaryKey;index"`
	CreatedBy int   `json:"created_by" gorm:"index"`
	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
}

func (UserSubscriptionAccessGroup) TableName() string { return "user_subscription_access_groups" }

type SubscriptionPlanAccessGroup struct {
	PlanId    int   `json:"plan_id" gorm:"primaryKey;index"`
	GroupId   int   `json:"group_id" gorm:"primaryKey;index"`
	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
}

func (SubscriptionPlanAccessGroup) TableName() string { return "subscription_plan_access_groups" }

func SeedDefaultSubscriptionAccessGroup() error {
	var group SubscriptionAccessGroup
	err := DB.Where("is_default = ?", true).First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		group = SubscriptionAccessGroup{
			Key:       DefaultSubscriptionAccessGroupKey,
			Name:      "默认权限",
			Enabled:   true,
			IsDefault: true,
		}
		return DB.Create(&group).Error
	}
	if err != nil {
		return err
	}
	return DB.Model(&SubscriptionAccessGroup{}).Where("id = ?", group.Id).Updates(map[string]interface{}{
		"key":     DefaultSubscriptionAccessGroupKey,
		"name":    "默认权限",
		"enabled": true,
	}).Error
}

func ListSubscriptionAccessGroups() ([]SubscriptionAccessGroup, error) {
	var groups []SubscriptionAccessGroup
	if err := DB.Order("is_default desc, sort_order desc, id desc").Find(&groups).Error; err != nil {
		return nil, err
	}
	for i := range groups {
		userQuery := DB.Model(&UserSubscriptionAccessGroup{}).Where("group_id = ?", groups[i].Id)
		if groups[i].IsDefault {
			userQuery = DB.Model(&User{})
		}
		if err := userQuery.Count(&groups[i].UserCount).Error; err != nil {
			return nil, err
		}
		if err := DB.Model(&SubscriptionPlanAccessGroup{}).Where("group_id = ?", groups[i].Id).Count(&groups[i].PlanCount).Error; err != nil {
			return nil, err
		}
	}
	return groups, nil
}

func CreateSubscriptionAccessGroup(group *SubscriptionAccessGroup) error {
	if group == nil {
		return errors.New("invalid subscription access group")
	}
	group.Name = strings.TrimSpace(group.Name)
	group.Description = strings.TrimSpace(group.Description)
	if group.Name == "" {
		return errors.New("订阅组名称不能为空")
	}
	if len(group.Name) > 100 || len(group.Description) > 255 {
		return errors.New("订阅组字段长度超限")
	}
	group.Id = 0
	group.IsDefault = false
	group.Key = "pending-" + common.GetRandomString(16)
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return err
		}
		group.Key = fmt.Sprintf("group-%d", group.Id)
		return tx.Model(group).Update("key", group.Key).Error
	})
}

func UpdateSubscriptionAccessGroup(group *SubscriptionAccessGroup) error {
	if group == nil || group.Id <= 0 {
		return errors.New("invalid subscription access group")
	}
	var existing SubscriptionAccessGroup
	if err := DB.First(&existing, group.Id).Error; err != nil {
		return err
	}
	group.Name = strings.TrimSpace(group.Name)
	group.Description = strings.TrimSpace(group.Description)
	if group.Name == "" || len(group.Name) > 100 || len(group.Description) > 255 {
		return errors.New("订阅组名称或描述无效")
	}
	if existing.IsDefault {
		return DB.Model(&existing).Updates(map[string]interface{}{
			"name":        group.Name,
			"description": group.Description,
			"sort_order":  group.SortOrder,
			"enabled":     true,
		}).Error
	}
	var affectedUserIds []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&existing).Updates(map[string]interface{}{
			"name":        group.Name,
			"description": group.Description,
			"enabled":     group.Enabled,
			"sort_order":  group.SortOrder,
		}).Error; err != nil {
			return err
		}
		if !existing.Enabled || group.Enabled {
			return nil
		}
		var emailRuleCount int64
		if err := tx.Model(&SubscriptionAccessEmailRuleGroup{}).Where("group_id = ?", group.Id).Count(&emailRuleCount).Error; err != nil {
			return err
		}
		if emailRuleCount > 0 {
			return errors.New("订阅组仍被邮箱默认权限规则使用，无法禁用")
		}
		if err := tx.Model(&UserSubscriptionAccessGroup{}).Where("group_id = ?", group.Id).Pluck("user_id", &affectedUserIds).Error; err != nil {
			return err
		}
		for _, userId := range affectedUserIds {
			if _, err := DeleteInaccessibleUserSubscriptionsTx(tx, userId); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, userId := range affectedUserIds {
		_ = InvalidateUserCache(userId)
	}
	return nil
}

func SetSubscriptionAccessGroupEnabled(id int, enabled bool) error {
	var group SubscriptionAccessGroup
	if err := DB.First(&group, id).Error; err != nil {
		return err
	}
	if group.IsDefault && !enabled {
		return errors.New("默认订阅组不允许禁用")
	}
	group.Enabled = enabled
	return UpdateSubscriptionAccessGroup(&group)
}

func DeleteSubscriptionAccessGroup(id int) error {
	var group SubscriptionAccessGroup
	if err := DB.First(&group, id).Error; err != nil {
		return err
	}
	if group.IsDefault {
		return errors.New("默认订阅组不允许删除")
	}
	var userCount, planCount, emailRuleCount int64
	if err := DB.Model(&UserSubscriptionAccessGroup{}).Where("group_id = ?", id).Count(&userCount).Error; err != nil {
		return err
	}
	if err := DB.Model(&SubscriptionPlanAccessGroup{}).Where("group_id = ?", id).Count(&planCount).Error; err != nil {
		return err
	}
	if err := DB.Model(&SubscriptionAccessEmailRuleGroup{}).Where("group_id = ?", id).Count(&emailRuleCount).Error; err != nil {
		return err
	}
	if userCount > 0 || planCount > 0 || emailRuleCount > 0 {
		return fmt.Errorf("订阅组仍关联 %d 个用户、%d 个套餐和 %d 条邮箱默认权限规则", userCount, planCount, emailRuleCount)
	}
	return DB.Delete(&group).Error
}

func getEnabledSubscriptionAccessGroupsTx(tx *gorm.DB, ids []int) ([]SubscriptionAccessGroup, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var groups []SubscriptionAccessGroup
	if err := tx.Where("id IN ? AND enabled = ?", ids, true).Find(&groups).Error; err != nil {
		return nil, err
	}
	if len(groups) != len(ids) {
		return nil, errors.New("订阅组不存在或已禁用")
	}
	return groups, nil
}

func normalizeSubscriptionAccessGroupIdsTx(tx *gorm.DB, ids []int, defaultWhenEmpty bool) ([]int, error) {
	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				normalized = append(normalized, id)
			}
		}
	}
	if len(normalized) > 64 {
		return nil, errors.New("订阅组数量不能超过64个")
	}
	if _, err := getEnabledSubscriptionAccessGroupsTx(tx, normalized); err != nil {
		return nil, err
	}
	if len(normalized) > 0 || !defaultWhenEmpty {
		return normalized, nil
	}
	var defaultGroup SubscriptionAccessGroup
	if err := tx.Where("is_default = ? AND enabled = ?", true, true).First(&defaultGroup).Error; err != nil {
		return nil, err
	}
	return []int{defaultGroup.Id}, nil
}

func ReplaceUserSubscriptionAccessGroupsTx(tx *gorm.DB, userId int, groupIds []int, operatorId int) error {
	if tx == nil || userId <= 0 {
		return errors.New("invalid user or transaction")
	}
	ids, err := normalizeSubscriptionAccessGroupIdsTx(tx, groupIds, false)
	if err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", userId).Delete(&UserSubscriptionAccessGroup{}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := tx.Create(&UserSubscriptionAccessGroup{UserId: userId, GroupId: id, CreatedBy: operatorId}).Error; err != nil {
			return err
		}
	}
	return nil
}

func ReplaceSubscriptionPlanAccessGroupsTx(tx *gorm.DB, planId int, groupIds []int) error {
	if tx == nil || planId <= 0 {
		return errors.New("invalid plan or transaction")
	}
	ids, err := normalizeSubscriptionAccessGroupIdsTx(tx, groupIds, true)
	if err != nil {
		return err
	}
	if err := tx.Where("plan_id = ?", planId).Delete(&SubscriptionPlanAccessGroup{}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := tx.Create(&SubscriptionPlanAccessGroup{PlanId: planId, GroupId: id}).Error; err != nil {
			return err
		}
	}
	return nil
}

func GetUserSubscriptionAccessGroups(userId int) ([]SubscriptionAccessGroup, error) {
	var groups []SubscriptionAccessGroup
	err := DB.Table("subscription_access_groups AS g").
		Joins("JOIN user_subscription_access_groups AS ug ON ug.group_id = g.id").
		Where("ug.user_id = ?", userId).
		Order("g.sort_order desc, g.id desc").Find(&groups).Error
	return groups, err
}

type SubscriptionAccessGroupUser struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type SubscriptionAccessGroupPlan struct {
	Id      int    `json:"id"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
}

func GetSubscriptionAccessGroupUsers(groupId int) ([]SubscriptionAccessGroupUser, error) {
	var users []SubscriptionAccessGroupUser
	var group SubscriptionAccessGroup
	if err := DB.First(&group, groupId).Error; err != nil {
		return nil, err
	}
	query := DB.Table("users AS u").Select("u.id, u.username, u.display_name")
	if !group.IsDefault {
		query = query.Joins("JOIN user_subscription_access_groups AS ug ON ug.user_id = u.id").Where("ug.group_id = ?", groupId)
	}
	err := query.Order("u.id desc").Scan(&users).Error
	return users, err
}

func GetSubscriptionAccessGroupPlans(groupId int) ([]SubscriptionAccessGroupPlan, error) {
	var plans []SubscriptionAccessGroupPlan
	err := DB.Table("subscription_plans AS p").
		Select("p.id, p.title, p.enabled").
		Joins("JOIN subscription_plan_access_groups AS pg ON pg.plan_id = p.id").
		Where("pg.group_id = ?", groupId).
		Order("p.sort_order desc, p.id desc").Scan(&plans).Error
	return plans, err
}

func RemoveUserFromSubscriptionAccessGroupTx(tx *gorm.DB, groupId int, userId int) ([]int, error) {
	if tx == nil || groupId <= 0 || userId <= 0 {
		return nil, errors.New("invalid subscription access group user")
	}
	var group SubscriptionAccessGroup
	if err := tx.First(&group, groupId).Error; err != nil {
		return nil, err
	}
	if group.IsDefault {
		return nil, errors.New("默认订阅组不允许移除用户")
	}
	if err := tx.Where("group_id = ? AND user_id = ?", groupId, userId).Delete(&UserSubscriptionAccessGroup{}).Error; err != nil {
		return nil, err
	}
	return DeleteInaccessibleUserSubscriptionsTx(tx, userId)
}

func RemovePlanFromSubscriptionAccessGroupTx(tx *gorm.DB, groupId int, planId int) error {
	if tx == nil || groupId <= 0 || planId <= 0 {
		return errors.New("invalid subscription access group plan")
	}
	var group SubscriptionAccessGroup
	if err := tx.First(&group, groupId).Error; err != nil {
		return err
	}
	if group.IsDefault {
		return errors.New("默认订阅组不允许移除套餐")
	}
	return tx.Where("group_id = ? AND plan_id = ?", groupId, planId).Delete(&SubscriptionPlanAccessGroup{}).Error
}

func PopulateSubscriptionPlanAccessGroups(plans []SubscriptionPlan) error {
	for i := range plans {
		var groups []SubscriptionAccessGroup
		if err := DB.Table("subscription_access_groups AS g").
			Joins("JOIN subscription_plan_access_groups AS pg ON pg.group_id = g.id").
			Where("pg.plan_id = ?", plans[i].Id).Order("g.sort_order desc, g.id desc").Find(&groups).Error; err != nil {
			return err
		}
		// Legacy plans without relations are public. Return the default group so
		// the admin editor reflects that effective access scope.
		if len(groups) == 0 {
			var defaultGroup SubscriptionAccessGroup
			if err := DB.Where("is_default = ?", true).First(&defaultGroup).Error; err != nil {
				return err
			}
			groups = []SubscriptionAccessGroup{defaultGroup}
		}
		plans[i].SubscriptionGroups = groups
		plans[i].SubscriptionGroupIds = make([]int, len(groups))
		for j := range groups {
			plans[i].SubscriptionGroupIds[j] = groups[j].Id
		}
	}
	return nil
}

func CanUserAccessSubscriptionPlanTx(tx *gorm.DB, userId int, planId int) (bool, error) {
	if tx == nil || userId <= 0 || planId <= 0 {
		return false, errors.New("invalid user or plan id")
	}
	var relationCount int64
	if err := tx.Model(&SubscriptionPlanAccessGroup{}).Where("plan_id = ?", planId).Count(&relationCount).Error; err != nil {
		return false, err
	}
	if relationCount == 0 {
		return true, nil
	}
	var defaultCount int64
	if err := tx.Table("subscription_plan_access_groups AS pg").
		Joins("JOIN subscription_access_groups AS g ON g.id = pg.group_id").
		Where("pg.plan_id = ? AND g.enabled = ? AND g.is_default = ?", planId, true, true).Count(&defaultCount).Error; err != nil {
		return false, err
	}
	if defaultCount > 0 {
		return true, nil
	}
	var matchedCount int64
	err := tx.Table("subscription_plan_access_groups AS pg").
		Joins("JOIN subscription_access_groups AS g ON g.id = pg.group_id").
		Joins("JOIN user_subscription_access_groups AS ug ON ug.group_id = pg.group_id").
		Where("pg.plan_id = ? AND ug.user_id = ? AND g.enabled = ?", planId, userId, true).
		Count(&matchedCount).Error
	return matchedCount > 0, err
}

func CanUserAccessSubscriptionPlan(userId int, planId int) (bool, error) {
	return CanUserAccessSubscriptionPlanTx(DB, userId, planId)
}

func GetAccessibleSubscriptionPlans(userId int) ([]SubscriptionPlan, error) {
	var plans []SubscriptionPlan
	if err := DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		return nil, err
	}
	accessible := make([]SubscriptionPlan, 0, len(plans))
	for _, plan := range plans {
		allowed, err := CanUserAccessSubscriptionPlan(userId, plan.Id)
		if err != nil {
			return nil, err
		}
		if allowed {
			plan.NormalizeDefaults()
			accessible = append(accessible, plan)
		}
	}
	return accessible, nil
}

func RemoveUserSubscriptionAccessGroupsTx(tx *gorm.DB, userId int) error {
	return tx.Where("user_id = ?", userId).Delete(&UserSubscriptionAccessGroup{}).Error
}

func RemoveSubscriptionPlanAccessGroupsTx(tx *gorm.DB, planId int) error {
	return tx.Where("plan_id = ?", planId).Delete(&SubscriptionPlanAccessGroup{}).Error
}

// DeleteInaccessibleUserSubscriptionsTx removes subscriptions whose plans are
// no longer visible to the user after a subscription permission change.
func DeleteInaccessibleUserSubscriptionsTx(tx *gorm.DB, userId int) ([]int, error) {
	if tx == nil || userId <= 0 {
		return nil, errors.New("invalid user or transaction")
	}
	var subscriptions []UserSubscription
	if err := tx.Where("user_id = ?", userId).Find(&subscriptions).Error; err != nil {
		return nil, err
	}
	deletedIds := make([]int, 0)
	for i := range subscriptions {
		allowed, err := CanUserAccessSubscriptionPlanTx(tx, userId, subscriptions[i].PlanId)
		if err != nil {
			return nil, err
		}
		if allowed {
			continue
		}
		if _, err := downgradeUserGroupForSubscriptionTx(tx, &subscriptions[i], common.GetTimestamp()); err != nil {
			return nil, err
		}
		if err := tx.Where("id = ?", subscriptions[i].Id).Delete(&UserSubscription{}).Error; err != nil {
			return nil, err
		}
		deletedIds = append(deletedIds, subscriptions[i].Id)
	}
	return deletedIds, nil
}
