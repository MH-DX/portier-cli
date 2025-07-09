//go:build windows
// +build windows

package tray

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	internalService "github.com/mh-dx/portier-cli/internal/service"
	"github.com/mh-dx/portier-cli/internal/utils"
)

// ServiceController manages the OS service for the tray application
type ServiceController struct {
	serviceManager *internalService.ServiceManager
	configFile     string
	apiKeyFile     string
}

// NewServiceController creates a new service controller
func NewServiceController() (*ServiceController, error) {
	home, err := utils.Home()
	if err != nil {
		return nil, fmt.Errorf("could not get home directory: %w", err)
	}

	configFile := filepath.Join(home, "config.yaml")
	apiKeyFile := filepath.Join(home, "credentials_device.yaml")

	// Create service manager with configuration
	serviceConfig := &internalService.Config{
		ConfigFile:   configFile,
		ApiTokenFile: apiKeyFile,
	}

	serviceManager, err := internalService.NewServiceManager(serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create service manager: %w", err)
	}

	return &ServiceController{
		serviceManager: serviceManager,
		configFile:     configFile,
		apiKeyFile:     apiKeyFile,
	}, nil
}

// Start starts the OS service
func (sc *ServiceController) Start() error {
	log.Println("Starting Portier CLI service...")
	err := sc.serviceManager.Start()
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	log.Println("Service started successfully")
	return nil
}

// Stop stops the OS service
func (sc *ServiceController) Stop() error {
	log.Println("Stopping Portier CLI service...")
	err := sc.serviceManager.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	log.Println("Service stopped successfully")
	return nil
}

// Restart restarts the OS service
func (sc *ServiceController) Restart() error {
	log.Println("Restarting Portier CLI service...")
	err := sc.serviceManager.Restart()
	if err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}
	log.Println("Service restarted successfully")
	return nil
}

// Status returns the current status of the OS service
func (sc *ServiceController) Status() (service.Status, error) {
	return sc.serviceManager.Status()
}

// IsRunning returns true if the service is running
func (sc *ServiceController) IsRunning() bool {
	return sc.serviceManager.IsRunning()
}

// Install installs the service
func (sc *ServiceController) Install() error {
	log.Println("Installing Portier CLI service...")
	err := sc.serviceManager.Install()
	if err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}
	log.Println("Service installed successfully")
	return nil
}

// Uninstall removes the service
func (sc *ServiceController) Uninstall() error {
	log.Println("Uninstalling Portier CLI service...")
	err := sc.serviceManager.Uninstall()
	if err != nil {
		return fmt.Errorf("failed to uninstall service: %w", err)
	}
	log.Println("Service uninstalled successfully")
	return nil
}

// EnsureConfigExists creates default configuration files if they don't exist
func (sc *ServiceController) EnsureConfigExists() error {
	// Ensure config file exists
	if _, err := os.Stat(sc.configFile); os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		dir := filepath.Dir(sc.configFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Create a default config
		defaultConfig, err := config.DefaultPortierConfig()
		if err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}

		// Save the default config
		if err := config.SaveConfig(sc.configFile, defaultConfig); err != nil {
			return fmt.Errorf("failed to save default config: %w", err)
		}

		log.Printf("Created default config file: %s", sc.configFile)
	}
	return nil
}

// GetConfigPath returns the path to the config file
func (sc *ServiceController) GetConfigPath() string {
	return sc.configFile
}

// GetAPIKeyPath returns the path to the API key file
func (sc *ServiceController) GetAPIKeyPath() string {
	return sc.apiKeyFile
}
