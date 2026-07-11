package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserSubscriptionToggleExcludesDisabledSubscriptionFromBilling(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	sub := &UserSubscription{
		Id:        9801,
		UserId:    801,
		PlanId:    1,
		StartTime: now - 60,
		EndTime:   now + 3600,
		Status:    "active",
	}
	require.NoError(t, DB.Create(sub).Error)

	disabled, err := UserDisableSubscription(sub.UserId, sub.Id, "  temporarily unused  ")
	require.NoError(t, err)
	assert.True(t, disabled.UserDisabled)
	assert.NotZero(t, disabled.UserDisabledAt)
	assert.Equal(t, "temporarily unused", disabled.UserDisabledReason)

	disabledAgain, err := UserDisableSubscription(sub.UserId, sub.Id, "different reason")
	require.NoError(t, err)
	assert.Equal(t, disabled.UserDisabledReason, disabledAgain.UserDisabledReason)

	active, err := GetAllActiveUserSubscriptions(sub.UserId)
	require.NoError(t, err)
	assert.Empty(t, active)
	hasActive, err := HasActiveUserSubscription(sub.UserId)
	require.NoError(t, err)
	assert.False(t, hasActive)

	enabled, err := UserEnableSubscription(sub.UserId, sub.Id)
	require.NoError(t, err)
	assert.False(t, enabled.UserDisabled)
	assert.Zero(t, enabled.UserDisabledAt)
	assert.Empty(t, enabled.UserDisabledReason)

	active, err = GetAllActiveUserSubscriptions(sub.UserId)
	require.NoError(t, err)
	assert.Len(t, active, 1)
}

func TestUserSubscriptionToggleRejectsOtherUsersAndExpiredSubscription(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	otherUserSub := &UserSubscription{Id: 9802, UserId: 802, PlanId: 1, StartTime: now - 60, EndTime: now + 3600, Status: "active"}
	expiredSub := &UserSubscription{Id: 9803, UserId: 801, PlanId: 1, StartTime: now - 7200, EndTime: now - 1, Status: "active"}
	require.NoError(t, DB.Create(otherUserSub).Error)
	require.NoError(t, DB.Create(expiredSub).Error)

	_, err := UserDisableSubscription(801, otherUserSub.Id, "")
	require.Error(t, err)
	_, err = UserEnableSubscription(801, expiredSub.Id)
	require.Error(t, err)
}

func TestUserSubscriptionToggleUsesHighestPriorityEnabledUpgradeGroup(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	user := &User{Id: 803, Username: "subscription-toggle-user", Group: "premium", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	highPlan := &SubscriptionPlan{Id: 9804, Title: "high-priority", SortOrder: 100}
	lowPlan := &SubscriptionPlan{Id: 9805, Title: "low-priority", SortOrder: 10}
	require.NoError(t, DB.Create(highPlan).Error)
	require.NoError(t, DB.Create(lowPlan).Error)
	highSub := &UserSubscription{Id: 9804, UserId: user.Id, PlanId: highPlan.Id, StartTime: now - 60, EndTime: now + 7200, Status: "active", UpgradeGroup: "svip", DowngradeGroup: "premium"}
	lowSub := &UserSubscription{Id: 9805, UserId: user.Id, PlanId: lowPlan.Id, StartTime: now - 60, EndTime: now + 3600, Status: "active", UpgradeGroup: "vip", DowngradeGroup: "premium"}
	require.NoError(t, DB.Create(highSub).Error)
	require.NoError(t, DB.Create(lowSub).Error)

	_, err := UserDisableSubscription(user.Id, highSub.Id, "pause")
	require.NoError(t, err)
	var updatedUser User
	require.NoError(t, DB.First(&updatedUser, user.Id).Error)
	assert.Equal(t, "vip", updatedUser.Group)

	_, err = UserDisableSubscription(user.Id, lowSub.Id, "pause")
	require.NoError(t, err)
	require.NoError(t, DB.First(&updatedUser, user.Id).Error)
	assert.Equal(t, "premium", updatedUser.Group)

	_, err = UserEnableSubscription(user.Id, highSub.Id)
	require.NoError(t, err)
	require.NoError(t, DB.First(&updatedUser, user.Id).Error)
	assert.Equal(t, "svip", updatedUser.Group)
}
