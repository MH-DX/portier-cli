package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	ptls_cmd "github.com/mh-dx/portier-cli/cmd/ptls"
	ptls_create_cmd "github.com/mh-dx/portier-cli/cmd/ptls/create"
	ptls_trust_cmd "github.com/mh-dx/portier-cli/cmd/ptls/trust"
	api "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

var deviceCredentials *config.DeviceCredentials

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portier-cli",
		Short: "Remotely access all your machines through Portier CLI. It's easy, efficient and reliable. For more info, visit portier.dev",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// PersistentPreRunE runs before any subcommand and loads device credentials
	// if present. It fetches the device GUID via the whoami API and stores
	// the credentials globally for later commands.
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		home, err := utils.Home()
		if err != nil {
			return nil
		}
		credPath := filepath.Join(home, "credentials_device.yaml")
		if _, err := os.Stat(credPath); err == nil {
			creds, err := config.LoadApiToken(credPath)
			if err != nil {
				return fmt.Errorf("failed to load credentials: %w", err)
			}
			guid, err := api.WhoAmI("https://api.portier.dev", creds.ApiToken)
			if err != nil {
				return fmt.Errorf("whoami failed: %w", err)
			}
			creds.DeviceID = guid
			deviceCredentials = creds
		}
		return nil
	}

	cmd.AddCommand(newVersionCmd(version)) // version subcommand
	cmd.AddCommand(NewManCmd().Cmd)        // man subcommand
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newRegisterCmd())
	tlsCmd := ptls_cmd.NewTLScmd()
	tlsCmd.AddCommand(ptls_create_cmd.NewCreatecmd())
	tlsCmd.AddCommand(ptls_trust_cmd.NewTrustcmd())
	cmd.AddCommand(tlsCmd)
	runCmd, err := newRunCmd()
	if err != nil {
		panic(err)
	}
	cmd.AddCommand(runCmd)

	forwardCmd, err := newForwardCmd()
	if err == nil {
		cmd.AddCommand(forwardCmd)
	}

	return cmd
}

// Execute invokes the command.
func Execute(version string) error {
	if err := newRootCmd(version).Execute(); err != nil {
		return err
	}

	return nil
}
