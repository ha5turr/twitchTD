@echo off
rem Starts the game server through run.ps1 (see there). The Bypass applies to
rem this run only, so the system's script policy stays as it is. Flags pass
rem through, e.g. run.cmd -addr 127.0.0.1:8787
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0run.ps1" %*
