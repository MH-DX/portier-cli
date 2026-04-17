package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	portier "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/spf13/cobra"
)

type registerTokenOptions struct {
	Token               string
	APIURL              string
	HomeFolderPath      string
	CredentialsFileName string
	ConfigFile          string
	CACertFile          string
	MetadataFile        string
	NoTLS               bool
}

func defaultRegisterTokenOptions() *registerTokenOptions {
	registerDefaults := defaultRegisterOptions()
	customerDefaults := defaultCustomerSetupOptions()

	return &registerTokenOptions{
		APIURL:              registerDefaults.ApiURL,
		HomeFolderPath:      registerDefaults.HomeFolderPath,
		CredentialsFileName: registerDefaults.CredentialsFileName,
		ConfigFile:          customerDefaults.ConfigFile,
		MetadataFile:        customerDefaults.MetadataFile,
	}
}

func newRegisterTokenCmd() *cobra.Command {
	o := defaultRegisterTokenOptions()

	cmd := &cobra.Command{
		Use:          "register-token",
		Short:        "Register this device with a one-time Portier registration token",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE:         o.run,
	}

	cmd.Flags().StringVar(&o.Token, "token", o.Token, "one-time registration token")
	cmd.Flags().StringVarP(&o.APIURL, "apiUrl", "a", o.APIURL, "base URL of the Portier backend")
	cmd.Flags().StringVarP(&o.HomeFolderPath, "home", "H", o.HomeFolderPath, "home folder path")
	cmd.Flags().StringVarP(&o.CredentialsFileName, "credentials", "c", o.CredentialsFileName, "credentials file name in home folder")
	cmd.Flags().StringVar(&o.ConfigFile, "config", o.ConfigFile, "path to config.yaml")
	cmd.Flags().StringVar(&o.CACertFile, "ca-cert", o.CACertFile, "path where the customer CA bundle PEM will be stored")
	cmd.Flags().StringVar(&o.MetadataFile, "metadata", o.MetadataFile, "path to the stored customer setup metadata")
	cmd.Flags().BoolVar(&o.NoTLS, "no-tls", o.NoTLS, "Do not generate or check TLS certificates for normal device registration")

	return cmd
}

func (o *registerTokenOptions) run(cmd *cobra.Command, _ []string) error {
	o.Token = strings.TrimSpace(o.Token)
	o.APIURL = strings.TrimSpace(o.APIURL)
	o.HomeFolderPath = strings.TrimSpace(o.HomeFolderPath)

	if o.Token == "" {
		return fmt.Errorf("--token is required")
	}
	if o.APIURL == "" {
		return fmt.Errorf("--apiUrl is required")
	}
	if o.HomeFolderPath == "" {
		return fmt.Errorf("--home is required")
	}

	response, err := portier.ExchangeDeviceRegistrationToken(o.APIURL, o.Token)
	if err != nil {
		return err
	}

	if strings.TrimSpace(response.CustomerGUID) != "" {
		return o.runCustomerSetup(cmd, response)
	}

	return o.storeNormalDevice(cmd, response)
}

func (o *registerTokenOptions) runCustomerSetup(cmd *cobra.Command, response *portier.DeviceRegistrationTokenExchangeResponse) error {
	setup := defaultCustomerSetupOptions()
	setup.APIKey = response.APIKey
	setup.APIURL = o.APIURL
	setup.CustomerGUID = response.CustomerGUID
	setup.HomeFolderPath = o.HomeFolderPath
	setup.CredentialsFileName = o.CredentialsFileName
	setup.ConfigFile = o.ConfigFile
	setup.CACertFile = o.CACertFile
	setup.MetadataFile = o.MetadataFile

	if !cmd.Flags().Changed("config") {
		setup.ConfigFile = filepath.Join(o.HomeFolderPath, "config.yaml")
	}
	if !cmd.Flags().Changed("metadata") {
		setup.MetadataFile = filepath.Join(o.HomeFolderPath, "customer_setup.yaml")
	}

	if err := setup.run(cmd, nil); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Registration token consumed. Device GUID: %s\n", response.DeviceGUID)
	return nil
}

func (o *registerTokenOptions) storeNormalDevice(cmd *cobra.Command, response *portier.DeviceRegistrationTokenExchangeResponse) error {
	if err := portier.StoreDeviceCredentials(response.APIKey, o.HomeFolderPath, o.CredentialsFileName); err != nil {
		return err
	}
	if err := persistRegisteredBaseURL(o.HomeFolderPath, o.APIURL); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Registration token consumed. Device GUID: %s\n", response.DeviceGUID)
	fmt.Fprintf(cmd.OutOrStdout(), "Credentials: %s\n", filepath.Join(o.HomeFolderPath, o.CredentialsFileName))

	if o.NoTLS {
		return nil
	}

	register := &registerOptions{
		ApiURL:              o.APIURL,
		ApiKey:              response.APIKey,
		HomeFolderPath:      o.HomeFolderPath,
		CredentialsFileName: o.CredentialsFileName,
	}
	return register.handleTLS(cmd, false)
}
