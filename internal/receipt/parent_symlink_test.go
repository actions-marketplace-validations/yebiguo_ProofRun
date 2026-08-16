package receipt

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yebiguo/proofrun/internal/git"
)

// TestLoadOrCreateSecret_ParentSymlinkToTrackedKeyIsRejected reproduces the
// MUST FIX a review round caught: a symlinked DirName (".proofrun ->
// planted") lets a repository plant a known, git-tracked key at
// "planted/secret" while it's invisible to git.IsTracked's exact-path
// query for ".proofrun/secret" — Lstat on the final path component
// resolves straight through the symlinked parent and sees an ordinary
// regular file, and IsTracked checks the wrong (logical, not real) path
// string. Confirmed standalone before writing this test: a small program
// calling LoadOrCreateSecret directly against exactly this setup returned
// the attacker's planted key.
//
// Once an attacker's known key gets adopted as "this machine's" signing
// key, they can forge signatures that verify perfectly for any commit they
// can predict — completely independent of, and much simpler than, trying
// to make a single receipt.json self-referentially embed its own commit
// hash. Key compromise alone is the severity-critical failure; nothing
// downstream needs to be exploited further to justify treating this as a
// MUST FIX.
func TestLoadOrCreateSecret_ParentSymlinkToTrackedKeyIsRejected(t *testing.T) {
	dir := newTestRepo(t)

	plantedDir := filepath.Join(dir, "planted")
	if err := os.MkdirAll(plantedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	knownKey := bytes.Repeat([]byte("C"), secretKeyLen)
	if err := os.WriteFile(filepath.Join(plantedDir, "secret"), knownKey, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(plantedDir, filepath.Join(dir, DirName)); err != nil {
		t.Skipf("cannot create symlinks in this environment: %v", err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "-A")
	run("commit", "-m", "adversarial: planted .proofrun symlink + tracked known key")

	// Sanity-check the exact bypass mechanism this test exists to close:
	// the resolved file is genuinely tracked, but not at the logical path
	// git.IsTracked queries.
	if git.IsTracked(dir, DirName+"/"+SecretFileName) {
		t.Fatal("test setup invalid: IsTracked(.proofrun/secret) unexpectedly true — this test needs it to be false to reproduce the bypass")
	}

	key, err := LoadOrCreateSecret(dir)
	if err == nil && bytes.Equal(key, knownKey) {
		t.Fatal("LoadOrCreateSecret adopted the attacker's known key via a symlinked parent directory")
	}
	// Either erroring outright (the actual fix's behavior — reject the
	// symlinked DirName before ever looking inside it) or generating some
	// other, unknown key would both be acceptable outcomes; only silently
	// returning the attacker's exact known key is the failure this test
	// checks for.
}

// TestLoad_ParentSymlinkIsRejected is the receipt.json half of the same
// bypass: DirName being a symlink would otherwise let Load silently read a
// receipt.json from outside the project too.
func TestLoad_ParentSymlinkIsRejected(t *testing.T) {
	dir := newTestRepo(t)
	outside := t.TempDir()

	if err := os.WriteFile(filepath.Join(outside, FileName), []byte(`{"schema":"proofrun/v2","checks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, DirName)); err != nil {
		t.Skipf("cannot create symlinks in this environment: %v", err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("Load should refuse to read through a symlinked .proofrun/, but returned no error")
	}
}

// TestSave_SchemaAlwaysNormalizedToCurrentVersion covers a SHOULD FIX from
// the same review round: Save used to only fill in Schema when it was
// empty, so a Receipt loaded from an older on-disk schema (e.g.
// "proofrun/v1", from before signing existed) and then re-saved kept
// carrying that stale marker forward — even though the content Save
// actually wrote was, by construction, always in the current signed
// format. Schema is a human-readable label, not something verification
// branches on, but a stale one is still misleading to read.
func TestSave_SchemaAlwaysNormalizedToCurrentVersion(t *testing.T) {
	r := &Receipt{Schema: "proofrun/v1", Checks: map[string]CheckResult{}}
	dir := t.TempDir()

	if err := r.Save(dir); err != nil {
		t.Fatal(err)
	}
	if r.Schema != SchemaVersion {
		t.Fatalf("Schema after Save = %q, want %q", r.Schema, SchemaVersion)
	}

	reloaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Schema != SchemaVersion {
		t.Fatalf("on-disk Schema after Save = %q, want %q", reloaded.Schema, SchemaVersion)
	}
}
