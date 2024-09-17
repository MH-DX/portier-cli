package tls_trust_cmd

import (
	"fmt"
	"log"
	"os"

	api "github.com/marinator86/portier-cli/internal/portier/api"
	"github.com/marinator86/portier-cli/internal/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type tlsTrustOptions struct {
	DeviceIDs           *[]string
	HomeFolderPath      string
	CredentialsFileName string
	KnownHostsFilePath  string
	ApiURL              string
}

func defaultTLSOptions() *tlsTrustOptions {
	home, err := utils.Home()
	if err != nil {
		log.Fatalf("could not get home directory: %v", err)
	}

	return &tlsTrustOptions{
		HomeFolderPath:      home,
		CredentialsFileName: "credentials_device.yaml",
		KnownHostsFilePath:  fmt.Sprintf("%s/known_hosts", home),
		ApiURL:              "https://api.portier.dev/api",
	}
}

func NewTrustcmd() *cobra.Command {
	o := defaultTLSOptions()

	cmd := &cobra.Command{
		Use:          "trust",
		Short:        "Trust a peer device by adding its TLS certificate fingerprint to known_hosts",
		SilenceUsage: true,
		RunE:         o.run,
	}

	o.DeviceIDs = cmd.Flags().StringSliceP("ids", "i", []string{}, "device ID of the device to trust. If not provided, will trust all devices that have uploaded their fingerprints")
	cmd.Flags().StringVarP(&o.HomeFolderPath, "home", "H", o.HomeFolderPath, "home folder path")
	cmd.Flags().StringVarP(&o.CredentialsFileName, "credentials", "c", o.CredentialsFileName, "credentials file name in home folder")
	cmd.Flags().StringVarP(&o.KnownHostsFilePath, "knownHosts", "f", o.KnownHostsFilePath, "path to the known_hosts file")
	cmd.Flags().StringVarP(&o.ApiURL, "apiUrl", "a", o.ApiURL, "base URL of the portier API")

	return cmd
}

func (o *tlsTrustOptions) run(cmd *cobra.Command, args []string) error {

	fingerprints, err := api.GetFingerprint(o.HomeFolderPath, o.ApiURL, *o.DeviceIDs)
	if err != nil {
		return err
	}

	fmt.Println("The following fingerprints were received (includes devices that are shared to you):")
	for deviceID, fingerprint := range fingerprints {
		if fingerprint == "" {
			fmt.Printf("DeviceID: %s, Fingerprint: <empty>\n", deviceID)
			continue
		}
		fmt.Printf("DeviceID: %s, Fingerprint: %s\n", deviceID, fingerprint)
	}

	fmt.Println()
	fmt.Printf("Adding fingerprints to %s\n", o.KnownHostsFilePath)
	fmt.Println()
	// load known_hosts file in yaml
	known_hosts, err := os.Open(o.KnownHostsFilePath)
	if err != nil {
		return err
	}

	decoder := yaml.NewDecoder(known_hosts)
	var knownHostsMap map[string]string
	err = decoder.Decode(&knownHostsMap)
	if err != nil {
		if err.Error() == "EOF" {
			knownHostsMap = make(map[string]string)
		} else {
			return err
		}
	}

	// add fingerprints to known_hosts, overwrite if already present
	for deviceID, fingerprint := range fingerprints {
		if fingerprint == "" {
			continue
		}
		knownHostsMap[deviceID] = fingerprint
	}

	// write the updated known_hosts file
	known_hosts, err = os.Create(o.KnownHostsFilePath)
	if err != nil {
		return err
	}
	encoder := yaml.NewEncoder(known_hosts)
	err = encoder.Encode(knownHostsMap)
	if err != nil {
		return err
	}

	fmt.Printf("Fingerprints added. Done.\n")
	return nil
}
