package main

import "syscall"

// QEMU keeps running when the menu/console closes. No -daemonize on Windows.
var windowsSysProcAttr = syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200, HideWindow: true}
