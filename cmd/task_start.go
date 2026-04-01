package cmd

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mh-dx/portier-cli/internal/portier/application"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/portier/endpoints"
	"github.com/mh-dx/portier-cli/internal/portier/taskcert"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type taskStartApplication interface {
	StartServices(*config.PortierConfig, *config.DeviceCredentials) error
	StopServices() error
}

var newTaskStartApplication = func() taskStartApplication {
	return application.NewPortierApplication()
}

var waitForTaskStartShutdown = func() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	<-sigs
}

var promptForTaskStartToken = defaultPromptForTaskStartToken

var taskStartNow = func() time.Time {
	return time.Now().UTC()
}

type taskStartOptions struct {
	APIURL         string
	TaskGUID       string
	TaskToken      string
	HomeFolderPath string
	MetadataPath   string
	DeviceGUID     string
	ListenAddress  string
}

type taskStartRuntime struct {
	Config         *config.PortierConfig
	Credentials    *config.DeviceCredentials
	Metadata       *taskcert.Metadata
	SelectedDevice uuid.UUID
	ListenAddress  string
	cleanup        func()
}

func defaultTaskStartOptions() *taskStartOptions {
	home, err := utils.Home()
	if err != nil {
		panic(fmt.Errorf("could not get home directory: %w", err))
	}

	return &taskStartOptions{
		HomeFolderPath: home,
	}
}

func newTaskStartCmd() *cobra.Command {
	o := defaultTaskStartOptions()

	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Start a task-scoped Portier session",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.APIURL, "apiUrl", "a", o.APIURL, "base URL of the Portier backend; defaults to the enrolled task metadata")
	cmd.Flags().StringVarP(&o.TaskGUID, "task-guid", "g", o.TaskGUID, "task GUID")
	cmd.Flags().StringVarP(&o.TaskToken, "task-token", "t", o.TaskToken, "task token; defaults to the enrolled task metadata and is prompted for if re-enrollment is needed")
	cmd.Flags().StringVarP(&o.HomeFolderPath, "home", "H", o.HomeFolderPath, "home folder path")
	cmd.Flags().StringVarP(&o.MetadataPath, "metadata", "m", o.MetadataPath, "path to the enrolled task metadata.yaml")
	cmd.Flags().StringVarP(&o.DeviceGUID, "device-guid", "d", o.DeviceGUID, "target device GUID from the enrolled task scope")
	cmd.Flags().StringVarP(&o.ListenAddress, "listen", "l", o.ListenAddress, "local listen address for the one-off tunnel, for example 127.0.0.1:10022")

	return cmd
}

func (o *taskStartOptions) run(cmd *cobra.Command, _ []string) error {
	runtime, err := o.prepare(cmd)
	if err != nil {
		return err
	}
	defer runtime.cleanup()

	app := newTaskStartApplication()
	if err := app.StartServices(runtime.Config, runtime.Credentials); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Task session started.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Listening on: tcp://%s\n", runtime.ListenAddress)
	fmt.Fprintf(cmd.OutOrStdout(), "Target device: %s\n", runtime.SelectedDevice.String())
	fmt.Fprintf(cmd.OutOrStdout(), "Scope: %s\n", runtime.Metadata.Scope)
	fmt.Fprintf(cmd.OutOrStdout(), "Validity: %s to %s\n", runtime.Metadata.NotBefore, runtime.Metadata.NotAfter)
	fmt.Fprintf(cmd.OutOrStdout(), "Press Ctrl+C to stop.\n")

	waitForTaskStartShutdown()

	return app.StopServices()
}

func (o *taskStartOptions) prepare(cmd *cobra.Command) (*taskStartRuntime, error) {
	metadataPath, err := o.resolveMetadataPath()
	if err != nil {
		return nil, err
	}

	metadata, err := loadTaskStartMetadata(metadataPath)
	if err != nil {
		return nil, err
	}

	taskGUID := strings.TrimSpace(o.TaskGUID)
	if taskGUID == "" && metadata != nil {
		taskGUID = strings.TrimSpace(metadata.TaskGUID)
	}
	if taskGUID == "" {
		return nil, fmt.Errorf("task GUID is required because no enrolled task metadata was found")
	}
	if metadata != nil && strings.TrimSpace(metadata.TaskGUID) != "" && metadata.TaskGUID != taskGUID {
		return nil, fmt.Errorf("task metadata task_guid %q does not match requested task GUID %q", metadata.TaskGUID, taskGUID)
	}

	apiURL := strings.TrimSpace(o.APIURL)
	if apiURL == "" && metadata != nil {
		apiURL = strings.TrimSpace(metadata.APIURL)
	}
	if apiURL == "" {
		return nil, fmt.Errorf("api URL was not provided and is missing from the enrolled task metadata")
	}

	taskToken := resolveTaskToken(strings.TrimSpace(o.TaskToken), metadata)
	needsEnrollment, enrollmentReason := taskEnrollmentRefreshReason(metadataPath, metadata, taskStartNow())
	if needsEnrollment {
		if taskToken == "" {
			taskToken, err = promptForTaskStartToken(taskGUID)
			if err != nil {
				return nil, err
			}
		}

		if cmd != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", enrollmentReason)
		}

		if _, _, err := enrollTaskCertificate(apiURL, taskGUID, taskToken, filepath.Dir(metadataPath)); err != nil {
			return nil, err
		}

		metadata, err = taskcert.LoadMetadata(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("could not load task metadata after enrollment: %w", err)
		}
	}

	if taskToken == "" && metadata != nil {
		taskToken = strings.TrimSpace(metadata.TaskToken)
	}
	if taskToken == "" {
		return nil, fmt.Errorf("task token was not provided and is missing from the enrolled task metadata")
	}
	if metadata == nil {
		return nil, fmt.Errorf("task metadata is unavailable after enrollment")
	}

	scope := strings.TrimSpace(metadata.Scope)
	if scope == "" {
		return nil, fmt.Errorf("task metadata does not include scope")
	}

	remoteURL, err := url.Parse(scope)
	if err != nil {
		return nil, fmt.Errorf("task scope %q is invalid: %w", scope, err)
	}
	if remoteURL.Hostname() == "" {
		return nil, fmt.Errorf("task scope %q does not include a host", scope)
	}
	if remoteURL.Port() == "" {
		return nil, fmt.Errorf("task scope %q does not include a port", scope)
	}

	selectedDevice, err := resolveTaskStartDevice(strings.TrimSpace(o.DeviceGUID), metadata.DeviceGUIDs)
	if err != nil {
		return nil, err
	}

	listenAddress, err := resolveTaskStartListenAddress(strings.TrimSpace(o.ListenAddress), remoteURL)
	if err != nil {
		return nil, err
	}

	localURL, err := url.Parse("tcp://" + listenAddress)
	if err != nil {
		return nil, fmt.Errorf("local listen address %q is invalid: %w", listenAddress, err)
	}

	fullChainPath, cleanup, err := prepareTaskClientCertificateBundle(filepath.Dir(metadataPath), metadata)
	if err != nil {
		return nil, err
	}

	portierConfig, err := config.DefaultPortierConfig()
	if err != nil {
		cleanup()
		return nil, err
	}
	normalizedBaseURL := endpoints.NormalizeBaseURL(apiURL)
	if normalizedBaseURL == "" {
		cleanup()
		return nil, fmt.Errorf("invalid api URL %q", apiURL)
	}
	parsedBaseURL, err := url.Parse(normalizedBaseURL)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("invalid api URL %q: %w", apiURL, err)
	}
	portierConfig.BaseURL = utils.YAMLURL{URL: parsedBaseURL}
	portierConfig.RelayPath = fmt.Sprintf("/task-spider/%s", taskGUID)
	portierConfig.TLSEnabled = true
	portierConfig.PTLSConfig = config.PTLSConfig{
		CertFile:       fullChainPath,
		KeyFile:        resolveTaskMaterialPath(filepath.Dir(metadataPath), metadata.PrivateKeyFile, "private-key.pem"),
		CAFile:         resolveTaskMaterialPath(filepath.Dir(metadataPath), metadata.CertificateChainFile, "certificate-chain.pem"),
		KnownHostsFile: "",
	}
	portierConfig.Services = []config.Service{
		{
			Name: fmt.Sprintf("task-%s-%s", taskGUID, selectedDevice.String()),
			Options: config.ServiceOptions{
				URLLocal:     utils.YAMLURL{URL: localURL},
				URLRemote:    utils.YAMLURL{URL: remoteURL},
				PeerDeviceID: selectedDevice,
				TLSEnabled:   true,
			},
		},
	}

	return &taskStartRuntime{
		Config:         portierConfig,
		Credentials:    &config.DeviceCredentials{DeviceID: uuid.New(), ApiToken: "Bearer " + taskToken},
		Metadata:       metadata,
		SelectedDevice: selectedDevice,
		ListenAddress:  listenAddress,
		cleanup:        cleanup,
	}, nil
}

func (o *taskStartOptions) resolveMetadataPath() (string, error) {
	metadataPath := strings.TrimSpace(o.MetadataPath)
	if metadataPath != "" {
		return metadataPath, nil
	}

	taskGUID := strings.TrimSpace(o.TaskGUID)
	if taskGUID == "" {
		return "", fmt.Errorf("--task-guid or --metadata is required")
	}

	return filepath.Join(o.HomeFolderPath, "tasks", taskGUID, "metadata.yaml"), nil
}

func loadTaskStartMetadata(metadataPath string) (*taskcert.Metadata, error) {
	metadata, err := taskcert.LoadMetadata(metadataPath)
	if err == nil {
		return metadata, nil
	}
	if os.IsNotExist(err) {
		return nil, nil
	}

	return nil, fmt.Errorf("could not load task metadata: %w", err)
}

func resolveTaskToken(requestedTaskToken string, metadata *taskcert.Metadata) string {
	if requestedTaskToken != "" {
		return requestedTaskToken
	}
	if metadata == nil {
		return ""
	}

	return strings.TrimSpace(metadata.TaskToken)
}

func taskEnrollmentRefreshReason(metadataPath string, metadata *taskcert.Metadata, now time.Time) (bool, string) {
	if metadata == nil {
		return true, "No enrolled task certificate was found. Enrolling now..."
	}

	validity, err := readTaskCertificateValidity(metadataPath, metadata)
	if err != nil {
		return true, fmt.Sprintf("Task certificate material is missing or invalid (%v). Re-enrolling now...", err)
	}

	if now.Before(validity.NotBefore) || !now.Before(validity.NotAfter) {
		return true, fmt.Sprintf("Task certificate is outside its validity window (%s to %s). Re-enrolling now...", validity.NotBefore.Format(time.RFC3339), validity.NotAfter.Format(time.RFC3339))
	}

	return false, ""
}

type taskCertificateValidity struct {
	NotBefore time.Time
	NotAfter  time.Time
}

func readTaskCertificateValidity(metadataPath string, metadata *taskcert.Metadata) (*taskCertificateValidity, error) {
	metadataDir := filepath.Dir(metadataPath)
	certificatePath := resolveTaskMaterialPath(metadataDir, metadata.CertificateFile, "certificate.pem")
	chainPath := resolveTaskMaterialPath(metadataDir, metadata.CertificateChainFile, "certificate-chain.pem")
	privateKeyPath := resolveTaskMaterialPath(metadataDir, metadata.PrivateKeyFile, "private-key.pem")

	certificatePEM, err := os.ReadFile(certificatePath)
	if err != nil {
		return nil, fmt.Errorf("could not read task certificate PEM: %w", err)
	}
	chainPEM, err := os.ReadFile(chainPath)
	if err != nil {
		return nil, fmt.Errorf("could not read task certificate chain PEM: %w", err)
	}
	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read task private key PEM: %w", err)
	}

	fullChainPEM := append([]byte{}, certificatePEM...)
	if len(fullChainPEM) > 0 && !strings.HasSuffix(string(fullChainPEM), "\n") {
		fullChainPEM = append(fullChainPEM, '\n')
	}
	fullChainPEM = append(fullChainPEM, chainPEM...)

	if _, err := tls.X509KeyPair(fullChainPEM, privateKeyPEM); err != nil {
		return nil, fmt.Errorf("task certificate material is invalid: %w", err)
	}

	block, rest := pem.Decode(certificatePEM)
	if block == nil {
		return nil, fmt.Errorf("task certificate PEM did not contain a certificate")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("task certificate PEM contains unexpected block %q", block.Type)
	}
	if strings.TrimSpace(string(rest)) != "" {
		return nil, fmt.Errorf("task certificate PEM contains multiple blocks")
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("task certificate PEM is invalid: %w", err)
	}

	return &taskCertificateValidity{
		NotBefore: certificate.NotBefore.UTC(),
		NotAfter:  certificate.NotAfter.UTC(),
	}, nil
}

func defaultPromptForTaskStartToken(taskGUID string) (string, error) {
	prompt := fmt.Sprintf("Task token for %s: ", taskGUID)
	if terminal := term.IsTerminal(int(os.Stdin.Fd())); terminal {
		if _, err := fmt.Fprint(os.Stderr, prompt); err != nil {
			return "", err
		}

		taskToken, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", fmt.Errorf("could not read task token: %w", err)
		}
		if _, err := fmt.Fprintln(os.Stderr); err != nil {
			return "", err
		}

		trimmedTaskToken := strings.TrimSpace(string(taskToken))
		if trimmedTaskToken == "" {
			return "", fmt.Errorf("task token is required for enrollment")
		}

		return trimmedTaskToken, nil
	}

	if _, err := fmt.Fprint(os.Stderr, prompt); err != nil {
		return "", err
	}

	reader := bufio.NewReader(os.Stdin)
	taskToken, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("could not read task token: %w", err)
	}

	trimmedTaskToken := strings.TrimSpace(taskToken)
	if trimmedTaskToken == "" {
		return "", fmt.Errorf("task token is required for enrollment")
	}

	return trimmedTaskToken, nil
}

func resolveTaskStartDevice(requestedDeviceGUID string, availableDeviceGUIDs []string) (uuid.UUID, error) {
	if len(availableDeviceGUIDs) == 0 {
		return uuid.Nil, fmt.Errorf("task metadata does not include any device_guids")
	}

	if requestedDeviceGUID == "" {
		if len(availableDeviceGUIDs) > 1 {
			return uuid.Nil, fmt.Errorf("--device-guid is required because the task is scoped to multiple devices: %s", strings.Join(availableDeviceGUIDs, ", "))
		}
		requestedDeviceGUID = availableDeviceGUIDs[0]
	} else {
		matched := false
		for _, availableDeviceGUID := range availableDeviceGUIDs {
			if availableDeviceGUID == requestedDeviceGUID {
				matched = true
				break
			}
		}
		if !matched {
			return uuid.Nil, fmt.Errorf("device GUID %q is not allowed for this task; available device GUIDs: %s", requestedDeviceGUID, strings.Join(availableDeviceGUIDs, ", "))
		}
	}

	parsedGUID, err := uuid.Parse(requestedDeviceGUID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("task device GUID %q is invalid: %w", requestedDeviceGUID, err)
	}

	return parsedGUID, nil
}

func resolveTaskStartListenAddress(listenAddress string, remoteURL *url.URL) (string, error) {
	if listenAddress == "" {
		port, err := strconv.Atoi(remoteURL.Port())
		if err != nil {
			return "", fmt.Errorf("task scope port %q is invalid: %w", remoteURL.Port(), err)
		}
		if port < 1024 {
			port += 10000
		}
		return fmt.Sprintf("127.0.0.1:%d", port), nil
	}

	listenAddress = strings.TrimSpace(listenAddress)
	listenAddress = strings.TrimPrefix(listenAddress, "tcp://")
	if !strings.Contains(listenAddress, ":") {
		if _, err := strconv.Atoi(listenAddress); err != nil {
			return "", fmt.Errorf("listen address %q must be a port or host:port", listenAddress)
		}
		return "127.0.0.1:" + listenAddress, nil
	}
	if strings.HasPrefix(listenAddress, ":") {
		return "127.0.0.1" + listenAddress, nil
	}

	return listenAddress, nil
}

func prepareTaskClientCertificateBundle(metadataDir string, metadata *taskcert.Metadata) (string, func(), error) {
	certificatePath := resolveTaskMaterialPath(metadataDir, metadata.CertificateFile, "certificate.pem")
	chainPath := resolveTaskMaterialPath(metadataDir, metadata.CertificateChainFile, "certificate-chain.pem")
	privateKeyPath := resolveTaskMaterialPath(metadataDir, metadata.PrivateKeyFile, "private-key.pem")

	certificatePEM, err := os.ReadFile(certificatePath)
	if err != nil {
		return "", nil, fmt.Errorf("could not read task certificate PEM: %w", err)
	}
	chainPEM, err := os.ReadFile(chainPath)
	if err != nil {
		return "", nil, fmt.Errorf("could not read task certificate chain PEM: %w", err)
	}
	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", nil, fmt.Errorf("could not read task private key PEM: %w", err)
	}

	fullChainPEM := append([]byte{}, certificatePEM...)
	if len(fullChainPEM) > 0 && !strings.HasSuffix(string(fullChainPEM), "\n") {
		fullChainPEM = append(fullChainPEM, '\n')
	}
	fullChainPEM = append(fullChainPEM, chainPEM...)

	if _, err := tls.X509KeyPair(fullChainPEM, privateKeyPEM); err != nil {
		return "", nil, fmt.Errorf("task certificate material is invalid: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "portier-task-start-*")
	if err != nil {
		return "", nil, err
	}

	fullChainPath := filepath.Join(tempDir, "certificate-fullchain.pem")
	if err := os.WriteFile(fullChainPath, fullChainPEM, 0o600); err != nil {
		os.RemoveAll(tempDir)
		return "", nil, err
	}

	return fullChainPath, func() {
		_ = os.RemoveAll(tempDir)
	}, nil
}

func resolveTaskMaterialPath(metadataDir, configuredPath, defaultFilename string) string {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return filepath.Join(metadataDir, defaultFilename)
	}
	if filepath.IsAbs(configuredPath) {
		return configuredPath
	}
	return filepath.Join(metadataDir, configuredPath)
}
