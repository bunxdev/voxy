# Canonical format: protocol IPv4 host-port guest-port. Never evaluated as code.
function fail(message) { print "ERROR: " message > "/dev/stderr"; failed=1; exit 1 }
function number(s) { return s ~ /^[0-9]+$/ && length(s) <= 5 && s+0 >= 1 && s+0 <= 65535 }
function address(s, a,n,i) {
 n=split(s,a,"."); if(n!=4) return 0
 for(i=1;i<=4;i++) if(a[i] !~ /^[0-9]+$/ || length(a[i])>3 || a[i]+0>255 || (length(a[i])>1 && substr(a[i],1,1)=="0")) return 0
 return !(a[1]+0>=224 && a[1]+0<=239) && s!="255.255.255.255"
}
function range(s, a,n) {
 n=split(s,a,"-"); if(n<1 || n>2 || !number(a[1]) || (n==2 && !number(a[2]))) fail("Puerto o rango inválido")
 first=a[1]+0; last=(n==2 ? a[2]+0 : first)
 if(last<first || last-first>=256) fail("Rango invertido o mayor de 256 puertos")
}
function add(p,a,h,g, i) {
 if((p!="tcp" && p!="udp") || !address(a) || !number(h) || !number(g)) fail("Regla de puertos inválida")
 if(p=="tcp" && h+0==ssh_port+0 && (a=="127.0.0.1" || a=="0.0.0.0")) fail("Conflicto con el puerto SSH de Voxy")
 for(i=1;i<=count;i++) if(proto[i]==p && host[i]==h+0 && (bind[i]==a || bind[i]=="0.0.0.0" || a=="0.0.0.0")) fail("Reglas duplicadas o direcciones solapadas")
 count++; if(count>256) fail("Máximo 256 puertos reenviados por VM")
 proto[count]=p; bind[count]=a; host[count]=h+0; guest[count]=g+0
}
/^[[:space:]]*(#|$)/ { next }
{
 if(NF!=4) fail("ports.conf: se esperan protocolo dirección puerto-host puerto-Debian")
 add($1,$2,$3,$4)
}
END {
 if(failed) exit 1
 if(mode=="add" || mode=="remove") {
  if((protocol!="tcp" && protocol!="udp") || !address(ip)) fail("Protocolo o IPv4 inválidos")
  range(host_range); start=first; end=last
  if(mode=="add") {
   range(guest_range); if(last-first!=end-start) fail("Los rangos deben tener igual longitud")
   for(p=start;p<=end;p++) add(protocol,ip,p,first+p-start)
  } else {
   for(i=1;i<=count;i++) if(proto[i]==protocol && bind[i]==ip && host[i]>=start && host[i]<=end) { removed[i]=1; found++ }
   if(found!=end-start+1) fail("No existen todos los puertos solicitados; no se modificó la configuración")
  }
 }
 for(i=1;i<=count;i++) if(!removed[i]) printf "%s %s %d %d\n",proto[i],bind[i],host[i],guest[i]
}
