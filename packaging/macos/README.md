# Paquetes macOS

`Voxy.app` abre un panel en Terminal mediante `open -a Terminal`; no necesita
controlar Terminal por Apple Events. Las opciones permiten iniciar Debian, abrir
SSH, apagar y cerrar el panel. La aplicación incluye la CLI `Contents/Resources/voxy`.
No incluye Docker ni Lupa todavía.

## Construir

En cada arquitectura nativa, instalar las Command Line Tools de Apple y QEMU
mediante [el instalador existente](../../scripts/README.md). Los paquetes publicados
usan QEMU 11.1.1 de Homebrew en M1/macOS 15.5 y MacPorts en Intel/macOS 13.6.9.
Después, desde la raíz del repositorio:

```sh
bash scripts/build-dmg.sh
bash scripts/test-dmg.sh dist/Voxy-0.5.0-macos-arm64.dmg  # o amd64
```

La construcción requiere Internet y rechaza sobrescribir su directorio de trabajo.
Para reconstruir, mueve o elimina explícitamente `dist/macos-<arquitectura>-0.5.0` y el
DMG anterior. Se genera un paquete nativo por Mac; no es un binario universal.
Las dependencias proceden del gestor instalado: no es una construcción reproducible
bit a bit ni una promesa de compatibilidad con otras versiones de esos paquetes.

El script:

- Compila el launcher AppleScript como `.app`.
- Convierte `assets/icons/Voxy.iconset` con `iconutil` y registra `Voxy.icns`
  en `CFBundleIconFile`. La versión se toma de `VERSION`.
- El icono identifica Voxy en Finder; el panel abierto continúa usando Terminal.
- Descarga la imagen limpia fijada en `images/*.lock` y comprueba SHA-256.
- Copia únicamente el emulador nativo, `qemu-img` y sus bibliotecas transitivas.
- Reescribe referencias de bibliotecas relativas al paquete con `install_name_tool`.
- Incluye el firmware necesario de PC; ARM usa arranque directo sin firmware adicional.
- Firma bibliotecas, ejecutables y aplicación ad-hoc. QEMU recibe el entitlement
  `com.apple.security.hypervisor` para HVF.
- Incluye avisos de licencias y procedencia en `Contents/Resources/licenses`, y
  referencias originales de bibliotecas en `dependencies.tsv`.
- Genera un DMG comprimido con acceso directo a Aplicaciones.

No se copian discos usados, claves SSH, credenciales de GitHub ni datos del usuario.
La imagen comprimida incluida se verifica otra vez en el primer inicio. La CLI
prioriza los binarios de la aplicación y usa su directorio de firmware con `-L`.

## Pruebas

`test-dmg.sh` verifica y monta el DMG, copia la aplicación a una ruta con espacios,
lo desmonta, comprueba firmas y referencias dinámicas, y crea una VM nueva en un
directorio temporal bajo el HOME. Usa puerto SSH 22333; debe estar disponible.
Comprueba inicialización con un proxy inválido para asegurar que se usa la imagen
incluida. El ciclo posterior prueba HVF, SSH, DNS, APT, kernel, ampliación de 1 a
2 GiB, rechazo de reducción, persistencia, integridad qcow2 y apagado.

La VM habitual del usuario no se modifica. Los datos y registros de prueba se
conservan en el directorio que imprime el script; al terminar correctamente la
VM queda apagada. El test automatizado comprueba también la opción de cerrar el
panel. La apertura de `.app` se comprueba aparte en una sesión gráfica activa.

Para usar la CLI después de instalar:

```sh
"/Applications/Voxy.app/Contents/Resources/voxy" status
"/Applications/Voxy.app/Contents/Resources/voxy" ssh
"/Applications/Voxy.app/Contents/Resources/voxy" stop
```

## Distribución pendiente

La firma ad-hoc permite verificar integridad y probar HVF, pero no identifica a un
editor reconocido por Apple. Faltan Developer ID, notarización, ticket y validación
de Gatekeeper en una descarga nueva sobre un equipo limpio. No se han desactivado
las protecciones del sistema en las Macs de prueba.

Compatibilidad actual declarada: ARM64 macOS 15+, Intel macOS 13+. Solo se probaron
las versiones concretas indicadas arriba. Actualización automática, GUI dedicada,
instalación Docker/Lupa, suspensión y recuperación siguen pendientes.

## Reenvío de puertos en 0.6.0

Opción 5 del panel, o `voxy ports add tcp 33033 33033 0.0.0.0`.
El paquete incluye `lib/ports.sh` y `lib/ports.awk` y usa Bash/awk del sistema.
[Guía de TCP/UDP, rangos, Tailscale y firewall](../../docs/PORTS.md).
