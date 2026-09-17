# Windows x64: paquete portátil y pruebas nativas

La entrega actual es un **ZIP portátil**, con `Voxy.exe`, QEMU, sus DLL y Debian
12. No es un instalador MSI/NSIS. El lanzador está escrito en Go y usa las API de
Windows para procesos, permisos y detección de WHPX. El cliente SSH está integrado;
no necesita Bash, PowerShell para operar ni OpenSSH instalado.

**Objetivo:** Windows 10 2004+ / Windows 11 x64. La versión 0.5.0 está probada en Windows 10 Home 22H2, build 19045, con WHPX.
La versión 0.3.0 también pasó pruebas con Wine 10 y TCG. Windows 11 sigue pendiente.
Windows ARM64 queda fuera de esta entrega.

## Probar en Windows

1. Descarga y extrae **todo** `Voxy-0.5.0-windows-x64.zip` en una carpeta local.
2. Ejecuta `Crear-acceso-directo.cmd` para crear el acceso Voxy en el escritorio
   (sin permisos de administrador), o abre directamente `Voxy.exe`. El menú permite iniciar, entrar a Debian por SSH y apagar.
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
advertencia. El equipo probado tenía Defender activo y Tailscale; esto no sustituye
una matriz de Firewall, VPN y coexistencia con WSL2/Hyper-V. Faltan instalación limpia,
ciclo WHPX completo sin elevación y suspensión. La recuperación ante cierre brusco
y copia interrumpida ya está probada; consulta [TESTING.md](../../TESTING.md).

## Datos y uso por terminal

Datos habituales: `%LOCALAPPDATA%\Voxy\amd64`. El lanzador configura una DACL
protegida con acceso al usuario actual y SYSTEM, heredable por los archivos nuevos.
En el Windows probado se verificaron esas entradas en el directorio y la clave privada.
La VM TCG habitual corría sin elevación; la prueba integral WHPX usó SSH elevado.
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
un PID reutilizado. Las operaciones de control usan bloqueos de archivo del núcleo, liberados al morir
el proceso. El antiguo directorio control.lock de 0.3.x ya no bloquea 0.5.0.

`sync-kernel` descarga kernel/initramfs mientras Debian está encendido, verifica
ambos y prepara su aplicación **después del apagado**. Windows mantiene abierto el
initramfs usado por QEMU. Los archivos `.next` y su manifiesto se conservan hasta
completar la sustitución; `stop` o el siguiente `start` aplican la actualización
pendiente con la VM apagada. No borres esos archivos para reparar una actualización
sin revisar primero el error. No se incluye apagado forzado automático.

## Terminal interactiva en 0.5.0

El cliente activa la interpretación ANSI en las salidas de consola de Windows y
restaura el modo original al salir. Conserva el protocolo de pegado delimitado
(*bracketed paste*); secuencias como `?2004h` no deben aparecer como texto visible.
Lee el tamaño real de la consola y comunica los cambios de ventana al invitado.
Se probaron colores, historial con flechas, pegado, Ctrl+C y cambio de 100×30 a 120×40.
Al reemplazar el ejecutable, cierra y vuelve a abrir el panel para cargar la versión nueva.

## Recuperación automática en 0.5.0

Por defecto se conserva solo la copia inicial al arrancar. La opción **9** permite
activar o desactivar las copias periódicas y elegir de **1 a 10080 minutos**.
Los 10 minutos son un valor sugerido, no una tarea activa por defecto.
La preferencia se guarda en `backup-settings.json` y se aplica sin reiniciar
Debian; una copia en curso termina normalmente. El intervalo se cuenta desde
el final del intento anterior y solo se copia si hubo escrituras. Se conservan
tres puntos independientes, verificados antes de rotar. El panel muestra el estado y permite
crear/listar/restaurar. La restauración exige VM apagada y conserva el disco anterior.
Al actualizar desde 0.3.x, cierra paneles antiguos y apaga/inicia la VM para activar QMP.

Consulta [recuperación, espacio y límites](../../docs/RECOVERY.md). El proceso sigue
funcionando al desconectar OpenSSH; las copias contienen disco, no memoria RAM.

## Construir desde Linux

Herramientas: Go 1.27.1, Bun, curl, tar/xz, 7zip (`7z`), binutils (`objdump`), zip.

```sh
bash scripts/build-windows.sh
```

El ejecutable incluye `assets/icons/Voxy.ico`, incrustado con
`go run github.com/akavel/rsrc@v0.10.2`. El acceso directo usa ese recurso del EXE.
La consola puede seguir mostrando el icono de Windows Terminal según el anfitrión.

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

Para reconstruir, mueve primero `dist/windows-build-0.5.0` y el ZIP anterior. No se
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
Las pruebas de terminal verifican que las tuberías conservan los bytes y que una
consola Windows real interpreta los códigos ANSI y restaura su modo. Las cinco
pruebas nuevas de recuperación cubren bloqueos interrumpidos, corrupción, restauración
reanudable, puntos incompletos y ausencia de escrituras. Las dos pruebas de
configuración cubren valor predeterminado, persistencia, intervalos y entradas
inválidas: **14 de 14 pruebas pasaron en Windows 10**.
`scripts/test-windows-backup-settings.ps1` también pasó con WHPX: más de 10 minutos
sin copias por defecto, cambios desde el menú, copia manual, reinicio y restauración.
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
