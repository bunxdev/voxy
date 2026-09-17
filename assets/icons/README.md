# Icono de Voxy

`voxy-source.png` es la imagen maestra generada con la herramienta integrada
**imagegen**. `Voxy.ico` contiene tamaños 16, 24, 32, 48, 64, 128 y 256 px;
`Voxy.iconset` contiene los tamaños estándar de macOS hasta 1024 px.

Regenerar los formatos (sin volver a generar el diseño):

```sh
bun install --cwd scripts/icons --frozen-lockfile
bun scripts/icons/build.ts
```

El constructor de Windows incrusta el ICO en el ejecutable mediante
[rsrc v0.10.2](https://github.com/akavel/rsrc). El de macOS convierte el iconset
con `iconutil` y configura
[CFBundleIconFile](https://developer.apple.com/documentation/bundleresources/information-property-list/cfbundleiconfile)
antes de firmar la aplicación. Los paneles continúan utilizando la terminal del
sistema; su icono puede seguir siendo el de Terminal/Windows Terminal.

Prompt original (herramienta integrada, sin CLI):

> Use case: logo-brand. Asset type: desktop application icon for Voxy, a minimal Debian virtual machine application for Windows and macOS. Create one polished, distinctive app icon, square 1024x1024 with genuinely transparent background outside its rounded-square tile. A simple bold folded V shape suggesting an open virtual cube, centered on a deep navy rounded-square tile, with restrained teal and blue highlights, subtle dimensional lighting. Strong silhouette readable at 16px, clean professional desktop software aesthetic, generous safe margins. No text, no wordmark, no additional objects, no mockup, no surrounding scene, no border outside the tile. Deliver only the icon.
