package zhipu_4v

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestRequestOpenAI2ZhipuPreservesStreamOptionsForUsage(t *testing.T) {
	stream := true
	includeUsage := true
	maxTokens := uint(128)
	input := dto.GeneralOpenAIRequest{
		Model:  "glm-4.5",
		Stream: &stream,
		StreamOptions: &dto.StreamOptions{
			IncludeUsage: includeUsage,
		},
		Messages:  []dto.Message{{Role: "user", Content: "hello"}},
		MaxTokens: &maxTokens,
	}

	out := requestOpenAI2Zhipu(input)

	require.NotNil(t, out.StreamOptions)
	require.True(t, out.StreamOptions.IncludeUsage)
	require.NotNil(t, out.MaxTokens)
	require.Equal(t, maxTokens, *out.MaxTokens)
}

func TestRequestOpenAI2ZhipuPreservesReasoningEffort(t *testing.T) {
	thinking, err := common.Marshal(map[string]string{"type": "enabled"})
	require.NoError(t, err)
	input := dto.GeneralOpenAIRequest{
		Model:           "glm-4.5",
		Messages:        []dto.Message{{Role: "user", Content: "hello"}},
		ReasoningEffort: "low",
		THINKING:        thinking,
	}

	out := requestOpenAI2Zhipu(input)

	require.Equal(t, "low", out.ReasoningEffort)
	require.JSONEq(t, `{"type":"enabled"}`, string(out.THINKING))
}
