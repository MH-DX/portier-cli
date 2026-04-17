package cmd

import (
	"os"

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
	cmd.AddCommand(newRegisterTokenCmd())
	cmd.AddCommand(newCustomerCmd())
	cmd.AddCommand(newTaskCmd())
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

	serviceCmd, err := newServiceCmd()
	if err == nil {
		cmd.AddCommand(serviceCmd)
	}

	trayCmd := newTrayCmd()
	cmd.AddCommand(trayCmd)

	return cmd
}

// Execute invokes the command.
func Execute(version string) error {
	return ExecuteArgs(version, os.Args[1:])
}

// ExecuteArgs invokes the command with an explicit argv slice.
func ExecuteArgs(version string, args []string) error {
	rootCmd := newRootCmd(version)
	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		return err
	}

	return nil
}
