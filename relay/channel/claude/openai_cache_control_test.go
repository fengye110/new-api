package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestRequestOpenAI2ClaudeMessagePreservesCacheControl(t *testing.T) {
	cacheControl, err := common.Marshal(map[string]string{"type": "ephemeral"})
	require.NoError(t, err)

	systemMessage := dto.Message{Role: "system"}
	systemMessage.SetMediaContent([]dto.MediaContent{{
		Type:         "text",
		Text:         "stable system prompt",
		CacheControl: cacheControl,
	}})
	userMessage := dto.Message{Role: "user"}
	userMessage.SetMediaContent([]dto.MediaContent{{
		Type:         "text",
		Text:         "stable context",
		CacheControl: cacheControl,
	}})
	request := dto.GeneralOpenAIRequest{
		Model:    "claude-sonnet-4-5",
		Messages: []dto.Message{systemMessage, userMessage},
	}

	converted, err := RequestOpenAI2ClaudeMessage(nil, request)
	require.NoError(t, err)
	system := converted.ParseSystem()
	require.Len(t, system, 1)
	require.JSONEq(t, `{"type":"ephemeral"}`, string(system[0].CacheControl))
	require.Len(t, converted.Messages, 1)
	content, ok := converted.Messages[0].Content.([]dto.ClaudeMediaMessage)
	require.True(t, ok)
	require.Len(t, content, 1)
	require.JSONEq(t, `{"type":"ephemeral"}`, string(content[0].CacheControl))
}
