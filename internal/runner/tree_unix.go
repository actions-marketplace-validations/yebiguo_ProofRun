//go:build !windows

package runner

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so killTree can
// signal the whole group at once instead of just the direct child.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree signals the process group rooted at p, killing descendants too
// (see the Windows implementation's doc comment for why this matters).
func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}
