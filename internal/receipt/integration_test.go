package receipt

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yebiguo/proofrun/internal/git"
)

// newTestRepo creates a throwaway git repository with one committed file
// and returns its path. Mirrors internal/git's test helper, but lives here
// too since a cross-package integration test shouldn't depend on another
// package's _test.go internals.
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

	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "app.py")
	run("commit", "-m", "initial commit")

	return dir
}

func currentFingerprint(t *testing.T, dir string) Fingerprint {
	t.Helper()
	head, err := git.Head(dir)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := git.DiffFingerprint(dir, DirName)
	if err != nil {
		t.Fatal(err)
	}
	return Fingerprint{Head: head, DiffSHA256: diff}
}

// TestIntegration_EditingTrackedFileTurnsPassIntoStale is the core
// guarantee the whole project plan calls out by name: run a check, edit a
// tracked file, and proofrun's status logic must report STALE — using the
// real git package end to end, not a hand-built fingerprint.
func TestIntegration_EditingTrackedFileTurnsPassIntoStale(t *testing.T) {
	dir := newTestRepo(t)

	fp := currentFingerprint(t, dir)
	r := New()
	r.Set("test", NewResult([]string{"pytest"}, 0, time.Now(), 0, fp))

	eval := Evaluate(r, "test", currentFingerprint(t, dir))
	if eval.Status != Pass {
		t.Fatalf("before any edit: Status = %q, want %q", eval.Status, Pass)
	}

	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("print('hi') \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eval = Evaluate(r, "test", currentFingerprint(t, dir))
	if eval.Status != Stale {
		t.Fatalf("after a single-space edit: Status = %q, want %q", eval.Status, Stale)
	}
}

// TestIntegration_NewUntrackedFileTurnsPassIntoStale covers the other half
// of "any byte of code changed": a brand new file nobody committed yet.
func TestIntegration_NewUntrackedFileTurnsPassIntoStale(t *testing.T) {
	dir := newTestRepo(t)

	fp := currentFingerprint(t, dir)
	r := New()
	r.Set("test", NewResult([]string{"pytest"}, 0, time.Now(), 0, fp))

	if err := os.WriteFile(filepath.Join(dir, "new_file.py"), []byte("print('surprise')\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eval := Evaluate(r, "test", currentFingerprint(t, dir))
	if eval.Status != Stale {
		t.Fatalf("after adding a new untracked file: Status = %q, want %q", eval.Status, Stale)
	}
}

// TestIntegration_UnrelatedRunDoesNotSelfInvalidate guards against the
// exact bug caught during Day 2 smoke testing: writing receipt.json itself
// must not change the fingerprint used to evaluate the check that was just
// recorded.
func TestIntegration_UnrelatedRunDoesNotSelfInvalidate(t *testing.T) {
	dir := newTestRepo(t)

	fp := currentFingerprint(t, dir)
	r := New()
	r.Set("test", NewResult([]string{"pytest"}, 0, time.Now(), 0, fp))
	if err := r.Save(dir); err != nil {
		t.Fatal(err)
	}

	// Reload as a fresh process would (e.g. `proofrun status` running
	// after `proofrun run` already wrote .proofrun/receipt.json).
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	eval := Evaluate(loaded, "test", currentFingerprint(t, dir))
	if eval.Status != Pass {
		t.Fatalf("Status = %q, want %q — writing receipt.json should not self-invalidate", eval.Status, Pass)
	}
}

// TestIntegration_ForgedCommandCannotSatisfyRequiredCheck reproduces the
// exact CLI-level exploit reported in review: `proofrun run test -- true`
// against a config that declares `test: go test ./...` must not be
// evaluated as PASS, even though `true` genuinely exited 0 against the
// current fingerprint.
func TestIntegration_ForgedCommandCannotSatisfyRequiredCheck(t *testing.T) {
	dir := newTestRepo(t)

	fp := currentFingerprint(t, dir)
	r := New()
	r.Set("test", NewResult([]string{"true"}, 0, time.Now(), 0, fp))

	eval := EvaluateAgainstCommand(r, "test", currentFingerprint(t, dir), []string{"go", "test", "./..."})
	if eval.Status == Pass {
		t.Fatal("forged command (`true` instead of the configured `go test ./...`) was reported as PASS")
	}
	if eval.Status != NotRun {
		t.Fatalf("Status = %q, want %q", eval.Status, NotRun)
	}
}

func TestIntegration_NoCommitsYet(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	if _, err := git.Head(dir); err != git.ErrNoCommits {
		t.Fatalf("Head() error = %v, want ErrNoCommits", err)
	}
}
