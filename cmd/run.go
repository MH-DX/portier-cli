package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/marinator86/portier-cli/internal/portier/application"
	"github.com/marinator86/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

type runOptions struct {
	ConfigFile   string
	ApiTokenFile string
	Output       string
}

func defaultRunOptions() (*runOptions, error) {
	home, err := utils.Home()
	if err != nil {
		log.Printf("could not get home directory: %v", err)
		return nil, err
	}

	configFile := filepath.Join(home, "config.yaml")
	apiTokenFile := filepath.Join(home, "credentials_device.yaml")

	return &runOptions{
		ConfigFile:   configFile,
		ApiTokenFile: apiTokenFile,
		Output:       "json",
	}, nil
}

func newRunCmd() (*cobra.Command, error) {
	o, err := defaultRunOptions()
	if err != nil {
		log.Printf("could not get default options: %v", err)
		return nil, err
	}

	cmd := &cobra.Command{
		Use:          "run",
		Short:        "starts the forwarding for all services defined",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         o.run,
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

	cmd.Flags().StringVarP(&o.ConfigFile, "config file", "c", o.ConfigFile, "custom config file path")
	cmd.Flags().StringVarP(&o.ApiTokenFile, "apiToken file", "t", o.ApiTokenFile, "custom apiToken file path")

	return cmd, nil
}

func (o *runOptions) run(cmd *cobra.Command, args []string) error {
	err := o.parseArgs(cmd, args)
	if err != nil {
		log.Println("could not parse args")
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "starting device, services %s, apiToken %s, out %s\n", o.ConfigFile, o.ApiTokenFile, o.Output)

	application := application.NewPortierApplication()

	application.LoadConfig(o.ConfigFile)

	application.LoadApiToken(o.ApiTokenFile)

	application.StartServices()

	// wait until process is killed
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	application.StopServices()

	return nil
}

func (o *runOptions) parseArgs(cmd *cobra.Command, _ []string) error {
	configFile, err := cmd.Flags().GetString("config")
	if err == nil {
		o.ConfigFile = configFile
	}

	apiTokenFile, err := cmd.Flags().GetString("apiToken")
	if err == nil {
		o.ApiTokenFile = apiTokenFile
	}

	return nil
}
