package cmd

import (
	ptls_cmd "github.com/mh-dx/portier-cli/cmd/ptls"
	ptls_create_cmd "github.com/mh-dx/portier-cli/cmd/ptls/create"
	ptls_trust_cmd "github.com/mh-dx/portier-cli/cmd/ptls/trust"
	"github.com/spf13/cobra"
)

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portier-cli",
		Short: "Remotely access all your machines through Portier CLI. It's easy, efficient and reliable. For more info, visit portier.dev",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
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
