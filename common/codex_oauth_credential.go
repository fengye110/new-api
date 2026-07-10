package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	codexCredentialKeyEnv = "CHATGPT_OAUTH_CREDENTIAL_KEY"
	codexCredentialPrefix = "codex-oauth:v1:"
)

func IsEncryptedCodexOAuthKey(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), codexCredentialPrefix)
}

func EncryptCodexOAuthKeyForChannel(raw string, channelID int) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("codex channel: empty oauth key")
	}
	key, err := getCredentialCipherKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	payload := append(nonce, gcm.Seal(nil, nonce, []byte(raw), credentialAdditionalData(channelID))...)
	return codexCredentialPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func DecryptCodexOAuthKeyForChannel(raw string, channelID int) (string, error) {
	raw = strings.TrimSpace(raw)
	if !IsEncryptedCodexOAuthKey(raw) {
		return raw, nil
	}
	key, err := getCredentialCipherKey()
	if err != nil {
		return "", err
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(raw, codexCredentialPrefix))
	if err != nil {
		return "", errors.New("codex channel: invalid encrypted oauth key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", errors.New("codex channel: invalid encrypted oauth key")
	}
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], credentialAdditionalData(channelID))
	if err != nil {
		return "", errors.New("codex channel: cannot decrypt oauth key")
	}
	return string(plain), nil
}

func getCredentialCipherKey() ([]byte, error) {
	encoded := strings.TrimSpace(os.Getenv(codexCredentialKeyEnv))
	if encoded == "" {
		return nil, fmt.Errorf("codex channel: %s is required for encrypted credentials", codexCredentialKeyEnv)
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("codex channel: %s must be base64-encoded 32 bytes", codexCredentialKeyEnv)
	}
	return key, nil
}

func credentialAdditionalData(channelID int) []byte {
	return []byte(fmt.Sprintf("codex-oauth:channel:%d", channelID))
}
