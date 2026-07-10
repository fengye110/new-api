package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const codexDeviceAuthorizationURL = "https://auth.openai.com/api/accounts/deviceauth/usercode"
const codexDeviceTokenURL = "https://auth.openai.com/api/accounts/deviceauth/token"
const codexDeviceRedirectURI = "https://auth.openai.com/deviceauth/callback"

type CodexDeviceAuthorization struct {
	DeviceAuthID string
	UserCode     string
	Interval     int
}

func StartCodexDeviceAuthorization(ctx context.Context, proxyURL string) (*CodexDeviceAuthorization, error) {
	client, err := getCodexOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	body, err := common.Marshal(map[string]string{"client_id": codexOAuthClientID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexDeviceAuthorizationURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api/"+common.Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("codex device authorization failed: status=%d", resp.StatusCode)
	}
	var payload struct {
		DeviceAuthID string `json:"device_auth_id"`
		UserCode     string `json:"user_code"`
		Interval     any    `json:"interval"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	if payload.DeviceAuthID == "" || payload.UserCode == "" {
		return nil, fmt.Errorf("codex device authorization response missing fields")
	}
	interval := 5
	switch value := payload.Interval.(type) {
	case float64:
		interval = int(value)
	case string:
		if parsed, err := strconv.Atoi(value); err == nil {
			interval = parsed
		}
	}
	if interval < 1 {
		interval = 5
	}
	return &CodexDeviceAuthorization{DeviceAuthID: payload.DeviceAuthID, UserCode: payload.UserCode, Interval: interval}, nil
}

func PollCodexDeviceAuthorization(ctx context.Context, proxyURL string, authorization *CodexDeviceAuthorization) (*CodexOAuthTokenResult, bool, error) {
	if authorization == nil || authorization.DeviceAuthID == "" || authorization.UserCode == "" {
		return nil, false, fmt.Errorf("invalid codex device authorization")
	}
	client, err := getCodexOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, false, err
	}
	body, err := common.Marshal(map[string]string{"device_auth_id": authorization.DeviceAuthID, "user_code": authorization.UserCode})
	if err != nil {
		return nil, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexDeviceTokenURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api/"+common.Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, false, fmt.Errorf("codex device authorization polling failed: status=%d", resp.StatusCode)
	}
	var payload struct {
		AuthorizationCode string `json:"authorization_code"`
		CodeVerifier      string `json:"code_verifier"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, false, err
	}
	if payload.AuthorizationCode == "" || payload.CodeVerifier == "" {
		return nil, false, fmt.Errorf("codex device authorization token response missing fields")
	}
	return exchangeCodexAuthorizationCode(ctx, client, payload.AuthorizationCode, payload.CodeVerifier)
}

func exchangeCodexAuthorizationCode(ctx context.Context, client *http.Client, code string, verifier string) (*CodexOAuthTokenResult, bool, error) {
	form := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  codexDeviceRedirectURI,
		"client_id":     codexOAuthClientID,
		"code_verifier": verifier,
	}
	values := make([]string, 0, len(form))
	for key, value := range form {
		values = append(values, key+"="+url.QueryEscape(value))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexOAuthTokenURL, strings.NewReader(strings.Join(values, "&")))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, false, fmt.Errorf("codex authorization code exchange failed: status=%d", resp.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, false, err
	}
	if payload.AccessToken == "" || payload.RefreshToken == "" || payload.ExpiresIn <= 0 {
		return nil, false, fmt.Errorf("codex authorization code exchange response missing fields")
	}
	return &CodexOAuthTokenResult{AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, ExpiresAt: time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)}, true, nil
}
