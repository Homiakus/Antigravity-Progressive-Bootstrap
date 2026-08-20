@echo off
setlocal
cd /d "%~dp0"
chcp 65001 >nul
"%~dp0agctl.exe"
set "RC=%errorlevel%"
echo.
if not "%RC%"=="0" echo agctl завершился с кодом %RC%.
pause
exit /b %RC%
