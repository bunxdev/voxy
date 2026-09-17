param([Parameter(Mandatory=$true)][string]$Exe,[Parameter(Mandatory=$true)][string]$ProbeURL)
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$exe=(Resolve-Path $Exe).Path
$env:VOXY_DATA_DIR=Join-Path $env:LOCALAPPDATA ('Voxy\auto-test-'+[guid]::NewGuid().ToString('N').Substring(0,8))
$env:VOXY_SSH_PORT='22470'
$env:VOXY_AUTO_BACKUP='0'
function Run { & $exe @args; if($LASTEXITCODE -ne 0){throw "Failed $args"} }
function Http($port) { $r=Invoke-WebRequest -UseBasicParsing -DisableKeepAlive -TimeoutSec 3 "http://127.0.0.1:$port/"; if($r.Content.Trim() -ne 'voxy-ports-ok'){throw 'HTTP mismatch'} }
try {
 if(!(Test-Path (Join-Path $env:VOXY_DATA_DIR 'disk.qcow2'))){Run init}
 Run ports clear
 Run ports add tcp 33470 8080
 Run ports auto on
 Run start; Run wait
 Run ssh 'command -v ss; ss -H -lntu4'
 Run ssh 'command -v curl >/dev/null || (apt-get update -qq && apt-get install -y --no-install-recommends curl >/tmp/apt.log)'
 Run ssh "curl -fsS '$ProbeURL' -o /root/ports-probe && chmod +x /root/ports-probe && systemd-run --unit=ports-probe /root/ports-probe"
 Start-Sleep 6
 Http 8080;Http 8081
 $u=[Net.Sockets.UdpClient]::new();$u.Client.ReceiveTimeout=5000;$u.Connect('127.0.0.1',8082)
 $b=[Text.Encoding]::ASCII.GetBytes('voxy-udp-ok');[void]$u.Send($b,$b.Length)
 $peer=[Net.IPEndPoint]::new([Net.IPAddress]::Any,0)
 if([Text.Encoding]::ASCII.GetString($u.Receive([ref]$peer)) -ne 'voxy-udp-ok'){throw 'UDP mismatch'}
 $u.Dispose()
 Run ports list
 Run ports auto off;Start-Sleep 5
 $open=$false;try {Http 8080;$open=$true}catch{};if($open){throw 'off failed'}
 Http 33470
 Run ports auto on;Start-Sleep 5;Http 8080
 Run ssh 'systemctl stop ports-probe';Start-Sleep 5
 $open=$false;try {Http 8080;$open=$true}catch{};if($open){throw 'removal failed'}
 Run stop;Run start;Run wait
 Run ssh 'systemd-run --unit=ports-probe /root/ports-probe'
 Start-Sleep 6;Http 8080;Run ports list
 'PASS auto Windows: TCP UDP live on/off removal restart manual coexistence'
} finally {& $exe stop}
