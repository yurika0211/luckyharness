param(
  [ValidateSet("Start", "Stop", "Restart", "Status")]
  [string]$Action = "Start"
)

$ErrorActionPreference = "Stop"
$InstallDir = Split-Path -Parent $PSCommandPath
$Binary = Join-Path $InstallDir "lh.exe"
$RuntimeDir = Join-Path $env:LOCALAPPDATA "LuckyAgent\runtime"
$LogDir = Join-Path $env:LOCALAPPDATA "LuckyAgent\logs"
$StaticDir = Join-Path $InstallDir "UI\GUI\dist"
$TuiDir = Join-Path $InstallDir "UI"
$NodeDir = Join-Path $InstallDir "runtime\node"

if (-not (Test-Path $Binary)) {
  throw "LuckyAgent executable was not found at $Binary"
}

New-Item -ItemType Directory -Force -Path $RuntimeDir, $LogDir | Out-Null
$env:LH_TUI_DIR = $TuiDir
$env:LH_DASHBOARD_STATIC = $StaticDir
$env:PATH = "$NodeDir;$env:PATH"

function Get-ComponentProcess([string]$Name) {
  $PidFile = Join-Path $RuntimeDir "$Name.pid"
  if (-not (Test-Path $PidFile)) { return $null }
  $PidValue = Get-Content -Raw $PidFile
  $Process = Get-Process -Id $PidValue.Trim() -ErrorAction SilentlyContinue
  if (-not $Process) { Remove-Item -Force $PidFile -ErrorAction SilentlyContinue }
  return $Process
}

function Start-Component([string]$Name, [string[]]$Arguments, [bool]$UseDashboardStatic) {
  $Existing = Get-ComponentProcess $Name
  if ($Existing) { return $Existing }
  if ($UseDashboardStatic) { $env:LH_DASHBOARD_STATIC = $StaticDir }
  $LogPath = Join-Path $LogDir "$Name.log"
  $ErrorLogPath = Join-Path $LogDir "$Name.error.log"
  $Process = Start-Process -FilePath $Binary -ArgumentList $Arguments -WorkingDirectory $InstallDir -RedirectStandardOutput $LogPath -RedirectStandardError $ErrorLogPath -WindowStyle Hidden -PassThru
  Set-Content -NoNewline -Path (Join-Path $RuntimeDir "$Name.pid") -Value $Process.Id
  return $Process
}

function Stop-Component([string]$Name) {
  $Process = Get-ComponentProcess $Name
  if ($Process) { Stop-Process -Id $Process.Id -ErrorAction SilentlyContinue }
  Remove-Item -Force (Join-Path $RuntimeDir "$Name.pid") -ErrorAction SilentlyContinue
}

function Show-Status {
  foreach ($Name in "serve", "dashboard") {
    $Process = Get-ComponentProcess $Name
    if ($Process) {
      Write-Host "$Name is running (PID $($Process.Id))."
    } else {
      Write-Host "$Name is stopped."
    }
  }
  Write-Host "Logs: $LogDir"
}

switch ($Action) {
  "Stop" {
    Stop-Component "dashboard"
    Stop-Component "serve"
    Show-Status
    break
  }
  "Restart" {
    Stop-Component "dashboard"
    Stop-Component "serve"
  }
  "Status" {
    Show-Status
    break
  }
}

if ($Action -eq "Start" -or $Action -eq "Restart") {
  & $Binary init | Out-Null
  Start-Component "serve" @("serve") $false | Out-Null
  Start-Component "dashboard" @("dashboard", "start") $true | Out-Null
  Start-Sleep -Seconds 2
  Start-Process "http://127.0.0.1:8765"
  Show-Status
}
