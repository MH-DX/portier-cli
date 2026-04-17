package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kardianos/service"
	"github.com/mh-dx/portier-cli/internal/portier/application"
	"github.com/mh-dx/portier-cli/internal/portier/config"
)

// Config holds the shared service configuration
type Config struct {
	ConfigFile   string
	ApiTokenFile string
	LogFile      string
}

// ServiceManager provides a unified interface for service management
type ServiceManager struct {
	config  *Config
	service service.Service
}

// NewServiceManager creates a new service manager with the given configuration
func NewServiceManager(cfg *Config) (*ServiceManager, error) {
	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not get executable path: %w", err)
	}

	// Build service arguments with config file paths
	args := []string{}
	if cfg.LogFile != "" {
		args = append(args, "-logfile", cfg.LogFile)
	}
	args = append(args, "service", "run")
	if cfg.ConfigFile != "" {
		args = append(args, "-c", cfg.ConfigFile)
	}
	if cfg.ApiTokenFile != "" {
		args = append(args, "-t", cfg.ApiTokenFile)
	}
	if cfg.LogFile != "" {
		args = append(args, "-l", cfg.LogFile)
	}

	svcConfig := &service.Config{
		Name:        "portier-cli",
		DisplayName: "Portier CLI Service",
		Description: "Portier CLI remote access tunneling service",
		Executable:  execPath,
		Arguments:   args,
	}

	prg := &portierServiceProgram{
		config:  cfg,
		app:     application.GetPortierApplication(),
		logFile: cfg.LogFile,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return &ServiceManager{
		config:  cfg,
		service: s,
	}, nil
}

// Start starts the OS service
func (sm *ServiceManager) Start() error {
	return sm.service.Start()
}

// Stop stops the OS service
func (sm *ServiceManager) Stop() error {
	return sm.service.Stop()
}

// Restart restarts the OS service
func (sm *ServiceManager) Restart() error {
	return sm.service.Restart()
}

// Status returns the current status of the OS service
func (sm *ServiceManager) Status() (service.Status, error) {
	return sm.service.Status()
}

// IsRunning returns true if the service is running
func (sm *ServiceManager) IsRunning() bool {
	status, err := sm.Status()
	if err != nil {
		return false
	}
	return status == service.StatusRunning
}

// Install installs the service
func (sm *ServiceManager) Install() error {
	if err := sm.service.Install(); err != nil {
		if !isAlreadyInstalledError(err) {
			return err
		}

		_ = sm.service.Stop()
		if uninstallErr := sm.service.Uninstall(); uninstallErr != nil {
			return fmt.Errorf("failed to replace existing service: %w", uninstallErr)
		}
		if installErr := sm.service.Install(); installErr != nil {
			return fmt.Errorf("failed to install replacement service: %w", installErr)
		}
	}

	return nil
}

// Uninstall removes the service
func (sm *ServiceManager) Uninstall() error {
	return sm.service.Uninstall()
}

// GetService returns the underlying service interface
func (sm *ServiceManager) GetService() service.Service {
	return sm.service
}

func isAlreadyInstalledError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already exists") ||
		(strings.Contains(message, "service") && strings.Contains(message, "exists"))
}

// portierServiceProgram implements the service.Interface
type portierServiceProgram struct {
	config  *Config
	app     *application.PortierApplication
	ctx     context.Context
	cancel  context.CancelFunc
	logFile string
}

func (p *portierServiceProgram) Start(s service.Service) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	go p.run()
	return nil
}

func (p *portierServiceProgram) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

func (p *portierServiceProgram) run() {
	// Load configuration
	portierConfig, err := config.LoadConfig(p.config.ConfigFile)
	if err != nil {
		return
	}

	// Load API credentials
	apiBaseURL := portierConfig.APIBaseURL()
	deviceCreds, err := config.LoadApiTokenWithBaseURL(p.config.ApiTokenFile, apiBaseURL)
	if err != nil {
		return
	}

	// Start services
	err = p.app.StartServices(portierConfig, deviceCreds)
	if err != nil {
		return
	}

	// Wait for cancellation
	<-p.ctx.Done()

	// Stop services
	p.app.StopServices()
}
