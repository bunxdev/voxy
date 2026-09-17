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

## Windows x64 v0.3.0 — validación preliminar con Wine

Entorno: Debian 13 x86_64, Wine 10.0 (paquete Debian `10.0~repack-6`), Xvfb,
Go 1.27.1 y Bun 1.4.2. Se ejecutaron **binarios Windows PE x64**; no se sustituyó
QEMU por el binario Linux. El paquete incluye QEMU 11.1.0 del instalador
`qemu-w64-setup-20260811.exe` de Stefan Weil, verificado con el SHA-512 fijado,
`qemu-img` y 104 DLL transitivas. Invitado Debian 12 AMD64 v0.1.0, 512 MiB y 1 CPU.

### Resultados

- Compilación cruzada Go y `GOOS=windows GOARCH=amd64 go vet ./...`: correctas.
- Cinco pruebas unitarias compiladas para Windows y ejecutadas en Wine: correctas.
  Cubren tamaños/overflow, discos existentes, imagen corrupta/reintento de init,
  PID reutilizado y verificación del par kernel/initramfs antes de reemplazarlo.
- `doctor`: cargan QEMU y `qemu-img` con las DLL incluidas. El binario informa
  aceleradores `tcg` y `whpx`; WHPX no está disponible en Wine.
- Ciclo integral con TCG: primer arranque, SSH integrado, DNS y APT, Debian 12 x64,
  ausencia de Docker y servicios fallidos, preparación/sincronización del kernel,
  rechazo de resize encendido, apagado, comprobación qcow2, ampliación de 1 a 2 GiB,
  rechazo de reducción, segundo arranque distinto y persistencia: correcto.
- Después del segundo arranque: sistema de archivos de 2.0 GiB, 276 MiB usados;
  invitado alrededor de 50 MiB RAM usada (no equivale al RSS de QEMU anfitrión).
- Procesos separados `start`, cierre del menú y `status`: QEMU continúa activo.
- Terminal SSH interactiva: recibe comandos y sale correctamente con `exit`.
- Rutas del programa y de datos con espacios y `ñ`: arranque y SSH correctos.
- WHPX solicitado explícitamente en Wine falla con diagnóstico; no se oculta
  cambiando a TCG. Los datos normales y los discos de prueba quedan separados.
- El ZIP final se extrae y se valida contra su manifiesto SHA-256 antes de probarlo
  con `scripts/test-wine.sh`, en un prefijo Wine dedicado y rutas con `ñ`.

### Correcciones obtenidas de las pruebas

1. QEMU mantiene abierto `initramfs` en Windows; reemplazarlo con la VM activa
   devolvía `Access denied`. Ahora `sync-kernel` deja ambos archivos preparados y
   con hashes; se aplican con QEMU apagado. Los archivos originales de la
   actualización se conservan hasta completar los dos reemplazos, permitiendo reintentar.
2. QEMU no podía abrir rutas absolutas con `ñ`. El launcher establece el directorio
   de trabajo usando las API Unicode de Windows y entrega nombres relativos ASCII
   a QEMU y `qemu-img`. El pequeño firmware PC se copia al directorio de datos.

### Límites

**Al publicar v0.3.0 no se había probado en Windows real.** Los siguientes límites
corresponden a esa entrega; la validación nativa posterior se documenta debajo. WHPX, permisos efectivos de usuario estándar,
ACL de claves, SmartScreen, Defender, Firewall, VPN, WSL2/Hyper-V, suspensión y
actualización siguen pendientes. Las pruebas Wine no certifican esos comportamientos.
No hay firma Authenticode de Voxy, instalador MSI/NSIS ni Windows ARM64. Tampoco
se han incorporado Docker ni Lupa al invitado. Las DLL opcionales de interfaces
multimedia/gráficas ajenas al uso headless de Voxy no están validadas.

El paquete es portátil. Para la prueba real, `Probar-Windows.cmd` usa WHPX y
`Probar-TCG.cmd` usa emulación por software. Ambos guardan un log compartible en
`%LOCALAPPDATA%\Voxy` y conservan sus discos de prueba para diagnóstico.

Artefacto Windows final: `Voxy-0.3.0-windows-x64.zip`, **157636969 bytes**.
SHA-256: `e972bbd929c36a8f9a1d801f603ee9c7a90d5d24564d7c998b43e6fd3970af76`.

## Windows x64 v0.3.1 — Windows real y terminal (2026-09-16)

Anfitrión: Windows 10 Home Single Language 22H2, build 19045, x64, aproximadamente
12 GiB RAM y 4 procesadores lógicos. HypervisorPlatform habilitado; doctor detectó
WHPX. QEMU 11.1.0 y Debian 12 AMD64, con 512 MiB y 1 CPU para las pruebas.

- Ciclo integral WHPX correcto con 0.3.0 y repetido con 0.3.1; último PASS
  a las 08:27:01 UTC. Primer arranque, SSH, DNS/APT, sincronización del kernel,
  apagado, integridad qcow2, ampliación de 1 a 2 GiB, rechazo de reducción y
  ampliación en caliente, segundo arranque y persistencia. VMs de prueba apagadas.
- Invitado después del segundo arranque: unos 52 MiB RAM usados y 276 MiB de disco
  utilizados. Estas cifras no representan la memoria del proceso QEMU anfitrión.
- Siete pruebas unitarias ejecutadas en Windows: PASS. Una consola Windows real
  reprodujo códigos de bracketed paste y color visibles con ANSI deshabilitado;
  al activar el modo VT mostró solo VOXY y restauró el modo original al finalizar.
- SSH interactivo contra la VM habitual: colores, flecha arriba/historial, pegado
  delimitado, Ctrl+C y salida correctos. El tamaño cambió de 30 filas × 100 columnas
  a 40 × 120 y se reflejó en stty size dentro de Debian.
- SSH escuchaba únicamente en 127.0.0.1. ACL del directorio de datos y la clave:
  usuario actual y SYSTEM con control total, directorio con DACL protegida.
  Defender y protección en tiempo real activos; conexión remota mediante Tailscale.
- Se observó QEMU TCG y un panel habitual sin elevación. Las pruebas WHPX por SSH
  usaron elevación: falta repetir ese ciclo completo como usuario estándar.
- Se instaló el ejecutable corregido conservando el anterior como respaldo. La
  VM habitual TCG y las sesiones existentes se mantuvieron; los paneles existentes
  deben cerrarse y abrirse para cargar la versión nueva.

Corrección: activar VT en stdout/stderr de consola, restaurar sus modos al salir,
consultar tamaño desde stdout y enviar cambios al PTY SSH. Las salidas redirigidas
no se modifican. El ejecutable empaquetado coincide byte por byte con el probado.

Pendiente: Windows 11, ARM64, instalación limpia/SmartScreen, firma Authenticode,
MSI, actualización/desinstalación, backup/restauración, suspensión y matriz amplia
de Firewall/VPN/WSL2. Docker y Lupa aún no están incluidos.

Artefacto: Voxy-0.3.1-windows-x64.zip, **157640854 bytes**.
SHA-256: `0d370f6bff06a09b764611ae7a6dab4d6a9d386a841625432367c625dd96bb9d`.

## Windows x64 v0.4.0 — recuperación (2026-09-16)

Mismo Windows 10 Home 22H2 x64 (19045), QEMU 11.1.0 y WHPX. Se usaron VMs de
prueba separadas, 512 MiB y 1 CPU. La VM habitual permaneció encendida durante
las pruebas destructivas. No se cortó la alimentación del equipo del usuario.

- Doce pruebas unitarias nativas: PASS. Las cinco nuevas cubren exclusión y
  liberación del bloqueo al terminar su proceso a la fuerza; rechazo de copia
  corrupta antes de tocar el disco actual; reanudación de restauración interrumpida;
  omisión de puntos incompletos/IDs inválidos; y omisión de copia automática cuando
  el contador QMP de escrituras no cambia.
- Ciclo completo WHPX con el código final: PASS a las 08:54:01 UTC. SSH, DNS/APT,
  kernel, ampliación, persistencia, QCOW2 y apagado; invitado 52 MiB usados y
  276 MiB de disco usados después de ampliar a 2 GiB.
- Se terminó QEMU a la fuerza después de escribir y ejecutar sync: el siguiente
  arranque conservó los datos. Una restauración recuperó el marcador anterior.
- Después de varias copias quedaron exactamente tres puntos completos. Se terminó
  QEMU y el proceso de copia mientras crecía un destino parcial: los tres puntos
  anteriores conservaron sus hashes. La restauración y la siguiente copia pasaron;
  el intento posterior limpió el temporal huérfano.
- Una restauración interrumpida entre archivos se reprodujo con un diario persistente:
  la reanudación completó los tres archivos y conservó todos los originales.
- Al cerrar una sesión OpenSSH se descubrió que DETACHED_PROCESS por sí solo no
  separaba los hijos del grupo de procesos de SSH. CREATE_BREAKAWAY_FROM_JOB
  resolvió el cierre de QEMU y del trabajador; ambos sobrevivieron a la desconexión.
- El ejecutable exacto del ZIP (compilado con trimpath y sin símbolos de depuración)
  creó y verificó una copia; después de cambiar un marcador, restauró el valor
  original y apagó Debian correctamente. SHA-256 del ejecutable:
  `e1f49aa0ab2faa0de939a3946f8e19798197daf761abe5384adbc75bd5ddfa3c`.
- Los 132 archivos del paquete instalado coincidieron con su manifiesto SHA-256.
- Temporizador real, sin acortar el intervalo: primera copia a las 08:56:03 UTC,
  siguiente a las 09:06:10 UTC (606,45 segundos). La sesión SSH que inició QEMU
  y el trabajador ya estaba desconectada. Se restauró el segundo punto y se
  comprobó el archivo escrito después del primero. La VM de prueba quedó apagada.
  El script reproducible espera a que termine la primera copia antes de medir el
  segundo intervalo, evitando confundir la copia inicial con una programada.

Las copias son de disco, sin RAM; se solicita sync pero no se congelan ni coordinan
transacciones de aplicaciones. No se ha simulado un corte eléctrico físico, fallo
de hardware, Windows 11, ARM64, suspensión, ni aplicado este mecanismo a macOS/Linux.
Las pruebas WHPX remotas siguen usando elevación. No se instaló un servicio de
arranque automático de Windows. [Operación y límites](docs/RECOVERY.md).

Artefacto: `Voxy-0.4.0-windows-x64.zip`, **157684368 bytes**.
SHA-256: `91c6c17fc28e98ee4030c650441056c471882a3ff0318ab6c79601dfc7904c15`.

## 0.4.1 — copias periódicas opcionales

El valor predeterminado conserva solo el intento de copia inicial. El menú permite
activar/desactivar las periódicas y persistir un intervalo de 1 a 10080 minutos.
Pruebas nuevas ejecutadas con el binario Windows bajo Wine: copia inicial con
periodicidad desactivada, límite exacto del intervalo, persistencia, desactivación
y rechazo de entradas inválidas/configuración corrupta. Compilación Windows x64 correcta.
La suite completa pasó 12 pruebas; dos pruebas previas no se pudieron completar
en este entorno Wine: socket AF_UNIX (error 10047) y AllocConsole (Access denied).
La validación posterior en Windows real se detalla abajo. La instalación habitual
remota no se ha reemplazado.

### Validación nativa adicional (2026-09-16)

La suite Go completa de 0.4.1 pasó **14/14** pruebas en Windows 10 real,
incluidas AF_UNIX/QMP y representación ANSI de consola que Wine no pudo ejecutar.
El ejecutable probado coincide con el del ZIP 0.4.1:
`9c3553f7c42c13d878ca5efa1c245c06dd5fbdd4c06f51f3f0791b50fb2373c4`.

La prueba de comportamiento real está en
`scripts/test-windows-backup-settings.ps1`: usa una VM WHPX y directorio únicos,
comprueba la ausencia de copias durante más de 10 minutos con cambios en disco,
activa/cambia/desactiva la periodicidad desde el menú, verifica la persistencia
tras reiniciar Debian y restaura una copia programada para comprobar su contenido.

Resultado de integración: **PASS**, 2026-09-16 10:05:33 UTC, Windows 10 + WHPX.

- Copia inicial completa y verificada.
- 620 segundos con escrituras nuevas, sin otra copia ni trabajo parcial, usando
  la configuración predeterminada (sin archivo de preferencias).
- Activación desde la opción 9 del menú con intervalo de 1 minuto, sin reiniciar.
- Cambio en vivo a 2 minutos: siguiente inicio de copia 155,83 segundos después
  del inicio anterior (unos 36 segundos de copia más 120 segundos de intervalo).
- Desactivación desde el menú y 145 segundos adicionales con escrituras, sin copia.
- Copia manual disponible con periodicidad desactivada.
- Preferencia desactivada conservada al reiniciar Debian; nueva copia inicial creada.
- Restauración de la copia programada recuperó el marcador `TWO_MINUTES` esperado.
- VM de prueba apagada al finalizar; la VM habitual no fue reiniciada ni actualizada.

Registro local completo: `.cache/windows-native/settings-integration-041.log`.

### Actualización de la instalación habitual (2026-09-17)

Instalado 0.4.1 en Windows 10 sobre la VM existente con WHPX. Los 132 archivos
coinciden con el manifiesto del ZIP. Se conservó una copia de la instalación anterior.
El proceso de copias 0.4.0 se sustituyó con el bloqueo de control adquirido, evitando
interrumpir una copia. La VM mantuvo su PID y boot ID; las dos terminales SSH
existentes y Docker siguieron activos. No fue necesario reiniciar Debian.

Probados con el ejecutable instalado: diagnóstico, menú/opción 9, conexión SSH,
resolución DNS, Docker cliente/servidor, escritura/lectura de un archivo temporal,
continuidad tras desconectar SSH y copia manual completa verificada.
La copia inicial del nuevo proceso conservó el último punto porque no había
escrituras nuevas. La copia manual posterior terminó correctamente a las 09:14:45 UTC.
Las copias periódicas quedaron desactivadas en la configuración persistente.
Los paneles que ya estaban abiertos mantienen su ejecutable anterior en memoria:
para ver el menú nuevo, salir del panel y abrir `Voxy/app/Voxy/Voxy.exe`.

Registros locales: `.cache/windows-native/install-live-041.log`,
`verify-live-041.log`, `post-live-041.log` y `live-backup-041.log`.

## 0.5.0 — icono nativo en Windows y macOS (2026-09-17)

Se generó un diseño común y se guardó su fuente PNG, ICO multirresolución e iconset
macOS en `assets/icons`. El prompt y la reconstrucción están documentados allí.
El icono se incrusta en el EXE mediante rsrc v0.10.2 y en la app mediante iconutil
más CFBundleIconFile, antes de la firma ad-hoc.

- Windows 10: extracción nativa del icono del EXE a 16, 32, 48 y 256 px correcta;
  inspección visual de la imagen extraída a 256 px. El ejecutable del ZIP pasó
  el ciclo completo WHPX: SSH, DNS/APT, kernel, ampliación del disco, persistencia,
  integridad QCOW2 y apagado en una VM separada.
- Mac Intel y M1: ambos DMG se verificaron, montaron y probaron con `test-dmg.sh`.
  Pasaron firmas, dependencias internas, inicialización sin descarga, HVF, SSH,
  APT, ampliación del disco, persistencia, integridad y apagado.
- En ambas Macs se verificaron CFBundleIconFile, versión 0.5.0 e iconutil, y se
  instalaron las apps en `/Applications/Voxy.app` conservando las anteriores.
  La apertura mediante LaunchServices se completó correctamente.
- Windows: instalación actualizada, 135 archivos verificados contra SHA256SUMS,
  acceso Voxy en el escritorio con destino e icono apuntando a Voxy.exe. La VM
  habitual mantuvo su PID, Docker y las dos terminales. Periodicidad desactivada.

La interfaz sigue siendo un panel de terminal. Este cambio no añade GUI propia,
notarización de Apple ni firma Authenticode. El icono de la ventana de terminal
puede depender de Terminal/Windows Terminal. Las copias de recuperación siguen
siendo una función del lanzador Windows; no se portaron a macOS en esta versión.

## Voxy 0.6.0 — reenvío de puertos (2026-09-17)

Se construyeron paquetes Windows x64, macOS Intel y macOS ARM64. La nueva
función usa `hostfwd` TCP/UDP de QEMU y configuración persistente `ports.conf`.

- Windows 10 Home 22H2 x64: pruebas unitarias ejecutadas en Windows real,
  incluidas las de copias, recuperación, consola y las nuevas de puertos.
  Después se corrigió la contención de `ports list` con el bloqueo de copias;
  las pruebas de puertos y de lectura con el bloqueo adquirido pasaron de nuevo.
- Mac M1/macOS 15.5 y Mac Intel/macOS 13.6.9: validación con el Bash 3.2 y awk
  del sistema, DMG verificado, firma ad-hoc comprobada y VM nuevas desde el
  paquete, sin herramientas de paquetes en PATH.
- En los tres equipos: dos puertos TCP consecutivos mapeados a HTTP 8080/8081,
  eco UDP, persistencia tras apagar/iniciar, reglas pendientes frente a activas,
  eliminación aplicada tras reinicio y rechazo de un puerto ocupado. Tras
  liberar el puerto, la misma VM volvió a arrancar correctamente.
- En ambos Macs: escucha en su IP Tailscale y peticiones TCP y UDP exitosas
  desde otro equipo de la tailnet usando la aplicación 0.6.0 instalada.
- Windows instalado: la aplicación Bun existente en `/root/zz` respondió
  HTTP 200 con `hola desde voxy` a través de la IP Tailscale del anfitrión,
  comprobado desde otro equipo. Se configuraron reglas localhost y Tailscale
  para TCP 33033 y una excepción de firewall limitada a ese puerto, IP local
  y origen Tailscale. El launcher no crea esa excepción automáticamente.
- Se conservaron los discos habituales, el disco Windows de 4 GiB y las
  preferencias de copias iniciales sin copias periódicas. Las pruebas usaron
  discos separados y apagaron sus VM al terminar.

Pruebas reproducibles: `scripts/test-ports.sh`, `windows/ports_windows_test.go`,
`scripts/test-ports-macos.sh`, `scripts/test-ports-windows.ps1` y el servidor de
prueba `scripts/ports-probe.go`. Las pruebas de integración instalan curl solo
si falta dentro de su VM separada.

Límites: no se ha probado IPv6, cambios de puertos en caliente, todos los puertos
simultáneos, otras versiones de Windows ni una instalación nueva con cuarentena
macOS. El máximo de 256 asignaciones está validado por el parser; la integración
real utilizó unos pocos puertos y no mide el rendimiento con 256 conexiones.

## Voxy 0.7.0 — reenvío automático (2026-09-17)

- Motor compartido: pruebas Go con detector de carreras; sockets IPv4 y doble
  pila, exclusión de loopback/IPv6 exclusivo/SSH/DHCP, deduplicación, cambio de
  interfaz, retirada, errores SSH, conflictos, reintentos y recuperación del
  registro de reenvíos. El registro se escribe antes de publicar el puerto.
- Windows real: suite completa de pruebas Go; configuración automática e IPv4
  inválidas, y reproducción de un lector que deniega temporalmente reemplazar
  el archivo de estado. La escritura reintenta sin perder el registro.
- Mac Intel y M1 (HVF): TCP 8080/8081 y eco UDP 8082 detectados sin reglas,
  on/off en caliente, retirada al detener el servicio, persistencia tras
  reiniciar, conservación de reglas manuales, puerto ocupado y reintento,
  recuperación tras terminar el controlador y cambio de localhost a Tailscale.
- Windows (WHPX): mismo recorrido TCP/UDP, on/off, retirada y reinicio; prueba
  adicional de prioridad manual, puerto ocupado, recuperación del controlador,
  copia de recuperación mientras funciona el detector y cambio de interfaz.
- Acceso TCP y UDP automático desde Linux a las tres máquinas por Tailscale.
  Las reglas temporales de firewall de Windows se retiran al finalizar.
- Las pruebas usan discos separados de las VM habituales. No se repite la suite
  histórica completa de restauración de imágenes: sí sus pruebas unitarias y
  una copia real concurrente en Windows.

Scripts reproducibles: `scripts/test-auto-ports-macos.sh` y
`scripts/test-auto-ports-windows.ps1`, con el servidor Linux compilado desde
`scripts/ports-probe.go`. En Windows las peticiones HTTP desactivan keep-alive:
retirar una escucha no garantiza terminar conexiones TCP ya establecidas.

Hallazgos corregidos antes de publicar: sockets de doble pila omitidos por
`ss -4`, monitor QMP todavía no listo tras CreateProcess, espera del bloqueo
entre generaciones de VM y reemplazo temporalmente denegado del estado en
Windows. QMP automático usa un canal separado del de copias de recuperación.
