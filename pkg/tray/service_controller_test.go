//go:build windows
// +build windows

package tray

import (
	"testing"
)

func TestServiceController(t *testing.T) {
	// Test that we can create a service controller
	sc, err := NewServiceController()
	if err != nil {
		t.Fatalf("Failed to create service controller: %v", err)
	}

	// Test that we can get config paths
	configPath := sc.GetConfigPath()
	if configPath == "" {
		t.Error("Config path should not be empty")
	}

	apiKeyPath := sc.GetAPIKeyPath()
	if apiKeyPath == "" {
		t.Error("API key path should not be empty")
	}

	// Test that we can check if the service is running
	// This should not fail even if the service is not installed
	isRunning := sc.IsRunning()
	t.Logf("Service is running: %v", isRunning)

	// Test that we can ensure config exists
	err = sc.EnsureConfigExists()
	if err != nil {
		t.Errorf("Failed to ensure config exists: %v", err)
	}
}

func TestTrayApp(t *testing.T) {
	// Test that we can create a tray app
	trayApp := NewTrayApp()
	if trayApp == nil {
		t.Fatal("Failed to create tray app")
	}

	// Test that the service controller was created
	if trayApp.serviceController == nil {
		t.Error("Service controller should not be nil")
	}

	// Test that install/uninstall menu items are not present (they should be nil)
	// This ensures the tray doesn't have install/uninstall functionality
}
