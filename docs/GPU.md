# Cómputo GPU compartido

Voxy 0.8 permite que Debian envíe cálculos WGSL a la GPU del anfitrión: NVIDIA mediante Vulkan en Linux, y Metal en macOS. El anfitrión conserva la GPU. No se instala CUDA dentro de Debian ni se añade un dispositivo PCI virtual; PyTorch/CUDA/MPS no detectan automáticamente este puente. Windows todavía no incluye esta función.

Construye el helper con Rust 1.87 o posterior y los controladores Vulkan de NVIDIA ya instalados (Linux):

```bash
./scripts/build-gpu.sh
./voxy gpu status
./voxy start
./voxy gpu enable
./voxy gpu test
./voxy ssh '/usr/local/bin/voxy-gpu compute -' < request.json
./voxy gpu disable
./voxy stop
```

El paquete macOS incluye el helper. No modifica controladores, servicios ni la asignación PCI de la GPU. Si falta un dispositivo compatible devuelve un error; nunca calcula silenciosamente en la CPU.

La habilitación es por sesión. El servicio escucha en loopback con un token aleatorio privado; un túnel SSH permite acceder desde Debian. El token no se imprime ni aparece en los argumentos de procesos. Apagar la VM revoca el servicio. Tras reiniciar hay que habilitarlo otra vez. Una desconexión del túnel exige desactivar y volver a habilitar el acceso.

Sólo ejecutes shaders y aplicaciones de confianza. Los límites de entrada y tiempo no impiden que un shader consuma recursos del controlador. Las operaciones HTTP tienen un plazo de 60 segundos; finalizar un proceso no garantiza cancelar inmediatamente trabajo ya enviado a la GPU. `voxy gpu run request.json` ejecuta directamente en el anfitrión, sin el supervisor HTTP.

Consulta el [formato de solicitudes, límites y pruebas](../gpu/README.md). El ejemplo de esa página multiplica un buffer de tres números por dos. La lectura de resultados permite comprobar el cálculo, no sólo la detección del adaptador.
