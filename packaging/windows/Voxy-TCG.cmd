@echo off
setlocal
chcp 65001 >nul
set VOXY_ACCEL=tcg
"%~dp0Voxy.exe"
if errorlevel 1 pause
