@echo off
setlocal
set "ORIG=%CD%"
cd /d "%~dp0"
chcp 65001 >nul
set "AGCTL=%~dp0agctl.exe"
cd /d "%ORIG%"
"%AGCTL%" project detect
if errorlevel 1 goto :fail
"%AGCTL%" project init
if errorlevel 1 goto :fail
"%AGCTL%" capabilities build --workspace "%ORIG%"
if errorlevel 1 goto :fail
echo.
echo Project initialization complete.
pause
exit /b 0
:fail
set "RC=%errorlevel%"
echo.
echo Ошибка. Код: %RC%
pause
exit /b %RC%
