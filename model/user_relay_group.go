package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

type UserRelayGroup struct {
	UserId    int    `json:"user_id" gorm:"primaryKey;index"`
	Group     string `json:"group" gorm:"primaryKey;size:64"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (UserRelayGroup) TableName() string { return "user_relay_groups" }

func normalizeUserRelayGroups(primaryGroup string, groups []string) []string {
	seen := make(map[string]struct{}, len(groups)+1)
	normalized := make([]string, 0, len(groups)+1)
	for _, group := range append([]string{primaryGroup}, groups...) {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		normalized = append(normalized, group)
	}
	return normalized
}

func ReplaceUserRelayGroupsTx(tx *gorm.DB, userId int, primaryGroup string, groups []string) error {
	if tx == nil || userId <= 0 {
		return errors.New("invalid user relay groups")
	}
	groups = normalizeUserRelayGroups(primaryGroup, groups)
	if err := tx.Where("user_id = ?", userId).Delete(&UserRelayGroup{}).Error; err != nil {
		return err
	}
	for _, group := range groups {
		if err := tx.Create(&UserRelayGroup{UserId: userId, Group: group}).Error; err != nil {
			return err
		}
	}
	return nil
}

func populateUserRelayGroups(tx *gorm.DB, users []*User) error {
	if len(users) == 0 {
		return nil
	}
	userIds := make([]int, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}
	var assignments []UserRelayGroup
	if err := tx.Where("user_id IN ?", userIds).Find(&assignments).Error; err != nil {
		return err
	}
	groupsByUser := make(map[int][]string, len(users))
	for _, assignment := range assignments {
		groupsByUser[assignment.UserId] = append(groupsByUser[assignment.UserId], assignment.Group)
	}
	for _, user := range users {
		user.RelayGroups = normalizeUserRelayGroups(user.Group, groupsByUser[user.Id])
	}
	return nil
}
