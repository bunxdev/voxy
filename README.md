# Voxy

QEMU + Debian mínimo como base independiente para ejecutar Lupa y otras
aplicaciones Linux. **La etapa actual no instala Docker ni Lupa en la VM.**

El launcher selecciona AMD64 para Intel/AMD y ARM64 para Apple Silicon/ARM.
En macOS usa HVF, en Linux KVM cuando está disponible y TCG como alternativa
explícita. Las pruebas reales y sus límites están en [TESTING.md](TESTING.md).
Windows x64 tiene un launcher nativo experimental y un ZIP portátil para pruebas;
validado en Windows 10 Home 22H2 x64 con WHPX. Windows 11 y ARM64 siguen pendientes.

| Anfitrión probado | Invitado | Aceleración | Resultado |
| --- | --- | --- | --- |
| Mac mini M1, macOS 15.5 | Debian 12 ARM64 | HVF | Instalación y ciclo completo correctos |
| MacBook Pro Intel i5, macOS 13.6.9 | Debian 12 AMD64 | HVF | Instalación y ciclo completo correctos |
| Debian 13 Linux x86_64 | Debian 12 AMD64 | KVM | Regresión correcta |
| Windows 10 Home 22H2, build 19045, x64 | Debian 12 AMD64 | WHPX | Ciclo completo y terminal correctos |
| Ejecutables Windows x64 bajo Wine 10, Debian 13 | Debian 12 AMD64 | TCG | Ciclo completo correcto bajo Wine |

Las dos Macs usan QEMU 11.1.1. Los paquetes `.dmg` incluyen `Voxy.app`, QEMU,
sus bibliotecas y Debian 12. La aplicación abre un panel en Terminal.
También se puede instalar desde el código fuente con Homebrew o MacPorts.

![Icono de Voxy](assets/icons/Voxy.iconset/icon_128x128.png)

El icono identifica el ejecutable y acceso directo en Windows y `Voxy.app` en
macOS. El panel sigue abriéndose en la terminal del sistema.

## Reenvío de puertos — 0.6.0

Configuración persistente TCP/UDP para macOS y Windows, desde el panel o con
`ports add tcp 33033 33033 0.0.0.0`. Admite rangos de igual longitud, hasta
256 asignaciones por VM. Los cambios se aplican al apagar e iniciar Debian.

Consulta [comandos, ejemplos Tailscale y firewall](docs/PORTS.md).

## Versiones disponibles

| Plataforma | Último paquete publicado |
| --- | --- |
| Windows x64 | **0.6.0** — puertos TCP/UDP, icono y copias opcionales |
| macOS Intel y Apple Silicon | **0.6.0** — puertos TCP/UDP e icono en los DMG |
| Linux | Instalación desde el código fuente |

La versión 0.5.0 unifica el icono de Windows y macOS. Las copias de recuperación
descritas para Windows todavía no están implementadas en macOS ni Linux.

## Descargar para Mac

Versión experimental [v0.6.0](https://github.com/bunxdev/voxy/releases/tag/v0.6.0):

| Equipo | Descarga | Versión mínima de macOS |
| --- | --- | --- |
| Apple Silicon (ARM64) | [Voxy ARM64.dmg](https://github.com/bunxdev/voxy/releases/download/v0.6.0/Voxy-0.6.0-macos-arm64.dmg) | 15 |
| Intel (AMD64/x86_64) | [Voxy Intel.dmg](https://github.com/bunxdev/voxy/releases/download/v0.6.0/Voxy-0.6.0-macos-amd64.dmg) | 13 |

1. Descarga el paquete de tu arquitectura y abre el DMG.
2. Arrastra **Voxy.app** a **Aplicaciones** y ábrela.
3. En el panel de Terminal elige **Iniciar Debian**, después **Abrir terminal Debian**.
4. Usa **Apagar Debian** antes de apagar el equipo. Cerrar el panel deja la VM encendida.

No hace falta instalar Homebrew, MacPorts ni QEMU. La imagen inicial de Debian
está incluida y se verifica con SHA-256 antes de crear el disco. Los datos se guardan
en `~/Library/Application Support/Voxy/<arquitectura>`; reemplazar la aplicación
no elimina ese directorio. El repositorio y las descargas son públicos.

Los paquetes tienen **firma ad-hoc**, sin Developer ID ni notarización de Apple.
Si macOS bloquea la apertura, sigue el procedimiento por aplicación de
[Apple: Privacidad y seguridad → Abrir igualmente](https://support.apple.com/es-es/102445).
No se ha validado todavía una descarga con cuarentena en un Mac sin herramientas
de desarrollo. Las pruebas realizadas y sus límites están en [TESTING.md](TESTING.md).

Construcción y verificación: [packaging/macos/README.md](packaging/macos/README.md).

## Descargar para Windows x64

[ZIP portátil v0.6.0](https://github.com/bunxdev/voxy/releases/download/v0.6.0/Voxy-0.6.0-windows-x64.zip)
con `Voxy.exe`, QEMU 11.1.0, sus 104 DLL y Debian 12. No necesita Bash ni instalar
QEMU/OpenSSH. **Probado en Windows 10 Home 22H2 con WHPX**, además de las pruebas previas con Wine + TCG.
La versión 0.5.0 mantiene por defecto **solo una copia inicial al arrancar**. La opción 9
del menú permite activar copias periódicas y elegir de 1 a 10080 minutos.
Conserva tres copias independientes y las correcciones de terminal.

1. Extrae todo el ZIP en una carpeta estable y abre `Voxy.exe`.
   Ejecuta `Crear-acceso-directo.cmd` para añadir el icono de Voxy al escritorio.
2. Para WHPX, habilita **Plataforma de hipervisor de Windows** desde
   `optionalfeatures.exe` y reinicia si se solicita.
3. Para probar emulación por software, abre `Voxy-TCG.cmd`.
4. Ejecuta `Probar-Windows.cmd` (WHPX) o `Probar-TCG.cmd` y comparte el registro
   que se guarda en `%LOCALAPPDATA%\Voxy\prueba-whpx.log` o `prueba-tcg.log`.

Objetivo: Windows 10 2004+ / Windows 11 **x64**. ARM64 no está incluido. El ZIP
no tiene firma Authenticode de Voxy ni instalador MSI. Los datos habituales se
guardan en `%LOCALAPPDATA%\Voxy\amd64`; las pruebas usan discos separados.
Cerrar el panel deja Debian encendido; apágalo desde el menú.

[Instrucciones, construcción y límites](packaging/windows/README.md).

### Recuperación en Windows

Después de actualizar desde 0.3.x, cierra los paneles antiguos y apaga/inicia Debian
con la versión nueva. Las copias siguen funcionando al cerrar el panel o desconectar
SSH. El menú permite crear, listar y restaurar puntos con Debian apagado; conserva
el disco anterior al restaurar. Son copias locales de disco, sin RAM.

[Cómo funciona, espacio necesario y límites](docs/RECOVERY.md). macOS/Linux aún no
incorporan estas copias automáticas.

## Instalación en macOS desde código fuente

Clonar el repositorio:

```bash
git clone https://github.com/bunxdev/voxy.git
cd voxy
./scripts/install-macos.sh
./voxy init
./voxy start
./voxy wait
./voxy ssh
./voxy stop
```

El instalador usa Homebrew o MacPorts, verifica QEMU y comprueba disponibilidad
de HVF. La VM se inicia como usuario normal, sin sudo. Los submódulos no son
necesarios para ejecutar el launcher; quedan como referencia del código ARM.

- **Apple Silicon:** instalar Homebrew si falta, siguiendo [sus instrucciones](https://brew.sh/).
  El script actualiza Homebrew antes de instalar QEMU para evitar incompatibilidades
  entre una instalación antigua y las fórmulas nuevas.
- **Intel con Ventura 13 sin gestor instalado:** ejecutar primero
  `./scripts/bootstrap-macports-ventura.sh`, que descarga MacPorts 2.12.6,
  verifica su SHA256 e invoca el instalador oficial con sudo. Luego ejecutar
  `./scripts/install-macos.sh`. La sincronización inicial puede tardar varios minutos.
- Otros sistemas macOS necesitan un gestor compatible con su versión.
  No equivalen a plataformas probadas solo porque el script pueda ejecutarse.

Datos persistentes en `~/Library/Application Support/Voxy/arm64` o
`~/Library/Application Support/Voxy/amd64`. Incluyen disco, kernel, claves y logs;
no se guardan en el repositorio. SSH solo escucha en `127.0.0.1:22222`.
El invitado usa root con clave por instalación, sin contraseña predeterminada.

## Instalación en Linux Debian/Ubuntu

```bash
sudo apt-get install --no-install-recommends qemu-system-x86 qemu-utils \
  openssh-client curl ca-certificates xz-utils libdigest-sha-perl
# En ARM sustituir qemu-system-x86 por qemu-system-arm.
./voxy init
./voxy start
./voxy wait
./voxy ssh
./voxy stop
```

Datos locales en `.runtime/native-amd64` o `.runtime/native-arm64`.
Bash, tar y herramientas básicas del sistema también son necesarios.
`VOXY_ACCEL=tcg ./voxy start` selecciona emulación. Si KVM/HVF no funciona,
se informa el error; no se cambia silenciosamente a otro acelerador.

## Comandos y recursos

```bash
./voxy status
./voxy stop
./voxy resize 8G
./voxy start
./voxy wait
./voxy ssh df -h /
```

Valores predeterminados: **512 MiB de RAM, 1 CPU, disco de 1 GiB ampliable**.
El disco QCOW2 crece físicamente según se escriben datos. El crecimiento de ext4
ocurre al arrancar. Se rechaza ampliar una VM encendida y reducir su capacidad.

| Variable | Uso |
| --- | --- |
| `VOXY_ARCH` | `amd64` o `arm64`; por defecto arquitectura del anfitrión |
| `VOXY_DATA_DIR` | Directorio independiente para otra VM, pruebas o datos |
| `VOXY_ACCEL` | `auto`, `hvf`, `kvm` o `tcg` |
| `VOXY_RAM_MB` / `VOXY_CPUS` | RAM y CPU para el próximo arranque |
| `VOXY_SSH_PORT` | Puerto local; por defecto 22222 |

Mantener las variables de arquitectura/directorio en los comandos posteriores.
El puerto de la VM en ejecución se recuerda para SSH y apagado.
Las rutas con espacios están probadas; las rutas con comas se rechazan por la
sintaxis de opciones de QEMU. `./voxy-arm` es un alias que selecciona ARM64.

Si una actualización de APT cambia el kernel, ejecutar `./voxy sync-kernel`
**antes del siguiente apagado/reinicio**. Esto actualiza kernel e initramfs externos.
Para respaldo, apagar y copiar el directorio completo de datos.

## Pruebas

```bash
# El test requiere una copia nueva apagada de 1 GiB y la amplía a 2 GiB.
export VOXY_DATA_DIR="$HOME/voxy-test-nuevo"
./voxy init
./voxy test
```

Comprueba descarga por checksum durante init, arquitectura, Debian 12, SSH,
DNS/APT, ausencia de Docker, sincronización de kernel, apagado, integridad QCOW2,
ampliación, rechazo de reducción y persistencia con un nuevo identificador de arranque.
**La VM queda apagada si la prueba termina correctamente.** No borrar una VM
con datos para repetirla: usar un directorio nuevo.

Un bloqueo por directorio evita operaciones de control simultáneas. Si un proceso
se interrumpe abruptamente, revisar que haya terminado antes de retirar su
`control.lock`. Todavía falta recuperación automática y un supervisor QMP.

## Imágenes y compatibilidad anterior

- [Debian 12 ARM64 mínimo](https://github.com/bunxdev/qemu-debian-arm), v0.1.1,
  aproximadamente 97 MiB de descarga.
- [Debian 12 AMD64 mínimo](https://github.com/bunxdev/qemu-debian-amd), v0.1.0,
  aproximadamente 105 MiB de descarga.
- URLs y hashes SHA256 fijados en `images/*.lock`. Las imágenes no se guardan en Git.

La referencia Debian 13 cloud previa sigue disponible como **`./voxy-cloud`**
en Linux. Conserva sus archivos `.cache`, `.runtime/debian.qcow2` y `image.lock`.
No se convierten ni sobrescriben discos antiguos automáticamente. La antigua
copia ARM en `.runtime/arm64` también se conserva; sus scripts originales pueden
seguir usándose. El nuevo CLI utiliza directorios distintos.

## Checklist del proyecto

### 1. Base QEMU + Debian

- [x] Crear proyecto separado de Lupa.
- [x] Incorporar `qemu-debian-arm` como base ARM con versión fijada.
- [x] Descargar y verificar la Release ARM por SHA256.
- [x] Preparar referencia Debian 13 amd64 oficial con SHA512.
- [x] Añadir comandos de descarga, arranque, SSH y apagado.
- [x] Guardar discos y claves fuera de Git; publicar puertos en localhost.
- [x] Validar aquí el ciclo completo ARM: arranque, SSH, DNS, APT y apagado.
- [x] Validar aquí ampliación ARM, integridad QCOW2 y persistencia.
- [x] Validar aquí el ciclo amd64 con KVM y persistencia.
- [x] Construir imagen mínima amd64 equivalente a ARM, con inventario de paquetes: [qemu-debian-amd](https://github.com/bunxdev/qemu-debian-amd). Integrada en el CLI nativo.
- [ ] Automatizar la construcción limpia de ambas arquitecturas y fijar dependencias.
- [ ] Medir descarga, disco real, RAM y tiempos; reducir sin romper APT, SSH o red.

### 2. Administración de la VM

- [x] Unificar las bases mínimas ARM64/AMD64 en un CLI con selección de arquitectura.
- [x] Seleccionar KVM/HVF/TCG en el launcher macOS/Linux; probar KVM AMD64 y HVF AMD64/ARM64.
- [x] Validar WHPX en Windows 10 x64.
- [ ] Validar KVM en ARM64 nativo.
- [x] Añadir QMP local Windows, bloqueo liberado al morir y procesos independientes de la sesión SSH.
- [ ] Extender QMP a macOS/Linux y completar manejo de puertos ocupados y otros fallos.
- [ ] Añadir configuración persistente de CPU, RAM, discos y puertos.
- [x] Implementar copias automáticas, retención y restauración reanudable en Windows x64.
- [ ] Extender la recuperación a macOS/Linux, importación y actualización con recuperación.
- [ ] Diseñar separación del sistema base y los datos de usuario.
- [ ] Resolver carpetas compartidas, permisos, reloj y suspensión del anfitrión.

### 3. Multiplataforma

- [ ] Probar Linux x86_64 y ARM64 en hardware nativo.
- [x] Implementar y probar Apple Silicon con HVF.
- [x] Completar la prueba macOS Intel con HVF.
- [x] Implementar y probar Windows 10 x64 con WHPX.
- [ ] Implementar y probar Windows ARM64 y ampliar la matriz a Windows 11.
- [x] Crear CLI con estado de VM, SSH, recursos y errores de operaciones.
- [ ] Crear interfaz gráfica para administrar la VM.
- [x] Empaquetar `.dmg` macOS Intel y Apple Silicon.
- [x] Empaquetar Windows x64 con `Voxy.exe` en ZIP portátil.
- [ ] Empaquetar Linux y crear instalador Windows.
- [ ] Revisar distribución/licencias de QEMU y Debian, firmas y notarización.
- [ ] CI y releases por plataforma, checksums y pruebas desde instalación limpia.

### 4. Aplicaciones, después de completar la base

- [ ] Instalar Docker Engine opcional dentro de Debian y comprobar red/volúmenes.
- [ ] Integrar Lupa slim y validar sus funciones completas sin regresiones.
- [ ] Construir/probar imagen Lupa ARM64; la imagen actual publicada es amd64.
- [ ] Conectar escritorio, API, CDP y MCP mediante puertos locales autenticados.
- [ ] Evaluar terminal tipo BunTTY y panel de tareas, permisos y licencias.
- [ ] Probar actualización, desinstalación y conservación de perfiles/descargas.

## Relación con Lupa

El estado del navegador, MCP, Docker slim y decisiones anteriores se guarda en
[Lupa, rama lupax](https://github.com/bunxdev/lupa/tree/lupax), particularmente
[`docs/LUPAX.md`](https://github.com/bunxdev/lupa/blob/lupax/docs/LUPAX.md).
No se han trasladado todavía sus servicios a esta VM.

## Fuentes

- [Código y documentación de la base ARM](https://github.com/bunxdev/qemu-debian-arm).
- [Imágenes oficiales Debian Cloud](https://cloud.debian.org/images/cloud/).
- [Opciones y aceleradores de QEMU](https://www.qemu.org/docs/master/system/invocation.html).

Si se usa Python en futuras herramientas del constructor, se ejecutará con `uv`.

## Checklist detallado: Apple y Windows

Las casillas marcadas reflejan pruebas realizadas; el detalle está en TESTING.md.
Los aceleradores y opciones deben validarse contra la versión concreta de QEMU
que distribuyamos, no solo contra la documentación de desarrollo.

### Base compartida

- [ ] Elegir versiones mínimas del sistema anfitrión y fijar una versión de QEMU.
- [x] Detectar SO/arquitectura y descargar el invitado correspondiente en macOS/Linux:
  ARM64 para Apple Silicon/Linux ARM; AMD64 para Mac Intel/Linux x86_64.
- [ ] Añadir la detección correspondiente al launcher Windows.
- [x] Usar la base mínima [qemu-debian-amd](https://github.com/bunxdev/qemu-debian-amd)
  en lugar de la referencia cloud amd64, manteniendo la base ARM existente.
- [ ] Reemplazar dependencias de Bash, `/proc`, `flock`, señales POSIX y sockets
  Unix por un supervisor portable con QMP y gestión de procesos propia.
- [ ] Desacoplar el inicio de SSH del uso de OpenSSH del anfitrión o distribuir
  un cliente probado; proteger claves con permisos/ACL correctos por sistema.
- [ ] Separar recursos de instalación de discos/configuración en el directorio
  de datos del usuario; probar rutas con espacios, Unicode y usuarios sin privilegios.
- [ ] Verificar descarga, firma/checksum, instalación interrumpida y recuperación.
- [ ] Probar red NAT, DNS, VPN/proxy y puertos locales ocupados sin exponer servicios.
- [ ] Gestionar apagado, cierre del launcher, suspensión/reanudación y actualizaciones
  del anfitrión; nunca modificar o ampliar un disco abierto por QEMU.

### Apple: macOS Intel y Apple Silicon

- [x] Instalar y verificar QEMU y dependencias nativas en las Macs Intel y M1.
- [x] Distribuir QEMU y sus dependencias dentro del paquete de Voxy para evitar compilar en el equipo del usuario.
- [x] Adaptar la base ARM para HVF con CPU host y máquina virt; probado en M1.
- [x] Completar validación HVF AMD64 con máquina q35 en Mac Intel.
- [x] Validar arranque directo del kernel, `fw_cfg`, virtio, red y crecimiento ext4
  bajo HVF en ambas Macs.
- [x] Configurar y probar permisos de Hypervisor y firma ad-hoc del ejecutable QEMU
  y sus dependencias dentro del paquete de la aplicación.
- [x] Guardar datos en `~/Library/Application Support/Voxy`, con claves privadas
  y rutas con espacios probadas en Intel y M1.
- [ ] Resolver permisos de carpetas compartidas seleccionadas por el usuario.
- [x] Crear `.app` y `.dmg` separados para Intel y Apple Silicon.
- [ ] Firmar, notarizar y adjuntar el ticket; verificar instalación con Gatekeeper
  desde una descarga nueva en un Mac sin herramientas de desarrollo.
- [x] Probar Apple Silicon y Mac Intel: primer inicio, SSH, APT, sincronización del kernel, ampliación, persistencia y apagado.
- [ ] Probar backup/restauración, actualización, suspensión y desinstalación conservando datos en ambas Macs.

### Windows x64 y ARM64

- [x] Preparar QEMU x64, `qemu-img` y DLL transitivas, con versiones y hashes fijados.
- [x] Crear `Voxy.exe` nativo con SSH integrado, sin dependencia de Bash/OpenSSH.
- [x] Implementar diagnóstico WHPX y selección explícita de TCG; mostrar cómo
  habilitar la característica de Windows y reiniciar.
- [x] Adaptar argumentos, rutas con espacios, PID/tiempo de creación, procesos
  independientes de la consola y actualización del kernel tras liberar archivos.
- [x] Guardar discos/configuración en `%LOCALAPPDATA%\Voxy` y configurar ACL privadas.
- [x] Crear ZIP portátil x64 y scripts para que el usuario ejecute pruebas separadas.
- [x] Probar el ciclo completo del ejecutable Windows con Wine + TCG, incluyendo
  rutas con espacios/acentos y tratamiento de archivos bloqueados.
- [x] Validar WHPX y ciclo completo en Windows 10 Home 22H2 x64.
- [x] Comprobar ACL de datos/claves (usuario y SYSTEM), SSH en loopback y operación con Defender activo.
- [x] Corregir y probar ANSI, pegado, flechas, Ctrl+C y cambio de tamaño de terminal en Windows real.
- [ ] Repetir el ciclo WHPX completo como usuario estándar: la prueba remota usó elevación; se observó la VM TCG habitual sin elevación.
- [ ] Validar Windows 11 y otras versiones del anfitrión.
- [ ] Preparar paquete Windows ARM64 y comprobar aceleración/build en hardware real.
- [ ] Evaluar una compilación QEMU Windows sin interfaces gráficas para reducir DLL
  y tamaño, manteniendo las funciones de la configuración de Voxy.
- [ ] Probar Windows Firewall, Defender, VPN y coexistencia con WSL2/Hyper-V;
  mantener SSH/API/CDP en loopback salvo configuración explícita del usuario.
- [x] Copia inicial en Windows; copias periódicas opcionales con intervalo configurable, retención de tres puntos y restauración con diario.
- [x] Probar cierre brusco de QEMU, interrupción de copia y desconexión SSH en una VM separada.
- [ ] Probar corte eléctrico real, suspensión/reanudación y recuperación en más equipos.
- [ ] Crear instalador `.exe` o `.msi`, firma de código, actualización y desinstalación
  con opción clara para conservar o borrar los discos del usuario.
- [ ] Probar x64 y ARM64 reales desde Windows limpio: primer arranque, SSH, APT,
  disco ampliable, persistencia, backup/restauración y suspensión/reanudación.

### Criterio para declarar soporte

- [x] Publicar matriz de versiones de SO, CPU, QEMU y acelerador realmente probados.
- [ ] Automatizar pruebas por plataforma y archivar resultados de hardware real
  cuando la CI no tenga virtualización disponible.
- [ ] Con QEMU + Debian estable, repetir la matriz incorporando Docker y todas
  las funciones de Lupa: escritorio, perfil, descargas, audio, CDP y MCP.

Referencias: [aceleradores QEMU](https://www.qemu.org/docs/master/system/introduction.html)
y [plataformas de construcción](https://www.qemu.org/docs/master/about/build-platforms.html).
