//go:build windows

package builder

import (
	"fmt"
	"os/exec"
)

// killProcessGroup kills the process and its children on Windows.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Taskkill kills the process tree
	exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
}

// setupProcessGroup is a no-op on Windows.
func setupProcessGroup(cmd *exec.Cmd) {
	// Windows handles process groups differently
}
