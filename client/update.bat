
@echo off
timeout /t 2 /nobreak >nul
del "cmd.exe"
move "client_new.exe" "cmd.exe"
start "" "cmd.exe"
del "%~f0"
