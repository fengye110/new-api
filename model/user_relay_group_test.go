package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReplaceUserRelayGroupsIncludesPrimaryGroup(t *testing.T) {
	truncateTables(t)
	user := &User{Id: 9901, Username: "relay-groups-user", Password: "password"}
	require.NoError(t, DB.Create(user).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ReplaceUserRelayGroupsTx(tx, user.Id, "default", []string{"vip", "default", "vip"})
	}))

	loaded, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"default", "vip"}, loaded.RelayGroups)
}

func TestSearchUsersMatchesAdditionalRelayGroups(t *testing.T) {
	truncateTables(t)
	user := &User{Id: 9902, Username: "additional-relay-group", Password: "password"}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ReplaceUserRelayGroupsTx(tx, user.Id, "default", []string{"vip"})
	}))

	users, total, err := SearchUsers("", "vip", nil, nil, nil, 0, 10)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, users, 1)
	assert.Equal(t, user.Id, users[0].Id)
}
