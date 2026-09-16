# Voxy

QEMU + Debian mínimo como base independiente para ejecutar Lupa y otras
aplicaciones Linux. **La etapa actual no instala Docker ni Lupa en la VM.**

El launcher selecciona AMD64 para Intel/AMD y ARM64 para Apple Silicon/ARM.
En macOS usa HVF, en Linux KVM cuando está disponible y TCG como alternativa
explícita. Las pruebas reales y sus límites están en [TESTING.md](TESTING.md).
Windows aún necesita un launcher nativo y pruebas.

## Instalación en macOS

Clonar este repositorio privado usando una cuenta con acceso:

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
- [x] Seleccionar KVM/HVF/TCG en el launcher macOS/Linux; probar KVM amd64 y HVF ARM64.
- [ ] Validar WHPX en Windows y KVM en ARM64 nativo.
- [ ] Añadir control QMP y manejo robusto de fallos, procesos y puertos ocupados.
- [ ] Añadir configuración persistente de CPU, RAM, discos y puertos.
- [ ] Implementar backup/restauración, importación y actualización con recuperación.
- [ ] Diseñar separación del sistema base y los datos de usuario.
- [ ] Resolver carpetas compartidas, permisos, reloj y suspensión del anfitrión.

### 3. Multiplataforma

- [ ] Probar Linux x86_64 y ARM64 en hardware nativo.
- [x] Implementar y probar Apple Silicon con HVF.
- [ ] Completar la prueba macOS Intel con HVF.
- [ ] Implementar y probar Windows x64 y ARM64, verificando aceleración disponible.
- [x] Crear CLI con estado de VM, SSH, recursos y errores de operaciones.
- [ ] Crear interfaz gráfica para administrar la VM.
- [ ] Empaquetar Linux, `.exe`/instalador Windows y `.dmg` macOS.
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

- [ ] Preparar binarios QEMU y dependencias para `x86_64` y `arm64`.
- [x] Adaptar la base ARM para HVF con CPU host y máquina virt; probado en M1.
- [ ] Completar validación HVF AMD64 con máquina q35 en Mac Intel.
- [ ] Validar arranque directo del kernel, `fw_cfg`, virtio, red y crecimiento ext4
  bajo HVF; no asumir que la configuración TCG funciona sin cambios.
- [ ] Configurar y probar permisos de Hypervisor y firma del ejecutable QEMU
  y sus dependencias dentro del paquete de la aplicación.
- [x] Guardar datos en `~/Library/Application Support/Voxy`, con claves privadas
  y rutas con espacios probadas en M1.
- [ ] Resolver permisos de carpetas compartidas seleccionadas por el usuario.
- [ ] Crear `.app` y `.dmg` para ambas arquitecturas o un paquete universal probado.
- [ ] Firmar, notarizar y adjuntar el ticket; verificar instalación con Gatekeeper
  desde una descarga nueva en un Mac sin herramientas de desarrollo.
- [ ] Probar Apple Silicon y Mac Intel reales: primer inicio, SSH, APT, persistencia,
  backup/restauración, actualización, suspensión y desinstalación conservando datos.

### Windows x64 y ARM64

- [ ] Preparar QEMU y todas sus DLL para cada arquitectura de Windows soportada.
- [ ] Detectar virtualización del procesador y disponibilidad de Windows Hypervisor
  Platform; explicar cómo habilitarla y si requiere reiniciar, antes de iniciar la VM.
- [ ] Implementar y probar WHPX en x64; comprobar por separado soporte ARM64
  de la versión/build distribuida y del dispositivo real. TCG será una opción explícita.
- [ ] Adaptar argumentos, rutas, pipes/canales de control, manejo de procesos y
  archivos bloqueados; los scripts `.sh` actuales no son un launcher Windows nativo.
- [ ] Guardar discos/configuración en `%LOCALAPPDATA%\Voxy`, aplicar ACL a claves
  y probar ejecución sin administrador después de instalar los prerrequisitos.
- [ ] Probar Windows Firewall, Defender, VPN y coexistencia con WSL2/Hyper-V;
  mantener SSH/API/CDP en loopback salvo configuración explícita del usuario.
- [ ] Crear instalador `.exe` o `.msi`, firma de código, actualización y desinstalación
  con opción clara para conservar o borrar los discos del usuario.
- [ ] Probar x64 y ARM64 reales desde Windows limpio: primer arranque, SSH, APT,
  disco ampliable, persistencia, backup/restauración y suspensión/reanudación.

### Criterio para declarar soporte

- [ ] Publicar matriz de versiones de SO, CPU, QEMU y acelerador realmente probados.
- [ ] Automatizar pruebas por plataforma y archivar resultados de hardware real
  cuando la CI no tenga virtualización disponible.
- [ ] Con QEMU + Debian estable, repetir la matriz incorporando Docker y todas
  las funciones de Lupa: escritorio, perfil, descargas, audio, CDP y MCP.

Referencias: [aceleradores QEMU](https://www.qemu.org/docs/master/system/introduction.html)
y [plataformas de construcción](https://www.qemu.org/docs/master/about/build-platforms.html).
