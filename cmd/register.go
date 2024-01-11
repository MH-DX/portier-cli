package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

type registerOptions struct {
	Name   string
	Output string
}

func defaultRegisterOptions() *registerOptions {
	return &registerOptions{
		Output: "json",
	}
}

func newRegisterCmd() *cobra.Command {
	o := defaultRegisterOptions()

	cmd := &cobra.Command{
		Use:          "register",
		Short:        "register a device",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "output format (yaml | json)")
	cmd.Flags().StringVarP(&o.Name, "name", "n", o.Name, "name of the device")

	return cmd
}

func (o *registerOptions) run(cmd *cobra.Command, args []string) error {
	err := o.parseArgs(cmd, args)
	if err != nil {
		return err
	}

	if o.Name == "" {
		log.Fatalf("name is required to register a device")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "create device %s, out %s\n", o.Name, o.Output)
	// TODO: create device
	// TODO: create api key and store in ~/.portier/config.yaml
	return nil
}

func (o *registerOptions) parseArgs(cmd *cobra.Command, _ []string) error {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		log.Fatalf("could not get output flag: %v", err)
		return err
	}
	o.Output = output

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		log.Fatalf("could not get name flag: %v", err)
		return err
	}
	o.Name = name

	return nil
}
