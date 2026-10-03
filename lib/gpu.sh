# Shared host GPU compute. Sourced by voxy; compatible with macOS Bash 3.2.
# It does not attach a PCI GPU to the guest or change host drivers.
GPU_STATE="$STATE/gpu"
GPU_BINARY=${VOXY_GPU_BINARY:-$ROOT/bin/voxy-gpu}
GPU_GUEST_PORT=47832

gpu_worker_required() {
  [[ -x "$GPU_BINARY" ]] || { echo 'Falta bin/voxy-gpu. Ejecuta scripts/build-gpu.sh o instala el paquete con soporte GPU.' >&2; return 1; }
}
gpu_server_alive() {
  local pid command
  if [[ -s "$GPU_STATE/server.pid" ]]; then read -r pid < "$GPU_STATE/server.pid"
  elif [[ -s "$GPU_STATE/launcher.pid" ]]; then read -r pid < "$GPU_STATE/launcher.pid"
  else return 1; fi
  case "$pid" in ''|*[!0-9]*) return 1;; esac
  (( pid > 1 )) || return 1
  command=$(ps -p "$pid" -o command= 2>/dev/null) || return 1
  [[ "$command" == *"$GPU_BINARY serve --state-dir $GPU_STATE --watch-pid "* ]]
}
gpu_tunnel_alive() {
  [[ -S "$GPU_STATE/tunnel.sock" ]] || return 1
  ssh -F /dev/null -S "$GPU_STATE/tunnel.sock" -O check root@127.0.0.1 >/dev/null 2>&1
}
gpu_shared() {
  running && [[ -f "$GPU_STATE/server.ready" ]] && gpu_server_alive && gpu_tunnel_alive
}
gpu_status() {
  local sharing=false probe
  if gpu_shared; then sharing=true; fi
  if [[ -x "$GPU_BINARY" ]]; then probe=$("$GPU_BINARY" probe) || return
  else probe='{"available":false,"reason":"El paquete no incluye el helper GPU"}'; fi
  printf '{"sharing":%s,"gpu":%s}\n' "$sharing" "$probe"
}
gpu_stop_service() {
  local pid port i
  if gpu_server_alive; then
    if [[ -s "$GPU_STATE/server.pid" ]]; then read -r pid < "$GPU_STATE/server.pid"
    else read -r pid < "$GPU_STATE/launcher.pid"; fi
    if [[ -r "$GPU_STATE/server.port" && -r "$GPU_STATE/curl.conf" ]]; then
      read -r port < "$GPU_STATE/server.port"
      case "$port" in ''|*[!0-9]*) port=0;; esac
      if (( port > 0 && port < 65536 )); then
        curl --config "$GPU_STATE/curl.conf" --noproxy '*' --connect-timeout 1 --max-time 5 -fsS -X POST \
          "http://127.0.0.1:$port/v1/shutdown" >/dev/null 2>&1 || true
      fi
    fi
    for ((i=0;i<30;i++)); do gpu_server_alive || break; sleep 0.1; done
    # Only the exact worker command and this VM's private state may be stopped.
    if gpu_server_alive; then kill -TERM "$pid" || return; fi
    for ((i=0;i<30;i++)); do gpu_server_alive || break; sleep 0.1; done
    gpu_server_alive && { echo 'No se confirmó el cierre del servicio GPU' >&2; return 1; }
  fi
  if gpu_tunnel_alive; then
    ssh -F /dev/null -S "$GPU_STATE/tunnel.sock" -O exit root@127.0.0.1 >/dev/null 2>&1 || true
  fi
  rm -f "$GPU_STATE/token" "$GPU_STATE/curl.conf" "$GPU_STATE/server.port" \
    "$GPU_STATE/server.pid" "$GPU_STATE/launcher.pid" "$GPU_STATE/server.ready" "$GPU_STATE/server.lock" "$GPU_STATE/tunnel.sock"
}
gpu_disable() {
  gpu_stop_service || return
  if running; then
    ssh_vm 'rm -f /etc/voxy-gpu/token /etc/voxy-gpu/curl.conf /etc/voxy-gpu/url' >/dev/null 2>&1 || true
  fi
  printf '{"sharing":false}\n'
}
gpu_enable_inner() {
  local qpid child i port token sshport=$PORT
  running || { echo 'Inicia Debian antes de compartir la GPU.' >&2; return 1; }
  gpu_worker_required || return
  "$GPU_BINARY" probe --require >/dev/null || return
  wait_ready || return
  gpu_stop_service || return
  (umask 077; mkdir -p "$GPU_STATE")
  chmod 700 "$GPU_STATE"
  read -r qpid < "$STATE/qemu.pid"
  (umask 077; exec nohup "$GPU_BINARY" serve --state-dir "$GPU_STATE" --watch-pid "$qpid" \
    > "$GPU_STATE/server.log" 2>&1 < /dev/null) &
  child=$!
  (umask 077; printf '%s\n' "$child" > "$GPU_STATE/launcher.pid")
  for ((i=0;i<150;i++)); do
    [[ -f "$GPU_STATE/server.ready" ]] && break
    kill -0 "$child" 2>/dev/null || { echo 'El servicio GPU no arrancó; consulta gpu/server.log.' >&2; return 1; }
    sleep 0.1
  done
  [[ -f "$GPU_STATE/server.ready" ]] && gpu_server_alive || { echo 'Tiempo de espera agotado al iniciar GPU' >&2; return 1; }
  read -r port < "$GPU_STATE/server.port"
  read -r token < "$GPU_STATE/token"
  [[ "$port" =~ ^[0-9]+$ && "$token" =~ ^[0-9a-f]{64}$ ]] || { echo 'Credenciales GPU inválidas' >&2; return 1; }
  (( port > 0 && port < 65536 )) || return 1
  (umask 077; printf 'header = "Authorization: Bearer %s"\n' "$token" > "$GPU_STATE/curl.conf")
  ssh_vm 'umask 077; mkdir -p /etc/voxy-gpu; chmod 700 /etc/voxy-gpu' || return
  cat "$GPU_STATE/token" | ssh_vm 'umask 077; cat > /etc/voxy-gpu/token' || return
  cat "$GPU_STATE/curl.conf" | ssh_vm 'umask 077; cat > /etc/voxy-gpu/curl.conf' || return
  printf 'http://127.0.0.1:%s\n' "$GPU_GUEST_PORT" | ssh_vm 'umask 077; cat > /etc/voxy-gpu/url' || return
  cat "$ROOT/lib/gpu-client.sh" | ssh_vm 'umask 022; cat > /usr/local/bin/voxy-gpu; chmod 755 /usr/local/bin/voxy-gpu' || return
  [[ ! -f "$STATE/ssh-port" ]] || read -r sshport < "$STATE/ssh-port"
  ssh -F /dev/null -i "$STATE/id_ed25519" -p "$sshport" -o IdentitiesOnly=yes -o BatchMode=yes \
    -o ConnectTimeout=5 -o ServerAliveInterval=10 -o ServerAliveCountMax=3 \
    -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile="$STATE/known_hosts" \
    -o ExitOnForwardFailure=yes -M -S "$GPU_STATE/tunnel.sock" -fNT \
    -R "127.0.0.1:$GPU_GUEST_PORT:127.0.0.1:$port" root@127.0.0.1 || return
  ssh_vm '/usr/local/bin/voxy-gpu info' >/dev/null || return
  gpu_shared
}
gpu_enable() {
  if gpu_shared; then gpu_status; return; fi
  if ! gpu_enable_inner; then
    gpu_stop_service || true
    echo 'No se habilitó el acceso compartido a GPU.' >&2
    return 1
  fi
  gpu_status
}
gpu_command() {
  case "${1:-status}" in
    status|info) gpu_status;;
    enable) gpu_enable;;
    disable) gpu_disable;;
    test|self-test)
      gpu_shared || { echo 'Activa primero voxy gpu enable.' >&2; return 1; }
      ssh_vm '/usr/local/bin/voxy-gpu self-test';;
    run)
      gpu_worker_required || return
      [[ "$#" = 2 ]] || { echo 'Uso: voxy gpu run archivo.json|-' >&2; return 1; }
      if [[ "$2" = - ]]; then "$GPU_BINARY" compute; else "$GPU_BINARY" compute < "$2"; fi;;
    *) echo 'Uso: voxy gpu {status|enable|disable|test|run archivo.json|-}' >&2; return 1;;
  esac
}
