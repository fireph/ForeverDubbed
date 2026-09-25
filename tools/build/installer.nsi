; Built from the same file manifest as the portable ZIP by tools/build.
Unicode true
RequestExecutionLevel user
SetCompressor /SOLID lzma
Name "ForeverDubbed"
OutFile @OUTPUT@
InstallDir "$LOCALAPPDATA\Programs\ForeverDubbed"
InstallDirRegKey HKCU "Software\ForeverDubbed" "InstallDir"
!include "MUI2.nsh"
!include "x64.nsh"

!define MUI_WELCOMEPAGE_TEXT "Setup will install ForeverDubbed and its offline voices.$\r$\n$\r$\nBefore updating, quit ForeverDubbed from its tray menu.$\r$\n$\r$\nThe WoW addon is included in the addon folder; copy it into your game's Interface\AddOns folder separately."
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "ForeverDubbed requires 64-bit Windows."
    Abort
  ${EndIf}
  SetShellVarContext current
FunctionEnd

Section "ForeverDubbed"
  SetOverwrite on
@INSTALL_FILES@
  SetOutPath "$INSTDIR"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateShortcut "$SMPROGRAMS\ForeverDubbed.lnk" "$INSTDIR\foreverdubbed.exe"
  WriteRegStr HKCU "Software\ForeverDubbed" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "DisplayName" "ForeverDubbed"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "DisplayIcon" "$INSTDIR\foreverdubbed.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "NoRepair" 1
SectionEnd

Section "Uninstall"
  SetShellVarContext current
@REMOVE_FILES@
  Delete "$INSTDIR\Uninstall.exe"
  ; Remove only empty directories, leaving any additional user files intact.
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\ForeverDubbed.lnk"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed"
  DeleteRegKey HKCU "Software\ForeverDubbed"
SectionEnd
