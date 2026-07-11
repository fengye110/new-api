package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoSubscribeFreePlansForNewUserSelectsOnePreferredPlan(t *testing.T) {
	truncateTables(t)

	defaultGroup := &SubscriptionAccessGroup{Id: 1001, Key: "default-test", Name: "Default", Enabled: true, IsDefault: true}
	memberGroup := &SubscriptionAccessGroup{Id: 1002, Key: "member-test", Name: "Member", Enabled: true}
	require.NoError(t, DB.Create(defaultGroup).Error)
	require.NoError(t, DB.Create(memberGroup).Error)
	require.NoError(t, DB.Create(&UserSubscriptionAccessGroup{UserId: 501, GroupId: memberGroup.Id}).Error)

	plans := []*SubscriptionPlan{
		{Id: 1101, Title: "Default", PriceAmount: 0, Enabled: true, SortOrder: 100, DurationUnit: SubscriptionDurationMonth, DurationValue: 1},
		{Id: 1102, Title: "Member low", PriceAmount: 0, Enabled: true, SortOrder: 1, DurationUnit: SubscriptionDurationMonth, DurationValue: 1},
		{Id: 1103, Title: "Member preferred", PriceAmount: 0, Enabled: true, SortOrder: 2, DurationUnit: SubscriptionDurationMonth, DurationValue: 1},
		{Id: 1104, Title: "Member same priority", PriceAmount: 0, Enabled: true, SortOrder: 2, DurationUnit: SubscriptionDurationMonth, DurationValue: 1},
	}
	for _, plan := range plans {
		require.NoError(t, DB.Create(plan).Error)
	}
	for _, relation := range []*SubscriptionPlanAccessGroup{
		{PlanId: 1101, GroupId: defaultGroup.Id},
		{PlanId: 1102, GroupId: memberGroup.Id},
		{PlanId: 1103, GroupId: memberGroup.Id},
		{PlanId: 1104, GroupId: memberGroup.Id},
	} {
		require.NoError(t, DB.Create(relation).Error)
	}

	require.NoError(t, AutoSubscribeFreePlansForNewUser(501))
	var subscriptions []UserSubscription
	require.NoError(t, DB.Where("user_id = ?", 501).Find(&subscriptions).Error)
	require.Len(t, subscriptions, 1)
	assert.Equal(t, 1103, subscriptions[0].PlanId)

	require.NoError(t, AutoSubscribeFreePlansForNewUser(502))
	require.NoError(t, DB.Where("user_id = ?", 502).Find(&subscriptions).Error)
	require.Len(t, subscriptions, 1)
	assert.Equal(t, 1101, subscriptions[0].PlanId)
}
