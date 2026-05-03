; ============================================================================
; DraftRight Windows installer — Inno Setup script
; ============================================================================
;
; Usage (locally):
;   "%LOCALAPPDATA%\Programs\Inno Setup 6\ISCC.exe" ^
;     /DAppVersion=1.0.0 ^
;     /DSourceDir=..\publish ^
;     /DArch=x64 ^
;     installer\draftright.iss
;
; CI passes the same /D defines from .github/workflows/build-windows.yml.
;
; Output:  installer\Output\DraftRight-Setup-{version}-{arch}.exe
;
; Notes:
; - Per-user install ({localappdata}\Programs\DraftRight). No UAC prompt.
;   Matches the existing UpdateService flow that drops the new .exe into the
;   running install folder atomically.
; - resources.pri is generated post-install by Inno Setup so the WinUI 3
;   theme XAML resolves at runtime (mirrors what generate-pri.ps1 does).
; - "Add to Add/Remove Programs" entry + Start menu shortcut + uninstaller.

#ifndef AppVersion
  #define AppVersion "1.0.0"
#endif
#ifndef SourceDir
  #define SourceDir "..\publish"
#endif
#ifndef Arch
  #define Arch "x64"
#endif

#define AppName       "DraftRight"
#define AppPublisher  "DraftRight"
#define AppURL        "https://draftright.info"
#define AppExeName    "DraftRightWindows.exe"
#define InstallerBase "DraftRight-Setup-" + AppVersion + "-" + Arch

[Setup]
; A unique GUID identifies this app in Add/Remove Programs and is required
; for upgrades to overwrite the previous install in place. Don't change this.
AppId={{D8B6F9C2-7E1A-4B5C-9F3D-2E0A1B5C8D7F}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}
AppUpdatesURL={#AppURL}/download

; Per-user install: no admin prompt, can install on locked-down corp machines.
DefaultDirName={localappdata}\Programs\{#AppName}
DefaultGroupName={#AppName}
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog

; Don't show "Select destination" / "Select start menu folder" pages — most
; users won't customize these, and the few who want to can re-run with /DIR=.
DisableDirPage=auto
DisableProgramGroupPage=auto

; Output to installer/Output/ relative to the .iss file.
OutputDir=Output
OutputBaseFilename={#InstallerBase}

; Compression — LZMA2/Ultra is plenty for ~150 MB of self-contained .NET DLLs.
Compression=lzma2/ultra
SolidCompression=yes

; Architecture restrictions — the installer .exe runs on any 64-bit Windows,
; but the payload is arch-specific. ArchitecturesAllowed prevents the user
; from running an x64 installer on an arm64 machine (or vice-versa) and
; ending up with a binary that won't launch.
#if Arch == "arm64"
  ArchitecturesAllowed=arm64
  ArchitecturesInstallIn64BitMode=arm64
#else
  ArchitecturesAllowed=x64compatible
  ArchitecturesInstallIn64BitMode=x64compatible
#endif

; Icon shown in the installer wizard + Add/Remove Programs entry.
; Resolved relative to this .iss file. Pulled from the repo (always present),
; not from {#SourceDir} — the latter would break for CI builds that use
; PublishSingleFile=true + IncludeAllContentForSelfExtract=true, which
; bundles content files into the single .exe and leaves no separate
; Assets folder in publish/.
SetupIconFile=..\DraftRightWindows\Assets\DraftRight.ico
WizardStyle=modern

; Application identity — version comes through to file properties + the "Date
; Modified" / "About" surfaces.
VersionInfoVersion={#AppVersion}
VersionInfoProductName={#AppName}
VersionInfoCompany={#AppPublisher}

; Minimum supported OS — matches csproj's TargetPlatformMinVersion (Win10 1809).
MinVersion=10.0.17763

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked
Name: "autostart";   Description: "Start DraftRight when I sign in to Windows"; GroupDescription: "Startup:"; Flags: unchecked

[Files]
; Recurse the entire publish folder. We *exclude* a few things that will be
; regenerated locally:
;   - Logs/, settings.json, auth.json — user-data, never shipped
;   - resources.pri — regenerated below from Microsoft.UI.Xaml.Controls.pri
;     after install so unpackaged WinUI theme XAML resolves at runtime
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs; \
  Excludes: "Logs\*,settings.json,auth.json,resources.pri,priconfig.xml"

[Icons]
; IconFilename points at the .exe — Windows pulls the embedded icon resource
; from there. This works whether or not the Assets folder is laid out in
; the install dir (single-file CI builds bundle Assets into the .exe).
Name: "{group}\{#AppName}";       Filename: "{app}\{#AppExeName}"; WorkingDir: "{app}"; IconFilename: "{app}\{#AppExeName}"
Name: "{group}\Uninstall {#AppName}"; Filename: "{uninstallexe}"
Name: "{userdesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; WorkingDir: "{app}"; IconFilename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Registry]
; "Start at login" toggle — user opts in via the autostart task. The app's own
; Settings → Launch at Login can also write/clear this same key, so the user
; can change their mind later without re-running the installer.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; \
  ValueType: string; ValueName: "{#AppName}"; ValueData: """{app}\{#AppExeName}"""; \
  Tasks: autostart; Flags: uninsdeletevalue

[Run]
Filename: "{app}\{#AppExeName}"; Description: "Launch {#AppName}"; \
  Flags: nowait postinstall skipifsilent

[UninstallRun]
; Stop a running instance before removing files so the uninstaller doesn't
; leave a locked exe behind. errorcontrol=warn so a missing taskkill doesn't
; abort the uninstall.
Filename: "{cmd}"; Parameters: "/C taskkill /IM {#AppExeName} /F"; \
  Flags: runhidden; RunOnceId: "KillRunningApp"

[Code]
{ ── Post-install hook: copy Microsoft.UI.Xaml.Controls.pri to resources.pri.

  WinUI 3 unpackaged apps can't resolve ms-appx:///Microsoft.UI.Xaml/Themes/
  themeresources.xaml without a top-level resources.pri. The framework PRI
  ships beside the .exe; we just need to make a copy named resources.pri so
  the resource manager finds it. This is the same workaround the local
  generate-pri.ps1 script does. Necessary for builds where
  EnableMsixTooling=false (i.e. all of CI's current self-contained builds). }
procedure CurStepChanged(CurStep: TSetupStep);
var
  SrcPri, DstPri: string;
begin
  if CurStep = ssPostInstall then
  begin
    SrcPri := ExpandConstant('{app}\Microsoft.UI.Xaml.Controls.pri');
    DstPri := ExpandConstant('{app}\resources.pri');
    if FileExists(SrcPri) and not FileExists(DstPri) then
    begin
      if not CopyFile(SrcPri, DstPri, False) then
        Log('WARN: could not copy ' + SrcPri + ' to ' + DstPri);
    end;
  end;
end;
