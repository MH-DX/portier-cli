package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "task",
		Short:        "Task-scoped workflows for operator handoffs",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Printf("subcommand %q not found", args[0])
			return cmd.Help()
		},
	}

	cmd.AddCommand(newTaskEnrollCmd())

	return cmd
}
