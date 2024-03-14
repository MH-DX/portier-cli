package cmd

import (
	"fmt"
	"log"

	"github.com/marinator86/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

// TODO rename to "service" command, i.e. portier service start --config config.yaml --apiToken apiToken.yaml ...

type startOptions struct {
	Command      string
	ConfigFile   string
	ApiTokenFile string
	Output       string
}

func defaultStartOptions() (*startOptions, error) {
	home, err := utils.Home()
	if err != nil {
		log.Printf("could not get home directory: %v", err)
		return nil, err
	}

	configFile := home + "/config.yaml"
	apiTokenFile := home + "/apiToken.yaml"

	return &startOptions{
		Command:      "start",
		ConfigFile:   configFile,
		ApiTokenFile: apiTokenFile,
		Output:       "json",
	}, nil
}

func newStartCmd() (*cobra.Command, error) {
	o, err := defaultStartOptions()
	if err != nil {
		log.Printf("could not get default options: %v", err)
		return nil, err
	}

	cmd := &cobra.Command{
		Use:          "start",
		Short:        "start all local services defined for this device (requires registration)",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.Command, "action", "a", o.Command, "Service action to perform. Valid actions: [start, stop, restart, install, uninstall]")
	cmd.Flags().StringVarP(&o.ConfigFile, "config file", "c", o.ConfigFile, "config file path")
	cmd.Flags().StringVarP(&o.ApiTokenFile, "apiToken file", "t", o.ApiTokenFile, "apiToken file path")
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "output format (yaml | json)")

	return cmd, nil
}

func (o *startOptions) run(cmd *cobra.Command, args []string) error {
	err := o.parseArgs(cmd, args)
	if err != nil {
		log.Println("could not parse args")
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "starting device, services %s, apiToken %s, out %s\n", o.ConfigFile, o.ApiTokenFile, o.Output)

	panic("not implemented - start daemon")

	return nil
}

func (o *startOptions) parseArgs(cmd *cobra.Command, _ []string) error {
	command, err := cmd.Flags().GetString("action")
	if err != nil {
		log.Printf("could not get action flag: %v", err)
		return err
	}

	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		log.Printf("could not get config flag: %v", err)
		return err
	}

	apiTokenFile, err := cmd.Flags().GetString("apiToken")
	if err != nil {
		log.Printf("could not get apiToken flag: %v", err)
		return err
	}

	output, err := cmd.Flags().GetString("output")
	if err != nil {
		log.Printf("could not get output flag: %v", err)
		return err
	}

	o.Command = command
	o.ConfigFile = configFile
	o.ApiTokenFile = apiTokenFile
	o.Output = output

	return nil
}
