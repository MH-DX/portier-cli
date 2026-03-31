package cmd

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	portier "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type customerSetupOptions struct {
	APIKey              string
	APIURL              string
	CustomerGUID        string
	HomeFolderPath      string
	CredentialsFileName string
	ConfigFile          string
	CACertFile          string
	MetadataFile        string
}

type customerSetupMetadata struct {
	APIURL       string   `yaml:"api_url"`
	CustomerGUID string   `yaml:"customer_guid"`
	DeviceGUID   string   `yaml:"device_guid"`
	Networks     []string `yaml:"networks"`
	CACertFile   string   `yaml:"ca_cert_file"`
	UpdatedAt    string   `yaml:"updated_at"`
}

func defaultCustomerSetupOptions() *customerSetupOptions {
	home, err := utils.Home()
	if err != nil {
		panic(fmt.Errorf("could not get home directory: %w", err))
	}

	return &customerSetupOptions{
		APIURL:              "https://api.portier.dev",
		HomeFolderPath:      home,
		CredentialsFileName: "credentials_device.yaml",
		ConfigFile:          filepath.Join(home, "config.yaml"),
		MetadataFile:        filepath.Join(home, "customer_setup.yaml"),
	}
}

func newCustomerSetupCmd() *cobra.Command {
	o := defaultCustomerSetupOptions()

	cmd := &cobra.Command{
		Use:          "setup",
		Short:        "Store a device API key and configure customer CA trust for PTLS",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.APIKey, "apiKey", "k", o.APIKey, "device API key to store for future run commands")
	cmd.Flags().StringVarP(&o.APIURL, "apiUrl", "a", o.APIURL, "base URL of the Portier backend")
	cmd.Flags().StringVarP(&o.CustomerGUID, "customer-guid", "g", o.CustomerGUID, "customer GUID; if omitted, reuses the GUID from the last customer setup on this machine")
	cmd.Flags().StringVarP(&o.HomeFolderPath, "home", "H", o.HomeFolderPath, "home folder path")
	cmd.Flags().StringVarP(&o.CredentialsFileName, "credentials", "c", o.CredentialsFileName, "credentials file name in home folder")
	cmd.Flags().StringVar(&o.ConfigFile, "config", o.ConfigFile, "path to config.yaml")
	cmd.Flags().StringVar(&o.CACertFile, "ca-cert", o.CACertFile, "path where the customer CA bundle PEM will be stored")
	cmd.Flags().StringVar(&o.MetadataFile, "metadata", o.MetadataFile, "path to the stored customer setup metadata")

	return cmd
}

func (o *customerSetupOptions) run(cmd *cobra.Command, _ []string) error {
	o.APIKey = strings.TrimSpace(o.APIKey)
	o.APIURL = strings.TrimSpace(o.APIURL)
	o.CustomerGUID = strings.TrimSpace(o.CustomerGUID)
	o.HomeFolderPath = strings.TrimSpace(o.HomeFolderPath)

	if !cmd.Flags().Changed("config") {
		o.ConfigFile = filepath.Join(o.HomeFolderPath, "config.yaml")
	}
	if !cmd.Flags().Changed("metadata") {
		o.MetadataFile = filepath.Join(o.HomeFolderPath, "customer_setup.yaml")
	}

	if o.APIKey == "" {
		return fmt.Errorf("--apiKey is required")
	}
	if o.APIURL == "" {
		return fmt.Errorf("--apiUrl is required")
	}

	device, err := portier.WhoAmIDevice(o.APIURL, o.APIKey)
	if err != nil {
		return fmt.Errorf("device identity lookup failed: %w", err)
	}

	customerGUID, err := o.resolveCustomerGUID(device)
	if err != nil {
		return err
	}

	caBundlePEM, err := portier.GetCurrentCustomerCABundle(o.APIURL, customerGUID)
	if err != nil {
		return err
	}
	certificateCount, err := validateCustomerCABundle(caBundlePEM)
	if err != nil {
		return err
	}

	_, configStatErr := os.Stat(o.ConfigFile)
	configExists := configStatErr == nil
	if configStatErr != nil && !os.IsNotExist(configStatErr) {
		return configStatErr
	}

	cfg, err := config.LoadConfig(o.ConfigFile)
	if err != nil {
		return err
	}
	if !configExists {
		cfg.PTLSConfig.CertFile = filepath.Join(o.HomeFolderPath, "cert.pem")
		cfg.PTLSConfig.KeyFile = filepath.Join(o.HomeFolderPath, "key.pem")
		cfg.PTLSConfig.CAFile = filepath.Join(o.HomeFolderPath, "cacert.pem")
		cfg.PTLSConfig.KnownHostsFile = filepath.Join(o.HomeFolderPath, "known_hosts")
	}

	caCertPath := strings.TrimSpace(o.CACertFile)
	if caCertPath == "" {
		caCertPath = strings.TrimSpace(cfg.PTLSConfig.CAFile)
	}
	if caCertPath == "" {
		caCertPath = filepath.Join(o.HomeFolderPath, "cacert.pem")
	}

	if err := os.MkdirAll(filepath.Dir(caCertPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(caCertPath, caBundlePEM, 0o644); err != nil {
		return err
	}

	cfg.TLSEnabled = true
	if cfg.PTLSConfig.CertFile == "" {
		cfg.PTLSConfig.CertFile = filepath.Join(o.HomeFolderPath, "cert.pem")
	}
	if cfg.PTLSConfig.KeyFile == "" {
		cfg.PTLSConfig.KeyFile = filepath.Join(o.HomeFolderPath, "key.pem")
	}
	if cfg.PTLSConfig.KnownHostsFile == "" {
		cfg.PTLSConfig.KnownHostsFile = filepath.Join(o.HomeFolderPath, "known_hosts")
	}
	cfg.PTLSConfig.CAFile = caCertPath

	if err := config.SaveConfig(o.ConfigFile, cfg); err != nil {
		return err
	}

	if err := portier.StoreDeviceCredentials(o.APIKey, o.HomeFolderPath, o.CredentialsFileName); err != nil {
		return err
	}

	if err := o.storeMetadata(&customerSetupMetadata{
		APIURL:       o.APIURL,
		CustomerGUID: customerGUID,
		DeviceGUID:   device.GUID,
		Networks:     append([]string(nil), device.Networks...),
		CACertFile:   caCertPath,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Customer setup complete.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Device: %s\n", device.GUID)
	fmt.Fprintf(cmd.OutOrStdout(), "Customer: %s\n", customerGUID)
	fmt.Fprintf(cmd.OutOrStdout(), "Networks: %d\n", len(device.Networks))
	fmt.Fprintf(cmd.OutOrStdout(), "CA certificates: %d\n", certificateCount)
	fmt.Fprintf(cmd.OutOrStdout(), "CA bundle: %s\n", caCertPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", o.ConfigFile)
	fmt.Fprintf(cmd.OutOrStdout(), "Credentials: %s\n", filepath.Join(o.HomeFolderPath, o.CredentialsFileName))

	if !customerSetupHasTLSMaterials(cfg) {
		fmt.Fprintf(cmd.OutOrStdout(), "Note: configure a device certificate and key at %s and %s before expecting full mutual TLS with task operators.\n", cfg.PTLSConfig.CertFile, cfg.PTLSConfig.KeyFile)
	}

	return nil
}

func (o *customerSetupOptions) resolveCustomerGUID(device *portier.DeviceWhoAmIResponse) (string, error) {
	if o.CustomerGUID != "" {
		return o.CustomerGUID, nil
	}

	metadata, err := o.loadMetadata()
	if err == nil && strings.TrimSpace(metadata.CustomerGUID) != "" {
		return strings.TrimSpace(metadata.CustomerGUID), nil
	}

	networkSummary := "none"
	if len(device.Networks) > 0 {
		networkSummary = strings.Join(device.Networks, ", ")
	}

	return "", fmt.Errorf("customer GUID could not be determined from device API key alone for device %s (networks: %s); pass --customer-guid once so portier-cli can download the customer CA bundle", device.GUID, networkSummary)
}

func (o *customerSetupOptions) loadMetadata() (*customerSetupMetadata, error) {
	metadataBytes, err := os.ReadFile(o.MetadataFile)
	if err != nil {
		return nil, err
	}

	metadata := &customerSetupMetadata{}
	if err := yaml.Unmarshal(metadataBytes, metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}

func (o *customerSetupOptions) storeMetadata(metadata *customerSetupMetadata) error {
	if err := os.MkdirAll(filepath.Dir(o.MetadataFile), 0o700); err != nil {
		return err
	}

	metadataBytes, err := yaml.Marshal(metadata)
	if err != nil {
		return err
	}

	return os.WriteFile(o.MetadataFile, metadataBytes, 0o600)
}

func validateCustomerCABundle(bundlePEM []byte) (int, error) {
	remaining := bundlePEM
	certificateCount := 0
	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return 0, fmt.Errorf("customer CA bundle contains unexpected PEM block %q", block.Type)
		}

		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return 0, fmt.Errorf("customer CA bundle contains an invalid certificate: %w", err)
		}
		if !certificate.IsCA {
			return 0, fmt.Errorf("customer CA bundle contains a non-CA certificate: %s", certificate.Subject.String())
		}

		certificateCount++
		remaining = rest
	}

	if certificateCount == 0 {
		return 0, fmt.Errorf("customer CA bundle did not contain any certificates")
	}
	if strings.TrimSpace(string(remaining)) != "" {
		return 0, fmt.Errorf("customer CA bundle contains trailing non-PEM data")
	}

	return certificateCount, nil
}

func customerSetupHasTLSMaterials(cfg *config.PortierConfig) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.PTLSConfig.CertFile) == "" || strings.TrimSpace(cfg.PTLSConfig.KeyFile) == "" {
		return false
	}
	if _, err := os.Stat(cfg.PTLSConfig.CertFile); err != nil {
		return false
	}
	if _, err := os.Stat(cfg.PTLSConfig.KeyFile); err != nil {
		return false
	}
	return true
}
