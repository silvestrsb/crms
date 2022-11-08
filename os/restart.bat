@echo off
echo Server restart in 5 seconds
TIMEOUT 5 /nobreak
set rootpath=%1
start "Server" %rootpath%/Server.exe
exit
