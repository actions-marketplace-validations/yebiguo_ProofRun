package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsTracked_TrackedFile(t *testing.T) {
	dir := newTestRepo(t)
	// newTestRepo commits tracked.txt.
	if !IsTracked(dir, "tracked.txt") {
		t.Fatal("IsTracked(tracked.txt) = false, want true — it was committed by newTestRepo")
	}
}

func TestIsTracked_UntrackedFile(t *testing.T) {
	dir := newTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "never-added.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsTracked(dir, "never-added.txt") {
		t.Fatal("IsTracked(never-added.txt) = true, want false — it was never git add'ed")
	}
}

func TestIsTracked_StagedButNotCommittedFile(t *testing.T) {
	dir := newTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "staged.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if !IsTracked(dir, "staged.txt") {
		t.Fatal("IsTracked(staged.txt) = false, want true — staged (even uncommitted) files are tracked")
	}
}

func TestIsTracked_NotARepoReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	if IsTracked(dir, "anything.txt") {
		t.Fatal("IsTracked outside a git repo should return false, not true")
	}
}
