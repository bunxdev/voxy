# Execute on Windows against an extracted package. Only fresh test VMs are terminated.
param([Parameter(Mandatory=$true)][string]$Exe, [switch]$TestSchedule)
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$Exe=(Resolve-Path $Exe).Path
$env:VOXY_DATA_DIR=Join-Path $env:LOCALAPPDATA ('Voxy\recovery-test-'+[guid]::NewGuid().ToString('N').Substring(0,8))
$env:VOXY_ACCEL='whpx'
$env:VOXY_AUTO_BACKUP='0'
$listener=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,0)
$listener.Start();$env:VOXY_SSH_PORT=[string]$listener.LocalEndpoint.Port;$listener.Stop()
function Run { & $Exe @args; if ($LASTEXITCODE -ne 0) { throw "Voxy failed: $args" } }
function Points { @(Get-ChildItem "$env:VOXY_DATA_DIR\backups" -Directory | Where-Object {$_.Name -match '^\d{8}T' -and (Test-Path (Join-Path $_.FullName 'manifest.json'))} | Sort-Object Name) }
function AssertMarker($want) { $got=& $Exe ssh 'cat /root/recovery-marker';if($LASTEXITCODE -ne 0 -or $got.Trim() -ne $want){throw "Marker mismatch: expected $want, got $got"} }
function KillTestVM {
 $state=Get-Content "$env:VOXY_DATA_DIR\process.json" | ConvertFrom-Json
 $proc=Get-Process -Id $state.PID
 if ($proc.Path -ne $state.Executable) {throw 'Unexpected process identity'}
 # The process record is created by this test in its unique directory.
 Stop-Process -Id $state.PID -Force
}
Write-Output "Test data: $env:VOXY_DATA_DIR"
try {
 Run init;Run start;Run wait
 Run ssh 'echo BEFORE > /root/recovery-marker; sync'
 Run backup
 $snapshot=(Points)[-1].Name
 Run ssh 'echo AFTER > /root/recovery-marker; sync'
 KillTestVM;Start-Sleep 2
 Run start;Run wait;AssertMarker 'AFTER'
 Write-Output 'PASS: abrupt QEMU termination and restart retained flushed data'
 Run stop;Run restore $snapshot;Run start;Run wait;AssertMarker 'BEFORE'
 Write-Output 'PASS: independent snapshot restored earlier data'
 # Retention is enforced only after a new complete checkpoint is verified.
 1..3 | ForEach-Object {Run backup}
 if((Points).Count -ne 3){throw 'Retention did not keep exactly 3 checkpoints'}
 Write-Output 'PASS: retention = 3 verified checkpoints'
 $good=(Points)[-1]
 $hash=(Get-FileHash (Join-Path $good.FullName 'disk.qcow2')).Hash
 $backup=Start-Process -FilePath $Exe -ArgumentList 'backup' -PassThru -RedirectStandardOutput "$env:VOXY_DATA_DIR\interrupted-backup.log" -RedirectStandardError "$env:VOXY_DATA_DIR\interrupted-backup-error.log"
 $deadline=(Get-Date).AddSeconds(30);$partial=$null
 do {
  Start-Sleep -Milliseconds 100
  $partial=Get-ChildItem "$env:VOXY_DATA_DIR\backups\.partial-*\disk.qcow2" -ErrorAction SilentlyContinue | Where-Object {$_.Length -gt 1048576} | Select-Object -First 1
 } while (!$partial -and !$backup.HasExited -and (Get-Date) -lt $deadline)
 if(!$partial){throw 'Could not interrupt an active backup'}
 KillTestVM
 if(!$backup.HasExited){Stop-Process -Id $backup.Id -Force}
 if((Get-FileHash (Join-Path $good.FullName 'disk.qcow2')).Hash -ne $hash){throw 'Previous good backup changed'}
 if((Points).Count -ne 3){throw 'Incomplete backup was published'}
 Run restore $good.Name;Run start;Run wait;AssertMarker 'BEFORE'
 Run backup
 if(Get-ChildItem "$env:VOXY_DATA_DIR\backups\.partial-*" -ErrorAction SilentlyContinue){throw 'Orphaned partial copy was not cleaned'}
 Write-Output 'PASS: interrupted backup preserved previous points; restore and next backup succeeded'
 Run stop
 if($TestSchedule){
  Remove-Item Env:VOXY_AUTO_BACKUP
  $previous=(Points)[-1].Name
  Run start;Run wait
  $initialDeadline=(Get-Date).AddMinutes(3)
  do {Start-Sleep 3;$first=(Points)[-1].Name} while($first -eq $previous -and (Get-Date) -lt $initialDeadline)
  if($first -eq $previous){throw 'Initial automatic checkpoint missing'}
  $firstManifest=Get-Content "$env:VOXY_DATA_DIR\backups\$first\manifest.json" | ConvertFrom-Json
  Run ssh 'echo SCHEDULED > /root/recovery-marker; sync'
  $deadline=(Get-Date).AddMinutes(12)
  do {Start-Sleep 10;$newest=(Points)[-1].Name} while($newest -eq $first -and (Get-Date) -lt $deadline)
  if($newest -eq $first){throw 'No scheduled checkpoint within 12 minutes'}
  $nextManifest=Get-Content "$env:VOXY_DATA_DIR\backups\$newest\manifest.json" | ConvertFrom-Json
  if(([DateTime]$nextManifest.Created-[DateTime]$firstManifest.Created).TotalSeconds -lt 590){throw 'Initial copy mistaken for scheduled copy'}
  Write-Output "PASS: scheduled checkpoint $newest after initial $first"
  Run stop
 }
 Write-Output ('PASS Windows recovery suite '+[DateTime]::UtcNow.ToString('o'))
} finally {
 # No normal VM paths or PIDs are used here. Keep disks and logs for inspection.
 & $Exe stop
}
