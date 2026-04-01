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
	"github.com/mh-dx/portier-cli/internal/portier/devicecert"
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
	APIURL             string   `yaml:"api_url"`
	CustomerGUID       string   `yaml:"customer_guid"`
	DeviceGUID         string   `yaml:"device_guid"`
	Networks           []string `yaml:"networks"`
	CACertFile         string   `yaml:"ca_cert_file"`
	DeviceCertFile     string   `yaml:"device_cert_file"`
	DeviceKeyFile      string   `yaml:"device_key_file"`
	NotBefore          string   `yaml:"not_before"`
	NotAfter           string   `yaml:"not_after"`
	CertificateProfile string   `yaml:"certificate_profile,omitempty"`
	UpdatedAt          string   `yaml:"updated_at"`
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
		Short:        "Store a device API key and install customer-signed PTLS material",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.APIKey, "apiKey", "k", o.APIKey, "device API key to store for future run commands")
	cmd.Flags().StringVarP(&o.APIURL, "apiUrl", "a", o.APIURL, "base URL of the Portier backend")
	cmd.Flags().StringVarP(&o.CustomerGUID, "customer-guid", "g", o.CustomerGUID, "optional customer GUID to verify against the device certificate response")
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

	infoResponse, err := portier.GetDeviceCertificateInfo(o.APIURL, device.GUID, o.APIKey)
	if err != nil {
		return err
	}

	notBefore, notAfter, err := validateDeviceCertificateInfo(device.GUID, o.CustomerGUID, infoResponse)
	if err != nil {
		return err
	}

	material, err := devicecert.Generate(device.GUID, infoResponse.RequiredDNSSANs, infoResponse.RequiredURISANs)
	if err != nil {
		return err
	}

	signResponse, err := portier.IssueDeviceCertificate(o.APIURL, device.GUID, o.APIKey, string(material.CSRPEM))
	if err != nil {
		return err
	}

	if err := validateDeviceCertificateResponse(infoResponse, signResponse); err != nil {
		return err
	}

	certificateCount, err := validateCustomerCABundle([]byte(signResponse.CertificateChain))
	if err != nil {
		return err
	}

	if err := devicecert.ValidateIssuedMaterials(
		[]byte(signResponse.Certificate),
		[]byte(signResponse.CertificateChain),
		material.PrivateKey,
		notBefore,
		notAfter,
		infoResponse.RequiredDNSSANs,
		infoResponse.RequiredURISANs,
	); err != nil {
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

	if cfg.PTLSConfig.CertFile == "" {
		cfg.PTLSConfig.CertFile = filepath.Join(o.HomeFolderPath, "cert.pem")
	}
	if cfg.PTLSConfig.KeyFile == "" {
		cfg.PTLSConfig.KeyFile = filepath.Join(o.HomeFolderPath, "key.pem")
	}
	if cfg.PTLSConfig.KnownHostsFile == "" {
		cfg.PTLSConfig.KnownHostsFile = filepath.Join(o.HomeFolderPath, "known_hosts")
	}

	caCertPath := strings.TrimSpace(o.CACertFile)
	if caCertPath == "" {
		caCertPath = strings.TrimSpace(cfg.PTLSConfig.CAFile)
	}
	if caCertPath == "" {
		caCertPath = filepath.Join(o.HomeFolderPath, "cacert.pem")
	}
	cfg.PTLSConfig.CAFile = caCertPath

	if err := writePEMFile(caCertPath, []byte(signResponse.CertificateChain), 0o644); err != nil {
		return err
	}
	if err := writePEMFile(cfg.PTLSConfig.CertFile, []byte(signResponse.Certificate), 0o644); err != nil {
		return err
	}
	if err := writePEMFile(cfg.PTLSConfig.KeyFile, material.PrivateKeyPEM, 0o600); err != nil {
		return err
	}

	cfg.TLSEnabled = true
	cfg.PTLSConfig.CAFile = caCertPath

	if err := config.SaveConfig(o.ConfigFile, cfg); err != nil {
		return err
	}

	if err := portier.StoreDeviceCredentials(o.APIKey, o.HomeFolderPath, o.CredentialsFileName); err != nil {
		return err
	}

	if err := o.storeMetadata(&customerSetupMetadata{
		APIURL:             o.APIURL,
		CustomerGUID:       signResponse.CustomerGUID,
		DeviceGUID:         device.GUID,
		Networks:           append([]string(nil), device.Networks...),
		CACertFile:         caCertPath,
		DeviceCertFile:     cfg.PTLSConfig.CertFile,
		DeviceKeyFile:      cfg.PTLSConfig.KeyFile,
		NotBefore:          signResponse.NotBefore,
		NotAfter:           signResponse.NotAfter,
		CertificateProfile: signResponse.CertificateProfile,
		UpdatedAt:          time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Customer setup complete.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Device: %s\n", device.GUID)
	fmt.Fprintf(cmd.OutOrStdout(), "Customer: %s\n", signResponse.CustomerGUID)
	fmt.Fprintf(cmd.OutOrStdout(), "Networks: %d\n", len(device.Networks))
	if strings.TrimSpace(signResponse.CertificateProfile) != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Certificate profile: %s\n", signResponse.CertificateProfile)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Validity: %s to %s\n", signResponse.NotBefore, signResponse.NotAfter)
	fmt.Fprintf(cmd.OutOrStdout(), "CA certificates: %d\n", certificateCount)
	fmt.Fprintf(cmd.OutOrStdout(), "Certificate: %s\n", cfg.PTLSConfig.CertFile)
	fmt.Fprintf(cmd.OutOrStdout(), "Key: %s\n", cfg.PTLSConfig.KeyFile)
	fmt.Fprintf(cmd.OutOrStdout(), "CA bundle: %s\n", caCertPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", o.ConfigFile)
	fmt.Fprintf(cmd.OutOrStdout(), "Credentials: %s\n", filepath.Join(o.HomeFolderPath, o.CredentialsFileName))
	fmt.Fprintf(cmd.OutOrStdout(), "PTLS is ready to validate task client certificates on the next run.\n")

	return nil
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

func validateDeviceCertificateInfo(expectedDeviceGUID, expectedCustomerGUID string, response *portier.DeviceCertificateInfoResponse) (time.Time, time.Time, error) {
	if response == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response was empty")
	}
	if strings.TrimSpace(response.DeviceGUID) == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response did not include device_guid")
	}
	if response.DeviceGUID != expectedDeviceGUID {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response device_guid %q does not match authenticated device %q", response.DeviceGUID, expectedDeviceGUID)
	}
	if strings.TrimSpace(response.CustomerGUID) == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response did not include customer_guid")
	}
	if expectedCustomerGUID != "" && response.CustomerGUID != expectedCustomerGUID {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response customer_guid %q does not match requested customer GUID %q", response.CustomerGUID, expectedCustomerGUID)
	}
	if len(response.RequiredDNSSANs) == 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response did not include required_dns_sans")
	}
	if !stringSliceContains(response.RequiredDNSSANs, expectedDeviceGUID) {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response required_dns_sans do not include authenticated device GUID %q", expectedDeviceGUID)
	}

	notBefore, err := time.Parse(time.RFC3339, response.NotBefore)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response contained an invalid not_before value: %w", err)
	}
	notAfter, err := time.Parse(time.RFC3339, response.NotAfter)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response contained an invalid not_after value: %w", err)
	}
	if !notAfter.After(notBefore) {
		return time.Time{}, time.Time{}, fmt.Errorf("device certificate info response validity window is invalid")
	}

	return notBefore.UTC(), notAfter.UTC(), nil
}

func validateDeviceCertificateResponse(infoResponse *portier.DeviceCertificateInfoResponse, response *portier.DeviceCertificateResponse) error {
	if response == nil {
		return fmt.Errorf("device certificate response was empty")
	}
	if response.DeviceGUID != infoResponse.DeviceGUID {
		return fmt.Errorf("device certificate response device_guid %q does not match the info response %q", response.DeviceGUID, infoResponse.DeviceGUID)
	}
	if response.CustomerGUID != infoResponse.CustomerGUID {
		return fmt.Errorf("device certificate response customer_guid %q does not match the info response %q", response.CustomerGUID, infoResponse.CustomerGUID)
	}
	if response.NotBefore != infoResponse.NotBefore {
		return fmt.Errorf("device certificate response not_before %q does not match the info response %q", response.NotBefore, infoResponse.NotBefore)
	}
	if response.NotAfter != infoResponse.NotAfter {
		return fmt.Errorf("device certificate response not_after %q does not match the info response %q", response.NotAfter, infoResponse.NotAfter)
	}
	if strings.TrimSpace(infoResponse.CertificateProfile) != "" && response.CertificateProfile != infoResponse.CertificateProfile {
		return fmt.Errorf("device certificate response certificate_profile %q does not match the info response %q", response.CertificateProfile, infoResponse.CertificateProfile)
	}
	if strings.TrimSpace(response.Certificate) == "" {
		return fmt.Errorf("device certificate response did not include certificate PEM")
	}
	if strings.TrimSpace(response.CertificateChain) == "" {
		return fmt.Errorf("device certificate response did not include certificate chain PEM")
	}

	return nil
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

func writePEMFile(path string, contents []byte, permissions os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	return os.WriteFile(path, contents, permissions)
}
