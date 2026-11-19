package main

import "syscall"

const ioctlGet, ioctlSet = syscall.TIOCGETA, syscall.TIOCSETA
