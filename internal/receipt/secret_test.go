package receipt

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateSecret_GeneratesOnFirstCall(t *testing.T) {
	dir := t.TempDir()

	key, err := LoadOrCreateSecret(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != secretKeyLen {
		t.Fatalf("key length = %d, want %d", len(key), secretKeyLen)
	}
	if _, err := os.Stat(SecretPath(dir)); err != nil {
		t.Fatalf("secret file was not created on disk: %v", err)
	}
}

func TestLoadOrCreateSecret_PersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()

	first, err := LoadOrCreateSecret(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateSecret(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("LoadOrCreateSecret returned different keys on the second call — it should persist and reuse the same key")
	}
}

func TestLoadOrCreateSecret_RegeneratesOnCorruptFile(t *testing.T) {
	dir := t.TempDir()

	if _, err := LoadOrCreateSecret(dir); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(SecretPath(dir))
	if err != nil {
		t.Fatal(err)
	}

	// Simulate corruption: truncate to the wrong length.
	if err := os.WriteFile(SecretPath(dir), []byte("not a valid key"), 0o600); err != nil {
		t.Fatal(err)
	}

	regenerated, err := LoadOrCreateSecret(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSecret should recover from a corrupt key file, not error: %v", err)
	}
	if len(regenerated) != secretKeyLen {
		t.Fatalf("regenerated key length = %d, want %d", len(regenerated), secretKeyLen)
	}
	if bytes.Equal(regenerated, original) {
		t.Fatal("regenerated key unexpectedly matches the pre-corruption key")
	}

	onDisk, err := os.ReadFile(SecretPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, regenerated) {
		t.Fatal("the regenerated key was not persisted to disk")
	}
}

// TestLoadOrCreateSecret_SymlinkToSameLengthFileIsNotTrusted covers the
// first half of the symlink attack a review round caught in the original
// version of this file: a checked-out, untrusted repository pre-plants
// .proofrun/secret as a symlink to some other file the current user can
// read, chosen to be exactly secretKeyLen bytes. The old os.ReadFile
// implementation would follow the symlink and silently accept that
// attacker-chosen content as this machine's signing key — worse than
// trusting nothing, since the attacker would then know the "secret" too and
// could forge signatures freely. This scenario's target file is a valid
// length, so the read path returns early and never reaches the write path;
// see the sibling test below for the write-side half of the same attack.
func TestLoadOrCreateSecret_SymlinkToSameLengthFileIsNotTrusted(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir() // simulates "anywhere else on disk"

	attackerChosenKey := bytes.Repeat([]byte("A"), secretKeyLen) // exactly 32 bytes
	targetPath := filepath.Join(outside, "not-proofruns-file.txt")
	if err := os.WriteFile(targetPath, attackerChosenKey, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, SecretPath(dir)); err != nil {
		t.Skipf("cannot create symlinks in this environment: %v", err)
	}

	key, err := LoadOrCreateSecret(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSecret should recover from a planted symlink, not error: %v", err)
	}

	if bytes.Equal(key, attackerChosenKey) {
		t.Fatal("LoadOrCreateSecret followed the symlink and trusted attacker-chosen content as the signing key")
	}

	afterward, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterward, attackerChosenKey) {
		t.Fatal("the file outside .proofrun/ that the symlink pointed at was modified — LoadOrCreateSecret wrote through the symlink")
	}

	info, err := os.Lstat(SecretPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("the symlink at .proofrun/secret was never replaced with a regular file")
	}
}

// TestLoadOrCreateSecret_SymlinkToWrongLengthFileIsNotWrittenThrough covers
// the write-side half of the same attack: the symlink target is the wrong
// length (judged "malformed", same as a genuinely corrupted key file),
// which is exactly the case that falls through to generating and writing a
// fresh key. The old os.WriteFile implementation opens with O_TRUNC and
// follows symlinks, so it would truncate and overwrite whatever file the
// planted symlink pointed at — anywhere on disk the current user has write
// access to, nothing to do with this project. That file must survive
// completely untouched.
func TestLoadOrCreateSecret_SymlinkToWrongLengthFileIsNotWrittenThrough(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir() // simulates "anywhere else on disk"

	victimContent := []byte("this file has nothing to do with proofrun and must not be touched")
	targetPath := filepath.Join(outside, "some-unrelated-file.txt")
	if err := os.WriteFile(targetPath, victimContent, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, SecretPath(dir)); err != nil {
		t.Skipf("cannot create symlinks in this environment: %v", err)
	}

	if _, err := LoadOrCreateSecret(dir); err != nil {
		t.Fatalf("LoadOrCreateSecret should recover from a planted symlink, not error: %v", err)
	}

	afterward, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("the file the symlink pointed at is gone: %v", err)
	}
	if !bytes.Equal(afterward, victimContent) {
		t.Fatalf("LoadOrCreateSecret wrote through the symlink and clobbered an unrelated file\ngot:  %q\nwant: %q", afterward, victimContent)
	}

	info, err := os.Lstat(SecretPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("the symlink at .proofrun/secret was never replaced with a regular file")
	}
}

// TestLoadOrCreateSecret_RejectsGitTrackedSecret covers a gap flagged
// during review as worth closing before signing goes live: a repository
// can ship a plausible-looking, correctly-sized "secret" file that's
// already committed. Nothing about its content, length, or file type looks
// wrong — the only thing wrong with it is that anyone who cloned the repo
// already knows what it says, which defeats the entire point of a
// machine-local signing key. LoadOrCreateSecret must refuse to trust it and
// generate a real local one instead, even though every other check
// (regular file, right length, not a symlink) passes.
func TestLoadOrCreateSecret_RejectsGitTrackedSecret(t *testing.T) {
	dir := newTestRepo(t)

	committedKey := bytes.Repeat([]byte("B"), secretKeyLen)
	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(SecretPath(dir), committedKey, 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "-f", filepath.Join(DirName, SecretFileName))
	run("commit", "-m", "adversarial: pre-committed secret")

	key, err := LoadOrCreateSecret(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSecret should recover from a tracked secret, not error: %v", err)
	}
	if bytes.Equal(key, committedKey) {
		t.Fatal("LoadOrCreateSecret trusted a git-tracked secret — anyone who cloned this repo knows this key too")
	}
}

func TestLoadOrCreateSecret_RestrictivePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows doesn't enforce POSIX permission bits the same way")
	}

	dir := t.TempDir()
	if _, err := LoadOrCreateSecret(dir); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(SecretPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("secret file permissions = %o, want 0600", perm)
	}
}
