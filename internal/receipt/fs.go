package receipt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yebiguo/proofrun/internal/git"
)

// ensureDir creates DirName inside dir (if needed) and makes a best-effort
// attempt to keep it out of the project's git history — see
// git.EnsureIgnored. Shared by Save and LoadOrCreateSecret, which both need
// DirName to exist before writing into it.
//
// Refuses outright if DirName already exists but isn't a real directory —
// in particular, a symlink. DirName's own existing content can come from a
// checked-out, untrusted repository, and os.MkdirAll happily follows a
// symlinked path component: a tracked ".proofrun -> /somewhere/else"
// symlink would make every subsequent write in this package (receipt.json,
// and the signing key once LoadOrCreateSecret calls this too) land inside
// whatever that symlink points at instead of the project. This is checked
// with Lstat, not Stat — Stat follows the symlink and would report it as a
// perfectly ordinary directory, which is exactly what writeFileAtomic's own
// symlink defense (see its doc comment) does NOT protect against: that
// defense only covers the final path component, not an attacker-controlled
// parent directory.
func ensureDir(dir string) error {
	proofDir := filepath.Join(dir, DirName)

	if info, err := os.Lstat(proofDir); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s exists but is not a real directory (found a symlink or other special file) — refusing to write through it", proofDir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

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
