package ptls_cmd

import (
	"log"

	"github.com/spf13/cobra"
)

// TODO add command to trust a peer device by adding its TLS cert FP to known_hosts
// TODO add check for existing TLS configs, auto-add if not present

func NewTLScmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:          "tls",
		Short:        "The tls commands manage TLS certificates for devices",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Printf("subcommand \"%s\" not found", args[0])
			return cmd.Help()
		},
	}

	return cmd
}
