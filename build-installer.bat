@echo off
echo Building Portier CLI Windows Installer...

:: Check if NSIS is available
set "MAKENSIS_PATH="
where makensis >nul 2>&1
if not errorlevel 1 (
    set "MAKENSIS_PATH=makensis"
    goto :nsis_found
)

:: Try common NSIS installation paths
for %%p in (
    "C:\Program Files\NSIS\makensis.exe"
    "C:\Program Files (x86)\NSIS\makensis.exe"
    "%LOCALAPPDATA%\Microsoft\WinGet\Packages\NSIS.NSIS_Microsoft.Winget.Source_8wekyb3d8bbwe\makensis.exe"
) do (
    if exist %%p (
        set MAKENSIS_PATH=%%p
        goto :nsis_found
    )
)

echo ERROR: NSIS (makensis) not found
echo Please install NSIS from https://nsis.sourceforge.io/
echo Or add NSIS to your PATH environment variable
exit /b 1

:nsis_found
echo Found NSIS: %MAKENSIS_PATH%

:: Build the binaries first
echo Building binaries...
call build.bat
if errorlevel 1 (
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
%MAKENSIS_PATH% portier-installer.nsi
if errorlevel 1 (
    echo Failed to build installer
    exit /b 1
)

echo Installer built successfully: portier-cli-installer.exe