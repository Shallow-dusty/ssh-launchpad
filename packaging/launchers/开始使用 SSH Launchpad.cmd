@echo off
setlocal
set "SSH_LAUNCHPAD_LANG=zh-CN"
set "SSH_LAUNCHPAD_LAUNCHER=1"
set "APP=%~dp0ssh-launchpad.exe"
if not exist "%APP%" (
  echo ssh-launchpad.exe was not found. Extract the complete ZIP first.
  echo.
  pause
  exit /b 2
)
"%APP%" --interactive --lang zh-CN
set "CODE=%ERRORLEVEL%"
echo.
if not "%CODE%"=="0" (
  echo SSH Launchpad did not finish. Exit code: %CODE%
  echo Open the Chinese offline help file in this folder.
)
echo.
pause
exit /b %CODE%
