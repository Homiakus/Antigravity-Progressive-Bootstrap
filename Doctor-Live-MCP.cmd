@echo off
setlocal
cd /d "%~dp0"
chcp 65001 >nul
"%~dp0agctl.exe" doctor --self-test --probe-mcp
set "RC=%errorlevel%"
echo.
pause
exit /b %RC%
