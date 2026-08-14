package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/yebiguo/proofrun/internal/config"
	"github.com/yebiguo/proofrun/internal/receipt"
	"github.com/yebiguo/proofrun/internal/runner"
)

// buildProofrun compiles the real CLI binary once per test run and returns
// its path. Used by tests that need to exercise the actual process
// lifecycle (signals, exit codes) rather than calling Go functions
// in-process.
func buildProofrun(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "proofrun")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building proofrun: %v\n%s", err, out)
	}
	return bin
}

func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "f.txt")
	run("commit", "-m", "initial commit")

	return dir
}

// TestRunAll_KilledMidRun_KeepsAlreadyCompletedResults is the real-process
// version of the acceptance criterion in the v0.2 plan: a check named "a"
// (alphabetically first, so it runs and finishes before anything else)
// must survive on disk even if the whole `proofrun run-all` process is
// killed while a later check ("b") is still running. This is exactly what
// happens when a CI job hits its timeout mid-way through a check suite —
// whatever already passed shouldn't vanish along with the run.
func TestRunAll_KilledMidRun_KeepsAlreadyCompletedResults(t *testing.T) {
	bin := buildProofrun(t)
	dir := newTestRepo(t)

	var sleepCmd, quickCmd []string
	if runtime.GOOS == "windows" {
		quickCmd = []string{"cmd", "/C", "exit", "/B", "0"}
		sleepCmd = []string{"cmd", "/C", "ping", "-n", "30", "127.0.0.1"}
	} else {
		quickCmd = []string{"true"}
		sleepCmd = []string{"sleep", "30"}
	}

	cfg := &config.Config{Checks: map[string]config.Check{
		"a": {Command: quickCmd, Required: true},
		"b": {Command: sleepCmd, Required: true},
		"c": {Command: quickCmd, Required: true},
	}}
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "run-all")
	cmd.Dir = dir
	// Real CI systems tear down the whole process tree/cgroup when a job
	// is killed, not just one PID — a plain cmd.Process.Kill() here would
	// only kill the outer proofrun process and orphan whatever child
	// command it was running (check "b"'s sleep), which is a less
	// realistic simulation and, concretely, left a lingering process
	// holding a file handle that broke t.TempDir()'s cleanup on Windows.
	runner.SetProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting proofrun run-all: %v", err)
	}

	// "a" is a near-instant command and sorts first; by the time "b" (a
	// long sleep) has been running for a bit, "a" must already be done
	// and saved. Give it generous slack — this only needs to prove
	// mid-run survival, not race a tight timing window.
	time.Sleep(2 * time.Second)

	if err := runner.KillTree(cmd.Process); err != nil {
		t.Fatalf("killing proofrun run-all's process tree: %v", err)
	}
	_ = cmd.Wait() // expected to report a kill-related error; not asserted on

	data, err := os.ReadFile(filepath.Join(dir, ".proofrun", "receipt.json"))
	if err != nil {
		t.Fatalf("reading receipt.json after kill: %v", err)
	}
	var r receipt.Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("receipt.json is not valid JSON after kill: %v\n%s", err, data)
	}

	a, ok := r.Checks["a"]
	if !ok {
		t.Fatal("check \"a\" (completed before the kill) is missing from the receipt — a mid-run kill lost already-finished work")
	}
	if a.Status != receipt.StatusPass {
		t.Fatalf("check \"a\" status = %q, want pass", a.Status)
	}

	if _, ok := r.Checks["c"]; ok {
		t.Fatal("check \"c\" is present in the receipt, but it should never have started (killed during \"b\")")
	}
}
