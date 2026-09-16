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

## Límites

No se ha probado HVF, WHPX ni aceleración ARM nativa. La variante ARM utiliza
TCG y puede tardar minutos. La referencia cloud amd64 no es una imagen mínima
construida a partir de la receta ARM. No se ha instalado Docker ni Lupa dentro
de ninguna de estas nuevas VMs. Los resultados anteriores del proyecto ARM
pertenecen a su documentación upstream; este informe distingue las pruebas locales.
