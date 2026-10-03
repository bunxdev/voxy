#!/bin/sh
# Guest client: the credential stays in a private curl config, never in argv.
set -eu
config=${VOXY_GPU_CONFIG_DIR:-/etc/voxy-gpu}
if [ ! -r "$config/url" ] || [ ! -r "$config/curl.conf" ]; then
  echo 'GPU compartida no habilitada; ejecuta voxy gpu enable en el anfitrión.' >&2
  exit 1
fi
IFS= read -r endpoint < "$config/url"
case "$endpoint" in http://127.0.0.1:47832) ;; *) echo 'Endpoint GPU inválido' >&2; exit 1;; esac
case "${1:-info}" in
  info|status) exec curl --config "$config/curl.conf" --noproxy '*' --connect-timeout 3 --max-time 70 --fail-with-body --silent --show-error "$endpoint/v1/info";;
  self-test|test) exec curl --config "$config/curl.conf" --noproxy '*' --connect-timeout 3 --max-time 70 --fail-with-body --silent --show-error -H 'Content-Type: application/json' --data '{}' "$endpoint/v1/self-test";;
  compute)
    [ "$#" -eq 2 ] || { echo 'Uso: voxy-gpu compute archivo.json|-' >&2; exit 1; }
    exec curl --config "$config/curl.conf" --noproxy '*' --connect-timeout 3 --max-time 70 --fail-with-body --silent --show-error -H 'Content-Type: application/json' --data-binary "@$2" "$endpoint/v1/compute";;
  *) echo 'Uso: voxy-gpu {info|self-test|compute archivo.json|-}' >&2; exit 1;;
esac
