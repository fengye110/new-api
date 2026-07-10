package model

import (
	"testing"

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
