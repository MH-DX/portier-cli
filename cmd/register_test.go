package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestRegisterPersistsApiURLForSubsequentRun(t *testing.T) {
	const (
		accessToken = "test-access-token"
		deviceName  = "lab-device_1"
		deviceGUID  = "18d642f6-0262-492b-b3c7-ab3551bb2af3"
		apiKey      = "03c91b77-5efa-4aee-9423-0fc5f152c99f"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+accessToken, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/device":
			require.Equal(t, http.MethodPost, r.Method)
			var request struct {
				Name string
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, deviceName, request.Name)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"GUID":          deviceGUID,
				"Name":          deviceName,
				"Networks":      []string{"7641f4a7-f3a3-4e23-8765-9a07dce64064"},
				"OutboundBytes": 0,
				"LastSeen":      time.Now().UTC(),
			})
		case "/api/device/" + deviceGUID + "/apikey":
			require.Equal(t, http.MethodPost, r.Method)
			var request struct {
				DeviceGUID  string
				Description string
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, deviceGUID, request.DeviceGUID)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"GUID":        "da2c9738-e46f-4330-a4ad-cfddf9f2080e",
				"CreatedAt":   time.Now().UTC(),
				"DeviceGUID":  deviceGUID,
				"Description": request.Description,
				"ApiKey":      apiKey,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	homeDir := t.TempDir()
	writeAccessToken(t, homeDir, accessToken)

	command := newRegisterCmd()
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{
		"--home", homeDir,
		"--apiUrl", server.URL,
		"--name", deviceName,
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

func TestRegisterExistingApiKeyPersistsApiURL(t *testing.T) {
	const (
		apiKey     = "existing-api-key"
		deviceGUID = "18d642f6-0262-492b-b3c7-ab3551bb2af3"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/spider/whoami", r.URL.Path)
		require.Equal(t, apiKey, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"GUID":     deviceGUID,
			"Name":     "existing-device",
			"UserGUID": "1631197b-0c41-4f85-981b-52b0cc9111c3",
			"Networks": []string{"7641f4a7-f3a3-4e23-8765-9a07dce64064"},
		})
	}))
	defer server.Close()

	homeDir := t.TempDir()
	command := newRegisterCmd()
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{
		"--home", homeDir,
		"--apiUrl", server.URL,
		"--apiKey", apiKey,
		"--no-tls",
	})

	require.NoError(t, command.Execute())

	cfg, err := config.LoadConfig(filepath.Join(homeDir, "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, server.URL, cfg.APIBaseURL())
}

func writeAccessToken(t *testing.T, homeDir string, accessToken string) {
	t.Helper()
	data := []byte("access_token: \"" + accessToken + "\"\nrefresh_token: \"refresh-token\"\n")
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, "credentials.yaml"), data, 0o600))
}
