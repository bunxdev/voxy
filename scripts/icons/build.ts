import sharp from 'sharp';
import {mkdir, writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const dest=resolve(import.meta.dir,'../../assets/icons');
const source=resolve(dest,'voxy-source.png');
await mkdir(resolve(dest,'Voxy.iconset'),{recursive:true});
const sizes=[16,24,32,48,64,128,256];
const pngs=await Promise.all(sizes.map(n=>sharp(source).resize(n,n).png().toBuffer()));
const header=Buffer.alloc(6+sizes.length*16); header.writeUInt16LE(1,2);header.writeUInt16LE(sizes.length,4);
let offset=header.length;
sizes.forEach((n,i)=>{const p=6+i*16;header[p]=n===256?0:n;header[p+1]=header[p];header.writeUInt16LE(1,p+4);header.writeUInt16LE(32,p+6);header.writeUInt32LE(pngs[i].length,p+8);header.writeUInt32LE(offset,p+12);offset+=pngs[i].length;});
await writeFile(resolve(dest,'Voxy.ico'),Buffer.concat([header,...pngs]));
for(const n of [16,32,128,256,512]) for(const scale of [1,2]) {
 await sharp(source).resize(n*scale,n*scale).png().toFile(resolve(dest,'Voxy.iconset',`icon_${n}x${n}${scale===2?'@2x':''}.png`));
}
console.log('ICO and macOS iconset created from voxy-source.png');
