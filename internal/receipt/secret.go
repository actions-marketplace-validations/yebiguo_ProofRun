package receipt

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yebiguo/proofrun/internal/git"
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
// leaves the local .proofrun/ directory, and ensureDir makes a best-effort
// attempt to keep .proofrun/ out of the project's git history even if the
// project's own .gitignore never mentions it — this key existing at all is
// only useful if it isn't the kind of thing a plain `git add .` picks up.
//
// This is tamper-evident, not tamper-proof: it exists to make casually
// hand-edited receipt.json content detectable, not to withstand an attacker
// who already has read access to this file. Anyone who can read
// receipt.json can read this key too, and forge a signature that matches
// it; that is an inherent limit of any local-only integrity scheme, not a
// bug in this one. See package doc for what this is actually meant to
// catch.
//
// A missing or malformed key file (wrong length — truncated, corrupted, or
// hand-edited) is treated the same way: silently regenerated. This is a
// deliberate choice, not an oversight — hard-failing every proofrun command
// because a local cache file got corrupted would be a worse outcome than
// the one real consequence of regenerating: any receipts signed with the
// old key stop verifying and read as NOT RUN, which is exactly the correct,
// conservative response to "we can no longer prove this result is genuine".
//
// The path this reads from and writes to can come from a checked-out,
// untrusted repository (DirName's contents aren't something ProofRun's own
// history gets to assume are trustworthy — that's the same reason the
// GitHub Action clears .proofrun/ before re-running rather than trusting
// it). Two things follow from that, deliberately: (1) the existing-file
// check uses Lstat, not Stat, and only accepts a regular file — a symlink
// at this exact path (planted by a hostile repo, pointing anywhere on
// disk) is treated as absent rather than followed and read, since a
// same-length symlink target would otherwise get silently trusted as this
// machine's signing key; (2) writing uses writeFileAtomic (create a temp
// file, then os.Rename over the target) rather than os.WriteFile, since
// WriteFile opens with O_TRUNC and follows symlinks — writing "through" a
// planted symlink would truncate and overwrite whatever it points at,
// anywhere the current user has write access to. os.Rename replaces the
// directory entry itself, symlink or not, without ever writing through it;
// (3) an existing key file that IS a valid regular file of the right
// length is still rejected if it's tracked by git (git.IsTracked) — a
// repository can ship a plausible-looking, correctly-sized "secret"
// pre-committed, and anyone who cloned it already knows its content just
// as well as the local machine does. Trusting it would let that same
// person forge signatures that verify perfectly, defeating the entire
// point of this mechanism while looking, from the outside, like it's
// working. This isn't hypothetical only for Day 2+: it's checked here, at
// the one place every consumer of the key already goes through.
//
// (4) validateDirIsReal runs first, before even attempting to read an
// existing key — not only in the create-a-new-key path below. A symlinked
// DirName defeats (1) and (3) both: os.Lstat on the *final* path component
// (secret) resolves straight through a symlinked *parent* directory and
// reports whatever regular file sits at the far end as perfectly ordinary,
// and git.IsTracked's exact-path-string query never notices that file is
// actually tracked under its real, different path. Validating DirName
// itself first closes that regardless of what either downstream check
// individually would have missed.
func LoadOrCreateSecret(dir string) ([]byte, error) {
	if err := validateDirIsReal(dir); err != nil {
		return nil, err
	}

	path := SecretPath(dir)

	if info, err := os.Lstat(path); err == nil {
		if info.Mode().IsRegular() && !git.IsTracked(dir, DirName+"/"+SecretFileName) {
			if data, err := os.ReadFile(path); err == nil && len(data) == secretKeyLen {
				return data, nil
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key := make([]byte, secretKeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating local signing key: %w", err)
	}
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}
