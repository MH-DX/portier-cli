package cmd

import (
	"fmt"
	"log"

	"github.com/marinator86/portier-cli/internal/daemon"
	"github.com/marinator86/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

type startOptions struct {
	Command      string
	ServicesFile string
	Output       string
}

func defaultStartOptions() *startOptions {
	home, err := utils.Home()
	if err != nil {
		log.Fatalf("could not get home directory: %v", err)
	}
	servicesFile := home + "/services.yaml"
	return &startOptions{
		Command:      "start",
		ServicesFile: servicesFile,
		Output:       "json",
	}
}

func newStartCmd() *cobra.Command {
	o := defaultStartOptions()

	cmd := &cobra.Command{
		Use:          "start",
		Short:        "start a device",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.Command, "action", "a", o.Command, "Service action to perform. Valid actions: [start, stop, restart, install, uninstall]")
	cmd.Flags().StringVarP(&o.ServicesFile, "services", "s", o.ServicesFile, "services file path")
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "output format (yaml | json)")

	return cmd
}

func (o *startOptions) run(cmd *cobra.Command, args []string) error {
	err := o.parseArgs(cmd, args)
	if err != nil {
		return err
	}

	// TODO: check if api key exists
	fmt.Fprintf(cmd.OutOrStdout(), "starting device, services %s out %s\n", o.ServicesFile, o.Output)
	err = daemon.StartDaemon(o.Command)
	if err != nil {
		log.Fatalf("could not start daemon: %v", err)
		return err
	}
	return nil
}

func (o *startOptions) parseArgs(cmd *cobra.Command, _ []string) error {
	command, err := cmd.Flags().GetString("action")
	if err != nil {
		log.Fatalf("could not get action flag: %v", err)
		return err
	}
	servicesFile, err := cmd.Flags().GetString("services")
	if err != nil {
		log.Fatalf("could not get services flag: %v", err)
		return err
	}
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		log.Fatalf("could not get output flag: %v", err)
		return err
	}
	o.Command = command
	o.ServicesFile = servicesFile
	o.Output = output
	return nil
}
