package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type storedDeviceCredentials struct {
	APIKey string `yaml:"APIKey"`
}

func TestCustomerSetupSuccess(t *testing.T) {
	_, _, caPEM := newTestCertificateAuthority(t)

	const (
		deviceAPIKey = "device-api-key"
		deviceGUID   = "d1b9c2af-ff76-43f1-8347-3b5db88d67bc"
		customerGUID = "0a0cc2b5-8c53-4c08-bff9-a2f655b6dd0a"
	)

	server := newCustomerSetupTestServer(t, customerGUID, deviceAPIKey, deviceGUID, []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"}, caPEM)
	defer server.Close()

	homeDir := t.TempDir()
	output := &bytes.Buffer{}

	command := newCustomerSetupCmd()
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{
		"--home", homeDir,
		"--apiUrl", server.URL,
		"--apiKey", deviceAPIKey,
		"--customer-guid", customerGUID,
	})

	require.NoError(t, command.Execute())

	var credentials storedDeviceCredentials
	credentialsBytes, err := os.ReadFile(filepath.Join(homeDir, "credentials_device.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(credentialsBytes, &credentials))
	require.Equal(t, deviceAPIKey, credentials.APIKey)

	caBundleBytes, err := os.ReadFile(filepath.Join(homeDir, "cacert.pem"))
	require.NoError(t, err)
	require.Equal(t, string(caPEM), string(caBundleBytes))

	cfg, err := config.LoadConfig(filepath.Join(homeDir, "config.yaml"))
	require.NoError(t, err)
	require.True(t, cfg.TLSEnabled)
	require.Equal(t, filepath.Join(homeDir, "cacert.pem"), cfg.PTLSConfig.CAFile)
	require.Equal(t, filepath.Join(homeDir, "cert.pem"), cfg.PTLSConfig.CertFile)
	require.Equal(t, filepath.Join(homeDir, "key.pem"), cfg.PTLSConfig.KeyFile)

	metadataBytes, err := os.ReadFile(filepath.Join(homeDir, "customer_setup.yaml"))
	require.NoError(t, err)
	var metadata customerSetupMetadata
	require.NoError(t, yaml.Unmarshal(metadataBytes, &metadata))
	require.Equal(t, customerGUID, metadata.CustomerGUID)
	require.Equal(t, deviceGUID, metadata.DeviceGUID)
	require.Equal(t, filepath.Join(homeDir, "cacert.pem"), metadata.CACertFile)

	stdout := output.String()
	require.Contains(t, stdout, "Customer setup complete.")
	require.Contains(t, stdout, "Customer: "+customerGUID)
	require.Contains(t, stdout, "CA certificates: 1")
	require.Contains(t, stdout, "Note: configure a device certificate and key")
}

func TestCustomerSetupUsesStoredCustomerMetadata(t *testing.T) {
	_, _, caPEM := newTestCertificateAuthority(t)

	const (
		deviceAPIKey = "device-api-key"
		deviceGUID   = "d1b9c2af-ff76-43f1-8347-3b5db88d67bc"
		customerGUID = "0a0cc2b5-8c53-4c08-bff9-a2f655b6dd0a"
	)

	server := newCustomerSetupTestServer(t, customerGUID, deviceAPIKey, deviceGUID, []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"}, caPEM)
	defer server.Close()

	homeDir := t.TempDir()
	metadataBytes, err := yaml.Marshal(&customerSetupMetadata{
		APIURL:       server.URL,
		CustomerGUID: customerGUID,
		DeviceGUID:   deviceGUID,
		Networks:     []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"},
		CACertFile:   filepath.Join(homeDir, "cacert.pem"),
		UpdatedAt:    "2026-03-31T10:00:00Z",
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, "customer_setup.yaml"), metadataBytes, 0o600))

	command := newCustomerSetupCmd()
	command.SetArgs([]string{
		"--home", homeDir,
		"--apiUrl", server.URL,
		"--apiKey", deviceAPIKey,
	})

	require.NoError(t, command.Execute())
}

func TestCustomerSetupRequiresCustomerGUIDBootstrapWhenNoMetadataExists(t *testing.T) {
	_, _, caPEM := newTestCertificateAuthority(t)

	const (
		deviceAPIKey = "device-api-key"
		deviceGUID   = "d1b9c2af-ff76-43f1-8347-3b5db88d67bc"
		customerGUID = "0a0cc2b5-8c53-4c08-bff9-a2f655b6dd0a"
	)

	server := newCustomerSetupTestServer(t, customerGUID, deviceAPIKey, deviceGUID, []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"}, caPEM)
	defer server.Close()

	command := newCustomerSetupCmd()
	command.SetArgs([]string{
		"--home", t.TempDir(),
		"--apiUrl", server.URL,
		"--apiKey", deviceAPIKey,
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "customer GUID could not be determined from device API key alone")
	require.Contains(t, err.Error(), "pass --customer-guid once")
}

func TestCustomerSetupRejectsMalformedCABundle(t *testing.T) {
	const (
		deviceAPIKey = "device-api-key"
		deviceGUID   = "d1b9c2af-ff76-43f1-8347-3b5db88d67bc"
		customerGUID = "0a0cc2b5-8c53-4c08-bff9-a2f655b6dd0a"
	)

	server := newCustomerSetupTestServer(t, customerGUID, deviceAPIKey, deviceGUID, []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"}, []byte("not a certificate"))
	defer server.Close()

	command := newCustomerSetupCmd()
	command.SetArgs([]string{
		"--home", t.TempDir(),
		"--apiUrl", server.URL,
		"--apiKey", deviceAPIKey,
		"--customer-guid", customerGUID,
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "customer CA bundle did not contain any certificates")
}

func newCustomerSetupTestServer(t *testing.T, customerGUID, expectedAPIKey, deviceGUID string, networks []string, bundlePEM []byte) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spider/whoami":
			require.Equal(t, expectedAPIKey, r.Header.Get("Authorization"))
			writeJSON(t, w, http.StatusOK, map[string]any{
				"GUID":     deviceGUID,
				"Networks": networks,
			})
		case "/api/customers/" + customerGUID + "/ca-certificates/current.pem":
			w.Header().Set("Content-Type", "application/x-pem-file")
			_, err := w.Write(bundlePEM)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
}
