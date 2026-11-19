//go:build darwin || linux

package main

import (
	"syscall"
	"unsafe"
)

// noEcho turns terminal echo off on fd and returns the undo. It does nothing when fd is not a terminal.
func noEcho(fd int) (restore func()) {
	var t syscall.Termios
	if ioctl(fd, ioctlGet, &t) != nil {
		return func() {}
	}
	old := t
	t.Lflag &^= syscall.ECHO
	if ioctl(fd, ioctlSet, &t) != nil {
		return func() {}
	}
	return func() { ioctl(fd, ioctlSet, &old) }
}

func ioctl(fd int, req uintptr, t *syscall.Termios) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(t))); e != 0 {
		return e
	}
	return nil
}
