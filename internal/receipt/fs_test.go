package receipt

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSave_RejectsSymlinkedProofrunDir reproduces the gap a review round
// caught after the per-file symlink defense (secret_test.go) landed:
// writeFileAtomic only protects the final path component. If DirName
// itself — not just the file inside it — is a symlink to somewhere outside
// the project, os.MkdirAll happily follows it (Go's MkdirAll uses Stat,
// which resolves symlinks, to decide a path "already exists" as a
// directory), and every subsequent write lands inside whatever that
// symlink points at instead of the project. A checked-out, untrusted repo
// can track ".proofrun -> /somewhere/else" directly; this must never be
// silently followed, and Save must fail loudly instead of writing anywhere
// outside the project.
func TestSave_RejectsSymlinkedProofrunDir(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir() // simulates "anywhere else on disk"

	if err := os.Symlink(outside, filepath.Join(dir, DirName)); err != nil {
		t.Skipf("cannot create symlinks in this environment: %v", err)
	}

	r := New()
	r.Set("test", CheckResult{Status: StatusPass})
	if err := r.Save(dir); err == nil {
		t.Fatal("Save should refuse to write through a symlinked .proofrun/, but returned no error")
	}

	if _, err := os.Stat(filepath.Join(outside, FileName)); err == nil {
		t.Fatal("receipt.json was created inside the symlink target, outside the project")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the symlink target directory should still be empty, found: %v", entries)
	}
}

func TestLoadOrCreateSecret_RejectsSymlinkedProofrunDir(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	if err := os.Symlink(outside, filepath.Join(dir, DirName)); err != nil {
		t.Skipf("cannot create symlinks in this environment: %v", err)
	}

	if _, err := LoadOrCreateSecret(dir); err == nil {
		t.Fatal("LoadOrCreateSecret should refuse to write through a symlinked .proofrun/, but returned no error")
	}

	if _, err := os.Stat(filepath.Join(outside, SecretFileName)); err == nil {
		t.Fatal("secret was created inside the symlink target, outside the project")
	}
}
