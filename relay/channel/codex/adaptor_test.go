package codex

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestDefaultsCodexFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("Session_id", "sess-123")

	input, err := common.Marshal("hello")
	require.NoError(t, err)

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponses, ChannelMeta: &relaycommon.ChannelMeta{}}, dto.OpenAIResponsesRequest{
		Model: "gpt-5.4",
		Input: input,
	})
	require.NoError(t, err)

	req := converted.(dto.OpenAIResponsesRequest)
	require.JSONEq(t, `[{"content":"hello","role":"user"}]`, string(req.Input))
	require.JSONEq(t, `""`, string(req.Instructions))
	require.JSONEq(t, `false`, string(req.Store))
	require.True(t, *req.Stream)
	require.JSONEq(t, `"sess-123"`, string(req.PromptCacheKey))
	require.Nil(t, req.MaxOutputTokens)
	require.Nil(t, req.Temperature)
	require.Nil(t, req.TopP)
}

func TestConvertOpenAIResponsesRequestKeepsExistingPromptCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("Session_id", "sess-123")

	input, err := common.Marshal([]map[string]string{{"role": "user", "content": "hello"}})
	require.NoError(t, err)
	cacheKey, err := common.Marshal("client-cache")
	require.NoError(t, err)

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponses, ChannelMeta: &relaycommon.ChannelMeta{}}, dto.OpenAIResponsesRequest{
		Model:          "gpt-5.4",
		Input:          input,
		PromptCacheKey: cacheKey,
	})
	require.NoError(t, err)

	req := converted.(dto.OpenAIResponsesRequest)
	require.JSONEq(t, `"client-cache"`, string(req.PromptCacheKey))
}
