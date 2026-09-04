@echo off
setlocal
set "APP_ROOT=%~dp0"
set "LH_TUI_DIR=%APP_ROOT%UI"
set "LH_DASHBOARD_STATIC=%APP_ROOT%UI\GUI\dist"
set "PATH=%APP_ROOT%runtime\node;%PATH%"
"%APP_ROOT%lh.exe" tui %*
