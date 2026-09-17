# Bash 3.2 compatible. The caller owns STATE, ROOT, PORT and the control lock.
ports_read() {
  local file=${1:-$STATE/ports.conf}
  [[ -e "$file" ]] || file=/dev/null
  awk -v ssh_port="$PORT" -f "$ROOT/lib/ports.awk" "$file"
}
ports_print() {
  awk 'NF==4 {printf "  %s %s:%s -> Debian:%s\n",$1,$2,$3,$4; n++} END {if(!n) print "  (ninguno)"}'
}
ports_usage() {
  echo 'Uso: ports list | add tcp|udp HOST[-FIN] DEBIAN[-FIN] [IPv4] | remove tcp|udp HOST[-FIN] [IPv4] | clear'
  echo 'IPv4 por defecto: 127.0.0.1; 0.0.0.0: todas las interfaces. Máximo 256 puertos.'
}
ports_command() {
  local action=${1:-list} rules file tmp protocol host_range guest_range ip
  [[ $# = 0 ]] || shift
  case "$action" in
    list)
      [[ $# = 0 ]] || { ports_usage; return 1; }
      rules=$(ports_read) || return
      echo 'Configurados para el próximo arranque:'
      printf '%s\n' "$rules" | ports_print
      if running; then
        echo 'Activos en la VM actual:'
        if [[ -f "$STATE/ports.active" ]]; then ports_print < "$STATE/ports.active"; else printf '\n' | ports_print; fi
      fi
      echo "Archivo: $STATE/ports.conf"
      return;;
    menu)
      ports_command list || true
      ports_usage
      printf 'Operación (ejemplo: add tcp 33033 33033 0.0.0.0; Enter cancela): '
      local fields=()
      read -r -a fields || return 1
      [[ ${#fields[@]} = 0 ]] || ports_command "${fields[@]}"
      return;;
    add)
      [[ $# = 3 || $# = 4 ]] || { ports_usage; return 1; }
      protocol=$1; host_range=$2; guest_range=$3; ip=${4:-127.0.0.1};;
    remove)
      [[ $# = 2 || $# = 3 ]] || { ports_usage; return 1; }
      protocol=$1; host_range=$2; guest_range=0; ip=${3:-127.0.0.1};;
    clear) [[ $# = 0 ]] || { ports_usage; return 1; };;
    *) ports_usage; return 1;;
  esac
  file="$STATE/ports.conf"
  [[ -e "$file" ]] || file=/dev/null
  tmp=$(mktemp "$STATE/.ports.XXXXXX") || return
  if [[ "$action" != clear ]]; then
    if ! awk -v ssh_port="$PORT" -v mode="$action" -v protocol="$protocol" -v host_range="$host_range" -v guest_range="$guest_range" -v ip="$ip" -f "$ROOT/lib/ports.awk" "$file" > "$tmp"; then
      rm -f "$tmp"; return 1
    fi
  fi
  chmod 600 "$tmp"
  mv -f "$tmp" "$STATE/ports.conf"
  echo 'Puertos guardados. Se aplican al apagar e iniciar Debian.'
}
ports_netdev() {
  local rules=$1 protocol ip host guest
  NETDEV="user,id=net,hostfwd=tcp:127.0.0.1:$PORT-:22"
  while read -r protocol ip host guest; do
    [[ -n "$protocol" ]] || continue
    NETDEV="$NETDEV,hostfwd=$protocol:$ip:$host-:$guest"
  done <<< "$rules"
}
