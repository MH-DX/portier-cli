# Windows System Tray Implementation

This document describes the Windows System Tray implementation for Portier CLI.

## Overview

The Windows System Tray implementation provides a user-friendly GUI interface for managing the Portier CLI service on Windows systems. It uses a dual binary approach to provide both console and GUI functionality.

## Components

### 1. System Tray Package (`pkg/tray/tray.go`)

The core system tray functionality that provides:

- **System Tray Icon**: Shows service status in the system tray
- **Context Menu**: Provides easy access to service controls
- **Status Updates**: Automatically updates service status every 10 seconds
- **Service Control**: Start, stop, and restart the Portier service
- **File Access**: Open configuration and API key files
- **Auto-configuration**: Creates default config files if missing

Key features:
- Uses `github.com/getlantern/systray` for tray functionality
- Windows-specific build tags for platform compatibility
- Automatic service status monitoring
- Integrated with existing Portier application lifecycle

### 2. Service Command (`cmd/service.go`)

Enhanced service management with Windows service support:

- **Service Installation**: Install/uninstall as Windows service
- **Service Control**: Start, stop, restart, and status commands
- **Background Operation**: Runs as a proper Windows service
- **Error Handling**: Comprehensive error handling and logging
- **Configuration**: Supports custom config and API token paths

### 3. Tray Command (`cmd/tray.go`)

CLI command to run the system tray interface:

- **Platform Detection**: Only runs on Windows systems
- **User Interface**: Launches the system tray GUI
- **Help Documentation**: Provides usage information

### 4. GUI Entry Point (`cmd/trayapp/main.go`)

Separate entry point for GUI-only executable:

- **No Console Window**: Built with `-H=windowsgui` flag
- **Automatic Tray Launch**: Automatically runs tray command
- **Clean UI**: No console window for better user experience

## Build Process

### Dual Binary Approach

The implementation creates two separate binaries:

1. **Console Binary** (`portier-cli.exe`):
   - Full CLI functionality
   - Shows console window
   - Suitable for command-line usage

2. **GUI Binary** (`portier-tray.exe`):
   - GUI-only interface
   - No console window
   - Automatically launches system tray

### Build Scripts

- **`build.bat`**: Builds both console and GUI versions
- **`build-installer.bat`**: Creates complete installer package

## Windows Installer

The NSIS installer (`portier-installer.nsi`) provides:

- **Binary Installation**: Installs both console and GUI executables
- **PATH Management**: Adds console binary to system PATH
- **Shortcuts**: Creates Start Menu and Desktop shortcuts
- **Auto-Start**: Adds tray application to startup folder
- **Uninstaller**: Complete removal including registry cleanup

## Usage

### Direct Usage

```bash
# Run system tray (console version)
portier-cli tray

# Service management
portier-cli service install
portier-cli service start
portier-cli service status
portier-cli service stop
portier-cli service uninstall
```

### GUI Usage

- Launch `portier-tray.exe` for GUI-only interface
- System tray icon appears in notification area
- Right-click for context menu with options:
  - Service status display
  - Start/Stop/Restart service
  - Open configuration files
  - Quit application

## Technical Details

### Dependencies

- `github.com/getlantern/systray`: System tray functionality
- `github.com/kardianos/service`: Windows service management
- `github.com/skratchdot/open-golang/open`: File opening

### Build Tags

- Windows-specific implementation using build tags
- Stub implementation for non-Windows platforms
- Platform detection and appropriate error messages

### Service Integration

- Integrates with existing `application.PortierApplication`
- Uses existing configuration system
- Maintains compatibility with existing functionality

## Platform Compatibility

- **Windows**: Full functionality with system tray and service support
- **Linux/macOS**: Service command available, tray shows appropriate error
- **Cross-platform**: Build system handles platform differences

## Error Handling

- Comprehensive error handling throughout
- Graceful degradation for missing dependencies
- User-friendly error messages
- Proper cleanup on exit

## Configuration

The tray application:
- Uses existing configuration files (`config.yaml`, `credentials_device.yaml`)
- Creates default configuration if missing
- Integrates with existing Portier CLI configuration system
- Respects custom configuration paths

This implementation provides a complete Windows system tray solution while maintaining compatibility with the existing Portier CLI architecture and functionality.