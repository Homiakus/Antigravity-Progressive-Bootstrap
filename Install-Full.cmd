@echo off
setlocal
cd /d "%~dp0"
chcp 65001 >nul
"%~dp0agctl.exe" install full --prereqs
set "RC=%errorlevel%"
echo.
"%~dp0agctl.exe" doctor --self-test
if not "%RC%"=="0" echo Установка завершилась с кодом %RC%.
pause
exit /b %RC%
