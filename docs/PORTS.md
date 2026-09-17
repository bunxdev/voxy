# Reenvío de puertos (Voxy 0.6.0)

Voxy publica servicios de Debian a través de la IP del anfitrión, usando el
reenvío TCP/UDP de QEMU. No requiere túneles SSH ni instalar Tailscale dentro
de Debian. Si Docker usa `network_mode: host`, sus servicios escuchan en Debian;
estas reglas cubren el siguiente salto: Windows/macOS → Debian.

## Desde el panel

En Windows, opción **10: Configurar puertos**. En macOS, opción **5**.
El panel muestra las reglas guardadas y las activas. Escribe una operación:

```text
add tcp 33033 33033 0.0.0.0
```

Apaga e inicia Debian para aplicarla. `start` sobre una VM encendida no modifica
sus puertos. La configuración se conserva al actualizar la aplicación.

## Línea de comandos

Ejemplo Windows PowerShell, ejecutado junto a `Voxy.exe`:

```powershell
.\Voxy.exe ports add tcp 33033 33033 100.85.206.59
.\Voxy.exe ports list
.\Voxy.exe stop
.\Voxy.exe start
.\Voxy.exe wait
```

Ejemplo macOS, usando la aplicación instalada:

```bash
VOXY=/Applications/Voxy.app/Contents/Resources/voxy
"$VOXY" ports add tcp 33033 33033 0.0.0.0
"$VOXY" stop
"$VOXY" start
"$VOXY" wait
```

Sintaxis común:

```text
ports list
ports add tcp|udp HOST[-FIN] DEBIAN[-FIN] [IPv4]
ports remove tcp|udp HOST[-FIN] [IPv4]
ports clear
```

- Sin IPv4: `127.0.0.1`, accesible desde el anfitrión.
- `0.0.0.0`: escucha en todas las interfaces IPv4, incluidas LAN y Tailscale.
- IP concreta del anfitrión: escucha únicamente en esa dirección. Debe existir
  al arrancar; si la IP Tailscale no está disponible, QEMU puede rechazar el inicio.
- HOST es el puerto de Windows/macOS; DEBIAN es el puerto del servicio invitado.
- El servicio invitado debe escuchar en `0.0.0.0` o su IP de red, no solo en
  `127.0.0.1`. Los contenedores con red bridge necesitan publicar el puerto en
  Debian; con `network_mode: host` comparten directamente sus puertos.

Ejemplos de rangos y UDP:

```text
ports add tcp 33000-33099 33000-33099 0.0.0.0
ports add udp 34000-34009 35000-35009 0.0.0.0
ports remove tcp 33000-33099 0.0.0.0
```

Los rangos deben tener igual longitud. El límite es **256 asignaciones** en
total, contando TCP y UDP por separado, para acotar recursos y mantener la
línea de comandos de Windows dentro de límites prácticos. No hay un modo
que reserve automáticamente los 65535 puertos: Windows/macOS y otras VM
pueden estar usando parte de ellos. No se detectan automáticamente los
servicios que aparecen en Debian.

## Persistencia y errores

`ports.conf` reside en el directorio de datos de cada VM, junto al disco:

- Windows: `%LOCALAPPDATA%\Voxy\amd64\ports.conf`.
- macOS: `~/Library/Application Support/Voxy/<arquitectura>/ports.conf`.
- También se respeta `VOXY_DATA_DIR`.

Cada línea contiene `protocolo IPv4 puerto-host puerto-Debian`; los rangos se
expanden a líneas individuales. Se admiten líneas vacías y comentarios `#`.
El archivo es datos, nunca código ejecutable. Las operaciones validan todo
antes de reemplazarlo, y comparten el bloqueo de control con inicio y apagado.
`ports clear` permite recuperarse de un archivo inválido.

Se rechazan puertos fuera de rango, protocolos distintos de TCP/UDP,
reglas duplicadas, solapamientos con `0.0.0.0` y conflictos con el SSH local de
Voxy. TCP y UDP pueden usar el mismo número. Dos direcciones concretas distintas
pueden usar el mismo número. El sistema operativo/QEMU decide si un puerto está
ocupado por otro proceso: si no puede abrir una regla, Voxy informa el fallo de
inicio; no elimina reglas ni escoge otro puerto silenciosamente.

`ports list` distingue configuración para el próximo arranque de reglas activas.
Las reglas de una VM iniciada antes de actualizar Voxy no se aplican hasta su
siguiente arranque. IPv6 y cambios en caliente quedan fuera de esta versión.

## Firewall y Tailscale

Escuchar en una dirección no cambia el firewall. Para entrar desde otro equipo,
el firewall del anfitrión y las reglas de Tailscale deben permitir el acceso.
Voxy no modifica automáticamente reglas globales de firewall.

Para el ejemplo Windows, desde PowerShell con privilegios de administrador,
puede crearse una regla acotada al puerto e IP elegidos (ajusta ambos valores):

```powershell
New-NetFirewallRule -DisplayName 'Voxy TCP 33033 Tailscale' `
  -Direction Inbound -Action Allow -Protocol TCP -LocalPort 33033 `
  -LocalAddress 100.85.206.59 -RemoteAddress 100.64.0.0/10 -Profile Any
```

En macOS, permite las conexiones entrantes del QEMU incluido en Voxy si el
firewall de aplicaciones lo solicita; no hace falta desactivar el firewall.

Con la regla aplicada y el servidor activo, abre `http://IP_DEL_ANFITRION:33033`.
La comprobación completa debe hacerse desde otro equipo, además de probar en
el anfitrión; una respuesta local no confirma acceso a través del firewall.

Referencia: [QEMU, hostfwd](https://www.qemu.org/docs/master/system/qemu-manpage.html).
