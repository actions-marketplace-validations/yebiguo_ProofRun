package receipt

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SecretFileName is the local signing key's filename inside DirName.
const SecretFileName = "secret"

// secretKeyLen is 32 bytes (256 bits) — the standard key size for HMAC-SHA256.
const secretKeyLen = 32

// SecretPath returns the full path to the local signing key inside project
// root dir.
func SecretPath(dir string) string {
	return filepath.Join(dir, DirName, SecretFileName)
}

// LoadOrCreateSecret returns this machine's local receipt-signing key,
// generating and persisting a new random one on first use. The key never
// leaves the local .proofrun/ directory and is never committed to git (see
// .gitignore) — it exists to make casually hand-edited receipt.json content
// detectable, not to withstand an attacker who already has read access to
// this file. Anyone who can read receipt.json can read this key too, and
// forge a signature that matches it; that is an inherent limit of any
// local-only integrity scheme, not a bug in this one. See package doc for
// what this is actually meant to catch.
//
// A missing or malformed key file (wrong length — truncated, corrupted, or
// hand-edited) is treated the same way: silently regenerated. This is a
// deliberate choice, not an oversight — hard-failing every proofrun command
// because a local cache file got corrupted would be a worse outcome than
// the one real consequence of regenerating: any receipts signed with the
// old key stop verifying and read as NOT RUN, which is exactly the correct,
// conservative response to "we can no longer prove this result is genuine".
func LoadOrCreateSecret(dir string) ([]byte, error) {
	path := SecretPath(dir)

	if data, err := os.ReadFile(path); err == nil && len(data) == secretKeyLen {
		return data, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key := make([]byte, secretKeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating local signing key: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}
