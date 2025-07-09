/**
 *  EnvVarUpdate.nsh
 *    : Environmental Variables: append, prepend, and remove entries
 *
 *     WARNING: If you use StrFunc.nsh header then include it before this file
 *              with all required definitions. This is to avoid conflicts
 *
 *  Usage:
 *    ${EnvVarUpdate} "ResultVar" "EnvVarName" "Action" "RegLoc" "PathString"
 *
 *  Credits:
 *  Version 1.0 
 *  * Cal Turney (turneyc@lifesci.gla.ac.uk)
 *  * Amir Szekely (kichik@users.sourceforge.net)
 *  * Joost Verburg (joost@luon.net)
 *  * Afrow UK (afrow@afrowsoft.co.uk)
 *
 */


!ifndef ENVVARUPDATE_FUNCTION
!define ENVVARUPDATE_FUNCTION
!verbose push
!verbose 3
!include "LogicLib.nsh"
!include "WinMessages.nsh"
!include "StrFunc.nsh"

; ---- Fix for conflict if StrFunc.nsh is already included ----
!ifndef STRFUNC_INCLUDED
!define STRFUNC_INCLUDED
${StrTok}
${StrStr}
${StrRep}
!endif

; ---- Workaround for WinNT.nsh $\r$\n defines ----
!define NT_current_env 'HKCU "Environment"'
!define NT_all_env     'HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment"'

!macro _EnvVarUpdateConstructor ResultVar EnvVarName Action Regloc PathString
  Push "${EnvVarName}"
  Push "${Action}"
  Push "${RegLoc}"
  Push "${PathString}"
    Call EnvVarUpdate
  Pop "${ResultVar}"
!macroend
!define EnvVarUpdate '!insertmacro "_EnvVarUpdateConstructor"'

!macro _unEnvVarUpdateConstructor ResultVar EnvVarName Action Regloc PathString
  Push "${EnvVarName}"
  Push "${Action}"
  Push "${RegLoc}"
  Push "${PathString}"
    Call un.EnvVarUpdate
  Pop "${ResultVar}"
!macroend
!define un.EnvVarUpdate '!insertmacro "_unEnvVarUpdateConstructor"'

; ---- EnvVarUpdate Function ----
Function EnvVarUpdate
  !define EnvVarUpdate_RegLoc      $0
  !define EnvVarUpdate_EnvVarName  $1
  !define EnvVarUpdate_Action      $2
  !define EnvVarUpdate_PathString  $3
  !define EnvVarUpdate_Delimiter   $4
  !define EnvVarUpdate_OldString   $5
  !define EnvVarUpdate_NewString   $6
  !define EnvVarUpdate_Tmp         $7
  !define EnvVarUpdate_Item        $8
  !define EnvVarUpdate_Index       $9
  !define EnvVarUpdate_Result      $R0

  ; Initialize variables
  SetRebootFlag true
  StrCpy ${EnvVarUpdate_Result} ""
  
  ; Get parameters
  Exch ${EnvVarUpdate_PathString}
  Exch
  Exch ${EnvVarUpdate_RegLoc}
  Exch 2
  Exch ${EnvVarUpdate_Action}
  Exch 2
  Exch ${EnvVarUpdate_EnvVarName}
  Exch 2
  Push ${EnvVarUpdate_Delimiter}
  Push ${EnvVarUpdate_OldString}
  Push ${EnvVarUpdate_NewString}
  Push ${EnvVarUpdate_Tmp}
  Push ${EnvVarUpdate_Item}
  Push ${EnvVarUpdate_Index}

  ; Default delimiter is semicolon
  StrCpy ${EnvVarUpdate_Delimiter} ";"

  ; Get current PATH
  ${If} ${EnvVarUpdate_RegLoc} == HKLM
    ReadRegStr ${EnvVarUpdate_OldString} HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "${EnvVarUpdate_EnvVarName}"
  ${Else}
    ReadRegStr ${EnvVarUpdate_OldString} HKCU "Environment" "${EnvVarUpdate_EnvVarName}"
  ${EndIf}

  ; Check if path is already in PATH
  StrCpy ${EnvVarUpdate_Index} 0
  ${Do}
    ${StrTok} ${EnvVarUpdate_Item} "${EnvVarUpdate_OldString}" "${EnvVarUpdate_Delimiter}" "${EnvVarUpdate_Index}" "0"
    ${If} ${EnvVarUpdate_Item} == ""
      ${Break}
    ${EndIf}
    ${If} ${EnvVarUpdate_Item} == ${EnvVarUpdate_PathString}
      ${If} ${EnvVarUpdate_Action} == "A"
        ; Already exists, don't add
        StrCpy ${EnvVarUpdate_Result} "0"
        GoTo EnvVarUpdate_Done
      ${ElseIf} ${EnvVarUpdate_Action} == "R"
        ; Remove this item
        ${StrRep} ${EnvVarUpdate_NewString} "${EnvVarUpdate_OldString}" "${EnvVarUpdate_PathString}${EnvVarUpdate_Delimiter}" ""
        ${StrRep} ${EnvVarUpdate_NewString} "${EnvVarUpdate_NewString}" "${EnvVarUpdate_Delimiter}${EnvVarUpdate_PathString}" ""
        ${StrRep} ${EnvVarUpdate_NewString} "${EnvVarUpdate_NewString}" "${EnvVarUpdate_PathString}" ""
        GoTo EnvVarUpdate_WriteReg
      ${EndIf}
    ${EndIf}
    IntOp ${EnvVarUpdate_Index} ${EnvVarUpdate_Index} + 1
  ${Loop}

  ; Add to PATH
  ${If} ${EnvVarUpdate_Action} == "A"
    ${If} ${EnvVarUpdate_OldString} == ""
      StrCpy ${EnvVarUpdate_NewString} "${EnvVarUpdate_PathString}"
    ${Else}
      StrCpy ${EnvVarUpdate_NewString} "${EnvVarUpdate_OldString}${EnvVarUpdate_Delimiter}${EnvVarUpdate_PathString}"
    ${EndIf}
  ${ElseIf} ${EnvVarUpdate_Action} == "P"
    ${If} ${EnvVarUpdate_OldString} == ""
      StrCpy ${EnvVarUpdate_NewString} "${EnvVarUpdate_PathString}"
    ${Else}
      StrCpy ${EnvVarUpdate_NewString} "${EnvVarUpdate_PathString}${EnvVarUpdate_Delimiter}${EnvVarUpdate_OldString}"
    ${EndIf}
  ${ElseIf} ${EnvVarUpdate_Action} == "R"
    ; Item not found, nothing to remove
    StrCpy ${EnvVarUpdate_Result} "0"
    GoTo EnvVarUpdate_Done
  ${EndIf}

  EnvVarUpdate_WriteReg:
  ; Write to registry
  ${If} ${EnvVarUpdate_RegLoc} == HKLM
    WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "${EnvVarUpdate_EnvVarName}" "${EnvVarUpdate_NewString}"
  ${Else}
    WriteRegExpandStr HKCU "Environment" "${EnvVarUpdate_EnvVarName}" "${EnvVarUpdate_NewString}"
  ${EndIf}

  ; Broadcast change
  SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment" /TIMEOUT=5000

  StrCpy ${EnvVarUpdate_Result} "1"

  EnvVarUpdate_Done:
  Pop ${EnvVarUpdate_Index}
  Pop ${EnvVarUpdate_Item}
  Pop ${EnvVarUpdate_Tmp}
  Pop ${EnvVarUpdate_NewString}
  Pop ${EnvVarUpdate_OldString}
  Pop ${EnvVarUpdate_Delimiter}
  Pop ${EnvVarUpdate_PathString}
  Pop ${EnvVarUpdate_RegLoc}
  Pop ${EnvVarUpdate_Action}
  Pop ${EnvVarUpdate_EnvVarName}
  Push ${EnvVarUpdate_Result}
FunctionEnd

; ---- Uninstaller Function ----
Function un.EnvVarUpdate
  !define un.EnvVarUpdate_RegLoc      $0
  !define un.EnvVarUpdate_EnvVarName  $1
  !define un.EnvVarUpdate_Action      $2
  !define un.EnvVarUpdate_PathString  $3
  !define un.EnvVarUpdate_Delimiter   $4
  !define un.EnvVarUpdate_OldString   $5
  !define un.EnvVarUpdate_NewString   $6
  !define un.EnvVarUpdate_Tmp         $7
  !define un.EnvVarUpdate_Item        $8
  !define un.EnvVarUpdate_Index       $9
  !define un.EnvVarUpdate_Result      $R0

  ; Initialize variables
  SetRebootFlag true
  StrCpy ${un.EnvVarUpdate_Result} ""
  
  ; Get parameters
  Exch ${un.EnvVarUpdate_PathString}
  Exch
  Exch ${un.EnvVarUpdate_RegLoc}
  Exch 2
  Exch ${un.EnvVarUpdate_Action}
  Exch 2
  Exch ${un.EnvVarUpdate_EnvVarName}
  Exch 2
  Push ${un.EnvVarUpdate_Delimiter}
  Push ${un.EnvVarUpdate_OldString}
  Push ${un.EnvVarUpdate_NewString}
  Push ${un.EnvVarUpdate_Tmp}
  Push ${un.EnvVarUpdate_Item}
  Push ${un.EnvVarUpdate_Index}

  ; Default delimiter is semicolon
  StrCpy ${un.EnvVarUpdate_Delimiter} ";"

  ; Get current PATH
  ${If} ${un.EnvVarUpdate_RegLoc} == HKLM
    ReadRegStr ${un.EnvVarUpdate_OldString} HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "${un.EnvVarUpdate_EnvVarName}"
  ${Else}
    ReadRegStr ${un.EnvVarUpdate_OldString} HKCU "Environment" "${un.EnvVarUpdate_EnvVarName}"
  ${EndIf}

  ; Remove from PATH
  ${StrRep} ${un.EnvVarUpdate_NewString} "${un.EnvVarUpdate_OldString}" "${un.EnvVarUpdate_PathString}${un.EnvVarUpdate_Delimiter}" ""
  ${StrRep} ${un.EnvVarUpdate_NewString} "${un.EnvVarUpdate_NewString}" "${un.EnvVarUpdate_Delimiter}${un.EnvVarUpdate_PathString}" ""
  ${StrRep} ${un.EnvVarUpdate_NewString} "${un.EnvVarUpdate_NewString}" "${un.EnvVarUpdate_PathString}" ""

  ; Write to registry
  ${If} ${un.EnvVarUpdate_RegLoc} == HKLM
    WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "${un.EnvVarUpdate_EnvVarName}" "${un.EnvVarUpdate_NewString}"
  ${Else}
    WriteRegExpandStr HKCU "Environment" "${un.EnvVarUpdate_EnvVarName}" "${un.EnvVarUpdate_NewString}"
  ${EndIf}

  ; Broadcast change
  SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment" /TIMEOUT=5000

  StrCpy ${un.EnvVarUpdate_Result} "1"

  Pop ${un.EnvVarUpdate_Index}
  Pop ${un.EnvVarUpdate_Item}
  Pop ${un.EnvVarUpdate_Tmp}
  Pop ${un.EnvVarUpdate_NewString}
  Pop ${un.EnvVarUpdate_OldString}
  Pop ${un.EnvVarUpdate_Delimiter}
  Pop ${un.EnvVarUpdate_PathString}
  Pop ${un.EnvVarUpdate_RegLoc}
  Pop ${un.EnvVarUpdate_Action}
  Pop ${un.EnvVarUpdate_EnvVarName}
  Push ${un.EnvVarUpdate_Result}
FunctionEnd

!verbose pop
!endif