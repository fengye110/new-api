package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddDefaultCodexModels(t *testing.T) {
	channel := &model.Channel{
		Type:   constant.ChannelTypeCodex,
		Models: "custom-model,gpt-5.4",
	}

	addDefaultCodexModels(channel)

	assert.ElementsMatch(t,
		append([]string{"custom-model"}, codex.DefaultModelList...),
		channel.GetModels(),
	)
	require.NotNil(t, channel.TestModel)
	assert.Equal(t, "gpt-5.4-mini", *channel.TestModel)
}

func TestAddDefaultCodexModelsLeavesOtherChannelsUnchanged(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Models: "custom-model"}

	addDefaultCodexModels(channel)

	assert.Equal(t, "custom-model", channel.Models)
}
