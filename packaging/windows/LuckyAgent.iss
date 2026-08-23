#ifndef SourceRoot
  #define SourceRoot "..\\..\\dist\\windows-installer"
#endif
#ifndef MyAppVersion
  #define MyAppVersion "dev"
#endif

[Setup]
AppId={{F7E5D335-EDC5-437B-9E6B-51FBBEA89A03}
AppName=LuckyAgent
AppVersion={#MyAppVersion}
AppPublisher=LuckyAgent
DefaultDirName={autolocalappdata}\LuckyAgent
DefaultGroupName=LuckyAgent
DisableProgramGroupPage=yes
OutputBaseFilename=LuckyAgent-Setup-{#MyAppVersion}-x64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=lowest
UninstallDisplayIcon={app}\lh.exe

[Files]
Source: "{#SourceRoot}\lh.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceRoot}\ConfigurationCenter.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceRoot}\UI\GUI\dist\*"; DestDir: "{app}\UI\GUI\dist"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\LuckyAgent Configuration Center"; Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\ConfigurationCenter.ps1"""; WorkingDir: "{app}"
Name: "{group}\Stop LuckyAgent"; Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\ConfigurationCenter.ps1"" -Action Stop"; WorkingDir: "{app}"
Name: "{autodesktop}\LuckyAgent Configuration Center"; Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\ConfigurationCenter.ps1"""; WorkingDir: "{app}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"

[Run]
Filename: "{app}\lh.exe"; Parameters: "init"; WorkingDir: "{app}"; Flags: runhidden waituntilterminated
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\ConfigurationCenter.ps1"""; WorkingDir: "{app}"; Description: "Launch LuckyAgent Configuration Center"; Flags: postinstall nowait skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{app}"
