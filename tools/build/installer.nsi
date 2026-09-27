; Built from the same file manifest as the portable ZIP by tools/build.
Unicode true
RequestExecutionLevel user
SetCompressor /SOLID lzma
Name "ForeverDubbed"
VIProductVersion "@VERSION@.0"
VIAddVersionKey /LANG=1033 "ProductName" "ForeverDubbed"
VIAddVersionKey /LANG=1033 "ProductVersion" "@VERSION@"
VIAddVersionKey /LANG=1033 "FileVersion" "@VERSION@"
VIAddVersionKey /LANG=1033 "FileDescription" "ForeverDubbed installer"
VIAddVersionKey /LANG=1033 "LegalCopyright" "ForeverDubbed contributors"
OutFile @OUTPUT@
InstallDir "$LOCALAPPDATA\Programs\ForeverDubbed"
InstallDirRegKey HKCU "Software\ForeverDubbed" "InstallDir"
!include "MUI2.nsh"
!include "x64.nsh"
!include "FileFunc.nsh"
Var InventoryCount
Var InventoryIndex
Var InventoryPass
Var InventoryKind
Var InventoryRelative
Var InventoryPath

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
  ClearErrors
  FileOpen $0 "$INSTDIR\@INVENTORY_NAME@" w
  IfErrors inventory_write_failed
@WRITE_INVENTORY@
  IfErrors inventory_write_close_failed
  FileClose $0
  Goto inventory_written
inventory_write_close_failed:
  FileClose $0
inventory_write_failed:
  Abort "Could not write the uninstall inventory."
inventory_written:
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateShortcut "$SMPROGRAMS\ForeverDubbed.lnk" "$INSTDIR\foreverdubbed.exe"
  WriteRegStr HKCU "Software\ForeverDubbed" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "DisplayName" "ForeverDubbed"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "DisplayVersion" "@VERSION@"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "DisplayIcon" "$INSTDIR\foreverdubbed.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed" "NoRepair" 1
SectionEnd

Function un.ValidateInventoryPath
  ClearErrors
  StrLen $0 $InventoryRelative
  IntCmp $0 0 invalid_path invalid_path
  IntCmp $0 800 0 0 invalid_path
  StrLen $1 $INSTDIR
  IntOp $0 $0 + $1
  IntCmp $0 1000 invalid_path 0 invalid_path
  StrCpy $0 0
validate_character:
  StrCpy $1 $InventoryRelative 1 $0
  StrCmp $1 "" validate_canonical
  StrCmp $1 "*" invalid_path
  StrCmp $1 "?" invalid_path
  StrCmp $1 ":" invalid_path
  StrCmp $1 "/" invalid_path
  StrCmp $1 '$\"' invalid_path
  StrCmp $1 "<" invalid_path
  StrCmp $1 ">" invalid_path
  StrCmp $1 "|" invalid_path
  IntOp $0 $0 + 1
  Goto validate_character
validate_canonical:
  GetFullPathName $InventoryPath "$INSTDIR\$InventoryRelative"
  IfErrors invalid_path
  ; Canonical equality rejects absolute paths, traversal and dot components.
  StrCmp $InventoryPath "$INSTDIR\$InventoryRelative" 0 invalid_path
  StrCmp $InventoryPath $INSTDIR invalid_path
  StrCpy $0 $InventoryPath
validate_parents:
  ; Never follow a directory junction or symlink outside the installation.
  System::Call 'kernel32::GetFileAttributesW(w r0) i .r1'
  IntCmp $1 -1 parent_checked
  IntOp $1 $1 & 0x400
  IntCmp $1 0 parent_checked invalid_path invalid_path
parent_checked:
  StrCmp $0 $INSTDIR valid_path
  ${GetParent} "$0" $0
  StrCmp $0 "" invalid_path
  Goto validate_parents
valid_path:
  ClearErrors
  Return
invalid_path:
  SetErrors
FunctionEnd

Section "Uninstall"
  SetShellVarContext current
  GetFullPathName $INSTDIR $INSTDIR
  ; Read a fresh inventory on every uninstall. No updater process is involved.
  ReadINIStr $0 "$INSTDIR\@INVENTORY_NAME@" "inventory" "version"
  StrCmp $0 "1" 0 inventory_failed
  ReadINIStr $InventoryCount "$INSTDIR\@INVENTORY_NAME@" "inventory" "count"
  IntOp $0 $InventoryCount + 0
  StrCmp $0 $InventoryCount 0 inventory_failed
  IntCmp $InventoryCount 1 0 inventory_failed
  IntCmp $InventoryCount 50000 0 0 inventory_failed
  ; Validate the entire inventory before deleting anything, then run it.
  StrCpy $InventoryPass 0
inventory_start:
  StrCpy $InventoryIndex 0
inventory_next:
  IntCmp $InventoryIndex $InventoryCount inventory_end
  ClearErrors
  ReadINIStr $0 "$INSTDIR\@INVENTORY_NAME@" "inventory" "$InventoryIndex"
  IfErrors inventory_failed
  StrCpy $InventoryKind $0 1
  StrCpy $InventoryRelative $0 "" 1
  StrCmp $InventoryKind "F" inventory_validate
  StrCmp $InventoryKind "D" 0 inventory_failed
inventory_validate:
  Call un.ValidateInventoryPath
  IfErrors inventory_failed
  StrCmp $InventoryPass 0 inventory_advance
  StrCmp $InventoryKind "D" inventory_directory
  IfFileExists "$InventoryPath" 0 inventory_advance
  ClearErrors
  Delete "$InventoryPath"
  IfErrors inventory_failed
  Goto inventory_advance
inventory_directory:
  ; RMDir without /r preserves directories containing unrelated user files.
  RMDir "$InventoryPath"
inventory_advance:
  IntOp $InventoryIndex $InventoryIndex + 1
  Goto inventory_next
inventory_end:
  StrCmp $InventoryPass 1 inventory_done
  StrCpy $InventoryPass 1
  Goto inventory_start
inventory_failed:
  Abort "Could not read or remove installed files. Quit ForeverDubbed and retry, or reinstall to repair the uninstall inventory."
inventory_done:
  Delete "$INSTDIR\@INVENTORY_NAME@"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\ForeverDubbed.lnk"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed"
  DeleteRegKey HKCU "Software\ForeverDubbed"
SectionEnd
