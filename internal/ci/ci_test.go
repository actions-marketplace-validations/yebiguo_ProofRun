package ci

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yebiguo/proofrun/internal/config"
	"github.com/yebiguo/proofrun/internal/receipt"
)

// newTestRepo creates a throwaway git repository with one committed file
// and returns its path. Mirrors the helper used by internal/git and
// internal/receipt's own tests.
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

func writeConfig(t *testing.T, dir string, cfg *config.Config) {
	t.Helper()
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}
}

func exitCommand(code int) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/C", "exit", "/B", itoa(code)}
	}
	return []string{"sh", "-c", "exit " + itoa(code)}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestRunAll_ExecutesAllDeclaredChecks(t *testing.T) {
	dir := newTestRepo(t)
	writeConfig(t, dir, &config.Config{Checks: map[string]config.Check{
		"a": {Command: exitCommand(0), Required: true},
		"b": {Command: exitCommand(0), Required: true},
	}})

	var stdout, stderr bytes.Buffer
	outcomes, err := RunAll(context.Background(), dir, 0, nil, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("got %d outcomes, want 2", len(outcomes))
	}
	for _, o := range outcomes {
		if o.Failed() {
			t.Fatalf("check %q unexpectedly failed: %+v", o.Name, o)
		}
	}

	r, err := receipt.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Checks["a"]; !ok {
		t.Fatal("receipt missing check \"a\"")
	}
	if _, ok := r.Checks["b"]; !ok {
		t.Fatal("receipt missing check \"b\"")
	}
}

func TestRunAll_ContinuesAfterFailingCheck(t *testing.T) {
	dir := newTestRepo(t)
	// "a" sorts before "b" and "c" alphabetically, so it fails first —
	// b and c must still run and still get real, correctly-recorded
	// results, not be silently skipped.
	writeConfig(t, dir, &config.Config{Checks: map[string]config.Check{
		"a": {Command: exitCommand(1), Required: true},
		"b": {Command: exitCommand(0), Required: true},
		"c": {Command: exitCommand(0), Required: true},
	}})

	var stdout, stderr bytes.Buffer
	outcomes, err := RunAll(context.Background(), dir, 0, nil, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 3 {
		t.Fatalf("got %d outcomes, want 3 — a failing check must not stop the rest from running", len(outcomes))
	}

	byName := map[string]CheckOutcome{}
	for _, o := range outcomes {
		byName[o.Name] = o
	}
	if !byName["a"].Failed() {
		t.Fatal("check \"a\" should have failed (exit 1)")
	}
	if byName["b"].Failed() {
		t.Fatal("check \"b\" should have passed")
	}
	if byName["c"].Failed() {
		t.Fatal("check \"c\" should have passed")
	}

	r, err := receipt.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Checks["a"].Status != receipt.StatusFail {
		t.Fatalf("stored status for \"a\" = %q, want fail", r.Checks["a"].Status)
	}
	if r.Checks["b"].Status != receipt.StatusPass {
		t.Fatalf("stored status for \"b\" = %q, want pass", r.Checks["b"].Status)
	}
	if r.Checks["c"].Status != receipt.StatusPass {
		t.Fatalf("stored status for \"c\" = %q, want pass", r.Checks["c"].Status)
	}
}

func TestRunAll_ZeroChecksDeclaredErrors(t *testing.T) {
	dir := newTestRepo(t)
	writeConfig(t, dir, &config.Config{Checks: map[string]config.Check{}})

	var stdout, stderr bytes.Buffer
	_, err := RunAll(context.Background(), dir, 0, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("RunAll() error = nil, want an error when .proofrun.yml declares zero checks — an empty set trivially satisfies --strict")
	}
}

func TestRunAll_MissingConfigErrors(t *testing.T) {
	dir := newTestRepo(t)
	var stdout, stderr bytes.Buffer
	_, err := RunAll(context.Background(), dir, 0, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("RunAll() error = nil, want an error when .proofrun.yml doesn't exist")
	}
}

func TestRunAll_OnlyFilterRestrictsToNamedChecks(t *testing.T) {
	dir := newTestRepo(t)
	writeConfig(t, dir, &config.Config{Checks: map[string]config.Check{
		"a": {Command: exitCommand(0), Required: true},
		"b": {Command: exitCommand(1), Required: true},
	}})

	var stdout, stderr bytes.Buffer
	outcomes, err := RunAll(context.Background(), dir, 0, map[string]bool{"a": true}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0].Name != "a" {
		t.Fatalf("outcomes = %+v, want exactly one outcome for \"a\"", outcomes)
	}

	r, err := receipt.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Checks["b"]; ok {
		t.Fatal("receipt has an entry for \"b\", which --only should have excluded from running")
	}
}

func TestRunAll_OnlyFilterMatchingNothingErrors(t *testing.T) {
	dir := newTestRepo(t)
	writeConfig(t, dir, &config.Config{Checks: map[string]config.Check{
		"a": {Command: exitCommand(0), Required: true},
	}})

	var stdout, stderr bytes.Buffer
	_, err := RunAll(context.Background(), dir, 0, map[string]bool{"does-not-exist": true}, &stdout, &stderr)
	if err == nil {
		t.Fatal("RunAll() error = nil, want an error when --only matches no declared check")
	}
}

// TestRunAll_ChecksThatMutateTheTreeDontPoisonLaterOnes proves the fix for a
// real false-PASS: a check earlier in the run (e.g. codegen, fixture setup)
// can change the working tree before a later check runs. If every check in
// the pass were bound to one fingerprint computed up front, a later check
// that actually ran against the mutated tree would still be recorded (and
// keep reading back as PASS) against the pre-mutation state — a stored
// result for code the check never actually saw.
func TestRunAll_ChecksThatMutateTheTreeDontPoisonLaterOnes(t *testing.T) {
	dir := newTestRepo(t)

	var mutate, restore []string
	if runtime.GOOS == "windows" {
		mutate = []string{"cmd", "/C", "echo mutated>>f.txt"}
		restore = []string{"git", "checkout", "--", "f.txt"}
	} else {
		mutate = []string{"sh", "-c", "echo mutated >> f.txt"}
		restore = []string{"git", "checkout", "--", "f.txt"}
	}

	writeConfig(t, dir, &config.Config{Checks: map[string]config.Check{
		"a-mutate":  {Command: mutate, Required: true},
		"b-observe": {Command: exitCommand(0), Required: true},
		"c-restore": {Command: restore, Required: true},
	}})

	var stdout, stderr bytes.Buffer
	if _, err := RunAll(context.Background(), dir, 0, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	r, err := receipt.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	// "a-mutate" ran before it changed the tree, and "b-observe" ran after
	// — they must be bound to different fingerprints, not the same one.
	if r.Checks["a-mutate"].VerifiedAgainst == r.Checks["b-observe"].VerifiedAgainst {
		t.Fatalf("a-mutate and b-observe are bound to the same fingerprint, but the tree changed between them: %+v",
			r.Checks["a-mutate"].VerifiedAgainst)
	}

	// By the time RunAll finished, "c-restore" had put the tree back to its
	// original state — the same state "a-mutate" (but not "b-observe") ran
	// against. Evaluating "b-observe" against the actual final tree state
	// must report STALE, not PASS: its stored result is evidence about the
	// mutated tree, not the tree that exists now.
	finalFP, err := currentFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	eval := receipt.Evaluate(r, "b-observe", finalFP)
	if eval.Status != receipt.Stale {
		t.Fatalf("b-observe status = %v, want Stale (it ran against a tree state that no longer exists)", eval.Status)
	}
}
