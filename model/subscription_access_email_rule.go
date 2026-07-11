package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

type SubscriptionAccessEmailRule struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	Email     string `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
	GroupIds  []int  `json:"group_ids" gorm:"-"`
}

func (SubscriptionAccessEmailRule) TableName() string { return "subscription_access_email_rules" }

type SubscriptionAccessEmailRuleGroup struct {
	RuleId  int `json:"rule_id" gorm:"primaryKey;index"`
	GroupId int `json:"group_id" gorm:"primaryKey;index"`
}

func (SubscriptionAccessEmailRuleGroup) TableName() string {
	return "subscription_access_email_rule_groups"
}

func ListSubscriptionAccessEmailRules() ([]SubscriptionAccessEmailRule, error) {
	var rules []SubscriptionAccessEmailRule
	if err := DB.Order("email asc").Find(&rules).Error; err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return rules, nil
	}
	ruleIds := make([]int, 0, len(rules))
	for _, rule := range rules {
		ruleIds = append(ruleIds, rule.Id)
	}
	var bindings []SubscriptionAccessEmailRuleGroup
	if err := DB.Where("rule_id IN ?", ruleIds).Order("rule_id asc, group_id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	groupIdsByRule := make(map[int][]int, len(rules))
	for _, binding := range bindings {
		groupIdsByRule[binding.RuleId] = append(groupIdsByRule[binding.RuleId], binding.GroupId)
	}
	for index := range rules {
		rules[index].GroupIds = groupIdsByRule[rules[index].Id]
	}
	return rules, nil
}

func ReplaceSubscriptionAccessEmailRules(rules []SubscriptionAccessEmailRule) error {
	if len(rules) > 1000 {
		return errors.New("邮箱订阅权限规则不能超过1000条")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SubscriptionAccessEmailRuleGroup{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SubscriptionAccessEmailRule{}).Error; err != nil {
			return err
		}
		seenEmails := make(map[string]struct{}, len(rules))
		for _, rule := range rules {
			rule.Email = strings.ToLower(strings.TrimSpace(rule.Email))
			if rule.Email == "" || len(rule.Email) > 255 || !strings.Contains(rule.Email, "@") {
				return errors.New("邮箱订阅权限规则中的邮箱无效")
			}
			if _, exists := seenEmails[rule.Email]; exists {
				return errors.New("邮箱订阅权限规则中存在重复邮箱")
			}
			seenEmails[rule.Email] = struct{}{}
			groupIds, err := normalizeSubscriptionAccessGroupIdsTx(tx, rule.GroupIds, true)
			if err != nil {
				return err
			}
			rule.Id = 0
			rule.GroupIds = nil
			if err := tx.Create(&rule).Error; err != nil {
				return err
			}
			for _, groupId := range groupIds {
				if err := tx.Create(&SubscriptionAccessEmailRuleGroup{RuleId: rule.Id, GroupId: groupId}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func ApplySubscriptionAccessEmailRuleTx(tx *gorm.DB, userId int, email string) error {
	if tx == nil || userId <= 0 {
		return errors.New("invalid user or transaction")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil
	}
	var rule SubscriptionAccessEmailRule
	if err := tx.Where("email = ?", email).First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var bindings []SubscriptionAccessEmailRuleGroup
	if err := tx.Where("rule_id = ?", rule.Id).Find(&bindings).Error; err != nil {
		return err
	}
	groupIds := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		groupIds = append(groupIds, binding.GroupId)
	}
	return ReplaceUserSubscriptionAccessGroupsTx(tx, userId, groupIds, 0)
}
