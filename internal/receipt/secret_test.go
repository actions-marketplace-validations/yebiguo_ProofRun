package receipt

import (
	"bytes"
	"os"
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
