package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

type startOptions struct {
	Output string
}

func defaultStartOptions() *startOptions {
	return &startOptions{
		Output: "json",
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

	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "output format (yaml | json)")

	return cmd
}

func (o *startOptions) run(cmd *cobra.Command, args []string) error {
	err := o.parseArgs(cmd, args)
	if err != nil {
		return err
	}

	// TODO: check if api key exists
	fmt.Fprintf(cmd.OutOrStdout(), "starting device, out %s\n", o.Output)
	// TODO: start the daemon
	return nil
}

func (o *startOptions) parseArgs(cmd *cobra.Command, args []string) error {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		log.Fatalf("could not get output flag: %v", err)
		return err
	}
	o.Output = output
	return nil
}
