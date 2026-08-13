//go:build !windows

package runner

import (
	"os"
	"os/exec"
	"syscall"
)

// SetProcessGroup puts the child in its own process group so KillTree can
// signal the whole group at once instead of just the direct child.
func SetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillTree signals the process group rooted at p, killing descendants too
// (see the Windows implementation's doc comment for why this matters, and
// for why this is exported).
func KillTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}
