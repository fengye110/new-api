package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const codexHeadlessLoginTTL = 15 * time.Minute

type codexHeadlessLogin struct {
	mu            sync.Mutex
	channel       model.Channel
	authorization *service.CodexDeviceAuthorization
	createdAt     time.Time
	channelID     int
	failedMessage string
}

var codexHeadlessLogins = struct {
	sync.Mutex
	items map[string]*codexHeadlessLogin
}{items: make(map[string]*codexHeadlessLogin)}

func StartCodexHeadlessLogin(c *gin.Context) {
	var request AddChannelRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Channel == nil {
		common.ApiErrorMsg(c, "invalid channel configuration")
		return
	}
	channel := *request.Channel
	channel.Type = constant.ChannelTypeCodex
	channel.Key = `{"access_token":"pending","account_id":"pending"}`
	channel.ChannelInfo.IsMultiKey = false
	channel.ChannelInfo.MultiKeySize = 0
	addDefaultCodexModels(&channel)
	if err := validateChannel(&channel, true); err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	authorization, err := service.StartCodexDeviceAuthorization(ctx, channel.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	loginID, err := newCodexHeadlessLoginID()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	codexHeadlessLogins.Lock()
	codexHeadlessLogins.items[loginID] = &codexHeadlessLogin{channel: channel, authorization: authorization, createdAt: time.Now()}
	codexHeadlessLogins.Unlock()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"login_id":         loginID,
		"status":           "pending",
		"verification_uri": "https://auth.openai.com/codex/device",
		"user_code":        authorization.UserCode,
		"interval":         authorization.Interval,
		"expires_in":       int(codexHeadlessLoginTTL.Seconds()),
	}})
}

func GetCodexHeadlessLogin(c *gin.Context) {
	loginID := c.Param("login_id")
	codexHeadlessLogins.Lock()
	login := codexHeadlessLogins.items[loginID]
	if login != nil && time.Since(login.createdAt) > codexHeadlessLoginTTL {
		delete(codexHeadlessLogins.items, loginID)
		login = nil
	}
	codexHeadlessLogins.Unlock()
	if login == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "login session not found or expired"})
		return
	}

	login.mu.Lock()
	defer login.mu.Unlock()
	if login.channelID > 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "success", "channel_id": login.channelID}})
		return
	}
	if login.failedMessage != "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "failed", "message": login.failedMessage}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	tokens, authorized, err := service.PollCodexDeviceAuthorization(ctx, login.channel.GetSetting().Proxy, login.authorization)
	if err != nil {
		login.failedMessage = "failed to complete ChatGPT authorization"
		common.SysError("codex headless login failed: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "failed", "message": login.failedMessage}})
		return
	}
	if !authorized {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "pending"}})
		return
	}

	accountID, ok := service.ExtractCodexAccountIDFromJWT(tokens.AccessToken)
	if !ok {
		login.failedMessage = "authorized account is missing a ChatGPT account ID"
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "failed", "message": login.failedMessage}})
		return
	}
	credential := service.CodexOAuthKey{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, AccountID: accountID, LastRefresh: time.Now().Format(time.RFC3339), Expired: tokens.ExpiresAt.Format(time.RFC3339), Type: "codex"}
	if email, ok := service.ExtractEmailFromJWT(tokens.AccessToken); ok {
		credential.Email = email
	}
	encoded, err := common.Marshal(credential)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	login.channel.Key = string(encoded)
	login.channel.CreatedTime = common.GetTimestamp()
	if err := login.channel.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	login.channelID = login.channel.Id
	recordManageAudit(c, "channel.codex_headless_login", map[string]interface{}{"id": login.channel.Id, "name": login.channel.Name})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "success", "channel_id": login.channelID}})
}

func newCodexHeadlessLoginID() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate login ID: %w", err)
	}
	return "codex_" + hex.EncodeToString(value), nil
}
