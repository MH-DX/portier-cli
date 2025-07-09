//go:build windows
// +build windows

package tray

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/getlantern/systray"
	"github.com/mh-dx/portier-cli/internal/portier/application"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/skratchdot/open-golang/open"
)

type TrayApp struct {
	app              *application.PortierApplication
	menuStatusItem   *systray.MenuItem
	menuStartItem    *systray.MenuItem
	menuStopItem     *systray.MenuItem
	menuRestartItem  *systray.MenuItem
	menuConfigItem   *systray.MenuItem
	menuAPIKeyItem   *systray.MenuItem
	menuQuitItem     *systray.MenuItem
	isRunning        bool
	configPath       string
	apiKeyPath       string
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewTrayApp creates a new system tray application
func NewTrayApp() *TrayApp {
	home, err := utils.Home()
	if err != nil {
		log.Printf("could not get home directory: %v", err)
		return nil
	}

	configPath := filepath.Join(home, "config.yaml")
	apiKeyPath := filepath.Join(home, "credentials_device.yaml")

	ctx, cancel := context.WithCancel(context.Background())

	return &TrayApp{
		app:        application.GetPortierApplication(),
		configPath: configPath,
		apiKeyPath: apiKeyPath,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run starts the system tray application
func (t *TrayApp) Run() {
	systray.Run(t.onReady, t.onExit)
}

// onReady initializes the system tray
func (t *TrayApp) onReady() {
	// Set icon - using a simple dot for now (in a real implementation, use a proper icon)
	systray.SetIcon(getIcon())
	systray.SetTitle("Portier CLI")
	systray.SetTooltip("Portier CLI - Remote Access Tool")

	// Create menu items
	t.menuStatusItem = systray.AddMenuItem("Status: Stopped", "Service Status")
	t.menuStatusItem.Disable()

	systray.AddSeparator()

	t.menuStartItem = systray.AddMenuItem("Start Service", "Start Portier Service")
	t.menuStopItem = systray.AddMenuItem("Stop Service", "Stop Portier Service")
	t.menuRestartItem = systray.AddMenuItem("Restart Service", "Restart Portier Service")

	systray.AddSeparator()

	t.menuConfigItem = systray.AddMenuItem("Open Config", "Open Configuration File")
	t.menuAPIKeyItem = systray.AddMenuItem("Open API Key", "Open API Key File")

	systray.AddSeparator()

	t.menuQuitItem = systray.AddMenuItem("Quit", "Quit Portier CLI")

	// Update initial state
	t.updateMenuState()

	// Start periodic status updates
	go t.statusUpdateLoop()

	// Handle menu clicks
	go t.handleMenuClicks()
}

// onExit handles cleanup when the tray application exits
func (t *TrayApp) onExit() {
	t.cancel()
	if t.isRunning {
		t.stopService()
	}
}

// statusUpdateLoop periodically updates the service status
func (t *TrayApp) statusUpdateLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.updateStatus()
		case <-t.ctx.Done():
			return
		}
	}
}

// updateStatus checks and updates the service status
func (t *TrayApp) updateStatus() {
	wasRunning := t.isRunning
	t.isRunning = t.app.IsRunning()

	if wasRunning != t.isRunning {
		t.updateMenuState()
	}
}

// updateMenuState updates the menu items based on service status
func (t *TrayApp) updateMenuState() {
	if t.isRunning {
		t.menuStatusItem.SetTitle("Status: Running")
		t.menuStartItem.Disable()
		t.menuStopItem.Enable()
		t.menuRestartItem.Enable()
		systray.SetTooltip("Portier CLI - Running")
	} else {
		t.menuStatusItem.SetTitle("Status: Stopped")
		t.menuStartItem.Enable()
		t.menuStopItem.Disable()
		t.menuRestartItem.Disable()
		systray.SetTooltip("Portier CLI - Stopped")
	}
}

// handleMenuClicks handles menu item clicks
func (t *TrayApp) handleMenuClicks() {
	for {
		select {
		case <-t.menuStartItem.ClickedCh:
			go t.startService()
		case <-t.menuStopItem.ClickedCh:
			go t.stopService()
		case <-t.menuRestartItem.ClickedCh:
			go t.restartService()
		case <-t.menuConfigItem.ClickedCh:
			go t.openConfigFile()
		case <-t.menuAPIKeyItem.ClickedCh:
			go t.openAPIKeyFile()
		case <-t.menuQuitItem.ClickedCh:
			systray.Quit()
			return
		case <-t.ctx.Done():
			return
		}
	}
}

// startService starts the Portier service
func (t *TrayApp) startService() {
	if t.isRunning {
		return
	}

	// Ensure config file exists
	if err := t.ensureConfigExists(); err != nil {
		log.Printf("Failed to create config file: %v", err)
		return
	}

	// Load configuration
	portierConfig, err := config.LoadConfig(t.configPath)
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return
	}

	// Load API credentials
	deviceCreds, err := config.LoadApiToken(t.apiKeyPath)
	if err != nil {
		log.Printf("Failed to load API token: %v", err)
		return
	}

	// Start services
	err = t.app.StartServices(portierConfig, deviceCreds)
	if err != nil {
		log.Printf("Failed to start services: %v", err)
		return
	}

	log.Println("Portier service started successfully")
	t.updateStatus()
}

// stopService stops the Portier service
func (t *TrayApp) stopService() {
	if !t.isRunning {
		return
	}

	err := t.app.StopServices()
	if err != nil {
		log.Printf("Failed to stop services: %v", err)
		return
	}

	log.Println("Portier service stopped successfully")
	t.updateStatus()
}

// restartService restarts the Portier service
func (t *TrayApp) restartService() {
	if t.isRunning {
		t.stopService()
		time.Sleep(2 * time.Second) // Wait a moment before restarting
	}
	t.startService()
}

// openConfigFile opens the configuration file in the default editor
func (t *TrayApp) openConfigFile() {
	if err := t.ensureConfigExists(); err != nil {
		log.Printf("Failed to create config file: %v", err)
		return
	}

	err := open.Run(t.configPath)
	if err != nil {
		log.Printf("Failed to open config file: %v", err)
	}
}

// openAPIKeyFile opens the API key file in the default editor
func (t *TrayApp) openAPIKeyFile() {
	if _, err := os.Stat(t.apiKeyPath); os.IsNotExist(err) {
		log.Printf("API key file does not exist: %s", t.apiKeyPath)
		return
	}

	err := open.Run(t.apiKeyPath)
	if err != nil {
		log.Printf("Failed to open API key file: %v", err)
	}
}

// ensureConfigExists creates a default config file if it doesn't exist
func (t *TrayApp) ensureConfigExists() error {
	if _, err := os.Stat(t.configPath); os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		dir := filepath.Dir(t.configPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Create a default config
		defaultConfig, err := config.DefaultPortierConfig()
		if err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}

		// Save the default config
		if err := config.SaveConfig(t.configPath, defaultConfig); err != nil {
			return fmt.Errorf("failed to save default config: %w", err)
		}

		log.Printf("Created default config file: %s", t.configPath)
	}
	return nil
}

// getIcon returns a simple icon for the system tray
// In a real implementation, this would load from an embedded icon file
func getIcon() []byte {
	// Simple 16x16 black dot icon in ICO format
	return []byte{
		0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x10, 0x10, 0x00, 0x00, 0x01, 0x00, 0x20, 0x00, 0x68, 0x04,
		0x00, 0x00, 0x16, 0x00, 0x00, 0x00, 0x28, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x20, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}