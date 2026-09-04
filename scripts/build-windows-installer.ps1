param(
  [string]$Version = "dev",
  [string]$OutputDir = "dist",
  [string]$NodeRuntimeRoot = ""
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$StageDir = Join-Path $RepoRoot "dist\windows-installer"
$OutputPath = Join-Path $RepoRoot $OutputDir

Push-Location $RepoRoot
try {
  if (-not $NodeRuntimeRoot) {
    $NodeExe = (Get-Command node -ErrorAction Stop).Source
    $NodeRuntimeRoot = Split-Path -Parent $NodeExe
  }
  if (-not (Test-Path (Join-Path $NodeRuntimeRoot "node.exe"))) {
    throw "Node runtime was not found at $NodeRuntimeRoot"
  }
  if (-not (Test-Path "$RepoRoot\UI\GUI\dist\index.html") -or -not (Test-Path "$RepoRoot\UI\TUI\dist\tui.mjs")) {
    throw "UI release assets are missing; run npm --prefix UI run build first"
  }
  New-Item -ItemType Directory -Force -Path "$StageDir\UI\GUI" | Out-Null
  Copy-Item -Force "$RepoRoot\dist\lh.exe" "$StageDir\lh.exe"
  Copy-Item -Force "$RepoRoot\packaging\windows\ConfigurationCenter.ps1" "$StageDir\ConfigurationCenter.ps1"
  Copy-Item -Force "$RepoRoot\packaging\windows\Install-Portable.ps1" "$StageDir\Install-Portable.ps1"
  Copy-Item -Force "$RepoRoot\packaging\windows\LuckyAgent-TUI.cmd" "$StageDir\LuckyAgent-TUI.cmd"
  Copy-Item -Force "$RepoRoot\packaging\windows\LuckyAgent-GUI.cmd" "$StageDir\LuckyAgent-GUI.cmd"
  Copy-Item -Recurse -Force "$RepoRoot\UI\GUI\dist" "$StageDir\UI\GUI\dist"
  New-Item -ItemType Directory -Force -Path "$StageDir\UI\TUI" | Out-Null
  Copy-Item -Recurse -Force "$RepoRoot\UI\TUI\dist" "$StageDir\UI\TUI\dist"
  New-Item -ItemType Directory -Force -Path "$StageDir\runtime" | Out-Null
  Copy-Item -Recurse -Force $NodeRuntimeRoot "$StageDir\runtime\node"
  $Compiler = Get-Command iscc -ErrorAction SilentlyContinue
  if (-not $Compiler) { throw "Inno Setup compiler (iscc) is required. Install it with: choco install innosetup" }
  & $Compiler.Source "/DSourceRoot=$StageDir" "/DMyAppVersion=$Version" "/O$OutputPath" "$RepoRoot\packaging\windows\LuckyAgent.iss"
} finally {
  Pop-Location
}
