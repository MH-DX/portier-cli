package cmd

import (
	"fmt"
	"runtime"

	"github.com/mh-dx/portier-cli/pkg/tray"
	"github.com/spf13/cobra"
)

func newTrayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tray",
		Short: "Run Portier CLI system tray interface",
		Long: `Run Portier CLI system tray interface.

This command starts the system tray application that allows you to:
- Monitor service status
- Start/stop/restart the service
- Open configuration files
- Control the service through a GUI interface

Note: This command is primarily designed for Windows systems.`,
		SilenceUsage: true,
		RunE:         runTray,
	}

	return cmd
}

func runTray(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("system tray functionality is only available on Windows")
	}

	fmt.Println("Starting Portier CLI system tray...")

	// Check if the service is installed, and install if not
	controller, err := tray.NewServiceController()
	if err != nil {
		return fmt.Errorf("failed to create service controller: %w", err)
	}
	status, err := controller.Status()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}
	if status == 3 { // service.StatusUnknown == 3
		fmt.Println("Service not installed. Installing...")
		err := controller.Install()
		if err != nil {
			return fmt.Errorf("failed to install service: %w", err)
		}
		fmt.Println("Service installed successfully.")
	}

	trayApp := tray.NewTrayApp()
	if trayApp == nil {
		return fmt.Errorf("failed to create tray application")
	}

	// This will block until the tray application exits
	trayApp.Run()

	return nil
}
