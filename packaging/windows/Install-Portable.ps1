param(
  [string]$SourceRoot = (Split-Path -Parent $PSCommandPath),
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA "LuckyAgent")
)

$ErrorActionPreference = "Stop"

foreach ($required in @(
  (Join-Path $SourceRoot "lh.exe"),
  (Join-Path $SourceRoot "UI\GUI\dist\index.html"),
  (Join-Path $SourceRoot "UI\TUI\dist\tui.mjs"),
  (Join-Path $SourceRoot "runtime\node\node.exe")
)) {
  if (-not (Test-Path $required)) { throw "LuckyAgent payload is missing $required" }
}

$Parent = Split-Path -Parent $InstallDir
New-Item -ItemType Directory -Force -Path $Parent | Out-Null
$Stage = Join-Path $Parent (".LuckyAgent-stage-" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force -Path $Stage | Out-Null
Copy-Item -Force (Join-Path $SourceRoot "lh.exe") (Join-Path $Stage "lh.exe")
Copy-Item -Recurse -Force (Join-Path $SourceRoot "UI") (Join-Path $Stage "UI")
Copy-Item -Recurse -Force (Join-Path $SourceRoot "runtime") (Join-Path $Stage "runtime")
Copy-Item -Force (Join-Path $SourceRoot "ConfigurationCenter.ps1") (Join-Path $Stage "ConfigurationCenter.ps1")
Copy-Item -Force (Join-Path $SourceRoot "LuckyAgent-TUI.cmd") (Join-Path $Stage "LuckyAgent-TUI.cmd")
Copy-Item -Force (Join-Path $SourceRoot "LuckyAgent-GUI.cmd") (Join-Path $Stage "LuckyAgent-GUI.cmd")

if (Test-Path $InstallDir) { Remove-Item -Recurse -Force $InstallDir }
Move-Item -Force $Stage $InstallDir

$pathEntries = @($InstallDir, (Join-Path $InstallDir "runtime\node"))
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
$parts = @($currentPath -split ';' | Where-Object { $_ })
foreach ($entry in $pathEntries) {
  if (-not ($parts | Where-Object { $_.TrimEnd('\') -ieq $entry.TrimEnd('\') })) {
    $parts += $entry
  }
}
[Environment]::SetEnvironmentVariable("Path", ($parts -join ';'), "User")

$runtimeDir = Join-Path $HOME ".luckyagent\runtime"
New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
Set-Content -NoNewline -Path (Join-Path $runtimeDir "tui-ui-dir") -Value (Join-Path $InstallDir "UI")

& (Join-Path $InstallDir "lh.exe") init | Out-Null
Write-Host "LuckyAgent installed to $InstallDir"
Write-Host "Commands: lh, LuckyAgent-GUI.cmd, LuckyAgent-TUI.cmd"
Write-Host "Open a new terminal so the updated user PATH is available."
