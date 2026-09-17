# Reenvío de puertos (Voxy 0.7.0)

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

Para las reglas manuales, apaga e inicia Debian para aplicarlas. `start` sobre una VM encendida no modifica
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
ports auto on [IPv4]
ports auto off
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

Los rangos manuales deben tener igual longitud. El límite de **256 asignaciones
manuales** evita exceder la línea de comandos de Windows. La detección automática
usa comandos en caliente y no comparte ese límite.

## Modo automático opcional (0.7.0)

```text
ports auto on                 # localhost: solo este equipo
ports auto on 100.85.206.59    # IP Tailscale del anfitrión (ajústala)
ports auto on 0.0.0.0         # todas las interfaces IPv4
ports auto off
ports list
```

El modo viene **desactivado**. La dirección se guarda en `ports-auto.conf` junto
al disco. Los cambios del modo automático se aplican en caliente. Al actualizar
desde 0.6.0 hay que apagar e iniciar Debian una vez para habilitar su controlador.

Cada tres segundos, el controlador consulta `ss` mediante el SSH local de Voxy.
Publica cada servicio en el mismo puerto del anfitrión; no reserva los 65535
puertos. Incluye TCP en escucha, UDP sin conectar y sockets de doble pila que
aceptan IPv4. Excluye loopback, IPv6 exclusivo, direcciones link-local/multicast,
el SSH de Debian (TCP 22) y DHCP (UDP 67/68). El acceso SSH de Voxy sigue disponible
por su puerto local habitual. Se necesita `ss` de iproute2, incluido en la imagen.
Los contenedores bridge deben publicar sus puertos en Debian; los contenedores
con `network_mode: host` se detectan directamente.

Las reglas manuales **activas** tienen prioridad. Si otro programa, otra VM o una
regla manual ocupa un puerto, el controlador lo indica en `ports list` y reintenta
en las siguientes consultas. No termina procesos ni cambia números de puerto.
Si la IP elegida no está disponible, se muestran los fallos y se reintenta.

Al parar el servicio, apagar el modo o cambiar de IP, retira únicamente sus
reenvíos automáticos. Las conexiones TCP ya establecidas pueden continuar hasta
que se cierren; desactivar bloquea nuevas conexiones. No modifica las reglas manuales. Ante un fallo temporal de
SSH conserva los reenvíos existentes: un fallo de consulta no prueba que el
servicio se haya detenido. Cada consulta tiene un tiempo límite, por lo que los
cambios pueden tardar más de tres segundos si Debian está ocupado.

El controlador queda en segundo plano al cerrar el panel y termina al apagarse
la VM. `ports-auto.json` contiene su última lectura, conflictos y registro de
reenvíos propios; `ports-worker.log` recoge errores. Si el controlador termina
inesperadamente, `start` sobre la VM encendida lo vuelve a iniciar y recupera sus
reenvíos. Un bloqueo del sistema operativo evita controladores duplicados.
El canal QMP es local y separado del usado por las copias de Windows.

El modo automático **no abre el firewall**. Para acceder por LAN o Tailscale,
autoriza los puertos o el programa QEMU con el alcance que necesites. Abrir el
modo en `0.0.0.0` hace visibles también servicios nuevos que arranques después,
siempre que el firewall y la red permitan acceder.

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
siguiente arranque. Las reglas manuales siguen requiriendo reinicio; el modo automático cambia en caliente.
El reenvío del anfitrión sigue siendo IPv4.

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
