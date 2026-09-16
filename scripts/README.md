# Preparación del anfitrión macOS

`install-macos.sh` instala QEMU si falta usando un gestor ya disponible y comprueba
los ejecutables y HVF. No instala Docker y no necesita sudo para iniciar Debian.

## Homebrew (probado en M1, macOS 15.5)

El script ejecuta `brew update` antes de instalar QEMU. Durante la prueba, una
versión antigua de Homebrew no reconocía `overwrite` en la fórmula de OpenSSL.
La recuperación fue:

```bash
brew update
brew postinstall openssl@3
```

No se ejecutó una actualización general de todas las aplicaciones con `brew upgrade`.
Las dependencias necesarias para QEMU sí se instalaron/actualizaron.

## MacPorts (preparación de Intel Ventura)

`bootstrap-macports-ventura.sh` instala la versión oficial 2.12.6 para macOS 13 Intel,
con checksum SHA256 fijado. Requiere acceso administrativo y Command Line Tools.
La sincronización del catálogo puede tardar y recurrir a servidores alternativos.

La selección de QEMU para Voxy desactiva Cocoa, curses, SPICE, VNC y USB, y conserva
el objetivo de CPU nativo. Esto no elimina HVF ni la red NAT de la VM. El uso actual
de Voxy es por SSH; no necesita el escritorio de QEMU.

Cuando no hay paquetes precompilados para el sistema, MacPorts instala también
herramientas de construcción y compila dependencias. El espacio del anfitrión no
es igual al tamaño de descarga de Debian. Las dependencias Python que instale
MacPorts pertenecen a ese gestor; Voxy no ejecuta scripts propios en Python.

Si se empezó manualmente con otras variantes y se quieren usar las de Voxy,
MacPorts puede pedir limpiar su construcción parcial:

```bash
sudo port clean qemu
./scripts/install-macos.sh
```

Este comando limpia la construcción de QEMU, no el disco Debian de Voxy. No se
limpian automáticamente compilaciones ni paquetes de otras aplicaciones.

Referencias: [Homebrew](https://docs.brew.sh/Installation),
[MacPorts](https://www.macports.org/install.php).

### QEMU 11.1.1: configuración sin descargas externas

Durante la prueba de Ventura, el configurador de QEMU quedó esperando en su
instalación de herramientas desde PyPI. El código fuente ya incluye la rueda
`qemu_qmp-0.0.6`; faltaban herramientas del intérprete usado por el port.

El instalador de Voxy prepara `py314-pip` y `py314-wheel` mediante MacPorts y pasa
`configure.pre_args=--prefix=/opt/local --disable-download` al port de QEMU
(adaptando el prefijo si es distinto). QEMU usa así sus ruedas incluidas y los
paquetes proporcionados por MacPorts. No se desactiva el sandbox del gestor ni
se instala con pip sobre un Python global fuera de su control.

Este ajuste corresponde a QEMU 11.1.1 y al port que utiliza Python 3.14; si MacPorts
cambia su intérprete de construcción, habrá que revisar esos nombres de paquetes.

## Construir instaladores DMG

Para usuarios finales están disponibles los paquetes en
[Releases](https://github.com/bunxdev/voxy/releases/tag/v0.2.0); incluyen QEMU y Debian.
Para generar y comprobar esos paquetes desde macOS, consulta
[packaging/macos/README.md](../packaging/macos/README.md) y los scripts
`build-dmg.sh` / `test-dmg.sh`.

## Windows x64 y Wine

- `build-windows.sh`: compilación cruzada del launcher Go, QEMU/DLL con hashes
  fijados, imagen Debian y ZIP portátil.
- `test-wine.sh /ruta/al/ZIP`: extracción, verificación de manifiesto y ciclo
  completo usando un prefijo Wine x64 aislado y TCG. Requiere Wine y Xvfb si no hay DISPLAY.

Consulta [la guía Windows](../packaging/windows/README.md). Las pruebas con Wine
no sustituyen la validación WHPX, permisos y seguridad en Windows real.
