import {createHash} from 'node:crypto';
const dir=process.argv[2]; if(!dir) throw new Error('Missing image directory');
const hashes:Record<string,string>={};
for(const name of ['kernel','initramfs','disk.qcow2']) {
 hashes[name]=createHash('sha256').update(Buffer.from(await Bun.file(`${dir}/${name}`).arrayBuffer())).digest('hex');
}
await Bun.write(`${dir}/SHA256.json`,JSON.stringify(hashes,null,2)+'\n');
