package cmd

import (
	"log"

	"github.com/mh-dx/portier-cli/pkg/webwizard"
	"github.com/spf13/cobra"
)

func newWizardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wizard",
		Short: "Launch the setup wizard",
		Long: `Launch the Portier CLI setup wizard.

This command starts a web-based setup wizard that guides you through:
- Logging in to your Portier account
- Registering this device
- Installing and starting the service
- Completing the initial configuration

The wizard will automatically open in your default web browser.`,
		SilenceUsage: true,
		RunE:         runWizard,
	}

	return cmd
}

func runWizard(cmd *cobra.Command, args []string) error {
	log.Println("Starting Portier CLI Setup Wizard...")

	wizard := webwizard.NewWizardServer()
	if err := wizard.Start(); err != nil {
		return err
	}

	log.Println("Setup wizard is running. Check your browser or press Ctrl+C to stop.")

	// Wait for the wizard to complete or be cancelled
	wizard.Wait()

	// Clean up
	if err := wizard.Stop(); err != nil {
		log.Printf("Error stopping wizard server: %v", err)
	}

	log.Println("Setup wizard completed")
	return nil
}