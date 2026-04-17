package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestRegisterTokenStoresCredentialsForNormalDevice(t *testing.T) {
	const (
		token      = "ptr_token_secret"
		deviceGUID = "18d642f6-0262-492b-b3c7-ab3551bb2af3"
		apiKey     = "registration-api-key"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/device-registration-tokens/exchange", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		var request struct {
			Token string `json:"token"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, token, request.Token)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_guid": deviceGUID,
			"api_key":     apiKey,
		})
	}))
	defer server.Close()

	homeDir := t.TempDir()
	command := newRegisterTokenCmd()
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{
		"--home", homeDir,
		"--apiUrl", server.URL,
		"--token", token,
		"--no-tls",
	})

	require.NoError(t, command.Execute())

	var credentials storedDeviceCredentials
	credentialsBytes, err := os.ReadFile(filepath.Join(homeDir, "credentials_device.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(credentialsBytes, &credentials))
	require.Equal(t, apiKey, credentials.APIKey)

	cfg, err := config.LoadConfig(filepath.Join(homeDir, "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, server.URL, cfg.APIBaseURL())
	require.Equal(t, "/spider", cfg.RelayPath)
}
