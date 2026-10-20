//go:build !windows

package app

import (
	"os"

	"golang.org/x/term"
)

// Console detection on non-Windows platforms.
//
// A process started from a terminal has stdin (or stdout) connected to a tty.
// A process started by double-clicking a file in a desktop file manager has no
// controlling terminal, so neither stream is a tty. That difference is what
// lets the launcher treat a double-click as "show the GUI" without needing a
// flag.

// hasConsole reports whether stdin or stdout is attached to a terminal.
func hasConsole() bool {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// showMessageBox is a no-op on platforms without a Win32 message box. The
// launcher only reaches it from the no-console path, so this exists so the
// shared reporting helper compiles everywhere.
func showMessageBox(title, text string) {}
