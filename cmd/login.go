package cmd

import (
	"fmt"

	"github.com/marinator86/portier-cli/internal/portier"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	// TODO add options
}

func defaultLoginOptions() *loginOptions {
	return &loginOptions{}
}

func newLoginCmd() *cobra.Command {
	o := defaultLoginOptions()

	cmd := &cobra.Command{
		Use:          "login",
		Short:        "login subcommand which logs in to the portier service using PKCE flow. Needs a browser to complete the login flow.",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE:         o.run,
	}

	return cmd
}

func (o *loginOptions) run(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Logging in...\n")
	return portier.Login()
}
