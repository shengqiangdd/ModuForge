//go:build !windows

package builder

import (
	"os/exec"
	"syscall"
)

// killProcessGroup sends SIGKILL to the entire process group of the given command.
// This ensures child processes (like go tool compile) are also terminated.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Kill the entire process group
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// setupProcessGroup sets up the command to create a new process group
// so we can kill the entire group on timeout.
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
