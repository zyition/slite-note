Unicode true

####
## NOTE: This file is a maintained fork of the Wails v3 NSIS template.
## `wails3 build` does NOT touch it, but `wails3 update build-assets` WOULD
## overwrite it - do not run that task (the CI does not). If you do, re-apply
## this file afterwards.
##
## Customizations over the stock template:
##  1. Display name "Slite Note" (shortcuts / install dir / DisplayName /
##     version resources) with a stable, space-free uninstall key name, so the
##     UI looks professional and upgrades can always find previous installs.
##  2. Upgrades: a previous install (new + legacy uninstall keys) is located
##     in .onInit, its uninstaller runs silently first, the install dir is
##     reused, and downgrades are blocked interactively.
##  3. Running-process detection (tasklist, no external plugins) before
##     install/uninstall: asks the user, then runs "slite-note.exe --quit" so
##     the app can flush pending saves and exit (releasing the exe lock);
##     force-kill is only the fallback.
##  4. Writes InstallLocation / DisplayVersion to the uninstall key.
####

!include "LogicLib.nsh"
!include "StrFunc.nsh"
${StrStr}
${UnStrStr}

# Override the display name before wails_tools.nsh provides its defaults.
!define INFO_PRODUCTNAME "Slite Note"
# Stable uninstall key name (no spaces): independent of the display name, so
# upgrades keep finding the previous install even if the name changes later.
!define UNINST_KEY_NAME "zyitionSliteNote"
# Uninstall key written by releases before the "Slite Note" rename.
!define LEGACY_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\zyitionslite-note"

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows you to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the wails_tools.nsh file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "my-project" # Default "slite"
## !define INFO_COMPANYNAME    "My Company" # Default "My Company"
## !define INFO_PRODUCTNAME    "My Product Name" # Default "My Product"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "0.1.0"
## !define INFO_COPYRIGHT      "(c) Now, My Company" # Default "(c) 2026, My Company"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
## !define WAILS_INSTALL_SCOPE     "user"             # Default "machine" - set to "user" for per-user install ($LOCALAPPDATA) without UAC prompt
####
## Include the wails tools
####
!include "wails_tools.nsh"
!include "WordFunc.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.
!define MUI_DIRECTORYPAGE_VERIFYONLEAVE # Reject invalid install directories.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_CONFIRM # Confirm uninstall page.
!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
!else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif
ShowInstDetails show # This will always show the installation details.

# --- state shared between the installer/uninstaller sections --------------
Var PREV_INSTALL_DIR   ; install dir of a previous version ("" if none)
Var PREV_UNINSTALLER   ; path to a previous version's uninstaller (quotes stripped)
Var PREV_VERSION       ; DisplayVersion of a previous version ("" if none)
Var APP_RUNNING        ; "1" while slite-note.exe is running

# --- helpers ---------------------------------------------------------------

# StripQuotes: removes one layer of wrapping double quotes from $0.
# NOTE: keeps the length in $2 - reusing $1 here would clobber the length
# with the quote char and corrupt the output.
Function StripQuotes
    StrCpy $1 $0 1
    ${If} $1 == '"'
        StrCpy $0 $0 "" 1
    ${EndIf}
    StrLen $2 $0
    IntOp $2 $2 - 1
    StrCpy $1 $0 1 $2
    ${If} $1 == '"'
        StrCpy $0 $0 $2
    ${EndIf}
FunctionEnd

# IsSliteNoteRunning: sets $APP_RUNNING to "1" if slite-note.exe is running.
Function IsSliteNoteRunning
    nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq slite-note.exe" /NH'
    Pop $0  ; exit code
    Pop $1  ; output
    ${StrStr} $0 $1 "slite-note.exe"
    StrCpy $APP_RUNNING 0
    ${If} $0 != ""
        StrCpy $APP_RUNNING 1
    ${EndIf}
FunctionEnd

# HandleRunningApp: handles a running instance for install/uninstall.
# Interactive: asks the user; on consent (or in silent mode) runs
# "slite-note.exe --quit" so the app flushes pending saves and exits,
# waits up to ~10s, then force-kills as a last resort.
# $0 out: 1 = proceed, 0 = user aborted.
Function HandleRunningApp
    IfSilent hraSilent hraPrompt
    hraPrompt:
        MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON1 "Slite Note is still running.$\r$\nClose it and continue?" IDYES hraProceed IDNO hraCancel
    hraSilent:
        ; Graceful quit first: the app listens for "--quit" via the
        # single-instance notification and flushes pending saves before
        # exiting (force-killing would risk losing the last keystrokes).
        Exec '"$INSTDIR\slite-note.exe" --quit'
        StrCpy $1 0
        ${Do}
            Sleep 500
            Call IsSliteNoteRunning
            ${If} $APP_RUNNING == 0
                ${Break}
            ${EndIf}
            IntOp $1 $1 + 1
        ${LoopWhile} $1 < 20  ; up to ~10s
        ${If} $APP_RUNNING == 1
            nsExec::ExecToStack 'taskkill /F /IM slite-note.exe'
            Pop $2
        ${EndIf}
        StrCpy $0 1
        Return
    hraCancel:
        StrCpy $0 0
        Return
    hraProceed:
        Goto hraSilent
FunctionEnd

Function .onInit
   !insertmacro wails.checkArchitecture

   ; --- locate a previous installation (new key, then HKLM, then legacy) ---
   ReadRegStr $PREV_INSTALL_DIR HKCU "${UNINST_KEY}" "InstallLocation"
   ReadRegStr $PREV_UNINSTALLER HKCU "${UNINST_KEY}" "UninstallString"
   ReadRegStr $PREV_VERSION HKCU "${UNINST_KEY}" "DisplayVersion"
   ${If} $PREV_INSTALL_DIR == ""
       ReadRegStr $PREV_INSTALL_DIR HKLM "${UNINST_KEY}" "InstallLocation"
   ${EndIf}
   ${If} $PREV_UNINSTALLER == ""
       ReadRegStr $PREV_UNINSTALLER HKLM "${UNINST_KEY}" "UninstallString"
   ${EndIf}
   ${If} $PREV_VERSION == ""
       ReadRegStr $PREV_VERSION HKLM "${UNINST_KEY}" "DisplayVersion"
   ${EndIf}
   ; Legacy key (installs before the "Slite Note" rename) never wrote
   ; InstallLocation; fall back to deriving the dir from its UninstallString.
   ${If} $PREV_INSTALL_DIR == ""
       ReadRegStr $PREV_UNINSTALLER HKCU "${LEGACY_UNINST_KEY}" "UninstallString"
       ${If} $PREV_UNINSTALLER == ""
           ReadRegStr $PREV_UNINSTALLER HKLM "${LEGACY_UNINST_KEY}" "UninstallString"
       ${EndIf}
   ${EndIf}
   ${If} $PREV_UNINSTALLER != ""
       StrCpy $0 $PREV_UNINSTALLER
       Call StripQuotes
       StrCpy $PREV_UNINSTALLER $0
   ${EndIf}
   ${If} $PREV_INSTALL_DIR == ""
       ${If} $PREV_UNINSTALLER != ""
           ; "C:\path\uninstall.exe" -> "C:\path\"
           ${StrStr} $0 $PREV_UNINSTALLER "uninstall.exe"
           ${If} $0 != ""
               StrLen $1 $0
               IntOp $1 $1 * -1
               StrCpy $PREV_INSTALL_DIR $PREV_UNINSTALLER $1
           ${EndIf}
       ${EndIf}
   ${EndIf}

   ; Reuse the previous install location so upgrades land in place (and so
   ; _?=$INSTDIR passed to the old uninstaller points at the right dir).
   ${If} $PREV_INSTALL_DIR != ""
       StrCpy $INSTDIR $PREV_INSTALL_DIR
   ${EndIf}

   ; --- block downgrades (interactive only; silent installs overwrite) ---
   ${If} $PREV_VERSION != ""
       ${VersionCompare} $PREV_VERSION "${INFO_PRODUCTVERSION}" $0
       ${If} $0 == 1
           ; /SD IDYES: in silent mode the prompt is never shown and the
           ; install proceeds (matching winget-style overwrite semantics).
           MessageBox MB_YESNO|MB_ICONEXCLAMATION "A newer version of Slite Note ($PREV_VERSION) is already installed.$\r$\nInstall ${INFO_PRODUCTVERSION} anyway?" /SD IDYES IDYES prevOk IDNO prevCancel
           prevCancel:
               Abort
           prevOk:
       ${EndIf}
   ${EndIf}
FunctionEnd

Section
    !insertmacro wails.setShellContext

    ; 1. Refuse (or gracefully close) a running instance before touching files.
    Call IsSliteNoteRunning
    ${If} $APP_RUNNING == 1
        IfSilent instSilent instPrompt
        instSilent:
            ; silent + running: cannot ask; try the graceful --quit path,
            ; force-kill as fallback. If it still refuses, fail the install.
            Call HandleRunningApp
            ${If} $APP_RUNNING == 1
                SetErrorLevel 1
                Abort
            ${EndIf}
            Goto instDone
        instPrompt:
            Call HandleRunningApp
            ${If} $0 == 0
                Abort
            ${EndIf}
        instDone:
    ${EndIf}

    ; 2. Run the previous version's uninstaller (keeps the install dir tidy).
    ;    Copy it to $PLUGINSDIR first so it can be deleted while the original
    ;    is running; _?= (last arg, unquoted even with spaces) forces
    ;    synchronous execution and stops the uninstaller deleting itself.
    ${If} $PREV_UNINSTALLER != ""
        DetailPrint "Removing previous version..."
        InitPluginsDir
        CopyFiles "$PREV_UNINSTALLER" "$PLUGINSDIR\uninstall.exe"
        ExecWait '"$PLUGINSDIR\uninstall.exe" /S _?=$INSTDIR' $0
        Delete "$PLUGINSDIR\uninstall.exe"
    ${EndIf}

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    
    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    
    !insertmacro wails.writeUninstaller
    ; InstallLocation lets later upgrades find this install (and winget/ARP
    # tooling report it); DisplayVersion is written by wails.writeUninstaller.
    WriteRegStr SHELL_CONTEXT "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
SectionEnd

Section "uninstall" 
    !insertmacro wails.setShellContext

    ; Close a running instance before deleting files (same policy as install).
    ; The uninstaller can only call un.-prefixed functions, so the detection
    ; and graceful-close logic is inlined here with the un. string macros.
    nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq slite-note.exe" /NH'
    Pop $0  ; exit code
    Pop $1  ; output
    ${UnStrStr} $0 $1 "slite-note.exe"
    ${If} $0 != ""
        IfSilent uninstSilent uninstPrompt
        uninstPrompt:
            MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON1 "Slite Note is still running.$\r$\nClose it and continue?" IDYES uninstProceed IDNO uninstCancel
            uninstCancel:
                Abort
            uninstProceed:
        uninstSilent:
        ; Graceful quit first (the app flushes pending saves on "--quit");
        ; force-kill only as a last resort.
        Exec '"$INSTDIR\slite-note.exe" --quit'
        StrCpy $1 0
        ${Do}
            Sleep 500
            nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq slite-note.exe" /NH'
            Pop $2  ; exit code
            Pop $3  ; output
            ${UnStrStr} $2 $3 "slite-note.exe"
            ${If} $2 == ""
                ${Break}
            ${EndIf}
            IntOp $1 $1 + 1
        ${LoopWhile} $1 < 20  ; up to ~10s
        ${If} $2 != ""
            nsExec::ExecToStack 'taskkill /F /IM slite-note.exe'
            Pop $2
        ${EndIf}
    ${EndIf}

    ; WebView2 cache only - user notes (%APPDATA%\slite) are intentionally kept.
    RMDir /r "$LOCALAPPDATA\slite\webview"
    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the legacy WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
