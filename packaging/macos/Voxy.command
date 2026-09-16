#!/bin/bash
export LC_ALL=C
cd "$(dirname "$0")" || exit 1
printf '\nVoxy — Debian mínimo con QEMU\n'
printf 'Los datos se conservan en ~/Library/Application Support/Voxy.\n'
while true; do
  printf '\n'
  ./voxy status
  printf '\n1) Iniciar Debian\n2) Abrir terminal Debian\n3) Apagar Debian\n4) Cerrar panel (la VM continúa si está encendida)\n> '
  IFS= read -r choice || exit 0
  case "$choice" in
    1) arch=$(uname -m); [ "$arch" != x86_64 ] || arch=amd64
       data=${VOXY_DATA_DIR:-"$HOME/Library/Application Support/Voxy/$arch"}
       if [ ! -f "$data/disk.qcow2" ]; then ./voxy init || continue; fi
       ./voxy start && ./voxy wait;;
    2) ./voxy ssh -t;;
    3) ./voxy stop;;
    4) exit 0;;
    *) printf 'Selecciona una opción del 1 al 4.\n';;
  esac
done
