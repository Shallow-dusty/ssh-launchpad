@echo off
setlocal
chcp 65001 >nul
set "SSH_LAUNCHPAD_LANG=en"
set "SSH_LAUNCHPAD_LAUNCHER=1"
set "APP=%~dp0ssh-launchpad.exe"
if not exist "%APP%" (
  echo ssh-launchpad.exe was not found. Extract the complete ZIP before opening this launcher.
  echo.
  pause
  exit /b 2
)
"%APP%" --interactive --lang en
set "CODE=%ERRORLEVEL%"
echo.
if "%CODE%"=="0" (
  echo SSH Launchpad finished.
) else (
  echo SSH Launchpad did not finish. Exit code: %CODE%
  echo See "Offline Help - English.md" in this folder.
)
echo.
pause
exit /b %CODE%
