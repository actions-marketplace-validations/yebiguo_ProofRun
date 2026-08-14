//go:build windows

package runner

import (
	"os"
	"os/exec"
	"strconv"
)

// SetProcessGroup is a no-op on Windows; KillTree uses taskkill's /T flag
// instead of a process-group signal.
func SetProcessGroup(cmd *exec.Cmd) {}

// KillTree kills p and its entire descendant tree. A direct
// cmd.Process.Kill() only terminates the immediate child (e.g. cmd.exe),
// leaving grandchildren (e.g. the node.exe an npm script spawns) running
// and still holding the stdout/stderr pipes open, which then makes
// cmd.Wait() block until those orphans exit on their own — silently
// defeating the timeout. taskkill /T recurses into the whole tree.
//
// Exported (not just used internally by Run's own timeout handling)
// because callers that kill a *proofrun process* from outside — e.g. a
// test simulating a CI job timeout — need the exact same tree-kill, not
// just cmd.Process.Kill() on the top-level PID: that would leave whatever
// check command proofrun itself was running as an orphan.
func KillTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid)).Run()
}
