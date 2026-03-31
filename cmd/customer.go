package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func newCustomerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "customer",
		Short:        "Customer-scoped Portier setup helpers",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Printf("subcommand %q not found", args[0])
			return cmd.Help()
		},
	}

	cmd.AddCommand(newCustomerSetupCmd())

	return cmd
}
