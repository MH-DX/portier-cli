//go:build !windows
// +build !windows

package tray

import (
	"fmt"
	"runtime"

	"github.com/kardianos/service"
)

// ServiceController manages the OS service (stub for non-Windows platforms)
type ServiceController struct{}

// NewServiceController creates a new service controller (stub for non-Windows platforms)
func NewServiceController() (*ServiceController, error) {
	return &ServiceController{}, nil
}

// Start starts the OS service (stub for non-Windows platforms)
func (sc *ServiceController) Start() error {
	return fmt.Errorf("service management is not available on %s", runtime.GOOS)
}

// Stop stops the OS service (stub for non-Windows platforms)
func (sc *ServiceController) Stop() error {
	return fmt.Errorf("service management is not available on %s", runtime.GOOS)
}

// Restart restarts the OS service (stub for non-Windows platforms)
func (sc *ServiceController) Restart() error {
	return fmt.Errorf("service management is not available on %s", runtime.GOOS)
}

// Status returns the current status of the OS service (stub for non-Windows platforms)
func (sc *ServiceController) Status() (service.Status, error) {
	return service.StatusUnknown, fmt.Errorf("service management is not available on %s", runtime.GOOS)
}

// IsRunning returns true if the service is running (stub for non-Windows platforms)
func (sc *ServiceController) IsRunning() bool {
	return false
}

// Install installs the service (stub for non-Windows platforms)
func (sc *ServiceController) Install() error {
	return fmt.Errorf("service management is not available on %s", runtime.GOOS)
}

// Uninstall removes the service (stub for non-Windows platforms)
func (sc *ServiceController) Uninstall() error {
	return fmt.Errorf("service management is not available on %s", runtime.GOOS)
}

// EnsureConfigExists creates default configuration files if they don't exist (stub for non-Windows platforms)
func (sc *ServiceController) EnsureConfigExists() error {
	return fmt.Errorf("service management is not available on %s", runtime.GOOS)
}

// GetConfigPath returns the path to the config file (stub for non-Windows platforms)
func (sc *ServiceController) GetConfigPath() string {
	return ""
}

// GetAPIKeyPath returns the path to the API key file (stub for non-Windows platforms)
func (sc *ServiceController) GetAPIKeyPath() string {
	return ""
}
