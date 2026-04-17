package endpoints

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURLStripsLegacyRelayPathAndKeepsPrefix(t *testing.T) {
	require.Equal(
		t,
		"https://gateway.example.test/portier",
		NormalizeBaseURL("wss://gateway.example.test/portier/task-spider/199b6650-b1d7-45f9-8627-08cec46c9af6"),
	)
	require.Equal(
		t,
		"https://gateway.example.test/portier",
		NormalizeBaseURL("wss://gateway.example.test/portier/api/tasks/199b6650-b1d7-45f9-8627-08cec46c9af6/spider"),
	)
	require.Equal(
		t,
		"https://gateway.example.test/portier",
		NormalizeBaseURL("wss://gateway.example.test/portier/spider"),
	)
}

func TestRelayWebsocketURLUsesBaseURLPrefix(t *testing.T) {
	relayURL, err := RelayWebsocketURL("https://gateway.example.test/portier", "/spider")
	require.NoError(t, err)
	require.Equal(t, "wss://gateway.example.test/portier/spider", relayURL)

	taskRelayURL, err := RelayWebsocketURL("https://gateway.example.test/portier", "/task-spider/task-guid")
	require.NoError(t, err)
	require.Equal(t, "wss://gateway.example.test/portier/task-spider/task-guid", taskRelayURL)
}
