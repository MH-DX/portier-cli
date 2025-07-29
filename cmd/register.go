package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	portier "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

type registerOptions struct {
	Name                string
	ApiKey              string
	ApiURL              string
	HomeFolderPath      string
	CredentialsFileName string
	Output              string
}

func defaultRegisterOptions() *registerOptions {
	home, err := utils.Home()
	if err != nil {
		log.Fatalf("could not get home directory: %v", err)
	}

	return &registerOptions{
		ApiURL:              "https://api.portier.dev/api",
		ApiKey:              "",
		HomeFolderPath:      home,
		CredentialsFileName: "credentials_device.yaml",
		Output:              "yaml",
	}
}

func newRegisterCmd() *cobra.Command {
	o := defaultRegisterOptions()

	cmd := &cobra.Command{
		Use:          "register",
		Short:        "Registers a new device in portier.dev and downloads the device's API key",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(3),
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.Name, "name", "n", o.Name, "name of the device")
	cmd.Flags().StringVarP(&o.ApiKey, "apiKey", "k", o.ApiKey, "existing API key to register with")
	cmd.Flags().StringVarP(&o.HomeFolderPath, "home", "H", o.HomeFolderPath, "home folder path")
	cmd.Flags().StringVarP(&o.CredentialsFileName, "credentials", "c", o.CredentialsFileName, "credentials file name in home folder")
	cmd.Flags().StringVarP(&o.ApiURL, "apiUrl", "a", o.ApiURL, "base URL of the API (https://api.portier.dev)")
	cmd.Flags().Bool("no-tls", false, "Do not generate or check TLS certificates")

	return cmd
}

func (o *registerOptions) run(cmd *cobra.Command, args []string) error {
	err := o.parseArgs(cmd, args)
	if err != nil {
		return err
	}

	noTLS, _ := cmd.Flags().GetBool("no-tls")

	if o.ApiKey != "" {
		if o.Name != "" {
			return fmt.Errorf("--name must not be provided when --apiKey is used")
		}
		guid, err := portier.WhoAmI(strings.TrimSuffix(o.ApiURL, "/api"), o.ApiKey)
		if err != nil {
			return err
		}
		err = portier.StoreDeviceCredentials(o.ApiKey, o.HomeFolderPath, o.CredentialsFileName)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Existing API key stored. Device GUID: %s\n", guid)
		if !noTLS {
			cert, key, known, err := resolveTLSPaths(o.HomeFolderPath)
			if err != nil {
				return err
			}
			certExists := false
			if _, err := os.Stat(cert); err == nil {
				if _, err := os.Stat(key); err == nil {
					certExists = true
				}
			}
			if err := ensureTLSCertificate(cmd, o.HomeFolderPath, filepath.Join(o.HomeFolderPath, o.CredentialsFileName), o.ApiURL, cert, key, known); err != nil {
				return err
			}
			if err := ensureConfigTLS(o.HomeFolderPath); err != nil {
				return err
			}
			if certExists {
				if err := ensureFingerprintUpToDate(cmd, o.HomeFolderPath, o.ApiURL, filepath.Join(o.HomeFolderPath, o.CredentialsFileName), cert); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if o.Name == "" {
		fmt.Print("Enter device name: ")
		_, err := fmt.Scanln(&o.Name)
		if err != nil || o.Name == "" {
			return fmt.Errorf("device name is required")
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Command: Create device %s at %s, out %s\n", o.Name, o.ApiURL, o.Output)

	err = portier.Register(o.Name, o.ApiURL, o.HomeFolderPath, o.CredentialsFileName)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			if err := o.takeOverExistingDevice(cmd, noTLS); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return o.handleTLS(cmd, noTLS)
}

func (o *registerOptions) parseArgs(cmd *cobra.Command, _ []string) error {
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		log.Fatalf("could not get name flag: %v", err)
		return err
	}
	o.Name = name

	apiKey, err := cmd.Flags().GetString("apiKey")
	if err == nil {
		o.ApiKey = apiKey
	}

	apiUrl, err := cmd.Flags().GetString("apiUrl")
	if err != nil {
		log.Fatalf("could not get apiUrl flag: %v", err)
		return err
	}
	o.ApiURL = apiUrl

	return nil
}

func (o *registerOptions) handleTLS(cmd *cobra.Command, noTLS bool) error {
	if noTLS {
		return nil
	}
	cert, key, known, err := resolveTLSPaths(o.HomeFolderPath)
	if err != nil {
		return err
	}
	certExists := false
	if _, err := os.Stat(cert); err == nil {
		if _, err := os.Stat(key); err == nil {
			certExists = true
		}
	}
	if err := ensureTLSCertificate(cmd, o.HomeFolderPath, filepath.Join(o.HomeFolderPath, o.CredentialsFileName), o.ApiURL, cert, key, known); err != nil {
		return err
	}
	if err := ensureConfigTLS(o.HomeFolderPath); err != nil {
		return err
	}
	if certExists {
		if err := ensureFingerprintUpToDate(cmd, o.HomeFolderPath, o.ApiURL, filepath.Join(o.HomeFolderPath, o.CredentialsFileName), cert); err != nil {
			return err
		}
	}
	return nil
}

func (o *registerOptions) takeOverExistingDevice(cmd *cobra.Command, noTLS bool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Device '%s' already exists. Take over? [y/N] ", o.Name)
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("device already exists")
	}

	guid, err := portier.GetDeviceByName(o.HomeFolderPath, strings.TrimSuffix(o.ApiURL, "/api"), o.Name)
	if err != nil {
		return err
	}

	authResponse, err := portier.LoadAccessToken(o.HomeFolderPath)
	if err != nil {
		return err
	}

	apiKey, err := portier.GenerateApiKey(o.ApiURL, guid, "Generated by portier CLI", authResponse.AccessToken, o.HomeFolderPath)
	if err != nil {
		return err
	}

	if err := portier.StoreDeviceCredentials(apiKey.ApiKey, o.HomeFolderPath, o.CredentialsFileName); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Device taken over. GUID: %s\n", guid)

	return o.handleTLS(cmd, noTLS)
}

func ensureConfigTLS(home string) error {
	configFile := filepath.Join(home, "config.yaml")
	portierConfig, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}

	portierConfig.TLSEnabled = true

	if portierConfig.PTLSConfig.CertFile == "" {
		portierConfig.PTLSConfig.CertFile = filepath.Join(home, "cert.pem")
	}
	if portierConfig.PTLSConfig.KeyFile == "" {
		portierConfig.PTLSConfig.KeyFile = filepath.Join(home, "key.pem")
	}
	if portierConfig.PTLSConfig.CAFile == "" {
		portierConfig.PTLSConfig.CAFile = filepath.Join(home, "cacert.pem")
	}
	if portierConfig.PTLSConfig.KnownHostsFile == "" {
		portierConfig.PTLSConfig.KnownHostsFile = filepath.Join(home, "known_hosts")
	}

	if err := config.SaveConfig(configFile, portierConfig); err != nil {
		return err
	}

	return nil
}
