// Package runner executes a check command as a real subprocess and reports
// what was actually observed: exit code and wall-clock duration. It never
// interprets the command's output — that is out of scope for ProofRun by
// design (see AGENTS.md).
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Result is what ProofRun observed about a single execution.
type Result struct {
	ExitCode  int
	StartedAt time.Time
	Duration  time.Duration
	TimedOut  bool
}

// Run executes command (an already-tokenized argv — no shell is involved)
// in dir, streaming its stdout/stderr to the given writers as it runs and
// capturing its exit code and duration. If timeout is zero, no timeout is
// applied.
//
// Run only returns an error when the command could not be started at all
// (e.g. the binary does not exist). A command that starts and exits
// non-zero is a successful observation — that non-zero code is reported in
// Result.ExitCode, not as a Go error.
func Run(ctx context.Context, dir string, command []string, timeout time.Duration, stdout, stderr io.Writer) (Result, error) {
	if len(command) == 0 {
		return Result{}, errors.New("no command given")
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	SetProcessGroup(cmd)
	cmd.Cancel = func() error { return KillTree(cmd.Process) }
	// Bounds how long Wait can block after cancellation if some descendant
	// process still holds a stdout/stderr pipe open despite KillTree; past
	// this, os/exec forcibly closes the pipes so Wait returns.
	cmd.WaitDelay = 5 * time.Second

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := Result{StartedAt: start, Duration: duration}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
		return result, nil
	}

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		result.ExitCode = 0
	case errors.As(err, &exitErr):
		result.ExitCode = exitErr.ExitCode()
	default:
		// The process never started (bad binary, permissions, etc.) — this
		// is not an observation of the check, so it is not a PASS/FAIL.
		return Result{}, fmt.Errorf("running %v: %w", command, err)
	}

	return result, nil
}
