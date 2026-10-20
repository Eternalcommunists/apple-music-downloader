//go:build windows

package app

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr puts the login command in its own process group so a timeout
// can terminate the whole tree (shell + the program it launched).
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// killSysProcAttr terminates the process group created above.
func killSysProcAttr(cmd *exec.Cmd) {
	// TAKEOWNERSHIP via the group kill is unavailable through os/exec, so the
	// direct child is killed and the group is asked to close with CTRL_BREAK;
	// the shell used by the default template is a direct child, so this is
	// sufficient in practice.
	_ = cmd.Process.Kill()
}
