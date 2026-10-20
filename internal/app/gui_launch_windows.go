//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

// Windows console-attachment check and message box.
//
// Telling "double-clicked in Explorer" apart from "typed in a terminal" is the
// whole job of this file, and GetConsoleWindow alone cannot do it: Windows gives
// a *fresh* console to a console-subsystem process started by Explorer, so both
// cases report a console window.
//
// GetConsoleProcessList is the reliable signal. It lists every process attached
// to the caller's console:
//
//	>= 2  the console is shared with a shell  -> started from a terminal
//	   1  the console holds only this process -> got its own, i.e. double-click
//	   0  no console at all                   -> launched by a GUI process
//
// Both non-terminal cases mean the user has nowhere to read printed output, so
// they are treated as "start the GUI" / "show a dialog on failure".
//
// Deliberately no AttachConsole here: attaching to the parent shell's console
// would make a double-clicked run appear to hang in an unrelated terminal
// window, and this program only needs to know whether a console exists.

var (
	modKernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleProcessList = modKernel32.NewProc("GetConsoleProcessList")

	modUser32       = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = modUser32.NewProc("MessageBoxW")
)

// hasConsole reports whether the process shares a console with a shell, which is
// only true when it was started from a terminal.
func hasConsole() bool {
	// A small fixed buffer is enough: the call only needs to distinguish "one
	// process" from "more than one", and the shell is normally the only other
	// entry. The array is oversized relative to that need on purpose.
	pids := make([]uint32, 8)
	n, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(len(pids)),
	)
	return n >= 2
}

// Message box flags: an error icon with an OK button. These are the raw
// constants from the Win32 header; naming them keeps the call readable.
const (
	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbSetForeground = 0x00010000
)

// showMessageBox displays a blocking dialog. It is the only way to report a
// failure to a user who double-clicked the program and has no console.
//
// Failures are swallowed on purpose: a dialog that cannot be shown is not a
// reason to crash, and the caller has nothing better to do with the result.
func showMessageBox(title, text string) {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	m, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	_, _, _ = procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		uintptr(mbOK|mbIconError|mbSetForeground),
	)
}
