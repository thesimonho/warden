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
; Stable GUID — do not change. Allows Windows to detect upgrades.
AppId={{5166f916-808d-48ad-b381-98f3f3530011}
AppName=Warden
AppVersion={#VERSION}
AppPublisher=thesimonho
AppPublisherURL=https://github.com/thesimonho/warden
DefaultDirName={autopf}\Warden
DefaultGroupName=Warden
UninstallDisplayIcon={app}\warden-desktop.exe
OutputDir=.
OutputBaseFilename=warden-desktop-windows-amd64
Compression=lzma2/max
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
WizardStyle=modern
SetupIconFile=packaging\windows\warden.ico
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ChangesEnvironment=yes

[Files]
Source: "warden-desktop-windows-amd64.exe"; DestDir: "{app}"; DestName: "warden-desktop.exe"; Flags: ignoreversion

[Icons]
Name: "{group}\Warden"; Filename: "{app}\warden-desktop.exe"
Name: "{autodesktop}\Warden"; Filename: "{app}\warden-desktop.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"
Name: "addtopath"; Description: "Add to PATH (allows running from terminal)"; GroupDescription: "Additional shortcuts:"; Flags: checkedonce

[Registry]
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Tasks: addtopath; Check: NeedsAddPath(ExpandConstant('{app}'))

[Code]
function NeedsAddPath(Param: string): Boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

[Run]
Filename: "{app}\warden-desktop.exe"; Description: "Launch Warden"; Flags: nowait postinstall skipifsilent
