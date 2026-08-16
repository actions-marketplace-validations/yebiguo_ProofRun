package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func infoExcludePath(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--git-path", "info/exclude")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse --git-path info/exclude: %v\n%s", err, out)
	}
	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return path
}

func TestEnsureIgnored_AddsToInfoExclude(t *testing.T) {
	dir := newTestRepo(t)

	EnsureIgnored(dir, ".proofrun/")

	data, err := os.ReadFile(infoExcludePath(t, dir))
	if err != nil {
		t.Fatalf("reading info/exclude: %v", err)
	}
	if !strings.Contains(string(data), ".proofrun/") {
		t.Fatalf("info/exclude does not mention .proofrun/, got:\n%s", data)
	}
}

func TestEnsureIgnored_IdempotentNoDuplicateLines(t *testing.T) {
	dir := newTestRepo(t)

	EnsureIgnored(dir, ".proofrun/")
	EnsureIgnored(dir, ".proofrun/")
	EnsureIgnored(dir, ".proofrun/")

	data, err := os.ReadFile(infoExcludePath(t, dir))
	if err != nil {
		t.Fatalf("reading info/exclude: %v", err)
	}
	count := strings.Count(string(data), ".proofrun/")
	if count != 1 {
		t.Fatalf("info/exclude has %d occurrences of .proofrun/, want exactly 1:\n%s", count, data)
	}
}

func TestEnsureIgnored_NotARepoIsSilentNoop(t *testing.T) {
	dir := t.TempDir()
	// Must not panic or otherwise misbehave outside a git repo.
	EnsureIgnored(dir, ".proofrun/")
}

func TestEnsureIgnored_ActuallyMakesGitIgnoreThePath(t *testing.T) {
	dir := newTestRepo(t)

	EnsureIgnored(dir, ".proofrun/")

	if err := os.MkdirAll(filepath.Join(dir, ".proofrun"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".proofrun", "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	if strings.Contains(string(out), ".proofrun") {
		t.Fatalf("git status still shows .proofrun/ as untracked after EnsureIgnored:\n%s", out)
	}
}
