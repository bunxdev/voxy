@echo off
setlocal
chcp 65001 >nul
set VOXY_ACCEL=tcg
if not exist "%LOCALAPPDATA%\Voxy" mkdir "%LOCALAPPDATA%\Voxy"
set "VOXY_TEST_LOG=%LOCALAPPDATA%\Voxy\prueba-tcg.log"
echo Ejecutando prueba TCG. Es mas lenta que WHPX.
"%~dp0Voxy.exe" test > "%VOXY_TEST_LOG%" 2>&1
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
