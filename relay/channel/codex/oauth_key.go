package codex

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type OAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	AccountID        string `json:"account_id,omitempty"`
	ChatGPTAccountID string `json:"chatgpt_account_id,omitempty"`
	ChatGPTUserID    string `json:"chatgpt_user_id,omitempty"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	ClientID         string `json:"client_id,omitempty"`
	Name             string `json:"name,omitempty"`
	PlanType         string `json:"plan_type,omitempty"`
	ChatGPTPlanType  string `json:"chatgpt_plan_type,omitempty"`
	LastRefresh      string `json:"last_refresh,omitempty"`
	Email            string `json:"email,omitempty"`
	Type             string `json:"type,omitempty"`
	Expired          string `json:"expired,omitempty"`
}

func ParseOAuthKey(raw string) (*OAuthKey, error) {
	return ParseOAuthKeyForChannel(raw, 0)
}

// NormalizeOAuthKey converts supported OAuth export formats to the credential
// schema used by Codex channels.
func NormalizeOAuthKey(raw string) (string, error) {
	var payload map[string]any
	if err := common.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return "", errors.New("invalid oauth key json")
	}

	credential := payload
	metadata := payload
	if accounts, ok := payload["accounts"].([]any); ok {
		for _, item := range accounts {
			account, ok := item.(map[string]any)
			if !ok || strings.ToLower(stringValue(account["platform"])) != "openai" {
				continue
			}
			if values, ok := account["credentials"].(map[string]any); ok {
				credential = values
				metadata = account
				break
			}
		}
	} else if openAI, ok := payload["openai"].(map[string]any); ok {
		credential = openAI
	}

	key := OAuthKey{
		IDToken:      firstString([]string{"id_token", "id"}, credential, payload),
		AccessToken:  firstStringByKeyPriority([]string{"access_token", "access"}, credential, payload),
		RefreshToken: firstStringByKeyPriority([]string{"refresh_token", "refresh"}, credential, payload),
		AccountID: firstStringByKeyPriority([]string{"account_id", "chatgpt_account_id", "chatgpt_user_id", "accountId"},
			credential, metadata, payload),
		ChatGPTAccountID: firstString([]string{"chatgpt_account_id"}, credential, metadata, payload),
		ChatGPTUserID:    firstString([]string{"chatgpt_user_id"}, credential, metadata, payload),
		WorkspaceID:      firstString([]string{"workspace_id"}, credential, metadata, payload),
		ClientID:         firstString([]string{"client_id"}, credential, metadata, payload),
		Name:             firstString([]string{"name"}, credential, metadata, payload),
		PlanType:         firstString([]string{"plan_type"}, credential, metadata, payload),
		ChatGPTPlanType:  firstString([]string{"chatgpt_plan_type"}, credential, metadata, payload),
		LastRefresh:      firstString([]string{"last_refresh"}, credential, metadata, payload),
		Email:            firstString([]string{"email"}, credential, metadata, payload),
		Type:             "codex",
		Expired:          firstString([]string{"expired"}, credential, metadata, payload),
	}
	if key.AccessToken == "" {
		return "", errors.New("access_token is required")
	}
	enrichOAuthKeyFromJWT(&key, key.IDToken)
	enrichOAuthKeyFromJWT(&key, key.AccessToken)
	if key.AccountID == "" {
		return "", errors.New("account_id is required")
	}
	if key.Email == "" {
		key.Email = emailFromAccessToken(key.AccessToken)
	}

	encoded, err := common.Marshal(key)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func firstString(keys []string, values ...map[string]any) string {
	for _, data := range values {
		for _, key := range keys {
			if value := stringValue(data[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func firstStringByKeyPriority(keys []string, values ...map[string]any) string {
	for _, key := range keys {
		for _, data := range values {
			if value := stringValue(data[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func emailFromAccessToken(token string) string {
	return stringValue(jwtClaims(token)["email"])
}

func enrichOAuthKeyFromJWT(key *OAuthKey, token string) {
	claims := jwtClaims(token)
	if len(claims) == 0 {
		return
	}
	if key.Email == "" {
		key.Email = stringValue(claims["email"])
		if profile, ok := claims["https://api.openai.com/profile"].(map[string]any); ok {
			key.Email = firstString([]string{"email"}, profile)
		}
	}
	auth, ok := claims["https://api.openai.com/auth"].(map[string]any)
	if !ok {
		return
	}
	if key.ChatGPTAccountID == "" {
		key.ChatGPTAccountID = stringValue(auth["chatgpt_account_id"])
	}
	if key.ChatGPTUserID == "" {
		key.ChatGPTUserID = firstString([]string{"chatgpt_user_id", "user_id"}, auth)
	}
	if key.PlanType == "" {
		key.PlanType = stringValue(auth["chatgpt_plan_type"])
	}
	if key.ChatGPTPlanType == "" {
		key.ChatGPTPlanType = stringValue(auth["chatgpt_plan_type"])
	}
}

func jwtClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if err := common.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

func ParseOAuthKeyForChannel(raw string, channelID int) (*OAuthKey, error) {
	if raw == "" {
		return nil, errors.New("codex channel: empty oauth key")
	}
	plain, err := common.DecryptCodexOAuthKeyForChannel(raw, channelID)
	if err != nil {
		return nil, err
	}
	var key OAuthKey
	if err := common.Unmarshal([]byte(plain), &key); err != nil {
		return nil, errors.New("codex channel: invalid oauth key json")
	}
	return &key, nil
}
