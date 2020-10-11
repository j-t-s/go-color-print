package main

// winsize code from https://stackoverflow.com/questions/16569433/get-terminal-size-in-go#16576712

import (
	"syscall"
	"unsafe"
)

type WinSize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getWinSize() WinSize {
	ws := &WinSize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		panic(errno)
	}
	return *ws
}
