@echo off
echo Building Portier CLI Windows binaries...

:: Set version from git tag or default
for /f "tokens=*" %%a in ('git describe --abbrev=0 --tags 2^>nul') do set VERSION=%%a
if "%VERSION%"=="" set VERSION=0.0.1

echo Version: %VERSION%

:: Build console version
echo Building console version (portier-cli.exe)...
go build -ldflags "-X main.version=%VERSION%" -o portier-cli.exe
if %errorlevel% neq 0 (
    echo Failed to build console version
    exit /b 1
)

:: Build GUI version (no console window)
echo Building GUI version (portier-tray.exe)...
go build -ldflags "-X main.version=%VERSION% -H=windowsgui" -o portier-tray.exe ./cmd/trayapp
if %errorlevel% neq 0 (
    echo Failed to build GUI version
    exit /b 1
)

echo Build completed successfully!
echo Console binary: portier-cli.exe
echo GUI binary: portier-tray.exe