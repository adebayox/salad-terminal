//go:build !windows

package harness

import (
	"os/exec"
	"syscall"
)

// prepareProcessGroup gives the carrier and every process it starts one
// private process group. This lets Ctrl-C clean up a dev server or test
// command as well as the carrier itself.
func prepareProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err == nil {
		return nil
	}
	return cmd.Process.Kill()
}
