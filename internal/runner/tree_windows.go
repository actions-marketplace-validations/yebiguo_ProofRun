//go:build windows

package runner

import (
	"os"
	"os/exec"
	"strconv"
)

// setProcessGroup is a no-op on Windows; killTree uses taskkill's /T flag
// instead of a process-group signal.
func setProcessGroup(cmd *exec.Cmd) {}

// killTree kills p and its entire descendant tree. A direct
// cmd.Process.Kill() only terminates the immediate child (e.g. cmd.exe),
// leaving grandchildren (e.g. the node.exe an npm script spawns) running
// and still holding the stdout/stderr pipes open, which then makes
// cmd.Wait() block until those orphans exit on their own — silently
// defeating the timeout. taskkill /T recurses into the whole tree.
func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid)).Run()
}
