; Inno Setup script for Warden Desktop installer.
; Usage (CI): iscc /DVERSION=0.5.2 packaging/windows/warden.iss
;
; Produces: Warden-Setup-amd64.exe

#ifndef VERSION
  #define VERSION "0.0.0"
#endif

[Setup]
; SourceDir is relative to this .iss file — resolve to repo root
SourceDir=..\..
AppName=Warden
AppVersion={#VERSION}
AppPublisher=thesimonho
AppPublisherURL=https://github.com/thesimonho/warden
DefaultDirName={autopf}\Warden
DefaultGroupName=Warden
UninstallDisplayIcon={app}\warden-desktop.exe
OutputDir=.
OutputBaseFilename=Warden-Setup-amd64
Compression=lzma2/max
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
WizardStyle=modern
SetupIconFile=packaging\windows\warden.ico
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog

[Files]
Source: "warden-desktop-windows-amd64.exe"; DestDir: "{app}"; DestName: "warden-desktop.exe"; Flags: ignoreversion

[Icons]
Name: "{group}\Warden"; Filename: "{app}\warden-desktop.exe"
Name: "{autodesktop}\Warden"; Filename: "{app}\warden-desktop.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"

[Run]
Filename: "{app}\warden-desktop.exe"; Description: "Launch Warden"; Flags: nowait postinstall skipifsilent
