param(
  [string]$Version = "dev",
  [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$StageDir = Join-Path $RepoRoot "dist\windows-installer"
$OutputPath = Join-Path $RepoRoot $OutputDir

Push-Location $RepoRoot
try {
  npm --prefix UI ci
  npm --prefix UI run build --workspace GUI
  New-Item -ItemType Directory -Force -Path "$StageDir\UI\GUI" | Out-Null
  Copy-Item -Force "$RepoRoot\dist\lh.exe" "$StageDir\lh.exe"
  Copy-Item -Force "$RepoRoot\packaging\windows\ConfigurationCenter.ps1" "$StageDir\ConfigurationCenter.ps1"
  Copy-Item -Recurse -Force "$RepoRoot\UI\GUI\dist" "$StageDir\UI\GUI\dist"
  $Compiler = Get-Command iscc -ErrorAction SilentlyContinue
  if (-not $Compiler) { throw "Inno Setup compiler (iscc) is required. Install it with: choco install innosetup" }
  & $Compiler.Source "/DSourceRoot=$StageDir" "/DMyAppVersion=$Version" "/O$OutputPath" "$RepoRoot\packaging\windows\LuckyAgent.iss"
} finally {
  Pop-Location
}
