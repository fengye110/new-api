package common

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncryptedOAuthKeyRoundTrip(t *testing.T) {
	t.Setenv(codexCredentialKeyEnv, base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))

	raw := `{"access_token":"access","refresh_token":"refresh","account_id":"account"}`
	encrypted, err := EncryptCodexOAuthKeyForChannel(raw, 12)
	require.NoError(t, err)
	require.True(t, IsEncryptedCodexOAuthKey(encrypted))
	require.NotContains(t, encrypted, "access")

	plain, err := DecryptCodexOAuthKeyForChannel(encrypted, 12)
	require.NoError(t, err)
	require.Equal(t, raw, plain)

	_, err = DecryptCodexOAuthKeyForChannel(encrypted, 13)
	require.Error(t, err)
}

func TestEncryptedOAuthKeyRequiresValidKey(t *testing.T) {
	t.Setenv(codexCredentialKeyEnv, "invalid")

	_, err := EncryptCodexOAuthKeyForChannel(`{"access_token":"access"}`, 1)
	require.Error(t, err)
}
