//go:build windows

package harness

import (
	"os/exec"
	"strconv"
)

// Windows has no portable process-group primitive in os/exec. taskkill's
// /T flag is the platform's built-in process-tree termination mechanism.
func prepareProcessGroup(_ *exec.Cmd) {}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err == nil {
		return nil
	}
	return cmd.Process.Kill()
}
