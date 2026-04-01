package cmd

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/portier/taskcert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type fakeTaskStartApp struct {
	t            *testing.T
	startErr     error
	startCalls   int
	stopCalls    int
	config       *config.PortierConfig
	credentials  *config.DeviceCredentials
	fullChainPEM string
}

func (f *fakeTaskStartApp) StartServices(cfg *config.PortierConfig, creds *config.DeviceCredentials) error {
	f.startCalls++
	f.config = cfg
	f.credentials = creds

	if cfg != nil {
		certificateBundle, err := os.ReadFile(cfg.PTLSConfig.CertFile)
		require.NoError(f.t, err)
		f.fullChainPEM = string(certificateBundle)
	}

	return f.startErr
}

func (f *fakeTaskStartApp) StopServices() error {
	f.stopCalls++
	return nil
}

func TestTaskStartUsesTaskTokenForWebsocketAndPTLSForTarget(t *testing.T) {
	taskGUID := "7ad5ae0e-f966-4b63-92d0-f994d50bce40"
	taskToken := "task-token"
	deviceGUIDs := []string{
		"8e6f919a-f4fa-4bb2-a99b-31d33431c437",
		"35dfcd4e-e209-4d6d-aa54-41c0eeec8226",
	}

	homeDir := t.TempDir()
	writeTaskStartMetadata(t, homeDir, &taskcert.Metadata{
		APIURL:       "https://api-staging.portier.dev/",
		TaskGUID:     taskGUID,
		TaskToken:    taskToken,
		DeviceGUIDs:  deviceGUIDs,
		Scope:        "ssh://operator@localhost:22",
		NotBefore:    "2026-03-31T10:00:00Z",
		NotAfter:     "2026-03-31T10:15:00Z",
		CustomerGUID: "bd7af1d4-89ab-49b3-a089-f521c6d9678e",
	})

	fakeApp := &fakeTaskStartApp{t: t}
	restoreTaskStartHooks := installTaskStartHooks(fakeApp)
	defer restoreTaskStartHooks()

	output := &bytes.Buffer{}
	command := newTaskStartCmd()
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{
		"--home", homeDir,
		"--task-guid", taskGUID,
		"--device-guid", deviceGUIDs[1],
	})

	require.NoError(t, command.Execute())
	require.Equal(t, 1, fakeApp.startCalls)
	require.Equal(t, 1, fakeApp.stopCalls)
	require.NotNil(t, fakeApp.config)
	require.NotNil(t, fakeApp.credentials)
	require.Equal(t, "Bearer "+taskToken, fakeApp.credentials.ApiToken)
	require.NotEqual(t, uuid.Nil, fakeApp.credentials.DeviceID)
	require.Equal(t, "https://api-staging.portier.dev", fakeApp.config.APIBaseURL())
	require.Equal(t, "/task-spider/"+taskGUID, fakeApp.config.RelayPath)
	relayURL, err := fakeApp.config.RelayURL()
	require.NoError(t, err)
	require.Equal(t, "wss://api-staging.portier.dev/task-spider/"+taskGUID, relayURL)
	require.Len(t, fakeApp.config.Services, 1)
	require.Equal(t, "tcp://127.0.0.1:10022", fakeApp.config.Services[0].Options.URLLocal.String())
	require.Equal(t, "ssh://operator@localhost:22", fakeApp.config.Services[0].Options.URLRemote.String())
	require.Equal(t, deviceGUIDs[1], fakeApp.config.Services[0].Options.PeerDeviceID.String())
	require.Contains(t, fakeApp.config.PTLSConfig.KeyFile, filepath.Join("tasks", taskGUID, "private-key.pem"))
	require.Contains(t, fakeApp.config.PTLSConfig.CAFile, filepath.Join("tasks", taskGUID, "certificate-chain.pem"))
	require.Equal(t, 2, strings.Count(fakeApp.fullChainPEM, "BEGIN CERTIFICATE"))

	stdout := output.String()
	require.Contains(t, stdout, "Task session started.")
	require.Contains(t, stdout, "Listening on: tcp://127.0.0.1:10022")
	require.Contains(t, stdout, "Target device: "+deviceGUIDs[1])
	require.Contains(t, stdout, "Scope: ssh://operator@localhost:22")
}

func TestTaskStartRequiresDeviceSelectionWhenTaskHasMultipleDevices(t *testing.T) {
	taskGUID := "7ad5ae0e-f966-4b63-92d0-f994d50bce40"
	homeDir := t.TempDir()
	writeTaskStartMetadata(t, homeDir, &taskcert.Metadata{
		APIURL:      "https://api-staging.portier.dev/",
		TaskGUID:    taskGUID,
		TaskToken:   "task-token",
		DeviceGUIDs: []string{"8e6f919a-f4fa-4bb2-a99b-31d33431c437", "35dfcd4e-e209-4d6d-aa54-41c0eeec8226"},
		Scope:       "ssh://operator@localhost:22",
	})

	fakeApp := &fakeTaskStartApp{t: t}
	restoreTaskStartHooks := installTaskStartHooks(fakeApp)
	defer restoreTaskStartHooks()

	command := newTaskStartCmd()
	command.SetArgs([]string{
		"--home", homeDir,
		"--task-guid", taskGUID,
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--device-guid is required")
	require.Equal(t, 0, fakeApp.startCalls)
}

func TestTaskStartSurfacesApplicationStartErrors(t *testing.T) {
	taskGUID := "7ad5ae0e-f966-4b63-92d0-f994d50bce40"
	homeDir := t.TempDir()
	writeTaskStartMetadata(t, homeDir, &taskcert.Metadata{
		APIURL:      "https://api-staging.portier.dev/",
		TaskGUID:    taskGUID,
		TaskToken:   "task-token",
		DeviceGUIDs: []string{"8e6f919a-f4fa-4bb2-a99b-31d33431c437"},
		Scope:       "ssh://operator@localhost:22",
	})

	fakeApp := &fakeTaskStartApp{
		t:        t,
		startErr: fmt.Errorf("websocket handshake failed: 403 Forbidden: task window expired"),
	}
	restoreTaskStartHooks := installTaskStartHooks(fakeApp)
	defer restoreTaskStartHooks()

	command := newTaskStartCmd()
	command.SetArgs([]string{
		"--home", homeDir,
		"--task-guid", taskGUID,
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "task window expired")
	require.Equal(t, 1, fakeApp.startCalls)
	require.Equal(t, 0, fakeApp.stopCalls)
}

func TestTaskStartAutoEnrollsWhenTaskMetadataIsMissing(t *testing.T) {
	taskGUID := "7ad5ae0e-f966-4b63-92d0-f994d50bce40"
	taskToken := "task-token"
	deviceGUID := "8e6f919a-f4fa-4bb2-a99b-31d33431c437"
	notBefore := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	notAfter := time.Date(2026, 3, 31, 10, 15, 0, 0, time.UTC)

	var infoCalls int
	var signCalls int
	server := newTaskStartEnrollmentServer(t, taskGUID, taskToken, []string{deviceGUID}, "ssh://operator@localhost:22", notBefore, notAfter, &infoCalls, &signCalls)
	defer server.Close()

	homeDir := t.TempDir()
	fakeApp := &fakeTaskStartApp{t: t}
	restoreTaskStartHooks := installTaskStartHooks(fakeApp)
	defer restoreTaskStartHooks()

	output := &bytes.Buffer{}
	command := newTaskStartCmd()
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{
		"--home", homeDir,
		"--task-guid", taskGUID,
		"--task-token", taskToken,
		"--apiUrl", server.URL,
	})

	require.NoError(t, command.Execute())
	require.Equal(t, 1, infoCalls)
	require.Equal(t, 1, signCalls)

	metadata, err := taskcert.LoadMetadata(filepath.Join(homeDir, "tasks", taskGUID, "metadata.yaml"))
	require.NoError(t, err)
	require.Equal(t, taskToken, metadata.TaskToken)
	require.Equal(t, notBefore.Format(time.RFC3339), metadata.NotBefore)
	require.Equal(t, notAfter.Format(time.RFC3339), metadata.NotAfter)

	stdout := output.String()
	require.Contains(t, stdout, "No enrolled task certificate was found. Enrolling now...")
	require.Contains(t, stdout, "Task session started.")
}

func TestTaskStartPromptsForTaskTokenWhenEnrollmentIsNeeded(t *testing.T) {
	taskGUID := "7ad5ae0e-f966-4b63-92d0-f994d50bce40"
	taskToken := "prompted-task-token"
	deviceGUID := "8e6f919a-f4fa-4bb2-a99b-31d33431c437"
	notBefore := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	notAfter := time.Date(2026, 3, 31, 10, 15, 0, 0, time.UTC)

	server := newTaskStartEnrollmentServer(t, taskGUID, taskToken, []string{deviceGUID}, "ssh://operator@localhost:22", notBefore, notAfter, nil, nil)
	defer server.Close()

	homeDir := t.TempDir()
	fakeApp := &fakeTaskStartApp{t: t}
	restoreTaskStartHooks := installTaskStartHooks(fakeApp)
	defer restoreTaskStartHooks()

	promptCalls := 0
	promptForTaskStartToken = func(requestedTaskGUID string) (string, error) {
		promptCalls++
		require.Equal(t, taskGUID, requestedTaskGUID)
		return taskToken, nil
	}

	command := newTaskStartCmd()
	command.SetArgs([]string{
		"--home", homeDir,
		"--task-guid", taskGUID,
		"--apiUrl", server.URL,
	})

	require.NoError(t, command.Execute())
	require.Equal(t, 1, promptCalls)
}

func TestTaskStartReenrollsWhenExistingTaskCertificateHasExpired(t *testing.T) {
	taskGUID := "7ad5ae0e-f966-4b63-92d0-f994d50bce40"
	taskToken := "stored-task-token"
	deviceGUID := "8e6f919a-f4fa-4bb2-a99b-31d33431c437"

	homeDir := t.TempDir()
	writeTaskStartMetadata(t, homeDir, &taskcert.Metadata{
		APIURL:       "https://api-staging.portier.dev/",
		TaskGUID:     taskGUID,
		TaskToken:    taskToken,
		DeviceGUIDs:  []string{deviceGUID},
		Scope:        "ssh://operator@localhost:22",
		NotBefore:    "2026-03-31T10:00:00Z",
		NotAfter:     "2026-03-31T10:15:00Z",
		CustomerGUID: "bd7af1d4-89ab-49b3-a089-f521c6d9678e",
	})

	refreshedNotBefore := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	refreshedNotAfter := time.Date(2026, 4, 1, 10, 15, 0, 0, time.UTC)
	var infoCalls int
	var signCalls int
	server := newTaskStartEnrollmentServer(t, taskGUID, taskToken, []string{deviceGUID}, "ssh://operator@localhost:22", refreshedNotBefore, refreshedNotAfter, &infoCalls, &signCalls)
	defer server.Close()

	fakeApp := &fakeTaskStartApp{t: t}
	restoreTaskStartHooks := installTaskStartHooks(fakeApp)
	defer restoreTaskStartHooks()
	taskStartNow = func() time.Time {
		return time.Date(2026, 4, 1, 10, 5, 0, 0, time.UTC)
	}

	output := &bytes.Buffer{}
	command := newTaskStartCmd()
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{
		"--home", homeDir,
		"--task-guid", taskGUID,
		"--apiUrl", server.URL,
	})

	require.NoError(t, command.Execute())
	require.Equal(t, 1, infoCalls)
	require.Equal(t, 1, signCalls)

	metadata, err := taskcert.LoadMetadata(filepath.Join(homeDir, "tasks", taskGUID, "metadata.yaml"))
	require.NoError(t, err)
	require.Equal(t, refreshedNotBefore.Format(time.RFC3339), metadata.NotBefore)
	require.Equal(t, refreshedNotAfter.Format(time.RFC3339), metadata.NotAfter)

	stdout := output.String()
	require.Contains(t, stdout, "Task certificate is outside its validity window")
	require.Contains(t, stdout, "Re-enrolling now...")
}

func installTaskStartHooks(app taskStartApplication) func() {
	previousFactory := newTaskStartApplication
	previousWait := waitForTaskStartShutdown
	previousPrompt := promptForTaskStartToken
	previousNow := taskStartNow

	newTaskStartApplication = func() taskStartApplication {
		return app
	}
	waitForTaskStartShutdown = func() {}
	promptForTaskStartToken = func(taskGUID string) (string, error) {
		return "", fmt.Errorf("unexpected task token prompt for %s", taskGUID)
	}
	taskStartNow = func() time.Time {
		return time.Date(2026, 3, 31, 10, 5, 0, 0, time.UTC)
	}

	return func() {
		newTaskStartApplication = previousFactory
		waitForTaskStartShutdown = previousWait
		promptForTaskStartToken = previousPrompt
		taskStartNow = previousNow
	}
}

func newTaskStartEnrollmentServer(t *testing.T, taskGUID, expectedTaskToken string, deviceGUIDs []string, scope string, notBefore, notAfter time.Time, infoCalls, signCalls *int) *httptest.Server {
	t.Helper()

	requiredURISANs := []string{
		"urn:portier:task:" + taskGUID,
		"urn:portier:not-before:" + notBefore.Format(time.RFC3339),
		"urn:portier:not-after:" + notAfter.Format(time.RFC3339),
	}
	caCertificate, caPrivateKey, caPEM := newTestCertificateAuthority(t)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/public/tasks/" + taskGUID + "/client-certificate-info":
			http.NotFound(w, r)
		case "/api/tasks/" + taskGUID + "/client-certificate-info":
			if infoCalls != nil {
				*infoCalls = *infoCalls + 1
			}

			var request struct {
				TaskToken string `json:"task_token"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, expectedTaskToken, request.TaskToken)

			writeJSON(t, w, http.StatusOK, map[string]any{
				"task_guid":         taskGUID,
				"customer_guid":     "bd7af1d4-89ab-49b3-a089-f521c6d9678e",
				"device_guids":      deviceGUIDs,
				"scope":             scope,
				"not_before":        notBefore.Format(time.RFC3339),
				"not_after":         notAfter.Format(time.RFC3339),
				"required_uri_sans": requiredURISANs,
			})
		case "/public/tasks/" + taskGUID + "/client-certificate":
			http.NotFound(w, r)
		case "/api/tasks/" + taskGUID + "/client-certificate":
			if signCalls != nil {
				*signCalls = *signCalls + 1
			}

			var request struct {
				TaskToken string `json:"task_token"`
				CSR       string `json:"csr"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, expectedTaskToken, request.TaskToken)

			csr := parseCSRFromPEM(t, request.CSR)
			assertCSRContainsURIs(t, csr, requiredURISANs)

			certificatePEM := signTestTaskCertificate(
				t,
				caCertificate,
				caPrivateKey,
				csr,
				notBefore,
				notAfter,
				"bd7af1d4-89ab-49b3-a089-f521c6d9678e",
				deviceGUIDs,
				requiredURISANs,
			)

			writeJSON(t, w, http.StatusOK, map[string]any{
				"task_guid":         taskGUID,
				"customer_guid":     "bd7af1d4-89ab-49b3-a089-f521c6d9678e",
				"device_guids":      deviceGUIDs,
				"scope":             scope,
				"certificate":       string(certificatePEM),
				"certificate_chain": string(caPEM),
				"not_before":        notBefore.Format(time.RFC3339),
				"not_after":         notAfter.Format(time.RFC3339),
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func writeTaskStartMetadata(t *testing.T, homeDir string, metadata *taskcert.Metadata) {
	t.Helper()

	taskDir := filepath.Join(homeDir, "tasks", metadata.TaskGUID)
	require.NoError(t, os.MkdirAll(taskDir, 0o700))

	certificatePEM, privateKeyPEM, chainPEM := newTaskStartCertificateMaterial(t, metadata.TaskGUID, metadata.DeviceGUIDs)

	privateKeyPath := filepath.Join(taskDir, "private-key.pem")
	certificatePath := filepath.Join(taskDir, "certificate.pem")
	chainPath := filepath.Join(taskDir, "certificate-chain.pem")
	metadata.PrivateKeyFile = privateKeyPath
	metadata.CertificateFile = certificatePath
	metadata.CertificateChainFile = chainPath

	require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))
	require.NoError(t, os.WriteFile(certificatePath, certificatePEM, 0o644))
	require.NoError(t, os.WriteFile(chainPath, chainPEM, 0o644))

	metadataBytes, err := yaml.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "metadata.yaml"), metadataBytes, 0o600))
}

func newTaskStartCertificateMaterial(t *testing.T, taskGUID string, deviceGUIDs []string) ([]byte, []byte, []byte) {
	t.Helper()

	caCertificate, caPrivateKey, caPEM := newTestCertificateAuthority(t)

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: "portier-task-" + taskGUID,
		},
	}, privateKey)
	require.NoError(t, err)

	csr, err := x509.ParseCertificateRequest(csrDER)
	require.NoError(t, err)
	require.NoError(t, csr.CheckSignature())

	certificatePEM := signTestTaskCertificate(
		t,
		caCertificate,
		caPrivateKey,
		csr,
		time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 31, 10, 15, 0, 0, time.UTC),
		"bd7af1d4-89ab-49b3-a089-f521c6d9678e",
		deviceGUIDs,
		[]string{"urn:portier:task:" + taskGUID},
	)

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})
	require.NotNil(t, privateKeyPEM)

	return certificatePEM, privateKeyPEM, caPEM
}
