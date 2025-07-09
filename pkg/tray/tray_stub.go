//go:build !windows
// +build !windows

package tray

import (
	"fmt"
	"runtime"
)

type TrayApp struct{}

// NewTrayApp creates a new system tray application (stub for non-Windows platforms)
func NewTrayApp() *TrayApp {
	return &TrayApp{}
}

// Run starts the system tray application (stub for non-Windows platforms)
func (t *TrayApp) Run() {
	fmt.Printf("System tray functionality is not available on %s\n", runtime.GOOS)
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}