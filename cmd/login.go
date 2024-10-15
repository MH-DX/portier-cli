package cmd

import (
	portier "github.com/mh-dx/portier-cli/internal/portier/api"
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
		Short:        "Login to the portier service using PKCE flow. Needs a browser to complete",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE:         o.run,
	}

	return cmd
}

func (o *loginOptions) run(cmd *cobra.Command, _ []string) error {
	return portier.Login()
}
