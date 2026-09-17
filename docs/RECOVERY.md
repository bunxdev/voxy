# Recuperación de Voxy en Windows (0.4.1)

Voxy conserva puntos de recuperación locales del disco de Debian. Están activados
por defecto en el launcher Windows x64; los launchers actuales de macOS y Linux
**todavía no implementan este mecanismo**.

## Uso

1. Al actualizar desde 0.3.x o 0.4.0, cierra los paneles antiguos. Apaga Debian desde el
   menú y vuelve a iniciarlo con 0.4.1 para reemplazar el proceso de copias anterior.
2. Se crea una primera copia cuando Debian responde por SSH. Por defecto no se repite.
   La opción **9) Configurar copias periódicas** permite activarlas y elegir un
   intervalo de **1 a 10080 minutos** (10 es solo el valor sugerido).
   La elección se guarda en `backup-settings.json` dentro del directorio de datos.
   El proceso vuelve a leerla cada 10 segundos, sin reiniciar Debian.
   Desactivarlas mantiene la copia inicial y las copias manuales; una copia en curso
   termina normalmente. Si el contador no cambió, se omite la copia periódica.
   Las escrituras del propio sistema también cuentan como actividad.
   El intervalo se cuenta desde que termina el intento anterior.
3. Se conservan los **tres últimos puntos completos**, cada uno independiente.
   La copia más antigua se elimina solamente después de verificar una nueva.
4. Puedes cerrar el panel o desconectar SSH: QEMU y el proceso de copias continúan.
   Tras reiniciar Windows, abre Voxy y selecciona **Iniciar Debian**. No se instala
   un servicio ni una tarea de arranque automático del sistema.

El panel muestra el resultado y la fecha de la última copia. Opciones nuevas:
**Crear punto de recuperación**, **Ver puntos** y **Restaurar un punto**.
Para restaurar, Debian debe estar apagado; el panel pide el ID y confirmación.

```powershell
.\Voxy.exe backup
.\Voxy.exe backups
.\Voxy.exe stop
.\Voxy.exe restore ID-MOSTRADO-EN-LA-LISTA
.\Voxy.exe start
.\Voxy.exe wait
```

`restore` es una orden explícita de recuperación: vuelve al estado del punto elegido.
El disco anterior queda en `before-restore-FECHA` para poder inspeccionarlo. Estas
carpetas se conservan hasta que el usuario decida retirarlas; no forman parte de la
rotación automática. Una copia nunca se restaura automáticamente sobre datos nuevos.

Los puntos están en `%LOCALAPPDATA%\Voxy\amd64\backups`, o en `backups` dentro de
`VOXY_DATA_DIR`. Contienen disco QCOW2, kernel, initramfs y un manifiesto SHA-256.
No guardan RAM ni sesiones de terminal abiertas. Las claves de la instalación
permanecen en el directorio de datos habitual.

## Espacio y límites

Las copias son completas e independientes, sin cadenas de archivos base. Los
bloques vacíos se representan mediante QCOW2, pero los datos ocupados se duplican
por cada punto. Durante una copia puede haber tres puntos completos y uno en curso.
La transferencia se limita a 32 MiB/s. Se exige espacio libre equivalente al tamaño
virtual del disco más 512 MiB antes de comenzar; si falta, se registra el error y
se mantienen los puntos anteriores. Una copia puede tardar más de 10 minutos en
discos grandes; nunca se ejecutan dos copias simultáneas. Hay un límite de 30 minutos
por intento. Con copias periódicas activadas, el siguiente intento espera el
intervalo elegido desde el fin del anterior. Si están desactivadas, un fallo de
la copia inicial queda visible en el panel y se puede reintentar manualmente.

La copia captura un instante del disco después de solicitar `sync` al invitado;
las aplicaciones continúan funcionando. Es una copia con consistencia equivalente
a recuperar tras un corte, **no una transacción coordinada con cada aplicación o
base de datos**. No garantiza recuperar cambios que solo estaban en RAM ni los
posteriores al último punto completo. Bases de datos importantes deben añadir sus
propios backups. Tampoco protege contra avería, pérdida o robo del disco anfitrión:
para eso hace falta otra copia fuera del equipo.

Para pruebas sin proceso automático, establece `VOXY_AUTO_BACKUP=0` **antes de
iniciar una VM apagada**. No detiene un proceso de copias que ya estuviera trabajando.
Las copias manuales siguen disponibles.

## Cómo resiste interrupciones

- QMP usa un socket local dentro del directorio privado, sin un puerto de control TCP.
- `blockdev-backup` realiza la copia del disco activo mediante QEMU. No se copia
  directamente un QCOW2 abierto usando herramientas externas.
- Los destinos `.partial-*` no aparecen como puntos recuperables. Tras confirmar
  el trabajo QEMU, se comprueba QCOW2, se sincronizan archivos, se calculan hashes
  y se publica el manifiesto. Un intento interrumpido no reemplaza el punto anterior.
- Los temporales huérfanos se limpian en el siguiente intento cuando no hay un
  trabajo QEMU activo. Los errores quedan en `backup-status.json`.
- Los bloqueos de operaciones pertenecen al núcleo de Windows: se liberan si muere
  el proceso, sin tener que borrar manualmente un directorio `control.lock` viejo.
- Los metadatos se escriben en un temporal, se sincronizan y se reemplazan de forma
  atómica. Antes de arrancar se comprueba la estructura del disco con `qemu-img check`.
  Un error detiene el arranque con diagnóstico; no se fuerza una reparación destructiva.
- La restauración verifica todos los hashes antes de cambiar archivos, conserva
  los anteriores y usa un diario persistente. Si se interrumpe, `start` completa
  la restauración pendiente antes de arrancar, evitando mezclar generaciones.
- QEMU y el proceso de copias se separan también del grupo de procesos de OpenSSH
  (`CREATE_BREAKAWAY_FROM_JOB`), además de desacoplarse de la consola.

No ejecutes simultáneamente launchers 0.3.x y 0.4.x sobre la misma VM: usan bloqueos
distintos. Cierra los paneles anteriores al actualizar.

## Pruebas reproducibles

Desde el repositorio, apuntando a un paquete extraído:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\test-windows-recovery.ps1 -Exe C:\Voxy\Voxy.exe -TestSchedule
```

La política indicada se limita al proceso de prueba. El script crea una VM nueva
con puerto libre; nunca termina la VM habitual. Prueba cierre brusco de QEMU,
restauración del marcador anterior, retención, interrupción de una copia y limpieza
del temporal. `-TestSchedule` añade la espera real del siguiente intervalo de
10 minutos. Conserva los datos de prueba y apaga la VM al terminar.

Para comprobar la política predeterminada y los cambios desde el menú:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\test-windows-backup-settings.ps1 -Exe C:\Voxy\Voxy.exe
```

Esta prueba usa otra VM, espera más de 10 minutos para confirmar la ausencia de
copias periódicas por defecto y luego prueba activación, cambio de intervalo,
desactivación, reinicio y restauración.

Pruebas Go adicionales: exclusión y liberación del bloqueo tras matar su proceso,
rechazo de copia corrupta, restauración interrumpida, exclusión de copias incompletas
y omisión de copias automáticas sin nuevas escrituras. Los resultados concretos y
las limitaciones de hardware están en [TESTING.md](../TESTING.md).

Referencia de implementación: [QEMU: copias de bloques en vivo](https://www.qemu.org/docs/master/interop/live-block-operations.html)
y [API QMP](https://www.qemu.org/docs/master/interop/qemu-qmp-ref.html).
