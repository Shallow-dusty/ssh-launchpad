Unicode true

!include "wails_tools.nsh"
!include "MUI.nsh"
!include "WordFunc.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"
VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"

!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
  !endif
!else
  InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif

ShowInstDetails show

Var PreviousVersion

Function .onInit
    !insertmacro wails.checkArchitecture
    SetRegView 64

    ReadRegStr $PreviousVersion HKCU "${UNINST_KEY}" "DisplayVersion"
    ReadRegStr $0 HKCU "${UNINST_KEY}" "InstallLocation"
    ${If} $0 != ""
        IfFileExists "$0\${PRODUCT_EXECUTABLE}" 0 +3
        IfFileExists "$0\uninstall.exe" 0 +2
        StrCpy $INSTDIR "$0"
    ${EndIf}

    ${If} $PreviousVersion != ""
        ${VersionCompare} "$PreviousVersion" "${INFO_PRODUCTVERSION}" $2
        ${If} $2 == 1
            IfSilent 0 +3
                SetErrorLevel 67
                Abort
            MessageBox MB_YESNO|MB_ICONEXCLAMATION \
                "A newer SSH Launchpad $PreviousVersion is already installed. Install ${INFO_PRODUCTVERSION} anyway?" \
                IDYES +2
            Abort
        ${EndIf}
    ${EndIf}

    checkRunning:
    FindWindow $3 "" "${INFO_PRODUCTNAME}"
    ${If} $3 != 0
        IfSilent 0 +3
            SetErrorLevel 66
            Abort
        MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION \
            "SSH Launchpad is running. Close it, then choose Retry to continue the upgrade." \
            IDRETRY checkRunning
        Abort
    ${EndIf}
FunctionEnd

Section
    !insertmacro wails.setShellContext
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    !insertmacro wails.files
    FileOpen $0 "$INSTDIR\.ssh-launchpad-install" w
    FileWrite $0 "${INFO_PRODUCTNAME}$\r$\n${INFO_PRODUCTVERSION}$\r$\n"
    FileClose $0

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    !insertmacro wails.writeUninstaller

    SetRegView 64
    WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    IfFileExists "$INSTDIR\.ssh-launchpad-install" +3 0
        MessageBox MB_OK|MB_ICONSTOP "The SSH Launchpad install marker is missing. Refusing to remove application files from $INSTDIR."
        Abort

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}"
    Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
    Delete "$INSTDIR\.ssh-launchpad-install"
    !insertmacro wails.deleteUninstaller
    RMDir "$INSTDIR"
SectionEnd
