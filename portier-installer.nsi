; Portier CLI Windows Installer
; Uses NSIS (Nullsoft Scriptable Install System)

!define PRODUCT_NAME "Portier CLI"
!define PRODUCT_VERSION "1.0.0"
!define PRODUCT_PUBLISHER "MH.DX UG"
!define PRODUCT_WEB_SITE "https://portier.dev"
!define PRODUCT_DIR_REGKEY "Software\Microsoft\Windows\CurrentVersion\App Paths\portier-cli.exe"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"
!define PRODUCT_UNINST_ROOT_KEY "HKLM"

!include "MUI2.nsh"
!include "EnvVarUpdate.nsh"

; MUI Settings
!define MUI_ABORTWARNING
!define MUI_ICON "icon.ico"
!define MUI_UNICON "icon.ico"

; Welcome page
!insertmacro MUI_PAGE_WELCOME
; License page
!insertmacro MUI_PAGE_LICENSE "LICENSE"
; Directory page
!insertmacro MUI_PAGE_DIRECTORY
; Instfiles page
!insertmacro MUI_PAGE_INSTFILES
; Finish page
!define MUI_FINISHPAGE_RUN "$INSTDIR\portier-tray.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Start Portier CLI System Tray"
!insertmacro MUI_PAGE_FINISH

; Uninstaller pages
!insertmacro MUI_UNPAGE_INSTFILES

; Language files
!insertmacro MUI_LANGUAGE "English"

; Reserve files
!insertmacro MUI_RESERVEFILE_INSTALLOPTIONS

; MUI end ------

Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "portier-cli-installer.exe"
InstallDir "$PROGRAMFILES\Portier CLI"
InstallDirRegKey HKLM "${PRODUCT_DIR_REGKEY}" ""
ShowInstDetails show
ShowUnInstDetails show

Section "MainSection" SEC01
  SetOutPath "$INSTDIR"
  SetOverwrite ifnewer
  File "portier-cli.exe"
  File "portier-tray.exe"
  File "LICENSE"
  File "README.md"
  
  ; Create shortcuts
  CreateDirectory "$SMPROGRAMS\Portier CLI"
  CreateShortCut "$SMPROGRAMS\Portier CLI\Portier CLI.lnk" "$INSTDIR\portier-cli.exe"
  CreateShortCut "$SMPROGRAMS\Portier CLI\Portier CLI Tray.lnk" "$INSTDIR\portier-tray.exe"
  CreateShortCut "$SMPROGRAMS\Portier CLI\Uninstall.lnk" "$INSTDIR\uninst.exe"
  
  ; Create desktop shortcut
  CreateShortCut "$DESKTOP\Portier CLI Tray.lnk" "$INSTDIR\portier-tray.exe"
  
  ; Add to startup folder
  CreateShortCut "$SMSTARTUP\Portier CLI.lnk" "$INSTDIR\portier-tray.exe"
  
  ; Add to PATH
  ${EnvVarUpdate} $0 "PATH" "A" "HKLM" "$INSTDIR"
SectionEnd

Section -AdditionalIcons
  WriteIniStr "$INSTDIR\${PRODUCT_NAME}.url" "InternetShortcut" "URL" "${PRODUCT_WEB_SITE}"
  CreateShortCut "$SMPROGRAMS\Portier CLI\Website.lnk" "$INSTDIR\${PRODUCT_NAME}.url"
SectionEnd

Section -Post
  WriteUninstaller "$INSTDIR\uninst.exe"
  WriteRegStr HKLM "${PRODUCT_DIR_REGKEY}" "" "$INSTDIR\portier-cli.exe"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "DisplayName" "$(^Name)"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "UninstallString" "$INSTDIR\uninst.exe"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "DisplayIcon" "$INSTDIR\portier-cli.exe"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "URLInfoAbout" "${PRODUCT_WEB_SITE}"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
SectionEnd

Function un.onUninstSuccess
  HideWindow
  MessageBox MB_ICONINFORMATION|MB_OK "$(^Name) was successfully removed from your computer."
FunctionEnd

Function un.onInit
  MessageBox MB_ICONQUESTION|MB_YESNO|MB_DEFBUTTON2 "Are you sure you want to completely remove $(^Name) and all of its components?" IDYES +2
  Abort
FunctionEnd

Section Uninstall
  ; Stop service if running
  ExecWait "$INSTDIR\portier-cli.exe service stop"
  ExecWait "$INSTDIR\portier-cli.exe service uninstall"
  
  ; Remove from PATH
  ${un.EnvVarUpdate} $0 "PATH" "R" "HKLM" "$INSTDIR"
  
  ; Remove files
  Delete "$INSTDIR\${PRODUCT_NAME}.url"
  Delete "$INSTDIR\uninst.exe"
  Delete "$INSTDIR\portier-cli.exe"
  Delete "$INSTDIR\portier-tray.exe"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\README.md"
  
  ; Remove shortcuts
  Delete "$SMPROGRAMS\Portier CLI\Uninstall.lnk"
  Delete "$SMPROGRAMS\Portier CLI\Website.lnk"
  Delete "$SMPROGRAMS\Portier CLI\Portier CLI.lnk"
  Delete "$SMPROGRAMS\Portier CLI\Portier CLI Tray.lnk"
  Delete "$DESKTOP\Portier CLI Tray.lnk"
  Delete "$SMSTARTUP\Portier CLI.lnk"
  
  ; Remove directories
  RMDir "$SMPROGRAMS\Portier CLI"
  RMDir "$INSTDIR"
  
  ; Remove registry keys
  DeleteRegKey ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}"
  DeleteRegKey HKLM "${PRODUCT_DIR_REGKEY}"
  SetAutoClose true
SectionEnd