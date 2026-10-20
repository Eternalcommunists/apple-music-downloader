//go:build !windows

package app

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr puts the login command in its own process group so a timeout
// can terminate the whole tree (shell + the program it launched).
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killSysProcAttr signals the whole process group, so a shell wrapper and the
// program it spawned both stop.
func killSysProcAttr(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		// Negative pid targets the group.
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	_ = cmd.Process.Kill()
}
