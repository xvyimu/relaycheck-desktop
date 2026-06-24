@echo off
cd /d E:\zidqiandao\relaycheck-desktop
set RELAYCHECK_PORT=3001
set RELAYCHECK_NO_OPEN=0
start "" /b dist\relaycheck.exe
exit /b 0
