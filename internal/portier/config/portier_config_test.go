package config

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigUsesBaseURLAndDefaultRelayPath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("baseUrl: https://api-staging.portier.dev\n"), 0o644))

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "https://api-staging.portier.dev", cfg.APIBaseURL())
	require.Equal(t, "/spider", cfg.RelayPath)

	relayURL, err := cfg.RelayURL()
	require.NoError(t, err)
	require.Equal(t, "wss://api-staging.portier.dev/spider", relayURL)
}

func TestLoadConfigMigratesLegacyPortierURL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := "portierUrl: wss://api-staging.portier.dev/api/tasks/199b6650-b1d7-45f9-8627-08cec46c9af6/spider\n"
	require.NoError(t, os.WriteFile(configPath, []byte(configYAML), 0o644))

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "https://api-staging.portier.dev", cfg.APIBaseURL())
	require.Equal(t, "/api/tasks/199b6650-b1d7-45f9-8627-08cec46c9af6/spider", cfg.RelayPath)
}

func TestLoadConfigMigratesTaskSpiderPortierURL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := "portierUrl: wss://api-staging.portier.dev/task-spider/199b6650-b1d7-45f9-8627-08cec46c9af6\n"
	require.NoError(t, os.WriteFile(configPath, []byte(configYAML), 0o644))

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "https://api-staging.portier.dev", cfg.APIBaseURL())
	require.Equal(t, "/task-spider/199b6650-b1d7-45f9-8627-08cec46c9af6", cfg.RelayPath)
}

func TestLoadConfigMigratesLegacyPortierURLWithBasePathPrefix(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := "portierUrl: wss://gateway.example.test/portier/api/tasks/199b6650-b1d7-45f9-8627-08cec46c9af6/spider\n"
	require.NoError(t, os.WriteFile(configPath, []byte(configYAML), 0o644))

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example.test/portier", cfg.APIBaseURL())
	require.Equal(t, "/api/tasks/199b6650-b1d7-45f9-8627-08cec46c9af6/spider", cfg.RelayPath)
}

func TestLoadConfigMigratesTaskSpiderPortierURLWithBasePathPrefix(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := "portierUrl: wss://gateway.example.test/portier/task-spider/199b6650-b1d7-45f9-8627-08cec46c9af6\n"
	require.NoError(t, os.WriteFile(configPath, []byte(configYAML), 0o644))

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example.test/portier", cfg.APIBaseURL())
	require.Equal(t, "/task-spider/199b6650-b1d7-45f9-8627-08cec46c9af6", cfg.RelayPath)
}

func TestSaveConfigWritesBaseURLOnly(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := DefaultPortierConfig()
	require.NoError(t, err)

	baseURL, err := url.Parse("https://api-staging.portier.dev")
	require.NoError(t, err)
	cfg.BaseURL = utils.YAMLURL{URL: baseURL}
	cfg.RelayPath = "/spider"

	require.NoError(t, SaveConfig(configPath, cfg))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)
	require.Contains(t, content, "baseUrl: https://api-staging.portier.dev")
	require.NotContains(t, content, "portierUrl:")
}

func TestAPIBaseURLFromPortierURLNormalizesLegacyValues(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "websocket spider URL",
			raw:  "wss://api-staging.portier.dev/spider",
			want: "https://api-staging.portier.dev",
		},
		{
			name: "api URL",
			raw:  "https://api-staging.portier.dev/api",
			want: "https://api-staging.portier.dev",
		},
		{
			name: "empty defaults to production",
			raw:  "",
			want: "https://api.portier.dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, APIBaseURLFromPortierURL(tt.raw))
		})
	}
}

func TestParseConfiguredConnectionRejectsInvalidBaseURL(t *testing.T) {
	_, _, err := parseConfiguredConnection("://bad", "")
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid base URL"))
}
