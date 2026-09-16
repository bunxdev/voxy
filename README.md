# Voxy

Base de máquina virtual pequeña para ejecutar Debian en QEMU. El objetivo futuro
es llevar Lupa y otras aplicaciones Linux a Linux, macOS y Windows mediante una
VM administrada por Voxy. **Esta primera etapa incluye solamente QEMU + Debian.**

## Base elegida

Reutilizamos [bunxdev/qemu-debian-arm](https://github.com/bunxdev/qemu-debian-arm)
como submódulo fijado a un commit en `upstream/qemu-debian-arm`. Su Release v0.1.1
contiene Debian 12 ARM64, kernel e initramfs, disco inicial de 1 GiB ampliable y
SSH por clave. La descarga comprimida ocupa unos **97 MiB**. El script de descarga
verifica el SHA256 fijado antes de extraer. No requiere cloud-init ni firmware UEFI.

Además mantenemos una **referencia amd64** con Debian 13 genericcloud oficial,
checksum SHA512 y versión fijados en `image.lock`: unos 326 MiB de descarga,
768 MiB de RAM, una CPU y 8 GiB de capacidad virtual mediante overlay QCOW2.
Sirve para validar KVM en este anfitrión x86_64. No es todavía una construcción
amd64 equivalente a la imagen ARM mínima; contiene cloud-init y más paquetes.

Los tamaños de descarga, disco ocupado y capacidad virtual son medidas distintas.
Ninguna imagen de VM, clave o disco de usuario se guarda en Git.

## Empezar en Linux Debian/Ubuntu

```bash
sudo apt-get update
sudo apt-get install --no-install-recommends git ca-certificates curl xz-utils \
  openssh-client qemu-system-arm qemu-utils

git clone --recurse-submodules https://github.com/bunxdev/voxy.git
cd voxy
./voxy-arm init
./voxy-arm start
# Esperar a que Debian termine de arrancar, especialmente bajo emulación TCG.
./voxy-arm ssh
./voxy-arm stop
```

Si ya clonaste sin submódulos: `git submodule update --init --recursive`.
El repo es privado inicialmente; se necesita acceso a la cuenta correspondiente.
ARM utiliza **TCG**, incluso en anfitriones ARM, porque así está implementado
actualmente el proyecto base. SSH se publica en `127.0.0.1:22223` y el reenvío
HTTP opcional en `127.0.0.1:28080`. No hay servidor web instalado por defecto.

```bash
# Con la VM apagada, aumentar capacidad total (no reduce discos):
./voxy-arm resize 8G
./voxy-arm start
./voxy-arm ssh df -h /
```

La VM ARM usa 512 MiB de RAM y una CPU por defecto. Se pueden pasar
`VM_RAM_MB`, `VM_CPUS`, `VM_SSH_PORT` y `VM_HTTP_PORT`; mantener los mismos puertos
en los comandos posteriores. Los datos están en `.runtime/arm64`.
Para actualizar kernel, seguir el procedimiento del README upstream y ejecutar
`./voxy-arm sync-kernel` antes de apagar y arrancar con el kernel actualizado.

### Referencia x86_64 con KVM

```bash
sudo apt-get install --no-install-recommends qemu-system-x86 cloud-image-utils
./voxy init
./voxy start
./voxy wait
./voxy ssh
./voxy status
./voxy stop
```

Requiere Bash, `flock`, herramientas GNU, Linux x86_64 y acceso funcional a
`/dev/kvm`. La virtualización anidada depende del anfitrión. Si no hay KVM,
`VOXY_ACCEL=tcg ./voxy start` selecciona emulación explícita. Esta alternativa
TCG amd64 aún no está validada. SSH solo en `127.0.0.1:22222`, usuario `voxy`,
sudo y autenticación por clave generada localmente. No hay contraseña predeterminada.
La base y el overlay tienen dependencia de ruta: no mover `.cache` ni `.runtime`
ni copiar únicamente el overlay. Para respaldar, apagar y conservar ambos.

## Pruebas

Ejecutar una VM a la vez:

```bash
# ARM: requiere copia nueva apagada de 1 GiB; la amplía a 2 GiB.
./voxy-arm test
./voxy-arm stop

# amd64: comprueba Debian, SSH, DNS y archivo persistente tras apagar y arrancar.
./voxy test
./voxy stop
```

El test ARM conserva registros en `.runtime/arm64/test-logs.*/` y no se repite
sobre la misma copia ya ampliada. No borres una VM que contenga datos para repetir
pruebas: prepara otra copia siguiendo el README upstream.
Ambas pruebas dejan la VM encendida cuando terminan correctamente.
Resultados y límites: [TESTING.md](TESTING.md).

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
- [ ] Construir imagen mínima amd64 equivalente a ARM, con inventario de paquetes.
- [ ] Automatizar la construcción limpia de ambas arquitecturas y fijar dependencias.
- [ ] Medir descarga, disco real, RAM y tiempos; reducir sin romper APT, SSH o red.

### 2. Administración de la VM

- [ ] Unificar ambos prototipos en un CLI con selección de arquitectura.
- [ ] Validar y seleccionar KVM, HVF y WHPX según anfitrión/arquitectura.
- [ ] Añadir control QMP y manejo robusto de fallos, procesos y puertos ocupados.
- [ ] Añadir configuración persistente de CPU, RAM, discos y puertos.
- [ ] Implementar backup/restauración, importación y actualización con recuperación.
- [ ] Diseñar separación del sistema base y los datos de usuario.
- [ ] Resolver carpetas compartidas, permisos, reloj y suspensión del anfitrión.

### 3. Multiplataforma

- [ ] Probar Linux x86_64 y ARM64 en hardware nativo.
- [ ] Implementar y probar macOS Intel y Apple Silicon con HVF.
- [ ] Implementar y probar Windows x64 y ARM64, verificando aceleración disponible.
- [ ] Crear launcher con estado de VM, consola, recursos y errores comprensibles.
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
