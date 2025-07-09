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
!include "WinMessages.nsh"

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
  ReadRegStr $0 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "PATH"
  StrCmp $0 "" AddToPath_NoPrevious
    StrCpy $0 "$0;$INSTDIR"
    Goto AddToPath_WriteReg
  AddToPath_NoPrevious:
    StrCpy $0 "$INSTDIR"
  AddToPath_WriteReg:
    WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "PATH" $0
    SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment" /TIMEOUT=5000
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

; Function to remove a path from PATH environment variable
Function un.RemoveFromPath
  Exch $0
  Exch
  Exch $1
  Push $2
  Push $3
  Push $4
  Push $5
  Push $6

  ReadRegStr $0 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "PATH"
  StrCpy $5 $0 1 -1 ; copy last char
  StrCmp $5 ";" +2 ; if last char != ;
    StrCpy $0 "$0;" ; append ;
  Push $0
  Push "$1;"
  Call un.StrStr ; Find `$1;` in $0
  Pop $2 ; pos of our dir
  StrCmp $2 "" unRemoveFromPath_done
    ; else, it is in path
    ; $0 now has the part before the path to remove
    StrLen $3 "$1;"
    StrLen $4 $2
    StrCpy $5 $0 -$4 ; $5 is now the part before the path to remove
    StrCpy $6 $2 "" $3 ; $6 is now the part after the path to remove
    StrCpy $0 "$5$6"
  unRemoveFromPath_done:
    Pop $6
    Pop $5
    Pop $4
    Pop $3
    Pop $2
    Pop $1
    Exch $0
FunctionEnd

; String search function for uninstaller
Function un.StrStr
  Exch $R1 ; st=haystack,old$R1, $R1=needle
  Exch    ; st=old$R1,haystack
  Exch $R2 ; st=old$R1,old$R2, $R2=haystack
  Push $R3
  Push $R4
  Push $R5
  StrLen $R3 $R1
  StrCpy $R4 0
  ; $R1=needle
  ; $R2=haystack
  ; $R3=len(needle)
  ; $R4=cnt
  ; $R5=tmp
  loop:
    StrCpy $R5 $R2 $R3 $R4
    StrCmp $R5 $R1 done
    StrCmp $R5 "" done
    IntOp $R4 $R4 + 1
    Goto loop
  done:
    StrCmp $R5 $R1 found
    StrCpy $R1 ""
    Goto final
  found:
    StrCpy $R1 $R2 "" $R4
  final:
    Pop $R5
    Pop $R4
    Pop $R3
    Pop $R2
    Exch $R1
FunctionEnd

Section Uninstall
  ; Stop service if running
  ExecWait "$INSTDIR\portier-cli.exe service stop"
  ExecWait "$INSTDIR\portier-cli.exe service uninstall"
  
  ; Remove from PATH
  ReadRegStr $0 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "PATH"
  Push $0
  Push "$INSTDIR"
  Call un.RemoveFromPath
  Pop $0
  WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "PATH" $0
  SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment" /TIMEOUT=5000
  
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