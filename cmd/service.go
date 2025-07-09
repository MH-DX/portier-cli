package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kardianos/service"
	"github.com/mh-dx/portier-cli/internal/portier/application"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	internalService "github.com/mh-dx/portier-cli/internal/service"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

type serviceOptions struct {
	ConfigFile   string
	ApiTokenFile string
	Action       string
}

type portierService struct {
	options *serviceOptions
	app     *application.PortierApplication
	ctx     context.Context
	cancel  context.CancelFunc
}

func newServiceOptions() (*serviceOptions, error) {
	home, err := utils.Home()
	if err != nil {
		return nil, fmt.Errorf("could not get home directory: %w", err)
	}

	return &serviceOptions{
		ConfigFile:   filepath.Join(home, "config.yaml"),
		ApiTokenFile: filepath.Join(home, "credentials_device.yaml"),
	}, nil
}

func newServiceCmd() (*cobra.Command, error) {
	o, err := newServiceOptions()
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:   "service [install|uninstall|start|stop|restart|status]",
		Short: "Manage Portier CLI as a system service",
		Long: `Manage Portier CLI as a system service.

Available actions:
  install   - Install Portier CLI as a system service
  uninstall - Remove Portier CLI system service
  start     - Start the Portier CLI service
  stop      - Stop the Portier CLI service
  restart   - Restart the Portier CLI service
  status    - Show the status of the Portier CLI service`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.ConfigFile, "config", "c", o.ConfigFile, "custom config file path")
	cmd.Flags().StringVarP(&o.ApiTokenFile, "apitoken", "t", o.ApiTokenFile, "custom API token file path")

	return cmd, nil
}

func (o *serviceOptions) run(cmd *cobra.Command, args []string) error {
	o.Action = args[0]

	// Create service manager with configuration
	serviceConfig := &internalService.Config{
		ConfigFile:   o.ConfigFile,
		ApiTokenFile: o.ApiTokenFile,
	}

	serviceManager, err := internalService.NewServiceManager(serviceConfig)
	if err != nil {
		return fmt.Errorf("failed to create service manager: %w", err)
	}

	switch o.Action {
	case "install":
		return o.installServiceManager(serviceManager)
	case "uninstall":
		return o.uninstallServiceManager(serviceManager)
	case "start":
		return o.startServiceManager(serviceManager)
	case "stop":
		return o.stopServiceManager(serviceManager)
	case "restart":
		return o.restartServiceManager(serviceManager)
	case "status":
		return o.statusServiceManager(serviceManager)
	case "run":
		return o.runService(serviceManager.GetService())
	default:
		return fmt.Errorf("unknown action: %s", o.Action)
	}
}

func (o *serviceOptions) installService(s service.Service) error {
	fmt.Println("Installing Portier CLI service...")
	err := s.Install()
	if err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}
	fmt.Println("Service installed successfully")
	return nil
}

func (o *serviceOptions) uninstallService(s service.Service) error {
	fmt.Println("Uninstalling Portier CLI service...")
	err := s.Uninstall()
	if err != nil {
		return fmt.Errorf("failed to uninstall service: %w", err)
	}
	fmt.Println("Service uninstalled successfully")
	return nil
}

func (o *serviceOptions) startService(s service.Service) error {
	fmt.Println("Starting Portier CLI service...")
	err := s.Start()
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	fmt.Println("Service started successfully")
	return nil
}

func (o *serviceOptions) stopService(s service.Service) error {
	fmt.Println("Stopping Portier CLI service...")
	err := s.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	fmt.Println("Service stopped successfully")
	return nil
}

func (o *serviceOptions) restartService(s service.Service) error {
	fmt.Println("Restarting Portier CLI service...")
	err := s.Restart()
	if err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}
	fmt.Println("Service restarted successfully")
	return nil
}

func (o *serviceOptions) statusService(s service.Service) error {
	status, err := s.Status()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	var statusStr string
	switch status {
	case service.StatusRunning:
		statusStr = "Running"
	case service.StatusStopped:
		statusStr = "Stopped"
	case service.StatusUnknown:
		statusStr = "Unknown"
	default:
		statusStr = "Unknown"
	}

	fmt.Printf("Service Status: %s\n", statusStr)
	return nil
}

func (o *serviceOptions) runService(s service.Service) error {
	log.Println("Starting Portier CLI service...")
	return s.Run()
}

// Service interface implementation
func (p *portierService) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *portierService) Stop(s service.Service) error {
	log.Println("Stopping Portier CLI service...")
	p.cancel()
	return nil
}

func (p *portierService) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Service panic recovered: %v", r)
		}
	}()

	log.Println("Portier CLI service started")

	// Load configuration
	portierConfig, err := config.LoadConfig(p.options.ConfigFile)
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return
	}

	// Load API credentials
	deviceCreds, err := config.LoadApiToken(p.options.ApiTokenFile)
	if err != nil {
		log.Printf("Failed to load API token: %v", err)
		return
	}

	// Start services
	err = p.app.StartServices(portierConfig, deviceCreds)
	if err != nil {
		log.Printf("Failed to start services: %v", err)
		return
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received signal: %v", sig)
	case <-p.ctx.Done():
		log.Println("Service context cancelled")
	}

	// Stop services
	log.Println("Stopping services...")
	err = p.app.StopServices()
	if err != nil {
		log.Printf("Error stopping services: %v", err)
	}

	log.Println("Portier CLI service stopped")
}

func (o *serviceOptions) installServiceManager(sm *internalService.ServiceManager) error {
	fmt.Println("Installing Portier CLI service...")
	err := sm.Install()
	if err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}
	fmt.Println("Service installed successfully")
	return nil
}

func (o *serviceOptions) uninstallServiceManager(sm *internalService.ServiceManager) error {
	fmt.Println("Uninstalling Portier CLI service...")
	err := sm.Uninstall()
	if err != nil {
		return fmt.Errorf("failed to uninstall service: %w", err)
	}
	fmt.Println("Service uninstalled successfully")
	return nil
}

func (o *serviceOptions) startServiceManager(sm *internalService.ServiceManager) error {
	fmt.Println("Starting Portier CLI service...")
	err := sm.Start()
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	fmt.Println("Service started successfully")
	return nil
}

func (o *serviceOptions) stopServiceManager(sm *internalService.ServiceManager) error {
	fmt.Println("Stopping Portier CLI service...")
	err := sm.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	fmt.Println("Service stopped successfully")
	return nil
}

func (o *serviceOptions) restartServiceManager(sm *internalService.ServiceManager) error {
	fmt.Println("Restarting Portier CLI service...")
	err := sm.Restart()
	if err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}
	fmt.Println("Service restarted successfully")
	return nil
}

func (o *serviceOptions) statusServiceManager(sm *internalService.ServiceManager) error {
	status, err := sm.Status()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	var statusStr string
	switch status {
	case service.StatusRunning:
		statusStr = "Running"
	case service.StatusStopped:
		statusStr = "Stopped"
	case service.StatusUnknown:
		statusStr = "Unknown"
	default:
		statusStr = "Unknown"
	}

	fmt.Printf("Service Status: %s\n", statusStr)
	return nil
}