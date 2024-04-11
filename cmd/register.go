package cmd

import (
	"fmt"
	"log"

	portier "github.com/marinator86/portier-cli/internal/portier/api"
	"github.com/marinator86/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

type registerOptions struct {
	Name                string
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
		HomeFolderPath:      home,
		CredentialsFileName: "credentials_device.yaml",
		Output:              "yaml",
	}
}

func newRegisterCmd() *cobra.Command {
	o := defaultRegisterOptions()

	cmd := &cobra.Command{
		Use:          "register",
		Short:        "registers a new device in portier.dev and downloads the device's API key",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(3),
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.Name, "name", "n", o.Name, "name of the device")
	cmd.Flags().StringVarP(&o.HomeFolderPath, "home", "H", o.HomeFolderPath, "home folder path")
	cmd.Flags().StringVarP(&o.CredentialsFileName, "credentials", "c", o.CredentialsFileName, "credentials file name in home folder")
	cmd.Flags().StringVarP(&o.ApiURL, "apiUrl", "a", o.ApiURL, "base URL of the API (https://api.portier.dev)")

	return cmd
}

func (o *registerOptions) run(cmd *cobra.Command, args []string) error {
	err := o.parseArgs(cmd, args)
	if err != nil {
		return err
	}

	if o.Name == "" {
		log.Fatalf("name is required to register a device")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Command: Create device %s at %s, out %s\n", o.Name, o.ApiURL, o.Output)

	portier.Register(o.Name, o.ApiURL, o.HomeFolderPath, o.CredentialsFileName)

	return nil
}

func (o *registerOptions) parseArgs(cmd *cobra.Command, _ []string) error {
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		log.Fatalf("could not get name flag: %v", err)
		return err
	}
	o.Name = name

	apiUrl, err := cmd.Flags().GetString("apiUrl")
	if err != nil {
		log.Fatalf("could not get apiUrl flag: %v", err)
		return err
	}
	o.ApiURL = apiUrl

	return nil
}
