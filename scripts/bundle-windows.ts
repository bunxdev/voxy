// Run with Bun on Linux. Follow every PE import; do not copy Wine's DLLs.
import { mkdirSync, readdirSync, copyFileSync, writeFileSync, readFileSync } from 'node:fs';
import { join, basename } from 'node:path';
import { createHash } from 'node:crypto';
const [source, dest] = process.argv.slice(2);
if (!source || !dest) throw new Error('Usage: bun bundle-windows.ts source dest');
mkdirSync(dest, {recursive:true});
const available = new Map(readdirSync(source).map(n=>[n.toLowerCase(),n]));
const system = new Set(['advapi32.dll','avrt.dll','bcrypt.dll','bcryptprimitives.dll','cfgmgr32.dll','comctl32.dll','comdlg32.dll','crypt32.dll','d3d11.dll','d3d9.dll','dbghelp.dll','dnsapi.dll','dwmapi.dll','dwrite.dll','dxgi.dll','gdi32.dll','gdiplus.dll','hid.dll','imm32.dll','iphlpapi.dll','kernel32.dll','msimg32.dll','msvcrt.dll','mswsock.dll','netapi32.dll','normaliz.dll','ntdll.dll','ncrypt.dll','ole32.dll','oleaut32.dll','opengl32.dll','powrprof.dll','propsys.dll','psapi.dll','rpcrt4.dll','secur32.dll','setupapi.dll','shell32.dll','shlwapi.dll','user32.dll','userenv.dll','usp10.dll','ucrtbase.dll','uxtheme.dll','version.dll','winhttp.dll','wininet.dll','winmm.dll','winspool.drv','wintrust.dll','wldap32.dll','ws2_32.dll','wtsapi32.dll']);
const queue = ['qemu-system-x86_64.exe','qemu-img.exe'];
const seen = new Set<string>();
const manifest: {file:string,sha256:string,imports:string[]}[] = [];
for(let i=0;i<queue.length;i++) {
 const name=queue[i]; if(seen.has(name.toLowerCase())) continue; seen.add(name.toLowerCase());
 const path=join(source,name);
 const result=Bun.spawnSync(['objdump','-p',path]);
 if(result.exitCode!==0) throw new Error(result.stderr.toString());
 const imports=[...result.stdout.toString().matchAll(/DLL Name:\s+(\S+)/g)].map(m=>m[1]);
 if(imports.length===0) throw new Error('No import table: '+name);
 for(const dll of imports) {
  const key=dll.toLowerCase();
  if(available.has(key)) queue.push(available.get(key)!);
  else if(!system.has(key) && !/^api-ms-win-|^ext-ms-win-/.test(key)) throw new Error(`Unresolved import ${dll} in ${name}`);
 }
 copyFileSync(path,join(dest,name));
 manifest.push({file:name,sha256:createHash('sha256').update(readFileSync(path)).digest('hex'),imports});
}
writeFileSync(join(dest,'dependencies.json'),JSON.stringify(manifest,null,2)+'\n');
console.log(`Bundled ${manifest.length} PE files (${manifest.length-2} DLLs)`);
