param([Parameter(Mandatory=$true)][string]$Exe,[Parameter(Mandatory=$true)][string]$ProbeURL)
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$Exe=(Resolve-Path $Exe).Path
$env:VOXY_DATA_DIR=Join-Path $env:LOCALAPPDATA ('Voxy\ports-test-'+[guid]::NewGuid().ToString('N').Substring(0,8))
$env:VOXY_SSH_PORT='22460'
$env:VOXY_AUTO_BACKUP='0'
$env:VOXY_ACCEL='whpx'
function Run { & $Exe @args; if($LASTEXITCODE -ne 0){throw "Voxy failed: $args"} }
function Http($port) {
 $r=Invoke-WebRequest -UseBasicParsing -TimeoutSec 5 "http://127.0.0.1:$port/"
 if($r.StatusCode -ne 200 -or $r.Content.Trim() -ne 'voxy-ports-ok'){throw 'HTTP mismatch'}
}
function Probe {
 Run ssh 'command -v curl >/dev/null || (apt-get update -qq && apt-get install -y --no-install-recommends curl >/tmp/ports-apt.log)'
 Run ssh "curl -fsS --max-time 60 '$ProbeURL' -o /tmp/voxy-port-probe && chmod +x /tmp/voxy-port-probe && systemd-run --unit=voxy-port-probe /tmp/voxy-port-probe"
 Start-Sleep 2
 Http 33460; Http 33461
 $udp=[Net.Sockets.UdpClient]::new()
 try {
  $udp.Client.ReceiveTimeout=5000
  $udp.Connect('127.0.0.1',33462)
  $b=[Text.Encoding]::ASCII.GetBytes('voxy-udp-ok')
  [void]$udp.Send($b,$b.Length)
  $peer=[Net.IPEndPoint]::new([Net.IPAddress]::Any,0)
  $reply=$udp.Receive([ref]$peer)
  if([Text.Encoding]::ASCII.GetString($reply) -ne 'voxy-udp-ok'){throw 'UDP mismatch'}
 } finally {$udp.Dispose()}
}
try {
 Write-Output "Isolated test data: $env:VOXY_DATA_DIR"
 Run init
 Run ports add tcp 33460-33461 8080-8081
 Run ports add udp 33462 8082
 & $Exe ports add tcp 33460 8080 0.0.0.0
 if($LASTEXITCODE -eq 0){throw 'Overlap accepted'}
 Run start; Run wait; Probe; Run ports list
 Run stop; Run start; Run wait; Probe
 Run ports clear; Http 33460; Run ports list
 Run stop; Run start; Run wait
 $connected=$false
 try {Http 33460;$connected=$true} catch {}
 if($connected){throw 'Removed mapping still open'}
 Run stop
 Run ports add tcp 33460 8080
 $listener=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,33460)
 $listener.Server.ExclusiveAddressUse=$true
 $listener.Start()
 try {
  & $Exe start
  if($LASTEXITCODE -eq 0){throw 'Occupied port accepted'}
 } finally {$listener.Stop()}
 Run start; Run wait; Run stop
 Write-Output 'PASS Windows WHPX: TCP range, UDP echo, restart, removal, busy port, clean recovery'
} finally {& $Exe stop}
