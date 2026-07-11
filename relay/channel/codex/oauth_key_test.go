package codex

import (
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func codexTestAccessToken(t *testing.T, accountID string) string {
	t.Helper()
	payload := `{"https://api.openai.com/auth":{"chatgpt_account_id":"` + accountID + `"},"email":"test@example.com"}`
	return "header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".signature"
}

func TestNormalizeOAuthKeySupportsCommonExportFormats(t *testing.T) {
	accessToken := codexTestAccessToken(t, "account-from-token")
	tests := []struct {
		name              string
		raw               string
		expectedAccountID string
		expectedEmail     string
	}{
		{
			name:              "account pool export",
			raw:               `{"accounts":[{"platform":"openai","credentials":{"access_token":"` + accessToken + `","refresh_token":"refresh-pool","chatgpt_account_id":"account-from-credential"}}]}`,
			expectedAccountID: "account-from-credential",
			expectedEmail:     "test@example.com",
		},
		{
			name:              "flat codex export",
			raw:               `{"type":"codex","access_token":"` + accessToken + `","refresh_token":"refresh-flat","account_id":"account-flat"}`,
			expectedAccountID: "account-flat",
			expectedEmail:     "test@example.com",
		},
		{
			name:              "opencode oauth export",
			raw:               `{"openai":{"type":"oauth","access":"` + accessToken + `","refresh":"refresh-open-code","accountId":"account-open-code"}}`,
			expectedAccountID: "account-open-code",
			expectedEmail:     "test@example.com",
		},
		{
			name:              "account pool chatgpt user id fallback",
			raw:               `{"accounts":[{"platform":"openai","credentials":{"access_token":"header.e30.signature","refresh_token":"refresh-pool","chatgpt_user_id":"user-fallback"}}]}`,
			expectedAccountID: "user-fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := NormalizeOAuthKey(tt.raw)
			require.NoError(t, err)

			var key OAuthKey
			require.NoError(t, common.Unmarshal([]byte(normalized), &key))
			if tt.expectedAccountID == "user-fallback" {
				assert.Equal(t, "header.e30.signature", key.AccessToken)
			} else {
				assert.Equal(t, accessToken, key.AccessToken)
			}
			assert.NotEmpty(t, key.RefreshToken)
			assert.Equal(t, tt.expectedAccountID, key.AccountID)
			assert.Equal(t, tt.expectedEmail, key.Email)
			if tt.expectedAccountID == "user-fallback" {
				assert.Equal(t, "user-fallback", key.ChatGPTUserID)
			}
			assert.Equal(t, "codex", key.Type)
		})
	}
}

func TestNormalizeOAuthKeyRequiresConfiguredAccountIdentifier(t *testing.T) {
	_, err := NormalizeOAuthKey(`{"access_token":"header.e30.signature","refresh_token":"refresh"}`)

	require.EqualError(t, err, "account_id is required")
}
