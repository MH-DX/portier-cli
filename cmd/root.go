package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portier-cli",
		Short: "golang-cli project template demo application",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newVersionCmd(version)) // version subcommand
	cmd.AddCommand(NewManCmd().Cmd)        // man subcommand
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newRegisterCmd())

	startCmd, err := newStartCmd()
	if err != nil {
		panic(err)
	}
	cmd.AddCommand(startCmd)

	runCmd, err := newRunCmd()
	if err != nil {
		panic(err)
	}
	cmd.AddCommand(runCmd)

	return cmd
}

// Execute invokes the command.
func Execute(version string) error {
	if err := newRootCmd(version).Execute(); err != nil {
		return fmt.Errorf("error executing root command: %w", err)
	}

	return nil
}
