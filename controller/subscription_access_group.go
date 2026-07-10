package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type subscriptionAccessGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

type replaceSubscriptionAccessGroupsRequest struct {
	GroupIds []int `json:"group_ids"`
}

func AdminListSubscriptionAccessGroups(c *gin.Context) {
	groups, err := model.ListSubscriptionAccessGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}

func AdminCreateSubscriptionAccessGroup(c *gin.Context) {
	var req subscriptionAccessGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	group := model.SubscriptionAccessGroup{
		Name:        req.Name,
		Description: req.Description,
		Enabled:     req.Enabled,
		SortOrder:   req.SortOrder,
	}
	if err := model.CreateSubscriptionAccessGroup(&group); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "subscription_access_group.create", map[string]interface{}{"group_id": group.Id, "key": group.Key})
	common.ApiSuccess(c, group)
}

func AdminUpdateSubscriptionAccessGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req subscriptionAccessGroupRequest
	if id <= 0 || c.ShouldBindJSON(&req) != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	group := model.SubscriptionAccessGroup{Id: id, Name: req.Name, Description: req.Description, Enabled: req.Enabled, SortOrder: req.SortOrder}
	if err := model.UpdateSubscriptionAccessGroup(&group); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "subscription_access_group.update", map[string]interface{}{"group_id": id})
	common.ApiSuccess(c, nil)
}

func AdminSetSubscriptionAccessGroupStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if id <= 0 || c.ShouldBindJSON(&req) != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.SetSubscriptionAccessGroupEnabled(id, *req.Enabled); err != nil {
		common.ApiError(c, err)
		return
	}
	action := "subscription_access_group.disable"
	if *req.Enabled {
		action = "subscription_access_group.enable"
	}
	recordManageAudit(c, action, map[string]interface{}{"group_id": id})
	common.ApiSuccess(c, nil)
}

func AdminDeleteSubscriptionAccessGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.DeleteSubscriptionAccessGroup(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "subscription_access_group.delete", map[string]interface{}{"group_id": id})
	common.ApiSuccess(c, nil)
}

func AdminListSubscriptionAccessGroupUsers(c *gin.Context) {
	groupId, _ := strconv.Atoi(c.Param("id"))
	if groupId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	users, err := model.GetSubscriptionAccessGroupUsers(groupId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, users)
}

func AdminListSubscriptionAccessGroupPlans(c *gin.Context) {
	groupId, _ := strconv.Atoi(c.Param("id"))
	if groupId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	plans, err := model.GetSubscriptionAccessGroupPlans(groupId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, plans)
}

func AdminRemoveUserFromSubscriptionAccessGroup(c *gin.Context) {
	groupId, _ := strconv.Atoi(c.Param("id"))
	userId, _ := strconv.Atoi(c.Param("user_id"))
	if groupId <= 0 || userId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var deletedSubscriptionIds []int
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		deletedSubscriptionIds, err = model.RemoveUserFromSubscriptionAccessGroupTx(tx, groupId, userId)
		return err
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	_ = model.InvalidateUserCache(userId)
	recordManageAuditFor(c, userId, "subscription_access_group.user_remove", map[string]interface{}{"group_id": groupId, "target_user_id": userId, "deleted_subscription_ids": deletedSubscriptionIds})
	common.ApiSuccess(c, gin.H{"deleted_subscription_ids": deletedSubscriptionIds})
}

func AdminRemovePlanFromSubscriptionAccessGroup(c *gin.Context) {
	groupId, _ := strconv.Atoi(c.Param("id"))
	planId, _ := strconv.Atoi(c.Param("plan_id"))
	if groupId <= 0 || planId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		return model.RemovePlanFromSubscriptionAccessGroupTx(tx, groupId, planId)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(planId)
	recordManageAudit(c, "subscription_access_group.plan_remove", map[string]interface{}{"group_id": groupId, "plan_id": planId})
	common.ApiSuccess(c, nil)
}

func AdminGetUserSubscriptionAccessGroups(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	groups, err := model.GetUserSubscriptionAccessGroups(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}

func AdminReplaceUserSubscriptionAccessGroups(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	var req replaceSubscriptionAccessGroupsRequest
	if userId <= 0 || c.ShouldBindJSON(&req) != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var deletedSubscriptionIds []int
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.ReplaceUserSubscriptionAccessGroupsTx(tx, userId, req.GroupIds, c.GetInt("id")); err != nil {
			return err
		}
		var err error
		deletedSubscriptionIds, err = model.DeleteInaccessibleUserSubscriptionsTx(tx, userId)
		return err
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	_ = model.InvalidateUserCache(userId)
	recordManageAuditFor(c, userId, "subscription_access_group.user_replace", map[string]interface{}{"target_user_id": userId, "new_group_ids": req.GroupIds, "deleted_subscription_ids": deletedSubscriptionIds})
	common.ApiSuccess(c, gin.H{"deleted_subscription_ids": deletedSubscriptionIds})
}
