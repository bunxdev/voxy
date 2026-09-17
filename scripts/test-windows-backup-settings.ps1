# Real Windows integration: isolated VM; initial-only default, live opt-in and opt-out.
param([Parameter(Mandatory=$true)][string]$Exe)
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$Exe=(Resolve-Path $Exe).Path
$env:VOXY_DATA_DIR=Join-Path $env:LOCALAPPDATA ('Voxy\settings-test-'+[guid]::NewGuid().ToString('N').Substring(0,8))
$env:VOXY_ACCEL='whpx'
Remove-Item Env:VOXY_AUTO_BACKUP -ErrorAction SilentlyContinue
$listener=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,0)
$listener.Start();$env:VOXY_SSH_PORT=[string]$listener.LocalEndpoint.Port;$listener.Stop()
function Run { & $Exe @args; if($LASTEXITCODE -ne 0){throw "Voxy failed: $args"} }
function Points { @(Get-ChildItem "$env:VOXY_DATA_DIR\backups" -Directory -ErrorAction SilentlyContinue | Where-Object {$_.Name -match '^\d{8}T' -and (Test-Path (Join-Path $_.FullName 'manifest.json'))} | Sort-Object Name) }
function Latest { $p=Points; if($p.Count){$p[-1].Name}else{''} }
function AwaitNew($previous,$seconds) {
 $deadline=(Get-Date).AddSeconds($seconds)
 do {Start-Sleep 3;$id=Latest} while($id -eq $previous -and (Get-Date) -lt $deadline)
 if($id -eq $previous){throw "Missing checkpoint after $seconds seconds"}
 return $id
}
function AssertUnchanged($previous,$seconds) {
 $deadline=(Get-Date).AddSeconds($seconds)
 do {
  Start-Sleep 5
  if((Latest) -ne $previous){throw 'Unexpected periodic checkpoint'}
  if(Get-ChildItem "$env:VOXY_DATA_DIR\backups\.partial-*" -ErrorAction SilentlyContinue){throw 'Unexpected backup in progress'}
 } while((Get-Date) -lt $deadline)
}
function Menu($inputText) {
 $inputText | & $Exe
 if($LASTEXITCODE -ne 0){throw 'Menu failed'}
}
Write-Output "Test data: $env:VOXY_DATA_DIR"
try {
 Run init;Run start;Run wait
 $first=AwaitNew '' 180
 if(Test-Path "$env:VOXY_DATA_DIR\backup-settings.json"){throw 'Unexpected initial configuration'}
 Write-Output "PASS initial checkpoint: $first; waiting 620 seconds with periodic copies disabled"
 Run ssh 'echo DEFAULT_ONLY > /root/settings-marker; sync'
 AssertUnchanged $first 620
 Write-Output 'PASS default: no periodic copy after more than 10 minutes despite disk writes'
 Menu "9`n1`n1`n5"
 $settings=Get-Content "$env:VOXY_DATA_DIR\backup-settings.json" | ConvertFrom-Json
 if(!$settings.periodic -or $settings.interval_minutes -ne 1){throw 'Menu did not persist one-minute interval'}
 $second=AwaitNew $first 180
 Write-Output "PASS enabled from menu without restarting VM: $second"
 Menu "9`n1`n2`n5"
 Run ssh 'echo TWO_MINUTES > /root/settings-marker; sync'
 AssertUnchanged $second 80
 $third=AwaitNew $second 100
 $m2=Get-Content "$env:VOXY_DATA_DIR\backups\$second\manifest.json" | ConvertFrom-Json
 $m3=Get-Content "$env:VOXY_DATA_DIR\backups\$third\manifest.json" | ConvertFrom-Json
 $elapsed=([DateTime]$m3.Created-[DateTime]$m2.Created).TotalSeconds
 if($elapsed -lt 120){throw "Interval ignored: $elapsed seconds"}
 Write-Output "PASS live interval change: next copy after $elapsed seconds"
 Menu "9`n2`n5"
 Run ssh 'echo DISABLED > /root/settings-marker; sync'
 AssertUnchanged $third 145
 Write-Output 'PASS disabled from menu: no new checkpoint despite writes'
 Run backup
 $manual=Latest
 if($manual -eq $third){throw 'Manual copy disabled unexpectedly'}
 Write-Output 'PASS manual backup remains available'
 Run stop;Run start;Run wait
 $restart=AwaitNew $manual 180
 $settings=Get-Content "$env:VOXY_DATA_DIR\backup-settings.json" | ConvertFrom-Json
 if($settings.periodic){throw 'Disabled preference did not survive restart'}
 Write-Output "PASS initial copy after VM restart with periodic copies still disabled: $restart"
 Run stop;Run restore $third
 $env:VOXY_AUTO_BACKUP='0'
 Run start;Run wait
 $marker=& $Exe ssh 'cat /root/settings-marker'
 if($LASTEXITCODE -ne 0 -or $marker.Trim() -ne 'TWO_MINUTES'){throw "Scheduled backup restore failed: $marker"}
 Write-Output 'PASS scheduled backup restores expected file contents'
 Write-Output ('PASS Windows backup settings integration '+[DateTime]::UtcNow.ToString('o'))
} finally {
 & $Exe stop
}
