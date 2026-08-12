package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newTestRepo creates a throwaway git repository in a temp directory with
// one commit containing a single tracked file, and returns its path.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	run("commit", "-m", "initial commit")

	return dir
}

func TestHead_NoCommits(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	if _, err := Head(dir); err != ErrNoCommits {
		t.Fatalf("Head() error = %v, want ErrNoCommits", err)
	}
}

func TestHead_NotARepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := Head(dir); err != ErrNotARepo {
		t.Fatalf("Head() error = %v, want ErrNotARepo", err)
	}
}

func TestHead_ReturnsCommit(t *testing.T) {
	dir := newTestRepo(t)
	head, err := Head(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(head) != 40 {
		t.Fatalf("Head() = %q, want a 40-char sha", head)
	}
}

func TestDiffFingerprint_StableWhenUnchanged(t *testing.T) {
	dir := newTestRepo(t)

	fp1, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 != fp2 {
		t.Fatalf("fingerprint changed with no edits: %q vs %q", fp1, fp2)
	}
}

func TestDiffFingerprint_ChangesOnSingleSpaceEdit(t *testing.T) {
	dir := newTestRepo(t)

	before, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Append a single space to the tracked file: the smallest possible edit.
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}

	if before == after {
		t.Fatal("fingerprint did not change after a single-space edit to a tracked file")
	}
}

func TestDiffFingerprint_ChangesOnNewUntrackedFile(t *testing.T) {
	dir := newTestRepo(t)

	before, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "new_untracked.txt"), []byte("surprise\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}

	if before == after {
		t.Fatal("fingerprint did not change after adding a new untracked file")
	}
}

func TestDiffFingerprint_ChangesOnStagedEdit(t *testing.T) {
	dir := newTestRepo(t)

	before, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "tracked.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	after, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}

	if before == after {
		t.Fatal("fingerprint did not change after staging an edit")
	}
}

func TestDiffFingerprint_IgnoresGitignoredFiles(t *testing.T) {
	dir := newTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".gitignore")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "add gitignore")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	before, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("should not count\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}

	if before != after {
		t.Fatal("fingerprint changed after editing a gitignored file — it should have been ignored")
	}
}

func TestDiffFingerprint_ExcludesGivenTopLevelDir(t *testing.T) {
	dir := newTestRepo(t)

	before, err := DiffFingerprint(dir, ".proofrun")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate proofrun writing its own receipt into .proofrun/ as an
	// untracked file — this must not shift the fingerprint, or every run
	// would invalidate the very result it just recorded.
	if err := os.MkdirAll(filepath.Join(dir, ".proofrun"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".proofrun", "receipt.json"), []byte(`{"schema":"proofrun/v1"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := DiffFingerprint(dir, ".proofrun")
	if err != nil {
		t.Fatal(err)
	}

	if before != after {
		t.Fatal("fingerprint changed after writing into an excluded top-level directory")
	}

	// Sanity check: without the exclusion, it does change.
	withoutExclusion, err := DiffFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if withoutExclusion == before {
		t.Fatal("expected fingerprint without exclusion to differ once .proofrun/ has untracked content")
	}
}

func TestIsRepo(t *testing.T) {
	repo := newTestRepo(t)
	if !IsRepo(repo) {
		t.Fatal("IsRepo() = false for a real repo")
	}

	notRepo := t.TempDir()
	if IsRepo(notRepo) {
		t.Fatal("IsRepo() = true for a non-repo directory")
	}
}
