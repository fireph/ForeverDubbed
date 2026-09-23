@echo off
cd /d "%~dp0"
if exist foreverdubbed.exe (
  foreverdubbed.exe %*
) else (
  dist\foreverdubbed.exe %*
)
