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
