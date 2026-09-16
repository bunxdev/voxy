# Windows x64: paquete portátil y pruebas preliminares

La primera entrega es un **ZIP portátil**, con `Voxy.exe`, QEMU, sus DLL y Debian
12. No es un instalador MSI/NSIS. El lanzador está escrito en Go y usa las API de
Windows para procesos, permisos y detección de WHPX. El cliente SSH está integrado;
no necesita Bash, PowerShell para operar ni OpenSSH instalado.

**Objetivo:** Windows 10 2004+ / Windows 11 x64. La validación inicial se hace con
Wine 10 en Debian Linux y TCG; no constituye una prueba de Windows real ni de WHPX.
Windows ARM64 queda fuera de esta entrega.

## Probar en Windows

1. Descarga y extrae **todo** `Voxy-0.3.0-windows-x64.zip` en una carpeta local.
2. Abre `Voxy.exe`. El menú permite iniciar, entrar a Debian por SSH y apagar.
3. Para aceleración de hardware, habilita **Plataforma de hipervisor de Windows**
   desde `optionalfeatures.exe` y reinicia si Windows lo solicita. También requiere
   virtualización habilitada en el procesador/firmware. Voxy solo diagnostica;
   no modifica características de Windows ni solicita elevación.
4. Para emulación por software explícita, abre `Voxy-TCG.cmd`. Es más lenta.
5. Ejecuta `Probar-Windows.cmd` para validar WHPX o `Probar-TCG.cmd` para TCG.

Las pruebas crean una VM separada, usan un puerto SSH local libre y no alteran el
disco habitual. Comprueban Debian, SSH, DNS/APT, sincronización del kernel,
apagado, integridad qcow2, ampliación 1 → 2 GiB, persistencia y segundo arranque.
Los registros están en `%LOCALAPPDATA%\Voxy\prueba-whpx.log` o `prueba-tcg.log`.
Los discos de prueba se conservan para diagnóstico; su ruta aparece en el registro.

Los binarios no tienen firma Authenticode de Voxy. SmartScreen puede mostrar una
advertencia. Falta validar instalación y permisos en un Windows limpio, Defender,
Firewall, coexistencia con WSL2/Hyper-V, suspensión y recuperación.

## Datos y uso por terminal

Datos habituales: `%LOCALAPPDATA%\Voxy\amd64`. El lanzador configura una DACL
protegida con acceso al usuario actual y SYSTEM, heredable por los archivos nuevos.
La efectividad de esas ACL debe comprobarse en Windows real; Wine no la certifica.
No copies claves privadas a reportes ni issues. Los logs de prueba no contienen claves.

```powershell
.\Voxy.exe doctor
.\Voxy.exe init
.\Voxy.exe start
.\Voxy.exe wait
.\Voxy.exe ssh "uname -a"
.\Voxy.exe ssh
.\Voxy.exe stop
.\Voxy.exe resize 8G
```

Variables: `VOXY_DATA_DIR`, `VOXY_RAM_MB` (512 por defecto), `VOXY_CPUS` (1),
`VOXY_SSH_PORT` (22222), `VOXY_ACCEL` (`auto`, `whpx`, `tcg`). `auto` exige WHPX
cuando está disponible y explica cómo habilitarlo si falta; **no cambia a TCG
silenciosamente**. En PowerShell: `$env:VOXY_ACCEL='tcg'`.

SSH escucha solo en `127.0.0.1`. No hay API/CDP ni Docker/Lupa en esta base todavía.
Cerrar el panel deja QEMU funcionando; usa `stop` para apagar Debian correctamente.
El registro del proceso incluye PID, ruta y tiempo de creación para evitar confundir
un PID reutilizado. Las operaciones de control usan `control.lock`; tras un cierre
inesperado revisa los procesos antes de retirar manualmente un bloqueo abandonado.

`sync-kernel` descarga kernel/initramfs mientras Debian está encendido, verifica
ambos y prepara su aplicación **después del apagado**. Windows mantiene abierto el
initramfs usado por QEMU. Los archivos `.next` y su manifiesto se conservan hasta
completar la sustitución; `stop` o el siguiente `start` aplican la actualización
pendiente con la VM apagada. No borres esos archivos para reparar una actualización
sin revisar primero el error. No se incluye apagado forzado automático.

## Construir desde Linux

Herramientas: Go 1.27.1, Bun, curl, tar/xz, 7zip (`7z`), binutils (`objdump`), zip.

```sh
bash scripts/build-windows.sh
```

La compilación usa `GOOS=windows GOARCH=amd64 CGO_ENABLED=0`; versiones Go fijadas
en `windows/go.mod` y `go.sum`. El constructor descarga el instalador de QEMU
publicado por Stefan Weil y enlazado desde [qemu.org](https://www.qemu.org/download/),
verifica el SHA-512 fijado en `qemu.lock` y extrae archivos sin ejecutar el instalador.

Se incluyen el emulador x86_64, `qemu-img`, **104 DLL de sus importaciones transitivas**
y el firmware PC necesario. `scripts/bundle-windows.ts` rechaza dependencias no
resueltas fuera de una lista explícita de bibliotecas del sistema Windows. No se
redistribuyen DLL de Wine ni DLL del sistema Windows. Las bibliotecas cargadas
opcionalmente en funciones ajenas a esta configuración headless no están validadas.

`qemu/dependencies.json` registra los hashes e importaciones de los 106 PE incluidos.
La imagen Debian se extrae de la versión fijada en `images/amd64.lock`; sus tres
archivos tienen un manifiesto SHA-256 verificado durante `init`. El ZIP también
incluye `SHA256SUMS`, avisos de licencia y referencias de fuentes/recetas.

Para reconstruir, mueve primero `dist/windows-build` y el ZIP anterior. No se
incluyen prefijos Wine, discos de prueba, claves ni credenciales dentro del paquete.

## Pruebas de desarrollo

```sh
cd windows
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go test -c -o ../.cache/windows/unit-tests.exe .
# En un prefijo Wine x64 dedicado:
wine ../.cache/windows/unit-tests.exe -test.v
```

Las pruebas unitarias cubren tamaños inválidos/overflow, protección de discos
existentes, rechazo de imagen corrupta, recuperación de inicialización interrumpida,
PID reutilizado y validación de ambos archivos antes de aplicar un kernel nuevo.
La prueba integral se ejecuta con el mismo `Voxy.exe` distribuido, mediante `test`.

Referencias: [QEMU Windows](https://qemu.weilnetz.de/w64/),
[WHPX y requisitos](https://www.qemu.org/docs/master/system/whpx.html),
[Wine](https://www.winehq.org/about).

### Rutas con acentos

El QEMU distribuido usa conversiones de rutas que fallan con nombres como `ñ` al
recibir rutas absolutas. El launcher establece el directorio de trabajo mediante
la API Unicode de Windows y pasa nombres relativos ASCII para disco, kernel,
claves y logs. Copia el pequeño firmware PC a `firmware/` dentro del directorio de
datos; `qemu-img` también opera desde ese directorio. La ruta del ejecutable y
las operaciones propias del launcher siguen usando las API Unicode de Go/Windows.
