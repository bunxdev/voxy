# Validación local — 2026-09-16

Anfitrión: Debian 13 Linux x86_64, QEMU 10.0.13. Se ejecutan las VMs
secuencialmente para limitar el consumo. No son pruebas de Windows, macOS o Android.

## Debian 13 amd64 de referencia

`./voxy test` terminó con código 0 y:

```
PASS: Debian 13, SSH, DNS y persistencia tras reinicio
```

- KVM funcionó dentro de este anfitrión virtualizado.
- Una CPU, 768 MiB de RAM configurada; Debian reportó 715 MiB totales,
  210 MiB usados y 505 MiB disponibles en esa medición.
- Disco raíz: 7,7 GiB de sistema de archivos, 596 MiB usados.
- Base descargada: aproximadamente 326 MiB; overlay después de la prueba: 26 MiB.
- SHA512 de la base verificado; SSH por clave, cloud-init finalizado,
  Debian 13/x86_64 y DNS comprobados; Docker ausente.
- Modo de arranque `multi-user.target` configurado explícitamente. La imagen
  cloud original declaraba `graphical.target` aunque no incluía escritorio.
- Apagado y arranque realizados; archivo de prueba conservado.
- Se corrigió la herencia del descriptor del bloqueo hacia QEMU para permitir
  ejecutar comandos posteriores mientras el proceso VM continúa.
- Apagada al terminar para iniciar la prueba ARM.

## Base ARM

Submódulo `qemu-debian-arm`: `24197335d1098d1e002fe525e5b4cf0b225ebf2f`.
Release v0.1.1, SHA256:

```
64438786232688a0b68faf83f2f8014b57688c68759505fc63d8641cbdf44375
```

`./voxy-arm test` terminó con **ALL TESTS PASSED**, código **0**.

- Debian 12 ARM64 sin Docker; SSH, DNS y actualización de índices APT correctos.
- Kernel e initramfs sincronizados con hashes idénticos.
- Ampliación con VM encendida y reducción del disco rechazadas.
- Apagado, integridad QCOW2 y ampliación de 1 a 2 GiB correctos.
- Segundo arranque, crecimiento de ext4 y persistencia del archivo comprobados.
- SSH, preparación y red activos; ninguna unidad systemd fallida.
- 512 MiB configurados; el invitado reportó 478 MiB totales, **43 MiB usados**
  y 435 MiB disponibles en la medición final.
- Sistema de archivos final: 2 GiB, **272 MiB usados**, 1,7 GiB disponibles.
- Se solicitó el apagado al terminar para liberar recursos.

Registro completo local: `.runtime/arm64/test-logs.FjNwYw/results.log`.
El registro no se versiona; estos resultados resumen esta ejecución concreta.
Los consumos medidos son del invitado y no equivalen al RSS del proceso QEMU.

## Límites de la primera validación Linux

No se ha probado HVF, WHPX ni aceleración ARM nativa. La variante ARM utiliza
TCG y puede tardar minutos. La referencia cloud amd64 no es una imagen mínima
construida a partir de la receta ARM. No se ha instalado Docker ni Lupa dentro
de ninguna de estas nuevas VMs. Los resultados anteriores del proyecto ARM
pertenecen a su documentación upstream; este informe distingue las pruebas locales.

## Launcher nativo macOS/Linux — actualización

Las pruebas anteriores corresponden al prototipo `voxy` original (ahora
`voxy-cloud`) y a `voxy-arm` original. El nuevo `voxy` usa directamente las bases
mínimas Debian 12 en las dos arquitecturas. No migra las VMs previas.

### Mac mini Apple M1, macOS 15.5

- QEMU 11.1.1 instalado con Homebrew, arquitectura ARM64 y aceleración **HVF**.
- Homebrew necesitó actualizarse: la fórmula nueva de OpenSSL usaba una opción
  desconocida para su versión anterior. `brew update` y `brew postinstall openssl@3`
  resolvieron el fallo. El instalador de Voxy ahora actualiza Homebrew antes de instalar.
- Imagen ARM v0.1.1 descargada desde GitHub y verificada por SHA256.
- Datos en `~/Library/Application Support/Voxy/arm64`: la ruta con espacios funciona.
- `./voxy test` pasó arranque, SSH, DNS/APT, ampliación de 1 a 2 GiB,
  rechazo de reducción, apagado, integridad QCOW2 y persistencia tras nuevo arranque.
- 512 MiB configurados; Debian informó 478 MiB totales, **41 MiB usados**,
  437 MiB disponibles. Disco: **272 MiB usados** después de actualizar índices APT.
- Prueba adicional del comando `sync-kernel`: hashes idénticos antes/después,
  otro arranque y archivo persistente correctos. VM apagada al finalizar.
- No se probó suspensión del Mac, instalación gráfica, firma/notarización de Voxy,
  carpetas compartidas, Docker o Lupa dentro de la VM.

### Regresión Linux x86_64

El launcher nuevo completó `./voxy init` y `./voxy test` en Debian 13 con
QEMU 10.0.13 y KVM: todos los pasos, incluida sincronización de kernel, pasaron.
Medición final: 57 MiB de RAM usados en el invitado y 276 MiB de disco ocupado.
VM apagada al terminar. Estos valores varían y no equivalen al RSS de QEMU.

### MacBook Pro Intel i5-8257U, macOS 13.6.9

- QEMU **11.1.1**, construido por MacPorts **2.12.6**, objetivo x86_64 y **HVF**.
- MacPorts para Ventura se descargó con SHA256 fijado. Algunas dependencias
  (GLib) y QEMU requirieron compilación local, además de Clang/LLVM 17. La instalación
  inicial fue considerablemente más lenta que usar el paquete Homebrew de la M1.
- QEMU usa la variante de objetivo x86_64 sin Cocoa, curses, SPICE, VNC o USB.
  El gestor informó instalación correcta y ninguna biblioteca/puerto roto.
- Se resolvió una espera en la descarga de herramientas de construcción:
  `py314-pip` y `py314-wheel` se instalaron mediante MacPorts y QEMU se configuró
  con `--disable-download`, usando sus ruedas incluidas. Ajuste incorporado al instalador.
- Se fijó `LC_ALL=C` en los scripts para evitar avisos de locale heredado por SSH.
- Imagen AMD64 v0.1.0 descargada de GitHub y verificada por SHA256.
- Instalación bajo el usuario normal, datos en
  `~/Library/Application Support/Voxy/amd64`; ruta con espacios comprobada.
- `./scripts/install-macos.sh` y `./voxy test` terminaron con código **0**:

```
PASS: Darwin amd64 hvf; SSH, APT, disco y persistencia
Voxy amd64 apagado
```

- Pasaron arranque con HVF, Debian 12/x86_64, SSH por clave, DNS/APT,
  sincronización de kernel con hashes idénticos, rechazo de resize encendido,
  apagado, integridad QCOW2, ampliación 1→2 GiB, rechazo de reducción y persistencia
  después de un arranque con identificador nuevo. Ninguna unidad systemd fallida.
- 512 MiB configurados; Debian informó 470 MiB totales, **53 MiB usados**,
  416 MiB disponibles. Disco final: **276 MiB usados**, aproximadamente 1,7 GiB libres.
- VM apagada al terminar; no se instaló Docker ni Lupa en ella.

### Estado entregado en ambas Macs

Código instalado en `~/voxy`, con historial Git del repositorio. Se transfirió
mediante Git bundle por SSH para no copiar credenciales de GitHub a las Macs.
Los discos de prueba ampliados a 2 GiB se conservan en el directorio de datos,
separados del código. Se puede iniciar con `./voxy start`, esperar con
`./voxy wait` y entrar mediante `./voxy ssh`.

El soporte comprobado es el CLI con QEMU + Debian en estos equipos y versiones.
Siguen pendientes el paquete gráfico firmado/notarizado, actualización y recuperación,
suspensión del anfitrión, carpetas compartidas, Windows y la integración de Lupa.

## DMG v0.2.0 — Intel y Apple Silicon

Se construyeron paquetes independientes en las dos Macs. Cada DMG incluye una
`Voxy.app` con panel de Terminal, el QEMU nativo 11.1.1, `qemu-img`, las bibliotecas
transitivas y la imagen limpia de Debian 12 fijada por SHA-256. No incluye Docker
ni Lupa. ARM declara macOS 15+ e Intel macOS 13+; los comandos `LC_BUILD_VERSION`
de sus binarios y bibliotecas indican respectivamente `minos 15.0` y `13.0`.

| Paquete | Máquina de prueba | Resultado |
| --- | --- | --- |
| `Voxy-0.2.0-macos-arm64.dmg` | Mac mini M1, macOS 15.5 | PASS, HVF |
| `Voxy-0.2.0-macos-amd64.dmg` | MacBook Pro Intel, macOS 13.6.9 | PASS, HVF |

Procedimiento ejecutado mediante `scripts/test-dmg.sh` sobre los DMG finales:

- `hdiutil verify`, montaje, copia a una ruta con espacios y desmontaje.
- `codesign --verify --deep --strict` sobre la aplicación copiada.
- Auditoría `otool -L` de todos los ejecutables/bibliotecas: sin referencias
  externas a Homebrew/MacPorts; traza de bibliotecas de `qemu-img` sin esos prefijos.
- PATH limitado a herramientas del sistema y binarios incluidos por la CLI.
  Los gestores siguen instalados en los anfitriones; no se simula un Mac limpio.
- Inicialización con proxy HTTPS inválido: utiliza la imagen incluida y valida SHA-256.
- VM nueva, 512 MiB RAM, 1 CPU, SSH local puerto 22333, HVF nativo.
- Debian 12 y arquitectura correctos; DNS, `apt-get update`, SSH y servicios sanos.
- Sincronización del kernel e initramfs sin cambios de checksum.
- Rechazo de ampliación con VM encendida; apagado, `qemu-img check`, ampliación
  de 1 a 2 GiB y rechazo de reducción; nuevo arranque con datos conservados.
- Comprobación del sistema de archivos ampliado y apagado limpio; integridad qcow2.
- Panel de Terminal: estado y opción de cierre. Apertura de `.app` mediante `open`
  en ambas sesiones gráficas y comprobación del proceso `Voxy.command` en Terminal.

Las VMs de prueba quedaron apagadas. Las VMs anteriores en Application Support
no se modificaron. Los directorios temporales con evidencias permanecen en cada
Mac; se imprime su ubicación al finalizar el script.

La firma es ad-hoc, con entitlement Hypervisor para QEMU. Ambas Macs reportaron
cero identidades Developer ID disponibles. No hay notarización ni ticket Apple.
No se verificó el flujo de cuarentena/Gatekeeper de una descarga web en un Mac
limpio; tampoco versiones de macOS distintas de las dos indicadas, suspensión,
actualización automática ni desinstalación. No se desactivó Gatekeeper.

Regresión adicional del launcher fuente en Linux x86_64: arranque KVM, espera
SSH, `uname -m` y apagado correctos después de añadir la detección del runtime
incluido en la aplicación.

Artefactos finales:

| Archivo | Bytes | SHA-256 |
| --- | ---: | --- |
| `Voxy-0.2.0-macos-arm64.dmg` | 119870094 | `1251c03c2ff9490599e5ff849cfee3a849c430f3dd1f6284c18370e0dad08bf5` |
| `Voxy-0.2.0-macos-amd64.dmg` | 124678059 | `0d5d53dfba81c07fac21efbf0643eb14a1d35e5ed1a247f7f5d8e08dda67dd06` |
