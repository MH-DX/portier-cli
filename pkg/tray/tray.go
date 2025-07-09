//go:build windows
// +build windows

package tray

import (
	"context"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
)

type TrayApp struct {
	serviceController *ServiceController
	menuStatusItem    *systray.MenuItem
	menuStartItem     *systray.MenuItem
	menuStopItem      *systray.MenuItem
	menuRestartItem   *systray.MenuItem
	menuConfigItem    *systray.MenuItem
	menuAPIKeyItem    *systray.MenuItem
	menuQuitItem      *systray.MenuItem
	isRunning         bool
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewTrayApp creates a new system tray application
func NewTrayApp() *TrayApp {
	serviceController, err := NewServiceController()
	if err != nil {
		log.Printf("Failed to create service controller: %v", err)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TrayApp{
		serviceController: serviceController,
		ctx:               ctx,
		cancel:            cancel,
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
	// Note: We don't stop the service here as it should continue running
	// The service is managed independently of the tray application
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
	t.isRunning = t.serviceController.IsRunning()

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
	if err := t.serviceController.EnsureConfigExists(); err != nil {
		log.Printf("Failed to create config file: %v", err)
		return
	}

	// Start the OS service
	err := t.serviceController.Start()
	if err != nil {
		log.Printf("Failed to start service: %v", err)
		return
	}

	// Update status after a short delay to allow service to start
	go func() {
		time.Sleep(2 * time.Second)
		t.updateStatus()
	}()
}

// stopService stops the Portier service
func (t *TrayApp) stopService() {
	if !t.isRunning {
		return
	}

	err := t.serviceController.Stop()
	if err != nil {
		log.Printf("Failed to stop service: %v", err)
		return
	}

	// Update status after a short delay to allow service to stop
	go func() {
		time.Sleep(2 * time.Second)
		t.updateStatus()
	}()
}

// restartService restarts the Portier service
func (t *TrayApp) restartService() {
	err := t.serviceController.Restart()
	if err != nil {
		log.Printf("Failed to restart service: %v", err)
		return
	}

	// Update status after a short delay to allow service to restart
	go func() {
		time.Sleep(3 * time.Second)
		t.updateStatus()
	}()
}

// openConfigFile opens the configuration file in the default editor
func (t *TrayApp) openConfigFile() {
	if err := t.serviceController.EnsureConfigExists(); err != nil {
		log.Printf("Failed to create config file: %v", err)
		return
	}

	err := open.Run(t.serviceController.GetConfigPath())
	if err != nil {
		log.Printf("Failed to open config file: %v", err)
	}
}

// openAPIKeyFile opens the API key file in the default editor
func (t *TrayApp) openAPIKeyFile() {
	apiKeyPath := t.serviceController.GetAPIKeyPath()
	if _, err := os.Stat(apiKeyPath); os.IsNotExist(err) {
		log.Printf("API key file does not exist: %s", apiKeyPath)
		return
	}

	err := open.Run(apiKeyPath)
	if err != nil {
		log.Printf("Failed to open API key file: %v", err)
	}
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
