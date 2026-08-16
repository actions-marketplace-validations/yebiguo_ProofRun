package receipt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yebiguo/proofrun/internal/git"
)

// validateDirIsReal refuses outright if DirName already exists but isn't a
// genuine, ordinary directory of this repository — a symlink, or a nested
// repository boundary (a submodule, or just an ad-hoc `git init` someone
// left inside it).
//
// This must run before anything — read or write — ever looks inside
// DirName, not just before writing. A checked-out, untrusted repository can
// track ".proofrun -> somewhere/else" directly: os.MkdirAll and os.Stat
// both resolve symlinks for intermediate path components (only the *final*
// component of a path is what Lstat vs Stat actually differ on), so
// anything built only on Lstat-ing the final file (receipt.json, secret)
// stays blind to an attacker-controlled parent directory. Concretely: if
// DirName is a symlink to a directory the attacker also controls,
// LoadOrCreateSecret's own Lstat-the-final-component check sees an
// ordinary regular file at the resolved location and happily reads it —
// silently adopting a key the attacker already knows, which lets them
// forge signatures that verify perfectly. Checking DirName itself with
// Lstat closes that: Lstat never follows the symlink at the path's own
// final component, so a symlinked DirName correctly reports as "not a
// directory" here instead of being resolved through.
//
// The nested-.git check exists for the same reason via a different
// mechanism: git.IsTracked queries the *outer* repository's index by exact
// path string. A submodule (or any nested repository) at DirName makes its
// own contents invisible to that query — a file genuinely tracked inside
// the nested repo reports as untracked from the outer repo's perspective,
// defeating the same "reject a git-tracked secret" defense the symlink
// check exists alongside.
func validateDirIsReal(dir string) error {
	proofDir := filepath.Join(dir, DirName)

	info, err := os.Lstat(proofDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s exists but is not a real directory (found a symlink or other special file) — refusing to read or write through it", proofDir)
	}
	if _, err := os.Lstat(filepath.Join(proofDir, ".git")); err == nil {
		return fmt.Errorf("%s contains its own .git — refusing to read or write through a nested repository boundary", proofDir)
	}
	return nil
}

// ensureDir creates DirName inside dir (if needed, after validateDirIsReal
// passes) and makes a best-effort attempt to keep it out of the project's
// git history — see git.EnsureIgnored. Shared by Save and
// LoadOrCreateSecret, which both need DirName to exist before writing into
// it.
func ensureDir(dir string) error {
	if err := validateDirIsReal(dir); err != nil {
		return err
	}
	proofDir := filepath.Join(dir, DirName)
	if err := os.MkdirAll(proofDir, 0o755); err != nil {
		return err
	}
	git.EnsureIgnored(dir, DirName+"/")
	return nil
}

// writeFileAtomic writes data to a temp file in the same directory as path,
// then renames it into place, instead of writing directly to path.
//
// This matters specifically because DirName's contents can come from a
// checked-out, untrusted repository (that's the whole reason ProofRun's
// GitHub Action clears .proofrun/ before re-running rather than trusting
// whatever's there). os.WriteFile opens path with O_TRUNC and follows
// symlinks — if something had pre-planted path as a symlink pointing
// outside the project (e.g. at an arbitrary file the user happens to have
// write access to), writing "through" it would truncate and overwrite
// whatever it points at. os.Rename, by contrast, replaces the directory
// entry at path — symlink or not — without ever following or writing
// through it, so this is safe against that regardless of what (if
// anything) currently sits at path.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
