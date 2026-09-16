@echo off
setlocal
chcp 65001 >nul
set VOXY_ACCEL=whpx
if not exist "%LOCALAPPDATA%\Voxy" mkdir "%LOCALAPPDATA%\Voxy"
set "VOXY_TEST_LOG=%LOCALAPPDATA%\Voxy\prueba-whpx.log"
echo Ejecutando diagnostico y prueba WHPX. Puede tardar varios minutos.
"%~dp0Voxy.exe" doctor > "%VOXY_TEST_LOG%" 2>&1
if errorlevel 1 goto failed
"%~dp0Voxy.exe" test >> "%VOXY_TEST_LOG%" 2>&1
if errorlevel 1 goto failed
type "%VOXY_TEST_LOG%"
echo Prueba completada. Log: %VOXY_TEST_LOG%
pause
exit /b 0
:failed
type "%VOXY_TEST_LOG%"
echo Prueba fallida. Comparte este log: %VOXY_TEST_LOG%
pause
exit /b 1
