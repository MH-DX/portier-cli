@echo off
echo Building Portier CLI Windows Installer...

:: Check if NSIS is available
where makensis >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: NSIS (makensis) not found in PATH
    echo Please install NSIS from https://nsis.sourceforge.io/
    exit /b 1
)

:: Build the binaries first
echo Building binaries...
call build.bat
if %errorlevel% neq 0 (
    echo Failed to build binaries
    exit /b 1
)

:: Check if required files exist
if not exist "portier-cli.exe" (
    echo ERROR: portier-cli.exe not found
    exit /b 1
)

if not exist "portier-tray.exe" (
    echo ERROR: portier-tray.exe not found
    exit /b 1
)

if not exist "LICENSE" (
    echo ERROR: LICENSE file not found
    exit /b 1
)

:: Create default icon if it doesn't exist
if not exist "icon.ico" (
    echo Creating default icon...
    echo. > icon.ico
)

:: Build the installer
echo Building installer...
makensis portier-installer.nsi
if %errorlevel% neq 0 (
    echo Failed to build installer
    exit /b 1
)

echo Installer built successfully: portier-cli-installer.exe