$ErrorActionPreference = 'Stop'
$exe = Join-Path $PSScriptRoot 'Voxy.exe'
if (!(Test-Path -LiteralPath $exe)) { throw 'Extrae todo el ZIP antes de crear el acceso directo.' }
$desktop = [Environment]::GetFolderPath('Desktop')
$path = Join-Path $desktop 'Voxy.lnk'
$shell = New-Object -ComObject WScript.Shell
if (Test-Path -LiteralPath $path) {
    $existing = $shell.CreateShortcut($path)
    if ($existing.TargetPath -ne $exe) {
        throw 'Ya existe otro acceso Voxy. Conserva o retira ese acceso antes de continuar.'
    }
}
$link = $shell.CreateShortcut($path)
$link.TargetPath = $exe
$link.WorkingDirectory = $PSScriptRoot
$link.IconLocation = "$exe,0"
$link.Description = 'Voxy - Debian con QEMU'
$link.Save()
Write-Output "Acceso creado: $path"
