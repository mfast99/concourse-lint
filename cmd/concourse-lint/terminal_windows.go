//go:build windows

package main

import (
	"os"
	"syscall"
)

// enableVirtualTerminalProcessing makes the Windows console interpret
// escape sequences (Windows 10 and later).
const enableVirtualTerminalProcessing = 0x0004

// kernel32.dll is a known DLL: Windows loads it from the system folder
// only, so it cannot be replaced by a file next to the binary.
var setConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")

// enableColors reports whether the file is a console that accepts escape
// sequences, and switches them on if needed. A pipe, a file and terminals
// like mintty (Git Bash), which look like a pipe, get no colors.
func enableColors(file *os.File) bool {
	console := syscall.Handle(file.Fd())
	var mode uint32
	if err := syscall.GetConsoleMode(console, &mode); err != nil {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	succeeded, _, _ := setConsoleMode.Call(uintptr(console), uintptr(mode|enableVirtualTerminalProcessing))
	return succeeded != 0
}
